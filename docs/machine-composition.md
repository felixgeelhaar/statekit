# Machine Composition (InvokeMachine)

> **Tier 2 — experimental.** `InvokeMachine` / `WithChildMachine` may change in a future v1.x minor. Pin a specific minor for production use. See [API Stability Tiers](./stability.md). Overlaps with the actor `Spawn` API; prefer this builder when typed child composition is enough.

Machine composition allows you to invoke child state machines within a state, with automatic lifecycle management.

## Overview

When a state invokes a child machine:
1. Child machine starts automatically on state entry
2. Child runs independently, processing its own events
3. When child reaches a final state, parent can transition via `OnDone`
4. Child is automatically stopped when parent exits the invoking state

## Basic Usage

```go
import (
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/internal/ir"
)

// Define child machine
childMachine, _ := statekit.NewMachine[struct{}]("worker").
    WithInitial("working").
    State("working").
        On("COMPLETE").Target("done").End().
    Done().
    State("done").Final().
    Done().
    Build()

// Define parent machine with child invocation
parent, _ := statekit.NewMachine[struct{}]("parent").
    WithInitial("idle").
    // Register child machine factory
    WithChildMachine("worker", func(ctx struct{}, send func(statekit.Event) error) ir.ChildInterpreter {
        interp := statekit.NewInterpreter(childMachine)
        return interp
    }).
    State("idle").
        On("START").Target("processing").End().
    Done().
    State("processing").
        // Invoke child when entering this state
        InvokeMachine("worker").
            ID("w1").
            OnDone("completed").
        End().
    Done().
    State("completed").Final().
    Done().
    Build()

interp := statekit.NewInterpreter(parent)
interp.Start()

// Transition to processing - child starts automatically
interp.Send(statekit.Event{Type: "START"})
// Parent is in "processing", child is in "working"

// When child reaches final state, parent transitions to "completed"
```

## Builder API

### WithChildMachine

Register a child machine factory on the machine builder:

```go
WithChildMachine(ref string, factory ChildMachineFactory[C])
```

The factory function receives:
- `parentCtx C` - The parent machine's context
- `parentSend func(Event) error` - Function to send events to parent

### InvokeMachine

Invoke a registered child machine within a state:

```go
State("processing").
    InvokeMachine("worker").  // Reference to registered child
        ID("myWorker").       // Optional: custom invoke ID
        OnDone("success").    // Optional: transition when child completes
    End().
Done()
```

## Factory Pattern

The factory pattern allows flexible child machine creation:

```go
// Child with its own context type
type WorkerContext struct {
    TaskID string
}

childMachine, _ := statekit.NewMachine[WorkerContext]("worker").
    // ... machine definition
    Build()

// Parent with different context
type ParentContext struct {
    OrderID string
    Items   []string
}

parent, _ := statekit.NewMachine[ParentContext]("parent").
    WithChildMachine("worker", func(ctx ParentContext, send func(statekit.Event) error) ir.ChildInterpreter {
        // Create child with derived context
        childInterp := statekit.NewInterpreter(childMachine)
        childInterp.UpdateContext(func(c *WorkerContext) {
            c.TaskID = ctx.OrderID + "-task"
        })
        return childInterp
    }).
    // ...
    Build()
```

## Multiple Invocations

A state can invoke multiple child machines:

```go
State("processing").
    InvokeMachine("validator").ID("v1").End().
    InvokeMachine("processor").ID("p1").End().
    InvokeMachine("notifier").ID("n1").OnDone("completed").End().
Done()
```

All children start when entering the state and stop when exiting.

## OnDone Transitions

When a child machine reaches a final state, the parent can automatically transition:

```go
State("processing").
    InvokeMachine("worker").
        OnDone("success").  // Parent transitions to "success" when child completes
    End().
Done()
```

Without `OnDone`, the parent remains in the current state when the child completes.

## Lifecycle Details

### State Entry
1. Parent enters invoking state
2. Entry actions execute
3. All child machines are started (via factory)
4. Child interpreters begin in their initial states

### During Invocation
- Child machines process events independently
- Parent can continue processing its own events
- Parent can transition to other states (which stops children)

### Child Completion
1. Child reaches a final state
2. If `OnDone` is configured, parent receives internal done event
3. Parent transitions to the OnDone target state
4. Child is stopped and cleaned up

### State Exit
1. Parent exits invoking state (via event or OnDone)
2. All active children are stopped
3. Exit actions execute
4. Parent enters new state

## Type-Erased Children

The `ChildInterpreter` interface allows children with different context types:

```go
type ChildInterpreter interface {
    Start()
    Stop()
    Done() bool
    OnDone(callback func())
}
```

This means parent and child can have completely different context types.

## Example: Order Processing

```go
type OrderContext struct {
    OrderID string
    Items   []Item
    Total   float64
}

type PaymentContext struct {
    Amount    float64
    Method    string
    Confirmed bool
}

// Payment machine
paymentMachine, _ := statekit.NewMachine[PaymentContext]("payment").
    WithInitial("pending").
    State("pending").
        On("CONFIRM").Target("confirmed").End().
        On("DECLINE").Target("declined").End().
    Done().
    State("confirmed").Final().Done().
    State("declined").Final().Done().
    Build()

// Order machine with payment invocation
orderMachine, _ := statekit.NewMachine[OrderContext]("order").
    WithInitial("cart").
    WithChildMachine("payment", func(ctx OrderContext, send func(statekit.Event) error) ir.ChildInterpreter {
        interp := statekit.NewInterpreter(paymentMachine)
        interp.UpdateContext(func(c *PaymentContext) {
            c.Amount = ctx.Total
        })
        return interp
    }).
    State("cart").
        On("CHECKOUT").Target("payment").End().
    Done().
    State("payment").
        InvokeMachine("payment").
            ID("pay").
            OnDone("shipped").
        End().
        On("CANCEL").Target("cancelled").End().
    Done().
    State("shipped").Final().Done().
    State("cancelled").Final().Done().
    Build()
```

## Best Practices

1. **Use meaningful IDs**: The invoke ID helps with debugging and logging
2. **Handle cancellation**: Parent transitions should gracefully handle child interruption
3. **Keep factories simple**: Complex initialization should be in child machine setup
4. **Consider state scope**: Children are tied to their invoking state's lifecycle
5. **Test independently**: Test child machines in isolation before composition
