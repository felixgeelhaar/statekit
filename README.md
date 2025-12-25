# Statekit

[![Go Reference](https://pkg.go.dev/badge/github.com/felixgeelhaar/statekit.svg)](https://pkg.go.dev/github.com/felixgeelhaar/statekit)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/statekit)](https://goreportcard.com/report/github.com/felixgeelhaar/statekit)
[![CI](https://github.com/felixgeelhaar/statekit/actions/workflows/ci.yml/badge.svg)](https://github.com/felixgeelhaar/statekit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Go-native statechart execution engine with XState JSON compatibility for visualization.**

Define and execute statecharts in Go — visualize them with XState tooling.

## Features

- **Fluent Builder API** — Type-safe machine construction with Go generics
- **Hierarchical States** — Compound/nested states with event bubbling
- **Guards & Actions** — Conditional transitions and side effects
- **XState Export** — Visualize with [Stately.ai](https://stately.ai/viz) and XState Inspector
- **Build-time Validation** — Catch configuration errors before runtime
- **Zero Dependencies** — Pure Go, no external dependencies

## Installation

```bash
go get github.com/felixgeelhaar/statekit
```

Requires Go 1.23 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type Context struct {
    Count int
}

func main() {
    machine, _ := statekit.NewMachine[Context]("counter").
        WithInitial("idle").
        WithContext(Context{Count: 0}).
        WithAction("increment", func(ctx *Context, e statekit.Event) {
            ctx.Count++
        }).
        State("idle").
            On("START").Target("running").
        Done().
        State("running").
            OnEntry("increment").
            On("STOP").Target("idle").
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    fmt.Println(interp.State().Value) // "idle"

    interp.Send(statekit.Event{Type: "START"})
    fmt.Println(interp.State().Value)   // "running"
    fmt.Println(interp.State().Context.Count) // 1
}
```

## Hierarchical States

Statekit supports nested (compound) states with proper entry/exit ordering and event bubbling:

```go
machine, _ := statekit.NewMachine[struct{}]("traffic").
    WithInitial("active").
    State("active").
        WithInitial("green").
        On("POWER_OFF").Target("off").End().  // Handled by parent
        State("green").
            On("TIMER").Target("yellow").
        End().
        End().
        State("yellow").
            On("TIMER").Target("red").
        End().
        End().
        State("red").
            On("TIMER").Target("green").
        End().
    End().
    Done().
    State("off").Final().
    Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()

fmt.Println(interp.State().Value)    // "green"
fmt.Println(interp.Matches("active")) // true (matches ancestor)

interp.Send(statekit.Event{Type: "POWER_OFF"}) // Bubbles to parent
fmt.Println(interp.State().Value) // "off"
```

### Hierarchical State Semantics

- **Event Bubbling**: Unhandled events bubble up to ancestor states
- **Child Priority**: Child state transitions take precedence over parent
- **Entry Order**: Ancestors enter before descendants (root → leaf)
- **Exit Order**: Descendants exit before ancestors (leaf → root)

## Guards

Conditional transitions using guard functions:

```go
machine, _ := statekit.NewMachine[Context]("guarded").
    WithInitial("idle").
    WithGuard("hasItems", func(ctx Context, e statekit.Event) bool {
        return ctx.Count > 0
    }).
    State("idle").
        On("CHECKOUT").Target("processing").Guard("hasItems").
    Done().
    State("processing").
    Done().
    Build()
```

## XState Visualization

Export your machine to XState JSON format for visualization:

```go
import "github.com/felixgeelhaar/statekit/export"

exporter := export.NewXStateExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(jsonStr)
```

Use the exported JSON with:
- [Stately.ai Visualizer](https://stately.ai/viz)
- [XState Inspector](https://stately.ai/docs/inspector)

## Examples

See the [examples](./examples) directory:

- **[traffic_light](./examples/traffic_light)** — Simple FSM with cyclic transitions
- **[pedestrian_light](./examples/pedestrian_light)** — Hierarchical states with event bubbling

## API Reference

### Machine Builder

```go
statekit.NewMachine[C](id string) *MachineBuilder[C]
    .WithInitial(id StateID)
    .WithContext(ctx C)
    .WithAction(id ActionType, fn Action[C])
    .WithGuard(id GuardType, fn Guard[C])
    .State(id StateID) *StateBuilder[C]
    .Build() (*ir.MachineConfig[C], error)
```

### State Builder

```go
StateBuilder[C]
    .WithInitial(id StateID)      // For compound states
    .OnEntry(action ActionType)
    .OnExit(action ActionType)
    .On(event EventType) *TransitionBuilder[C]
    .State(id StateID)            // Nested state
    .Final()                      // Mark as final state
    .Done()                       // Return to machine builder
    .End()                        // Return to parent state builder
```

### Transition Builder

```go
TransitionBuilder[C]
    .Target(id StateID)
    .Guard(id GuardType)
    .Do(action ActionType)
    .End()                        // Return to state builder
```

### Interpreter

```go
statekit.NewInterpreter[C](machine) *Interpreter[C]
    .Start()                      // Enter initial state
    .Send(event Event)            // Process event
    .State() State[C]             // Current state
    .Matches(id StateID) bool     // Check state or ancestor
    .Done() bool                  // In final state?
    .UpdateContext(fn func(*C))   // Modify context
```

## Design Philosophy

- **Go-first Execution** — Explicit, deterministic, testable
- **Statecharts over FSMs** — Hierarchy enables complex behavior
- **Visualization as a Feature** — XState compatibility for free tooling
- **Small Surface Area** — Fewer features, better guarantees

## Scope

Explicitly **out of scope** for v1:
- Parallel/orthogonal states
- History states
- Delayed/timed transitions
- Invoked actors/services
- Persistence/durability

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Felix Geelhaar
