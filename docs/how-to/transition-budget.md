# Transition budgets (halt after N attempts)

Runaway retries are a common production hazard. Statekit does not hard-cap
transitions for you — instead, put the budget in **context + a guard**, and
route the exhausted path to a terminal failure state.

This is the pattern used by the [Stripe webhook saga](../tutorials/stripe-webhook-saga.md)
and answers the same need as qmuntal/stateless [#77](https://github.com/qmuntal/stateless/issues/77)
(“halt after N transitions”).

## Pattern

```go
type Ctx struct {
    Attempts    int
    MaxAttempts int
}

machine, _ := statekit.NewMachine[Ctx]("job").
    WithContext(Ctx{MaxAttempts: 3}).
    WithInitial("working").
    WithAction("bump", func(ctx *Ctx, e statekit.Event) {
        ctx.Attempts++
    }).
    WithGuard("retriable", func(ctx Ctx, e statekit.Event) bool {
        return ctx.Attempts < ctx.MaxAttempts
    }).
    State("working").
    On("FAIL").Target("retrying").Do("bump").End().
    On("OK").Target("done").End().
    Done().
    State("retrying").
    On("RETRY").Target("working").Guard("retriable").End().
    On("RETRY").Target("failed").End(). // Unguarded fallback when budget exhausted
    Done().
    State("done").Final().Done().
    State("failed").Final().Done().
    Build()
```

Key points:

1. **Count in context** — `Attempts` is durable across `Snapshot` / restore.
2. **Guarded retry + unguarded fallback** — when `retriable` fails, the second
   `RETRY` transition to `failed` fires (ordered evaluation).
3. **Lint** — `non-determinism` allows multiple transitions on the same event
   when at least one is guarded; keep the guarded arm first.

## See also

- [`examples/stripe_webhook`](../../examples/stripe_webhook) — retry budget in a webhook saga
- [Guards and actions](./guards-actions.md)
- [Lint rules](./lint.md)
