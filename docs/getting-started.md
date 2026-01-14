# Getting Started with Statekit

Statekit is a Go-native statechart execution engine with XState JSON compatibility for visualization.

## Installation

```bash
go get github.com/felixgeelhaar/statekit
```

## Your First State Machine

Here's a simple traffic light machine:

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type Context struct{}

func main() {
    // Build the machine using the fluent API
    machine, err := statekit.NewMachine[Context]("traffic_light").
        WithInitial("green").
        State("green").
            On("TIMER").Target("yellow").
        Done().
        State("yellow").
            On("TIMER").Target("red").
        Done().
        State("red").
            On("TIMER").Target("green").
        Done().
        Build()

    if err != nil {
        panic(err)
    }

    // Create an interpreter to run the machine
    interp := statekit.NewInterpreter(machine)

    // Start the machine
    interp.Start()
    fmt.Println("Current state:", interp.State().Value) // "green"

    // Send events to transition
    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println("Current state:", interp.State().Value) // "yellow"

    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println("Current state:", interp.State().Value) // "red"

    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println("Current state:", interp.State().Value) // "green"
}
```

## Key Concepts

### 1. Machine Definition

Machines are defined using the fluent builder API:

```go
machine, err := statekit.NewMachine[MyContext]("machine_id").
    WithInitial("initial_state").
    State("state1").
        On("EVENT").Target("state2").
    Done().
    State("state2").
        // ... transitions ...
    Done().
    Build()
```

### 2. Interpreter

The interpreter executes the machine:

```go
interp := statekit.NewInterpreter(machine)
interp.Start()                           // Enter initial state
interp.Send(statekit.Event{Type: "X"})   // Send events
interp.State()                           // Get current state
interp.Matches("state_id")               // Check if in a state
interp.Done()                            // Check if in final state
```

### 3. Context

Context is type-safe state data:

```go
type OrderContext struct {
    OrderID string
    Total   float64
}

machine, _ := statekit.NewMachine[OrderContext]("order").
    WithContext(OrderContext{OrderID: "123", Total: 99.99}).
    // ...
    Build()

// Access context
ctx := interp.State().Context
fmt.Println(ctx.OrderID) // "123"
```

## Alternative: Reflection DSL

For a more declarative approach, use struct tags:

```go
type MyMachine struct {
    statekit.MachineDef `id:"traffic_light" initial:"green"`
    Green  statekit.StateNode `on:"TIMER->yellow"`
    Yellow statekit.StateNode `on:"TIMER->red"`
    Red    statekit.StateNode `on:"TIMER->green"`
}

registry := statekit.NewActionRegistry[Context]()
machine, _ := statekit.FromStruct[MyMachine, Context](registry)
```

See [Reflection DSL Guide](reflection-dsl.md) for more details.

## Visualization

Visualize your machine using the `statekit viz` command:

```bash
statekit viz --go-package . --format html -o machine.html
```

Or export to JSON programmatically:

```go
exporter := export.NewNativeExporter(machine)
```

See [Visualization Guide](visualization.md) for more details.

## Next Steps

- [Guards and Actions](guards-actions.md) - Add behavior to your machines
- [Hierarchical States](hierarchical-states.md) - Nested states and event bubbling
- [Patterns & Recipes](patterns-recipes.md) - Common patterns and best practices
- [Performance Tuning](performance-tuning.md) - Optimization techniques
- [Reflection DSL](reflection-dsl.md) - Declarative machine definition
- [Visualization](visualization.md) - Interactive graphs and diagrams
- [XState Migration](xstate-migration.md) - Coming from XState/TypeScript?
- [API Reference](api-reference.md) - Complete API documentation
