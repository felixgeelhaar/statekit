# Guards and Actions

Guards and actions add behavior to your state machines. Guards control when transitions can occur, and actions perform side effects during transitions.

## Actions

Actions are functions that execute side effects. They can:
- Modify context
- Log events
- Call external services
- Update UI

### Defining Actions

```go
type OrderContext struct {
    Total   float64
    Items   int
    Shipped bool
}

machine, _ := statekit.NewMachine[OrderContext]("order").
    WithInitial("pending").
    // Register named actions
    WithAction("addItem", func(ctx *OrderContext, e statekit.Event) {
        ctx.Items++
        if price, ok := e.Payload.(float64); ok {
            ctx.Total += price
        }
    }).
    WithAction("markShipped", func(ctx *OrderContext, e statekit.Event) {
        ctx.Shipped = true
    }).
    WithAction("logState", func(ctx *OrderContext, e statekit.Event) {
        fmt.Printf("Event: %s, Items: %d, Total: $%.2f\n",
            e.Type, ctx.Items, ctx.Total)
    }).
    // ... states ...
    Build()
```

### Action Types

#### 1. Entry Actions

Execute when entering a state:

```go
State("processing").
    OnEntry("logState").
    OnEntry("sendNotification"). // Multiple entry actions
    // ...
Done()
```

#### 2. Exit Actions

Execute when leaving a state:

```go
State("editing").
    OnExit("saveChanges").
    OnExit("clearCache").
    // ...
Done()
```

#### 3. Transition Actions

Execute during a transition (after exit, before entry):

```go
State("cart").
    On("CHECKOUT").Target("payment").Do("validateCart").Do("calculateTax").
Done()
```

### Action Execution Order

For a transition from state A to state B:

1. Exit actions of A (and ancestors up to LCA)
2. Transition actions
3. Entry actions of B (and ancestors from LCA)

```go
// Transition from "idle" to "active"
// Order: exitIdle → transitionAction → enterActive

State("idle").
    OnExit("exitIdle").
    On("START").Target("active").Do("transitionAction").
Done().
State("active").
    OnEntry("enterActive").
Done()
```

## Guards

Guards are predicates that determine if a transition should occur. They return `true` to allow the transition, `false` to block it.

### Defining Guards

```go
machine, _ := statekit.NewMachine[OrderContext]("order").
    WithInitial("cart").
    WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
        return ctx.Items > 0
    }).
    WithGuard("hasMinimumOrder", func(ctx OrderContext, e statekit.Event) bool {
        return ctx.Total >= 10.00
    }).
    State("cart").
        // Transition only if cart has items
        On("CHECKOUT").Target("payment").Guard("hasItems").
    Done().
    // ...
    Build()
```

### Guard Semantics

- Guards receive context **by value** (immutable)
- Guards should be **pure functions** (no side effects)
- If a guard returns `false`, the transition is **blocked**
- The machine stays in its current state

```go
interp.Start() // In "cart" state

// With empty cart (Items = 0)
interp.Send(statekit.Event{Type: "CHECKOUT"})
fmt.Println(interp.State().Value) // Still "cart" - guard blocked it

// Add item first
interp.Send(statekit.Event{Type: "ADD_ITEM"})
interp.Send(statekit.Event{Type: "CHECKOUT"})
fmt.Println(interp.State().Value) // "payment" - guard passed
```

### Guards with Actions

Combine guards and actions on the same transition:

```go
State("cart").
    On("CHECKOUT").
        Target("payment").
        Guard("hasItems").
        Guard("hasMinimumOrder").  // Multiple guards (all must pass)
        Do("calculateTax").
        Do("reserveInventory").
Done()
```

## Context Updates

### In Actions

Actions receive a pointer to context for modification:

```go
WithAction("increment", func(ctx *OrderContext, e statekit.Event) {
    ctx.Items++  // Modifies context
})
```

### UpdateContext Method

For runtime updates outside of actions:

```go
interp.UpdateContext(func(ctx *OrderContext) {
    ctx.Total = 0  // Reset total
    ctx.Items = 0  // Reset items
})
```

## Event Payloads

Events can carry data via the Payload field:

```go
type ItemPayload struct {
    Name  string
    Price float64
}

// In action
WithAction("addItem", func(ctx *OrderContext, e statekit.Event) {
    if item, ok := e.Payload.(ItemPayload); ok {
        ctx.Items++
        ctx.Total += item.Price
    }
})

// Sending event with payload
interp.Send(statekit.Event{
    Type:    "ADD_ITEM",
    Payload: ItemPayload{Name: "Widget", Price: 9.99},
})
```

## Reflection DSL

With the reflection DSL, reference actions and guards by name:

```go
type CartState struct {
    statekit.StateNode `on:"CHECKOUT->payment:hasItems" entry:"logCart"`
}

type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"cart"`
    Cart CartState
    Payment statekit.StateNode
}

registry := statekit.NewActionRegistry[OrderContext]().
    WithAction("logCart", func(ctx *OrderContext, e statekit.Event) {
        fmt.Printf("Cart: %d items, $%.2f\n", ctx.Items, ctx.Total)
    }).
    WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
        return ctx.Items > 0
    })

machine, _ := statekit.FromStruct[OrderMachine, OrderContext](registry)
```

### Tag Syntax

```
on:"EVENT->target:guard"           # With guard
on:"EVENT->target/action"          # With action
on:"EVENT->target/action:guard"    # With both
on:"EVENT->target/a1;a2:guard"     # Multiple actions
entry:"action1,action2"            # Entry actions
exit:"action1,action2"             # Exit actions
```

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type CartContext struct {
    Items []string
    Total float64
}

func main() {
    machine, _ := statekit.NewMachine[CartContext]("shopping").
        WithInitial("browsing").
        WithAction("addToCart", func(ctx *CartContext, e statekit.Event) {
            if item, ok := e.Payload.(string); ok {
                ctx.Items = append(ctx.Items, item)
                ctx.Total += 10.00 // Simplified pricing
            }
        }).
        WithAction("checkout", func(ctx *CartContext, e statekit.Event) {
            fmt.Printf("Checking out %d items for $%.2f\n",
                len(ctx.Items), ctx.Total)
        }).
        WithGuard("cartNotEmpty", func(ctx CartContext, e statekit.Event) bool {
            return len(ctx.Items) > 0
        }).
        State("browsing").
            On("ADD_ITEM").Target("browsing").Do("addToCart").
            On("CHECKOUT").Target("checkout").Guard("cartNotEmpty").
        Done().
        State("checkout").
            OnEntry("checkout").
            Final().
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Try to checkout with empty cart
    interp.Send(statekit.Event{Type: "CHECKOUT"})
    fmt.Println("After empty checkout:", interp.State().Value) // "browsing"

    // Add items
    interp.Send(statekit.Event{Type: "ADD_ITEM", Payload: "Widget"})
    interp.Send(statekit.Event{Type: "ADD_ITEM", Payload: "Gadget"})

    // Checkout with items
    interp.Send(statekit.Event{Type: "CHECKOUT"})
    fmt.Println("After checkout:", interp.State().Value) // "checkout"
    fmt.Println("Done:", interp.Done()) // true
}
```

## Best Practices

1. **Keep actions small and focused** - One action, one responsibility.

2. **Guards should be pure** - No side effects, deterministic output.

3. **Name descriptively** - `canSubmit`, `hasPermission`, `validateOrder`.

4. **Prefer guards over in-action conditionals** - Let the state machine control flow.

5. **Handle errors in actions gracefully** - Actions can't fail transitions after guard passes.

## See Also

- [Getting Started](getting-started.md)
- [Hierarchical States](hierarchical-states.md)
- [Reflection DSL](reflection-dsl.md)
