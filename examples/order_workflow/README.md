# Order Workflow Example

An e-commerce order processing workflow demonstrating the Reflection DSL.

## Features Demonstrated

- Reflection DSL for machine definition
- Guards for conditional transitions
- Entry and transition actions
- Context for order state management
- XState JSON export for visualization

## State Diagram

```
                         SUBMIT (hasItems guard)
┌─────────┐──────────────────────────────────────────┐
│ pending │                                          │
└────┬────┘                                          ▼
     │ CANCEL                                  ┌────────────┐
     │                        VALID            │ validating │
     │                      ┌──────────────────┴────┬───────┘
     │                      ▼                       │ INVALID
     │                ┌─────────┐                   │
     │                │ payment │◄──┐               │
     │                └────┬────┘   │               │
     │                     │ PAID   │ RETRY         │
     │                     ▼        │               │
     │              ┌─────────────┐ │               │
     │              │ fulfillment │ │               │
     │              └──────┬──────┘ │               │
     │                     │        │               │
     │         ┌───────────┼────────┘               │
     │         │ SHIPPED   │ OUT_OF_STOCK           │
     │         ▼           ▼                        │
     │   ┌───────────┐  ┌───────────┐               │
     │   │ completed │  │ refunding │               │
     │   └───────────┘  └─────┬─────┘               │
     │                        │ REFUNDED            │
     │                        ▼                     │
     │   ┌───────────┐  ┌──────────┐                │
     └──►│ cancelled │  │ refunded │◄───────────────┘
         └───────────┘  └──────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/order_workflow/
```

### Programmatic Usage

```go
package main

import (
    "fmt"
    "go.klarlabs.de/statekit"
)

// Define states using struct tags
type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"pending"`

    Pending   statekit.StateNode `on:"SUBMIT->validating:hasItems"`
    Validating statekit.StateNode `on:"VALID->payment"`
    Payment   statekit.StateNode `on:"PAID->fulfillment"`
    Fulfillment statekit.StateNode `on:"SHIPPED->completed"`
    Completed statekit.FinalNode
}

func main() {
    registry := statekit.NewActionRegistry[OrderContext]().
        WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
            return len(ctx.Items) > 0
        })

    machine, _ := statekit.FromStructWithContext[OrderMachine, OrderContext](
        registry,
        OrderContext{Items: []Item{{Name: "Widget"}}},
    )

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    interp.Send(statekit.Event{Type: "SUBMIT"})
    interp.Send(statekit.Event{Type: "VALID"})
    interp.Send(statekit.Event{Type: "PAID"})
    interp.Send(statekit.Event{Type: "SHIPPED"})

    fmt.Println(interp.Done()) // true
}
```

## Run Tests

```bash
go test -v ./examples/order_workflow/...
```

## Key Concepts

### Reflection DSL Syntax

The tag syntax follows the pattern: `on:"EVENT->target/action:guard,EVENT2->target2"`

```go
type MyMachine struct {
    statekit.MachineDef `id:"name" initial:"state1"`

    State1 statekit.StateNode `on:"GO->state2:canGo" entry:"logEntry"`
    State2 statekit.StateNode `on:"BACK->state1/doBack"`
    Done   statekit.FinalNode
}
```

- `EVENT->target` - Basic transition
- `EVENT->target:guard` - Guarded transition
- `EVENT->target/action` - Transition with action
- `entry:"action"` - Entry action

### ActionRegistry

Actions and guards are registered separately:

```go
registry := statekit.NewActionRegistry[Context]().
    WithAction("name", func(ctx *Context, e Event) { ... }).
    WithGuard("check", func(ctx Context, e Event) bool { ... })
```

### XState Export

Visualize your machine at [stately.ai/viz](https://stately.ai/viz):

```go
exporter := export.NewXStateExporter(machine)
json, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(json)
```
