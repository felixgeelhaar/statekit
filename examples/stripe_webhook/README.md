# Stripe webhook saga

Inbound webhook → idempotency → fulfilment → retry budget, modeled as a statechart with Invoke services and OnError routing.

```bash
go run .
go test .
```

Full walkthrough: [docs/tutorials/stripe-webhook-saga.md](../../docs/tutorials/stripe-webhook-saga.md).
