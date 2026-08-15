# Tutorial: Stripe webhook saga with outbox

Webhook handlers are the canonical “stateful microservice” job. Most teams write them as `switch event.Type { ... }` and accumulate bugs around partial failure, retries, and idempotency.

This tutorial walks the companion example at [`examples/stripe_webhook`](../../examples/stripe_webhook): inbound webhook → idempotency check → fulfilment → notify, with `Invoke` services, `OnError` routing, and a retry budget. The state machine is the outbox driver; the outbox (your DB in production) drives external effects.

## What you will build

A saga that:

1. Accepts a `WEBHOOK_RECEIVED` event with a Stripe-style payload
2. Checks idempotency (duplicate event IDs short-circuit to success)
3. Fulfils the order via an invoked service
4. Retries up to three times on fulfilment failure, then fails

Run it:

```bash
cd examples/stripe_webhook
go run .
go test .
```

## Domain context

```go
type PaymentIntentEvent struct {
    ID        string  // Stripe's event id — the idempotency key
    Customer  string
    AmountUSD float64
    OrderID   string
}

type SagaContext struct {
    Event        PaymentIntentEvent
    Idempotent   bool
    FulfilmentID string
    LastError    string
    Attempts     int
}
```

Production: replace the in-memory stores with a transactional idempotency table and a fulfilment write in the same DB transaction as the outbox row.

## The machine shape

```
received
   │ WEBHOOK_RECEIVED
   ▼
checking_idempotency ──DUPLICATE──► succeeded (final)
   │ FRESH
   ▼
fulfilling ──FULFILLED──► succeeded (final)
   │ OnError
   ▼
retry_decision ──RETRY (attempts < 3)──► fulfilling
               ──RETRY (else)──────────► failed (final)
```

Guards and services, not booleans sprinkled through a handler, decide the path.

## Build the saga

### Actions and guards

```go
WithAction("recordEvent", func(c *SagaContext, e statekit.Event) {
    if p, ok := e.Payload.(PaymentIntentEvent); ok {
        c.Event = p
    }
}).
WithAction("incAttempt", func(c *SagaContext, _ statekit.Event) {
    c.Attempts++
}).
WithGuard("retriable", func(c SagaContext, _ statekit.Event) bool {
    return c.Attempts < 3
})
```

### Idempotency as an Invoke service

Services start on state entry and cancel on exit. Success and failure are explicit transitions — no “did the goroutine finish?” bookkeeping in the handler.

```go
WithService("checkIdempotency", func(svc statekit.ServiceContext[SagaContext]) error {
    if idem.SeenAndMark(svc.MachineContext.Event.ID) {
        svc.Send(statekit.Event{Type: "DUPLICATE"})
        return nil
    }
    svc.Send(statekit.Event{Type: "FRESH"})
    return nil
}).
State("checking_idempotency").
    Invoke("checkIdempotency").ID("idem").OnError("failed").End().
    On("DUPLICATE").Target("succeeded").
    On("FRESH").Target("fulfilling").
    Done()
```

`SeenAndMark` must be atomic in production (`INSERT ... ON CONFLICT` or equivalent). That atomicity is what gives idempotency its teeth.

### Fulfilment with OnError → retry

```go
State("fulfilling").
    OnEntry("incAttempt").
    Invoke("fulfilOrder").ID("ful").OnError("retry_decision").End().
    On("FULFILLED").Target("succeeded").Do("recordFulfilment").
    Done().
State("retry_decision").
    On(EvtRetry).Target("fulfilling").Guard("retriable").
    On(EvtRetry).Target("failed"). // unguarded fallback when budget exhausted
    Done()
```

Two transitions on the same event: the guarded one wins when it can; otherwise the unguarded fallback fires. Lint’s `non-determinism` rule stays quiet because one branch is guarded.

Always give Invoke an `OnError` (lint rule `invoke-missing-onerror`). Silent service failures are the production hazard this tutorial exists to avoid.

## Driving the interpreter

```go
interp := statekit.NewInterpreter(machine)
defer interp.Close()
interp.Start()

interp.Send(statekit.Event{
    Type:    "WEBHOOK_RECEIVED",
    Payload: PaymentIntentEvent{ID: "evt_1", OrderID: "ord_1", AmountUSD: 49},
})
```

In the demo, a short sleep yields the scheduler so the Invoke goroutine can complete. In production, prefer:

- Transitioning on events the service itself sends (`FULFILLED`, `DUPLICATE`), or
- Persisting with `PersistentInterpreter` and committing after the outbox write (Tier 2 — pin a minor if you use it)

HTTP tip: return `200` once the saga has written to the outbox, not when fulfilment finishes. Stripe retries on non-2xx; your idempotency key absorbs the replay.

## What to verify in tests

The example’s tests cover three paths:

| Case | Expectation |
|------|-------------|
| Happy path | Ends in `succeeded` with a fulfilment ID |
| Replay same event ID | Ends in `succeeded` with empty fulfilment ID (short-circuit) |
| Bad payload / exhausted retries | Ends in `failed` after three attempts |

Use `statetest` assertions when you grow the suite; start here with direct `State()` checks.

## Visualize it

```bash
# From the repo root, after exporting Native JSON from the machine
statekit viz machine.json --format mermaid -o saga.md
```

Or parse the Go package:

```bash
statekit viz --go-package ./examples/stripe_webhook --format ascii
```

## Next steps

- [Guards & Actions](../how-to/guards-actions.md) — deepen the guard/action model
- [Testing](../how-to/testing.md) — `statetest` helpers and recorders
- [Static Analysis (Lint)](../how-to/lint.md) — catch missing OnError and dead ends at build time
- [API Stability Tiers](../reference/stability.md) — what is safe to pin for persistence / actors
- Full source: [`examples/stripe_webhook`](../../examples/stripe_webhook)
