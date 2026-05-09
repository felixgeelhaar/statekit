package main

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit"
)

func waitForSagaSettled(t *testing.T, interp *statekit.Interpreter[SagaContext]) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := string(interp.State().Value)
		if interp.Done() || s == "retry_decision" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("saga did not settle within 2s; state=%s", interp.State().Value)
}

func TestSaga_HappyPath(t *testing.T) {
	t.Parallel()
	idem := newIdempotencyStore()
	ful := newFulfilmentStore()
	machine := buildSaga(idem, ful)

	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(statekit.Event{Type: EvtReceived, Payload: PaymentIntentEvent{
		ID: "evt_happy", OrderID: "ord_happy", AmountUSD: 10,
	}})
	waitForSagaSettled(t, interp)

	if got := string(interp.State().Value); got != "succeeded" {
		t.Errorf("state = %q, want succeeded", got)
	}
	if interp.State().Context.FulfilmentID == "" {
		t.Error("expected fulfilment ID to be set")
	}
}

func TestSaga_IdempotencyShortCircuit(t *testing.T) {
	t.Parallel()
	idem := newIdempotencyStore()
	ful := newFulfilmentStore()
	machine := buildSaga(idem, ful)

	// First processing.
	{
		interp := statekit.NewInterpreter(machine)
		defer func() { _ = interp.Close() }()
		interp.Start()
		interp.Send(statekit.Event{Type: EvtReceived, Payload: PaymentIntentEvent{
			ID: "evt_dup", OrderID: "ord_dup", AmountUSD: 10,
		}})
		waitForSagaSettled(t, interp)
	}

	// Replay — should short-circuit to succeeded without fulfilling.
	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()
	interp.Send(statekit.Event{Type: EvtReceived, Payload: PaymentIntentEvent{
		ID: "evt_dup", OrderID: "ord_dup", AmountUSD: 10,
	}})
	waitForSagaSettled(t, interp)

	if got := string(interp.State().Value); got != "succeeded" {
		t.Errorf("replay: state = %q, want succeeded", got)
	}
	if id := interp.State().Context.FulfilmentID; id != "" {
		t.Errorf("replay: FulfilmentID = %q, want empty (idempotent)", id)
	}
}

func TestSaga_RetriesExhaust(t *testing.T) {
	t.Parallel()
	idem := newIdempotencyStore()
	ful := newFulfilmentStore()
	machine := buildSaga(idem, ful)

	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	// OrderID is empty → fulfilment errors.
	interp.Send(statekit.Event{Type: EvtReceived, Payload: PaymentIntentEvent{
		ID: "evt_bad", AmountUSD: 10,
	}})

	for !interp.Done() {
		waitForSagaSettled(t, interp)
		if interp.Done() {
			break
		}
		if string(interp.State().Value) == "retry_decision" {
			interp.Send(statekit.Event{Type: EvtRetry})
		}
	}

	if got := string(interp.State().Value); got != "failed" {
		t.Errorf("state = %q, want failed", got)
	}
	if got := interp.State().Context.Attempts; got < 1 {
		t.Errorf("Attempts = %d, want >= 1", got)
	}
}
