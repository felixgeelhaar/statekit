# Reflection DSL

The reflection DSL provides a declarative way to define state machines using Go struct tags. This approach offers a more compact, readable syntax for machine definitions.

## Basic Example

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type Context struct{}

// Define machine using struct tags
type TrafficLight struct {
    statekit.MachineDef `id:"traffic_light" initial:"green"`
    Green  statekit.StateNode `on:"TIMER->yellow"`
    Yellow statekit.StateNode `on:"TIMER->red"`
    Red    statekit.StateNode `on:"TIMER->green"`
}

func main() {
    registry := statekit.NewActionRegistry[Context]()
    machine, err := statekit.FromStruct[TrafficLight, Context](registry)
    if err != nil {
        panic(err)
    }

    interp := statekit.NewInterpreter(machine)
    interp.Start()
    fmt.Println(interp.State().Value) // "green"
}
```

## Marker Types

### MachineDef

Embedded in the machine struct to define machine-level configuration:

```go
type MyMachine struct {
    statekit.MachineDef `id:"machine_id" initial:"first_state"`
    // states...
}
```

Tags:
- `id:"..."` - Required. Machine identifier.
- `initial:"..."` - Required. Initial state name (snake_case).

### StateNode

Defines an atomic (simple) state:

```go
Idle statekit.StateNode `on:"START->running" entry:"logIdle" exit:"cleanup"`
```

Tags:
- `on:"..."` - Transition definitions
- `entry:"..."` - Entry actions (comma-separated)
- `exit:"..."` - Exit actions (comma-separated)

### CompoundNode

Defines a compound (parent) state with children:

```go
type ActiveState struct {
    statekit.CompoundNode `initial:"idle" on:"RESET->done"`
    Idle    statekit.StateNode `on:"START->working"`
    Working statekit.StateNode `on:"STOP->idle"`
}
```

Tags:
- `initial:"..."` - Required. Initial child state.
- `on:"..."` - Parent-level transitions
- `entry:"..."` - Parent entry actions
- `exit:"..."` - Parent exit actions

### FinalNode

Defines a final (terminal) state:

```go
Done statekit.FinalNode
```

Final states typically have no transitions.

## Tag Syntax

### Transitions

Basic format: `on:"EVENT->target"`

```go
`on:"SUBMIT->processing"`
```

With guard: `on:"EVENT->target:guardName"`

```go
`on:"SUBMIT->processing:hasItems"`
```

With action: `on:"EVENT->target/actionName"`

```go
`on:"SUBMIT->processing/validateForm"`
```

With multiple actions: `on:"EVENT->target/action1;action2"`

```go
`on:"SUBMIT->processing/validate;log"`
```

With action and guard: `on:"EVENT->target/action:guard"`

```go
`on:"SUBMIT->processing/validate:hasItems"`
```

### Multiple Transitions

Separate with commas:

```go
`on:"START->running,CANCEL->cancelled,SKIP->done"`
```

### Entry/Exit Actions

Comma-separated action names:

```go
`entry:"logEntry,startTimer"`
`exit:"cleanup,stopTimer"`
```

## ActionRegistry

Register action and guard implementations:

```go
type OrderContext struct {
    Items int
    Total float64
}

registry := statekit.NewActionRegistry[OrderContext]().
    WithAction("logOrder", func(ctx *OrderContext, e statekit.Event) {
        fmt.Printf("Order: %d items, $%.2f\n", ctx.Items, ctx.Total)
    }).
    WithAction("addItem", func(ctx *OrderContext, e statekit.Event) {
        ctx.Items++
        ctx.Total += 10.00
    }).
    WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
        return ctx.Items > 0
    }).
    WithGuard("canCheckout", func(ctx OrderContext, e statekit.Event) bool {
        return ctx.Total >= 25.00
    })
```

## Building the Machine

### FromStruct

```go
machine, err := statekit.FromStruct[MachineType, ContextType](registry)
```

### FromStructWithContext

Provide an initial context value:

```go
initialCtx := OrderContext{Items: 0, Total: 0.0}
machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](
    registry,
    initialCtx,
)
```

## Hierarchical States

Define nested states using embedded structs:

```go
// Child states as struct types
type IdleState struct {
    statekit.StateNode `on:"START->working"`
}

type WorkingState struct {
    statekit.StateNode `on:"PAUSE->paused,COMPLETE->done"`
}

type PausedState struct {
    statekit.StateNode `on:"RESUME->working"`
}

// Parent state embedding CompoundNode
type ActiveState struct {
    statekit.CompoundNode `initial:"idle" on:"CANCEL->cancelled"`
    Idle    IdleState
    Working WorkingState
    Paused  PausedState
}

// Machine definition
type WorkflowMachine struct {
    statekit.MachineDef `id:"workflow" initial:"active"`
    Active    ActiveState
    Done      statekit.FinalNode
    Cancelled statekit.FinalNode
}
```

## State Naming

Field names are converted to snake_case for state IDs. Acronyms are handled intelligently:

| Field Name | State ID |
|------------|----------|
| `Idle` | `idle` |
| `DontWalk` | `dont_walk` |
| `PaymentError` | `payment_error` |
| `HTTPServer` | `http_server` |
| `APIGateway` | `api_gateway` |
| `XMLParser` | `xml_parser` |

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type OrderContext struct {
    Items int
    Total float64
}

type CartState struct {
    statekit.StateNode `on:"ADD_ITEM->cart/addItem,CHECKOUT->payment:hasItems" entry:"logCart"`
}

type PaymentState struct {
    statekit.StateNode `on:"PAY->processing/processPayment:canPay"`
}

type ProcessingState struct {
    statekit.StateNode `on:"SUCCESS->completed,FAILURE->payment"`
}

type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"cart"`
    Cart       CartState
    Payment    PaymentState
    Processing ProcessingState
    Completed  statekit.FinalNode
}

func main() {
    registry := statekit.NewActionRegistry[OrderContext]().
        WithAction("addItem", func(ctx *OrderContext, e statekit.Event) {
            ctx.Items++
            ctx.Total += 10.00
        }).
        WithAction("logCart", func(ctx *OrderContext, e statekit.Event) {
            fmt.Printf("Cart: %d items\n", ctx.Items)
        }).
        WithAction("processPayment", func(ctx *OrderContext, e statekit.Event) {
            fmt.Printf("Processing payment: $%.2f\n", ctx.Total)
        }).
        WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
            return ctx.Items > 0
        }).
        WithGuard("canPay", func(ctx OrderContext, e statekit.Event) bool {
            return ctx.Total > 0
        })

    machine, err := statekit.FromStruct[OrderMachine, OrderContext](registry)
    if err != nil {
        panic(err)
    }

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    interp.Send(statekit.Event{Type: "ADD_ITEM"})
    interp.Send(statekit.Event{Type: "ADD_ITEM"})
    interp.Send(statekit.Event{Type: "CHECKOUT"})
    interp.Send(statekit.Event{Type: "PAY"})
    interp.Send(statekit.Event{Type: "SUCCESS"})

    fmt.Println("Final state:", interp.State().Value)
    fmt.Println("Done:", interp.Done())
}
```

## Validation

Machines are validated at build time:

```go
type InvalidMachine struct {
    statekit.MachineDef `id:"invalid" initial:"nonexistent"`
    Idle statekit.StateNode
}

_, err := statekit.FromStruct[InvalidMachine, Context](registry)
// Error: initial state 'nonexistent' does not exist
```

Common validation errors:
- Missing required `id` or `initial` tags
- Initial state doesn't exist
- Transition target doesn't exist
- Referenced action not in registry
- Referenced guard not in registry
- Compound state missing initial child

## Fluent vs Reflection

| Fluent Builder | Reflection DSL |
|---------------|----------------|
| More verbose | More compact |
| IDE autocomplete | Tag syntax |
| Inline actions | Named actions |
| Build-time type safety | Struct-level type safety |
| Good for dynamic machines | Good for static definitions |

Both approaches produce identical `MachineConfig` and can be used interchangeably.

## Best Practices

1. **Use descriptive field names** - They become state IDs.

2. **Keep registries organized** - Group related actions and guards.

3. **Validate early** - Check errors from `FromStruct` at startup.

4. **Use type aliases for reuse** - Define state types once, reuse in multiple machines.

5. **Document tag syntax** - Add comments explaining complex transitions.

## See Also

- [Getting Started](getting-started.md)
- [Guards and Actions](guards-actions.md)
- [Hierarchical States](hierarchical-states.md)
