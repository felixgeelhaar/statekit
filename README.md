# Statekit

[![Go Reference](https://pkg.go.dev/badge/github.com/felixgeelhaar/statekit.svg)](https://pkg.go.dev/github.com/felixgeelhaar/statekit)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/statekit)](https://goreportcard.com/report/github.com/felixgeelhaar/statekit)
[![CI](https://github.com/felixgeelhaar/statekit/actions/workflows/ci.yml/badge.svg)](https://github.com/felixgeelhaar/statekit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Stop modeling order, payment, and incident lifecycles with switch statements and ad-hoc FSMs.** Statekit is a typed statechart library for Go: hierarchical states, guards, actions, delayed and parallel transitions, with built-in visualization and lint.

```go
machine, _ := statekit.NewMachine[Order]("checkout").
    WithInitial("cart").
    State("cart").On("CHECKOUT").Target("processing").Done().
    State("processing").On("PAID").Target("shipped").Done().
    State("shipped").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
defer interp.Close()
interp.Start()
interp.Send(statekit.Event{Type: "CHECKOUT"})
```

A working machine in 10 lines. Hierarchy and parallel states scale from there. `statekit viz` renders any machine to ASCII, Mermaid, an interactive HTML simulator, or a TUI.

## Why

- **Type-safe over typed** — `[C any]` context, typed events, typed actions and guards. No `interface{}` casts at action time.
- **Statecharts, not just FSMs** — compound, parallel, history, and delayed transitions handle the workflows that flat FSM libraries can't model without manual bookkeeping.
- **Visualization as a feature** — every machine renders to multiple formats from one source of truth. Stately.ai-compatible JSON for round-trip editing.
- **Static analysis** — `lint.Lint(machine)` catches unreachable states, dead ends, non-determinism, missing OnError on Invoke, and more — at build time.
- **Determinism for tests** — inject a `FakeClock` to make timer-driven behavior reproducible. No `time.Sleep` flakes.

## Core features

- Hierarchical states with event bubbling
- History states (shallow and deep)
- Delayed transitions and parallel/orthogonal regions
- Guards, actions, entry/exit hooks
- Reflection DSL — define machines with struct tags
- Build-time validation
- Visualization: ASCII, Mermaid, interactive HTML, TUI
- Static analysis (`lint`)
- Snapshot / Restore
- Plugin system with lifecycle hooks
- Testing utilities (`statetest`)
- HTTP integration, OpenTelemetry tracing, Prometheus metrics, Kubernetes health probes
- Code generation from Native JSON

## Advanced (Tier 2 — see [stability tiers](./docs/stability.md))

These ship in v1.0 but reserve room to iterate within v1.x:

- **Actor model** — `Spawn`, supervision strategies (Escalate / Recover / Restart / Stop)
- **Persistent interpreter** — event-sourced state, snapshot-on-final, configurable strategies
- **Distributed execution** — `StreamLock` interface (Redis / etcd / PostgreSQL) + `DistributedInterpreter`
- **Machine composition** — `InvokeMachine` for typed child-machine composition
- **MCP integration** — `mcp.NewServer` for AI-assisted authoring; `mcp.ExposeInterpreter` to drive a running machine from an agent
- **AI plugins** — `aiplugin.TokenCounter`, `aiplugin.PromptRecorder` for LLM observability and replay-based debugging

## Installation

```bash
go get github.com/felixgeelhaar/statekit
```

Requires Go 1.24 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine, _ := statekit.NewMachine[struct{}]("traffic").
        WithInitial("green").
        State("green").On("TIMER").Target("yellow").Done().
        State("yellow").On("TIMER").Target("red").Done().
        State("red").On("TIMER").Target("green").Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    fmt.Println(interp.State().Value) // "green"
    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "yellow"
}
```

## Hierarchical States

Nested states with event bubbling and proper entry/exit ordering:

```go
machine, _ := statekit.NewMachine[struct{}]("editor").
    WithInitial("editing").
    State("editing").
        WithInitial("idle").
        On("SAVE").Target("saved").End().  // Parent handles SAVE
        State("idle").On("TYPE").Target("dirty").End().End().
        State("dirty").On("CLEAR").Target("idle").End().End().
    Done().
    State("saved").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()

fmt.Println(interp.State().Value)     // "idle"
fmt.Println(interp.Matches("editing")) // true
interp.Send(statekit.Event{Type: "SAVE"}) // Bubbles to parent
fmt.Println(interp.State().Value)     // "saved"
```

## History States

Remember and restore previous states:

```go
machine, _ := statekit.NewMachine[struct{}]("player").
    WithInitial("playing").
    State("playing").
        WithInitial("track1").
        On("PAUSE").Target("paused").End().
        History("hist").Shallow().Default("track1").End().
        State("track1").On("NEXT").Target("track2").End().End().
        State("track2").On("NEXT").Target("track3").End().End().
        State("track3").End().
    Done().
    State("paused").
        On("PLAY").Target("hist").  // Resume last track
    Done().
    Build()
```

- **Shallow history** — Remembers immediate child state
- **Deep history** — Remembers exact leaf state

## Delayed Transitions

Timer-based automatic transitions:

```go
machine, _ := statekit.NewMachine[struct{}]("loading").
    WithInitial("loading").
    State("loading").
        After(5*time.Second).Target("timeout").
        On("LOADED").Target("ready").
    Done().
    State("timeout").Done().
    State("ready").Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
// Timer starts automatically, canceled if LOADED received
defer interp.Stop() // Always clean up timers
```

## Parallel States

Multiple regions active simultaneously:

```go
machine, _ := statekit.NewMachine[struct{}]("editor").
    WithInitial("active").
    State("active").Parallel().
        Region("bold").WithInitial("off").
            State("off").On("TOGGLE_BOLD").Target("on").EndState().
            State("on").On("TOGGLE_BOLD").Target("off").EndState().
        EndRegion().
        Region("italic").WithInitial("off").
            State("off").On("TOGGLE_ITALIC").Target("on").EndState().
            State("on").On("TOGGLE_ITALIC").Target("off").EndState().
        EndRegion().
    Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
// bold: on, italic: off (independent regions)
```

## Reflection DSL

Define machines using struct tags:

```go
type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"pending"`
    Pending   statekit.StateNode `on:"SUBMIT->processing:hasItems"`
    Processing statekit.StateNode `on:"COMPLETE->shipped"`
    Shipped   statekit.FinalNode
}

type OrderContext struct {
    Items []string
}

registry := statekit.NewActionRegistry[OrderContext]().
    WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
        return len(ctx.Items) > 0
    })

machine, _ := statekit.FromStruct[OrderMachine, OrderContext](registry)
```

## Guards & Actions

Conditional transitions and side effects:

```go
type Context struct{ Count int }

machine, _ := statekit.NewMachine[Context]("counter").
    WithInitial("idle").
    WithContext(Context{Count: 0}).
    WithAction("increment", func(ctx *Context, e statekit.Event) {
        ctx.Count++
    }).
    WithGuard("hasCount", func(ctx Context, e statekit.Event) bool {
        return ctx.Count > 0
    }).
    State("idle").
        OnEntry("increment").
        On("NEXT").Target("done").Guard("hasCount").
    Done().
    State("done").Final().Done().
    Build()
```

## Visualization

Statekit provides a native `viz` command to visualize state machines from Go source code or JSON.

Try the [Live Visualizer](https://felixgeelhaar.github.io/statekit/visualizer.html) to paste your JSON and interact with it.

```bash
# Interactive HTML simulation
statekit viz --go-package ./examples/order_workflow --format html -o machine.html

# Mermaid diagram
statekit viz --go-package ./examples/order_workflow --format mermaid
```

It supports multiple output formats:
- **HTML**: Interactive simulator with Cytoscape graph.
- **Mermaid**: Markdown-friendly state diagrams.
- **ASCII**: Terminal box diagrams.
- **TUI**: Interactive terminal UI.

To export JSON programmatically:

```go
import "github.com/felixgeelhaar/statekit/export"

exporter := export.NewNativeExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(jsonStr)
```

## MCP Server

Statekit includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) server for AI-assisted state machine development. Create, manage, and visualize machines directly from Claude Code or any MCP host.

```bash
# Add to your MCP configuration
go install github.com/felixgeelhaar/statekit/cmd/statekit-mcp@latest
```

```json
{
  "mcpServers": {
    "statekit": {
      "command": "statekit-mcp"
    }
  }
}
```

**Available tools:**

| Tool | Description |
|------|-------------|
| `create_machine` | Create a machine from a Native JSON definition |
| `list_machines` | List all running machine instances |
| `get_state` | Get current state, done status, and state path |
| `send_event` | Send an event to trigger a transition |
| `get_context` | Get the machine's context data |
| `visualize_machine` | Get visualization data with interactive MCP App |
| `validate_machine` | Validate a definition using lint rules |
| `export_machine` | Export as JSON, Mermaid, or ASCII |

The `visualize_machine` tool includes an interactive Vue.js + Cytoscape.js visualizer that MCP Apps hosts render inline — with dark mode, transition animations, and a full state history log. All JS dependencies are bundled inline for CSP-compatible rendering in any MCP host.

## Additional Packages

| Package | Description |
|---------|-------------|
| [`mcp`](./mcp) | MCP server for AI-assisted state machine management |
| [`statetest`](./statetest) | Testing utilities: assertions, recorders, helpers |
| [`debug`](./debug) | Runtime inspection and state graph analysis |
| [`metrics`](./metrics) | Prometheus metrics for monitoring |
| [`health`](./health) | Kubernetes liveness/readiness probes |
| [`lint`](./lint) | Static analysis for detecting structural issues |
| [`export`](./export) | XState JSON exporter |
| [`generate`](./generate) | Go code generation from XState JSON |
| [`http`](./http) | HTTP handlers and middleware |
| [`otel`](./otel) | OpenTelemetry tracing |

## Examples

See the [examples](./examples) directory:

| Example | Description |
|---------|-------------|
| [traffic_light](./examples/traffic_light) | Simple FSM with cyclic transitions |
| [pedestrian_light](./examples/pedestrian_light) | Hierarchical states with event bubbling |
| [order_workflow](./examples/order_workflow) | Reflection DSL for business workflows |
| [incident_lifecycle](./examples/incident_lifecycle) | Complex IT incident management |
| [llm_agent](./examples/llm_agent) | Deterministic RAG pipeline with HITL approval |

## API Reference

See the full [API documentation on pkg.go.dev](https://pkg.go.dev/github.com/felixgeelhaar/statekit).

### Core Types

```go
// Machine construction
statekit.NewMachine[C](id string) *MachineBuilder[C]
statekit.FromStruct[M, C](registry) (*MachineConfig[C], error)

// Runtime
statekit.NewInterpreter[C](machine) *Interpreter[C]
    .Start()                      // Enter initial state
    .Send(event Event)            // Process event
    .Stop()                       // Cancel timers, cleanup
    .State() State[C]             // Current state
    .Matches(id StateID) bool     // Check state or ancestor
    .Done() bool                  // In final state?
```

## Design Philosophy

- **Go-first Execution** — Explicit, deterministic, testable
- **Statecharts over FSMs** — Hierarchy enables complex behavior
- **Visualization as a Feature** — XState compatibility for free tooling
- **Small Surface Area** — Fewer features, better guarantees

## Documentation

- [Getting Started](./docs/getting-started.md)
- [Hierarchical States](./docs/hierarchical-states.md)
- [Guards & Actions](./docs/guards-actions.md)
- [XState Migration](./docs/xstate-migration.md)
- [Migration from looplab/fsm](./docs/migration-from-looplab-fsm.md)
- [Migration from qmuntal/stateless](./docs/migration-from-qmuntal-stateless.md)
- [API Stability Tiers](./docs/stability.md)
- [Reflection DSL](./docs/reflection-dsl.md)
- [Testing](./docs/testing.md)
- [Observability](./docs/observability.md)
- [Static Analysis (Lint)](./docs/lint.md)
- [API Reference](./docs/api-reference.md)

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Felix Geelhaar
