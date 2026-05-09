// Package main models a Stripe webhook saga as a statechart with the
// outbox pattern.
//
// Inbound webhook → idempotency check → fulfilment → notify customer.
// Each side effect is an Invoke service; failures route to OnError.
//
// Why this example exists:
//
//   - Webhook handlers are the canonical "stateful microservice" job.
//     Most teams write them as `switch event.Type { ... }` and accumulate
//     bugs around partial failure, retries, and idempotency.
//   - Statekit gives you: explicit states (no implicit "is it done?"
//     boolean), transition guards (idempotency check), Invoke services
//     with cancellation (no goroutine leaks), and OnError routing
//     (bounded blast radius).
//   - The outbox pattern is the standard way to bridge the database
//     transaction boundary with external side effects. The state
//     machine drives the outbox; the outbox drives external effects.
//
// In production:
//   - Replace `idempotencyStore`/`fulfilmentStore` with your DB.
//   - Wire PersistentInterpreter so the saga survives process restarts.
//   - Wrap the HTTP handler so the response returns 200 once the saga
//     has written to the outbox (not when it has finished running).
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/felixgeelhaar/statekit"
)

// -- Domain types ------------------------------------------------------------

// PaymentIntentEvent is the inbound webhook payload (subset).
type PaymentIntentEvent struct {
	ID        string // Stripe's idempotency key — distinct per event.
	Customer  string
	AmountUSD float64
	OrderID   string
}

// SagaContext holds the saga's working state.
type SagaContext struct {
	Event        PaymentIntentEvent
	Idempotent   bool   // True if we've already processed this event.
	FulfilmentID string // Set on successful fulfilment.
	LastError    string // Captured on OnError transitions.
	Attempts     int    // Retry counter.
}

// -- Side-effect "stores" (production: replace with DB) ----------------------

type idempotencyStore struct {
	mu  sync.Mutex
	set map[string]struct{}
}

func newIdempotencyStore() *idempotencyStore {
	return &idempotencyStore{set: make(map[string]struct{})}
}

// SeenAndMark returns whether the event was previously processed and
// atomically records it. The atomicity is what gives idempotency its
// teeth — production code must use a transactional INSERT ... ON
// CONFLICT or equivalent.
func (s *idempotencyStore) SeenAndMark(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.set[eventID]; ok {
		return true
	}
	s.set[eventID] = struct{}{}
	return false
}

type fulfilmentStore struct {
	mu  sync.Mutex
	out map[string]string // orderID → fulfilmentID
}

func newFulfilmentStore() *fulfilmentStore {
	return &fulfilmentStore{out: make(map[string]string)}
}

func (f *fulfilmentStore) Fulfil(orderID, fulfilmentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if orderID == "" {
		return errors.New("missing order id")
	}
	f.out[orderID] = fulfilmentID
	return nil
}

// -- Event types -------------------------------------------------------------

const (
	EvtReceived statekit.EventType = "WEBHOOK_RECEIVED"
	EvtRetry    statekit.EventType = "RETRY"
)

// -- Saga state machine ------------------------------------------------------

func buildSaga(idem *idempotencyStore, ful *fulfilmentStore) *statekit.MachineConfig[SagaContext] {
	machine, err := statekit.NewMachine[SagaContext]("stripe-payment-saga").
		WithInitial("received").
		// Actions update the saga context as it progresses.
		WithAction("recordEvent", func(c *SagaContext, e statekit.Event) {
			if p, ok := e.Payload.(PaymentIntentEvent); ok {
				c.Event = p
			}
		}).
		WithAction("incAttempt", func(c *SagaContext, _ statekit.Event) {
			c.Attempts++
		}).
		WithAction("recordError", func(c *SagaContext, e statekit.Event) {
			if p, ok := e.Payload.(map[string]any); ok {
				if msg, ok := p["error"].(string); ok {
					c.LastError = msg
				}
			}
		}).
		// Guards inspect either context or event payload.
		WithGuard("notSeen", func(c SagaContext, _ statekit.Event) bool {
			return !c.Idempotent
		}).
		WithGuard("retriable", func(c SagaContext, _ statekit.Event) bool {
			return c.Attempts < 3
		}).
		// Services are invoked on state entry and cancelled on exit.
		WithService("checkIdempotency", func(svc statekit.ServiceContext[SagaContext]) error {
			if idem.SeenAndMark(svc.MachineContext.Event.ID) {
				// Already processed — emit the duplicate signal so the
				// saga can short-circuit to "succeeded".
				svc.Send(statekit.Event{Type: "DUPLICATE"})
				return nil
			}
			svc.Send(statekit.Event{Type: "FRESH"})
			return nil
		}).
		WithService("fulfilOrder", func(svc statekit.ServiceContext[SagaContext]) error {
			fulfilmentID := fmt.Sprintf("ful_%s_%d", svc.MachineContext.Event.OrderID, time.Now().UnixNano())
			if err := ful.Fulfil(svc.MachineContext.Event.OrderID, fulfilmentID); err != nil {
				return err // Routes to OnError.
			}
			svc.Send(statekit.Event{
				Type:    "FULFILLED",
				Payload: map[string]any{"fulfilment_id": fulfilmentID},
			})
			return nil
		}).
		WithAction("recordFulfilment", func(c *SagaContext, e statekit.Event) {
			if p, ok := e.Payload.(map[string]any); ok {
				if id, ok := p["fulfilment_id"].(string); ok {
					c.FulfilmentID = id
				}
			}
		}).
		// State graph.
		State("received").
		On(EvtReceived).Target("checking_idempotency").Do("recordEvent").
		Done().
		State("checking_idempotency").
		Invoke("checkIdempotency").
		ID("idem").
		OnError("failed").
		End().
		On("DUPLICATE").Target("succeeded"). // Already processed — replay-safe exit.
		On("FRESH").Target("fulfilling").
		Done().
		State("fulfilling").
		OnEntry("incAttempt").
		Invoke("fulfilOrder").
		ID("ful").
		OnError("retry_decision").
		End().
		On("FULFILLED").Target("succeeded").Do("recordFulfilment").
		Done().
		State("retry_decision").
		OnEntry("recordError").
		On(EvtRetry).Target("fulfilling").Guard("retriable").
		On(EvtRetry).Target("failed"). // Falls through when guard fails.
		Done().
		State("succeeded").Final().Done().
		State("failed").Final().Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}

// -- Demo --------------------------------------------------------------------

func main() {
	idem := newIdempotencyStore()
	ful := newFulfilmentStore()
	machine := buildSaga(idem, ful)

	run := func(label string, evt PaymentIntentEvent) {
		interp := statekit.NewInterpreter(machine)
		defer func() { _ = interp.Close() }()
		interp.Start()

		fmt.Printf("\n--- %s ---\n", label)
		interp.Send(statekit.Event{Type: EvtReceived, Payload: evt})

		// Drive any retries up to the budget. waitForServices yields the
		// scheduler so the goroutine-backed Invoke service can complete
		// before we inspect state. Production code uses a real signal:
		// pair Invoke with a transition that fires when the service
		// completes (OnDone), or wait on PersistentInterpreter.Commit.
		waitForServices()
		for !interp.Done() && string(interp.State().Value) == "retry_decision" {
			interp.Send(statekit.Event{Type: EvtRetry})
			waitForServices()
		}

		fmt.Printf("Final state:  %s\n", interp.State().Value)
		fmt.Printf("Done:         %v\n", interp.Done())
		fmt.Printf("FulfilmentID: %s\n", interp.State().Context.FulfilmentID)
		fmt.Printf("Attempts:     %d\n", interp.State().Context.Attempts)
		if e := interp.State().Context.LastError; e != "" {
			fmt.Printf("LastError:    %s\n", e)
		}
	}

	// First processing of evt_1 — fulfils.
	run("First webhook", PaymentIntentEvent{
		ID: "evt_1", Customer: "cus_alice", AmountUSD: 49.00, OrderID: "ord_1",
	})

	// Replay evt_1 — idempotency short-circuits.
	run("Replay (same event ID)", PaymentIntentEvent{
		ID: "evt_1", Customer: "cus_alice", AmountUSD: 49.00, OrderID: "ord_1",
	})

	// New event with empty OrderID — fulfil fails, retries exhaust.
	run("Bad event (no order id)", PaymentIntentEvent{
		ID: "evt_2", Customer: "cus_bob", AmountUSD: 25.00,
	})

	_ = context.Background // imported for production-typical signatures
}

// waitForServices yields long enough for goroutine-backed Invoke
// services to drive the saga to its next event. Demo-only — production
// code drives the saga via its own event loop or Commit point.
func waitForServices() {
	time.Sleep(50 * time.Millisecond)
}
