# Statekit

[![Go Reference](https://pkg.go.dev/badge/go.klarlabs.de/statekit.svg)](https://pkg.go.dev/go.klarlabs.de/statekit)
[![Go Report Card](https://goreportcard.com/badge/go.klarlabs.de/statekit)](https://goreportcard.com/report/go.klarlabs.de/statekit)
[![CI](https://github.com/klarlabs-studio/statekit/actions/workflows/ci.yml/badge.svg)](https://github.com/klarlabs-studio/statekit/actions/workflows/ci.yml)
[![Security: A](https://raw.githubusercontent.com/klarlabs-studio/statekit/main/.nox/security-badge.svg)](https://github.com/klarlabs-studio/statekit/security)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Stop modeling order, payment, and incident lifecycles with `switch` and ad-hoc FSMs.** Statekit is a typed statechart library for Go — hierarchical states, guards, actions, delayed and parallel transitions — with built-in visualization and lint. One library, one mental model, still v1.x stable on the core API.

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

A working machine in 10 lines. Hierarchy and parallel states scale from there. `statekit viz` renders any machine to ASCII, Mermaid, an interactive HTML simulator, or a TUI:

```text
┌─────────┐  CHECKOUT   ┌────────────┐  PAID   ┌─────────┐
│  cart   │ ──────────► │ processing │ ──────► │ shipped │
└─────────┘             └────────────┘         └─────────┘
```

## Two jobs

Statekit targets **backend domain workflows** — order lifecycles, payment sagas, incident management, KYC. The kind of state most teams scatter across `switch event.Type { ... }` and accumulate bugs around partial failure, retry, and idempotency. Start with the [Stripe webhook saga tutorial](./docs/tutorials/stripe-webhook-saga.md) or the [`examples/stripe_webhook`](./examples/stripe_webhook) package for a webhook-saga template with idempotency, retry budget, and the outbox pattern.

These workflows benefit from a consistent set of primitives — typed context, hierarchy, lint, visualization, snapshots — so one library, one mental model.

## Coming from another FSM library?

If you're using one of the incumbent Go FSM libraries and have hit a ceiling, this is the migration path:

- **Need persistence** that doesn't choke on unexported fields? Statekit's `Snapshot` round-trips through `encoding/json` and `encoding/gob` (see [snapshot serialization tests](./snapshot_serialization_test.go)).
- **Need recovery** after a service or context error without ending up in a stuck-transition limbo? Statekit's `OnError` routes cleanly; the interpreter accepts events again immediately (see [cancellation recovery tests](./cancellation_recovery_test.go)).
- **Need hierarchy** (compound, parallel, history)? First-class. Lint catches structural bugs the flat-FSM libraries can't even check for.
- **Need typed everything**? `[C any]` context threads through actions, guards, and services — no `interface{}` casts at handler time.
- **Need a Mermaid / ASCII / interactive HTML diagram**? `statekit viz` ships them.

Step-by-step migration guides:

- [From looplab/fsm](./docs/tutorials/migration-from-looplab-fsm.md)
- [From qmuntal/stateless](./docs/tutorials/migration-from-qmuntal-stateless.md)
- [From XState (JS)](./docs/tutorials/xstate-migration.md)

## Why

- **Type-safe over typed** — `[C any]` context, typed events, typed actions and guards. No `interface{}` casts at action time.
- **Statecharts, not just FSMs** — compound, parallel, history, and delayed transitions handle the workflows that flat FSM libraries can't model without manual bookkeeping.
- **Visualization as a feature** — every machine renders to multiple formats from one source of truth. XState v5 export for [Stately Studio](https://stately.ai/studio) round-trip editing.
- **Static analysis** — `lint.Lint(machine)` catches unreachable states, dead ends, non-determinism, missing OnError on Invoke, and more — at build time.
- **Determinism for tests** — inject a `FakeClock` to make timer-driven behavior reproducible. No `time.Sleep` flakes.

## Core features

- Hierarchical states with event bubbling
- History states (shallow and deep)
- Delayed transitions and parallel/orthogonal regions
- Eventless (`Always`) transitions, `Raise` internal events, and state `Tags`
- Wildcard (`*`) event handlers, internal transitions, and `Choose` conditional actions
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

## Advanced (Tier 2 — see [stability tiers](./docs/reference/stability.md))

**Experimental:** these ship in v1.x but may change in a future minor. Prefer core (Tier 1) primitives when they are enough; pin an exact minor if you depend on these in production.

- **Actor model** — `Spawn`, supervision strategies (Escalate / Recover / Restart / Stop)
- **Persistent interpreter** — event-sourced state, snapshot-on-final, configurable strategies
- **Distributed execution** — `StreamLock` interface (Redis / etcd / PostgreSQL) + `DistributedInterpreter`
- **Machine composition** — `InvokeMachine` for typed child-machine composition

## Installation

```bash
go get go.klarlabs.de/statekit
```

Requires Go 1.25 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "go.klarlabs.de/statekit"
)

func main() {
    machine, _ := statekit.NewMachine[struct{}]("traffic").
        WithInitial("green").
        State("green").On("TIMER").Target("yellow").Done().
        State("yellow").On("TIMER").Target("red").Done().
        State("red").On("TIMER").Target("green").Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    defer interp.Close()
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
        State("idle").On("TYPE").Target("dirty").Up().
        State("dirty").On("CLEAR").Target("idle").Up().
    Done().
    State("saved").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
defer interp.Close()
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
        State("track1").On("NEXT").Target("track2").Up().
        State("track2").On("NEXT").Target("track3").Up().
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
defer interp.Close() // Always clean up timers
interp.Start()
// Timer starts automatically, canceled if LOADED received
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
defer interp.Close()
interp.Start()
interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
// bold: on, italic: off (independent regions)
```

## Closing the Builder

Every state you open with `State()` must be closed again, and which terminator
closes it depends on where the state sits. Three destinations:

| Terminator | Returns to | Use for |
|------------|------------|---------|
| `Done()` | `MachineBuilder` | a top-level state |
| `End()` | the enclosing state | a child of a compound state |
| `EndState()` | the enclosing region | a state inside a parallel region |

Plus `EndRegion()`, which closes a region and returns to the parallel state
that owns it. `EndMachine()` is a deprecated second spelling of `Done()`.

The three shapes side by side — **flat**, every state closed with `Done()`:

```go
State("cart").On("CHECKOUT").Target("paid").Done().
State("paid").Final().Done().
```

**Nested** — prefer `Up()` after a transition on a child (≡ `End().End()`), then
`Done()` on the top-level parent. Use `EndTo("ancestor")` to unwind deep nesting:

```go
State("editing").
    WithInitial("idle").
    State("idle").On("TYPE").Target("dirty").Up().
    State("dirty").On("CLEAR").Target("idle").Up().
Done().
```

**Parallel** — region states closed with `EndState()`, the region with
`EndRegion()`, the parallel state with `Done()`:

```go
State("active").Parallel().
    Region("bold").WithInitial("off").
        State("off").On("TOGGLE_BOLD").Target("on").EndState().
        State("on").On("TOGGLE_BOLD").Target("off").EndState().
    EndRegion().
Done().
```

Building states in a loop hides the shape, which is where the sequence is
easiest to get wrong. A flat machine assembled from a transition table:

```go
builder := statekit.NewMachine[Ctx]("lifecycle").WithInitial(states[0])
for _, s := range states {
    sb := builder.State(s)
    if isFinal(s) {
        sb = sb.Final()
    }
    for _, tr := range transitionsFrom(s) {
        sb = sb.On(tr.Event).Target(tr.Target).End() // close the transition
    }
    builder = sb.Done()                              // close the state
}
machine, err := builder.Build()
```

Picking the wrong terminator often still compiles, since several of them return
chainable types. Two cases have no valid destination at all and now fail
immediately with a message naming the terminator to use instead, rather than
returning a nil builder that panics somewhere later: `End()` on a top-level
state, and `EndState()` on a state that is not inside a region.

## Eventless Transitions, Raise & Tags

**`Always`** transitions fire automatically on state entry (and after every
transition), choosing the first whose guard passes — ideal for conditional
routing without an explicit event. **`Raise`** enqueues an internal event that
is processed in the same step, before control returns and before any external
event. **`Tags`** categorize states for lightweight querying via `HasTag`.

```go
machine, _ := statekit.NewMachine[Ctx]("checkout").
    WithInitial("validating").
    WithGuard("ok", func(c Ctx, e statekit.Event) bool { return c.Valid }).
    State("validating").
        Tags("busy").
        Always().Target("approved").Guard("ok").End().
        Always().Target("rejected").End().            // guardless fallback
        Done().
    State("approved").
        On("SHIP").Target("shipping").Raise("NOTIFY").End().  // raise internal event
        Done().
    State("shipping").
        On("NOTIFY").Target("done").End().            // handled in the same step
        Done().
    State("rejected").Final().Done().
    State("done").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
// validating → approved|rejected resolved automatically via Always

fmt.Println(interp.HasTag("busy"))  // true while in a tagged active state
```

Key behaviors:
- `Always` transitions are evaluated in declaration order; the first enabled wins. A target is required (build-time validated) to prevent infinite loops.
- A macrostep settles all eventless transitions and drains raised events before `Start()`/`Send()` returns (bounded to guard against always-true cycles).
- `HasTag` matches the active leaf, its ancestors, and active parallel-region leaves.
- All three round-trip through the Native JSON and XState v5 exporters (`always`, `tags`, and `xstate.raise` action descriptors).

## Wildcard Events, Internal Transitions & Choose

**Wildcard `*`** catches any event not handled by a specific transition (exact
matches always win; it bubbles like any handler). **`Internal()`** runs a
transition's actions without exiting or re-entering the state — no entry/exit
hooks, no state change — in contrast to an external self-transition. **`Choose`**
is a conditional-action combinator: it runs the first branch whose guard passes.

```go
machine, _ := statekit.NewMachine[Ctx]("ops").
    WithInitial("running").
    WithAction("audit", statekit.Choose(
        statekit.ChooseBranch[Ctx]{When: isAdmin, Then: logAdmin},
        statekit.ChooseBranch[Ctx]{Then: logUser}, // else
    )).
    State("running").
        On("TICK").Internal().Do("audit").End().  // no exit/entry, no state change
        On("*").Target("unknown").End().           // catch-all fallback
        On("STOP").Target("stopped").End().         // exact match beats "*"
        Done().
    State("unknown").Final().Done().
    State("stopped").Final().Done().
    Build()
```

Key behaviors:
- Wildcard `*` is lowest priority within a state and honors guards; child handlers still take priority over ancestors.
- Internal transitions accept an empty target (or the owning state); build-time validated.
- `Choose` is a plain `Action[C]` — register it and reference it anywhere an action is used; a branch with a nil `When` is the else.
- Wildcard and internal transitions round-trip through both exporters (`on["*"]`, `internal: true`).

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

Try the [Live Visualizer](https://klarlabs-studio.github.io/statekit/play) to paste your JSON and interact with it. The [landing page](https://klarlabs-studio.github.io/statekit/) has the project overview.

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

To render a compiled machine programmatically, hand it to `viz.FromMachine` —
no JSON file and no source parsing in between. This is the route for a machine
assembled at runtime from a transition table, and it makes a diagram in your
documentation testable against the machine the runtime actually executes:

```go
import (
    "go.klarlabs.de/statekit/viz"
    "go.klarlabs.de/statekit/viz/mermaid"
)

diagram := mermaid.NewRenderer().Render(viz.FromMachine(machine))
```

To export JSON programmatically:

```go
import "go.klarlabs.de/statekit/export"

exporter := export.NewNativeExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(jsonStr)
```

## Additional Packages

| Package | Description |
|---------|-------------|
| [`statetest`](./statetest) | Testing utilities: assertions, recorders, helpers |
| [`debug`](./debug) | Runtime inspection and state graph analysis |
| [`metrics`](./metrics) | Prometheus metrics for monitoring |
| [`health`](./health) | Kubernetes liveness/readiness probes |
| [`lint`](./lint) | Static analysis for detecting structural issues |
| [`export`](./export) | Native + XState v5 JSON exporters |
| [`generate`](./generate) | Go code generation from Native JSON |
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

## API Reference

See the full [API documentation on pkg.go.dev](https://pkg.go.dev/go.klarlabs.de/statekit).

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
- **Statecharts over FSMs** — Hierarchy, parallel, history enable complex behavior without manual bookkeeping
- **Visualization as a Feature** — Multiple renderers + XState v5 export for Stately Studio round-trip
- **Determinism for tests** — Inject `FakeClock` to remove timer flakes
- **Stable core, experimental edge** — Tier-1 surface follows semver; Tier-2 features (actor, persistent, distributed, AI) reserve room to iterate

## Documentation

The [docs index](./docs/README.md) organizes everything by Diataxis category (tutorials, how-to, reference, explanation). Quick links:

- [Getting Started](./docs/tutorials/getting-started.md)
- [Stripe webhook saga](./docs/tutorials/stripe-webhook-saga.md) — flagship workflow tutorial
- [Choosing an API](./docs/how-to/choosing-an-api.md) — builder vs reflection DSL vs codegen
- [Hierarchical States](./docs/how-to/hierarchical-states.md)
- [Guards & Actions](./docs/how-to/guards-actions.md)
- [XState Migration](./docs/tutorials/xstate-migration.md)
- [Migration from looplab/fsm](./docs/tutorials/migration-from-looplab-fsm.md)
- [Migration from qmuntal/stateless](./docs/tutorials/migration-from-qmuntal-stateless.md)
- [API Stability Tiers](./docs/reference/stability.md)
- [Reflection DSL](./docs/how-to/reflection-dsl.md)
- [Testing](./docs/how-to/testing.md)
- [Observability](./docs/how-to/observability.md)
- [Static Analysis (Lint)](./docs/how-to/lint.md)
- [API Reference](./docs/reference/api-reference.md)

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Felix Geelhaar
