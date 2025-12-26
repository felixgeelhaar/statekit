# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Statekit is a Go-native statechart execution engine with XState JSON compatibility for visualization. It enables backend engineers to define, execute, and visualize statecharts using existing XState tooling (Stately Visualizer, XState Inspect).

**One-liner:** Define and execute statecharts in Go — visualize them with XState tooling.

## Build Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run single test
go test -run TestInterpreter_Send_BasicTransition ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...
```

## Project Structure

```
statekit/
├── types.go              # Public types (Event, State, StateID, etc.)
├── builder.go            # Fluent API (NewMachine, StateBuilder, TransitionBuilder, HistoryBuilder)
├── interpreter.go        # Runtime execution (Start, Send, State, Matches, Done)
├── reflect.go            # Reflection DSL (FromStruct, MachineDef, ActionRegistry)
├── hierarchy_test.go     # Comprehensive hierarchical state tests
├── history_test.go       # History state tests (shallow and deep)
├── delayed_test.go       # Delayed transition tests
├── internal/
│   ├── ir/
│   │   ├── types.go      # Core type definitions
│   │   ├── machine.go    # MachineConfig, StateConfig, TransitionConfig
│   │   └── validate.go   # Build-time validation
│   └── parser/
│       ├── parser.go     # Struct tag parsing for reflection DSL
│       └── parser_test.go
├── export/
│   ├── xstate.go         # XState JSON exporter
│   └── xstate_test.go    # Exporter tests
├── examples/
│   ├── traffic_light/    # Simple FSM example
│   ├── pedestrian_light/ # Hierarchical states example
│   ├── order_workflow/   # Reflection DSL example
│   └── incident_lifecycle/ # Complex workflow example
└── docs/
    ├── reflection-dsl.md # Reflection DSL guide
    ├── api-reference.md  # Complete API reference
    ├── prd.md            # Product Requirements Document
    └── tdd.md            # Technical Design Document
```

## Architecture

### Core Components

1. **Types** (`types.go`) - Public API types re-exported from internal/ir
   - `StateID`, `EventType`, `ActionType`, `GuardType`
   - `Event`, `State[C]`, `Action[C]`, `Guard[C]`

2. **Builder** (`builder.go`) - Fluent machine construction
   - `NewMachine[C](id)` → `MachineBuilder`
   - `.WithInitial()`, `.WithContext()`, `.WithAction()`, `.WithGuard()`
   - `.State(id)` → `StateBuilder`
   - `.On(event)` → `TransitionBuilder`

3. **Interpreter** (`interpreter.go`) - Runtime execution
   - `NewInterpreter(machine)` creates interpreter
   - `.Start()` enters initial state (recursively enters nested initial states)
   - `.Send(event)` processes events (with hierarchy event bubbling)
   - `.State()` returns current state
   - `.Matches(id)` checks current state or any ancestor
   - `.Done()` checks if in final state
   - `.UpdateContext(fn)` updates context with a function

4. **XState Exporter** (`export/xstate.go`) - Visualization export
   - `NewXStateExporter(machine)` creates exporter
   - `.Export()` returns XStateMachine struct
   - `.ExportJSON()` returns compact JSON string
   - `.ExportJSONIndent()` returns formatted JSON

5. **Reflection DSL** (`reflect.go`) - Struct-based machine definitions
   - `MachineDef`, `StateNode`, `CompoundNode`, `FinalNode` marker types
   - `ActionRegistry[C]` for named actions and guards
   - `FromStruct[M, C](registry)` builds machine from struct tags
   - `FromStructWithContext[M, C](registry, ctx)` with initial context

6. **Internal IR** (`internal/ir/`) - Immutable machine representation
   - `MachineConfig[C]` - Complete machine definition
   - `StateConfig` - State with transitions
   - `TransitionConfig` - Event → target mapping
   - `Validate()` - Build-time validation

### Execution Flow

```
Builder API → Build() → Validate → MachineConfig → NewInterpreter → Start() → Send(event)
```

### Transition Resolution

1. Find matching transition for event in current state (bubbles up to ancestors)
2. Evaluate guard (if present)
3. Calculate Lowest Common Ancestor (LCA) between source and target
4. Execute exit actions from current state up to (but not including) LCA
5. Execute transition actions
6. Enter target state hierarchy from LCA down to leaf, executing entry actions

### Hierarchical State Semantics

- **Event bubbling**: Events unhandled by current state bubble up to ancestors
- **Child priority**: Child state transitions take priority over parent transitions
- **Entry order**: Ancestors enter before descendants (root → leaf)
- **Exit order**: Descendants exit before ancestors (leaf → root)
- **Self-transitions**: Exit and re-enter the same state (external transition semantics)
- **Compound state entry**: Transitioning to a compound state enters its initial leaf

## API Usage

```go
type Context struct { Count int }

machine, err := statekit.NewMachine[Context]("example").
    WithInitial("idle").
    WithAction("increment", func(ctx *Context, e statekit.Event) {
        ctx.Count++
    }).
    WithGuard("hasCount", func(ctx Context, e statekit.Event) bool {
        return ctx.Count > 0
    }).
    State("idle").
        OnEntry("increment").
        On("START").Target("running").Guard("hasCount").
        Done().
    State("running").
        On("STOP").Target("idle").
        Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
interp.Send(statekit.Event{Type: "START"})
fmt.Println(interp.State().Value) // "running"
```

### Hierarchical States Example

```go
machine, _ := statekit.NewMachine[struct{}]("nested").
    WithInitial("active").
    State("active").
        WithInitial("idle").
        On("GLOBAL_RESET").Target("done").End().  // Parent handles this event
        State("idle").
            On("START").Target("working").
        End().  // Return to idle StateBuilder
        End().  // Return to active StateBuilder
        State("working").
            On("STOP").Target("idle").
        End().
    End().
    Done().
    State("done").Final().
    Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
fmt.Println(interp.State().Value)  // "idle" (initial leaf)
fmt.Println(interp.Matches("active"))  // true (matches ancestor)

interp.Send(statekit.Event{Type: "GLOBAL_RESET"})  // Bubbles up to "active"
fmt.Println(interp.State().Value)  // "done"
```

### XState Export Example

```go
exporter := export.NewXStateExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(jsonStr)
// Use with stately.ai/viz or XState Inspector
```

### Reflection DSL Example

```go
// Define machine using struct tags
type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"pending"`
    Pending   statekit.StateNode `on:"SUBMIT->validating:hasItems" entry:"logPending"`
    Validating statekit.StateNode `on:"VALID->payment,INVALID->pending"`
    Payment   statekit.StateNode `on:"PAID->completed/recordPayment"`
    Completed statekit.FinalNode
}

type OrderContext struct {
    Items []string
}

// Register actions and guards
registry := statekit.NewActionRegistry[OrderContext]().
    WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {
        fmt.Println("Order pending")
    }).
    WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {
        fmt.Println("Payment recorded")
    }).
    WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
        return len(ctx.Items) > 0
    })

// Build machine from struct
machine, err := statekit.FromStruct[OrderMachine, OrderContext](registry)
if err != nil {
    panic(err)
}

interp := statekit.NewInterpreter(machine)
interp.Start()
```

## Design Principles

- **Go-first execution** - Explicit, deterministic, testable
- **Statecharts over FSMs** - Full hierarchy support
- **Visualization as a feature** - XState JSON export for existing tooling
- **Small surface area** - Fewer features, better guarantees

## Current Status (v2.0)

Implemented:
- ✅ Core types
- ✅ Fluent builder API
- ✅ Synchronous interpreter
- ✅ Guards and actions
- ✅ Build-time validation
- ✅ Final states
- ✅ Hierarchical (nested) states
- ✅ Event bubbling to ancestors
- ✅ Proper entry/exit ordering
- ✅ XState JSON exporter
- ✅ Reflection DSL for struct-based definitions (v0.3)
- ✅ ActionRegistry for named actions/guards (v0.3)
- ✅ History states (shallow and deep) (v2.0)
- ✅ Delayed transitions with timers (v2.0)
- ✅ Parallel/orthogonal states (v2.0)

## History States

History states remember the last active child when re-entering a compound state:

```go
machine, _ := statekit.NewMachine[struct{}]("history_example").
    WithInitial("active").
    State("active").
        WithInitial("idle").
        On("PAUSE").Target("paused").End().
        History("hist").Shallow().Default("idle").End().  // Shallow history
        History("deepHist").Deep().Default("idle").End(). // Deep history
        State("idle").On("START").Target("working").End().End().
        State("working").On("NEXT").Target("done").End().End().
        State("done").End().
    Done().
    State("paused").
        On("RESUME").Target("hist").  // Resume to last child
    Done().
    Build()
```

- **Shallow history**: Remembers immediate child of compound state
- **Deep history**: Remembers exact leaf state (full path)

## Delayed Transitions

Delayed transitions trigger automatically after a specified duration:

```go
machine, _ := statekit.NewMachine[struct{}]("timeout_example").
    WithInitial("loading").
    State("loading").
        After(time.Second).Target("timeout").        // Timeout after 1s
        After(5*time.Second).Target("error").        // Error after 5s
        On("LOADED").Target("ready").                // Event cancels timers
    Done().
    State("timeout").Done().
    State("error").Done().
    State("ready").Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
// Timer starts automatically

// Option 1: Wait for timeout
// time.Sleep(2*time.Second) → state becomes "timeout"

// Option 2: Cancel via event
interp.Send(statekit.Event{Type: "LOADED"}) // → state becomes "ready", timer canceled

// Always call Stop() to clean up timers
interp.Stop()
```

Key behaviors:
- Timers are scheduled on state entry
- Timers are canceled on state exit (including via event transitions)
- Guards are evaluated when timer fires
- Multiple delayed transitions are supported (first to fire wins)
- `interp.Stop()` cancels all active timers

## Scope Constraints (v1)

Explicitly **out of scope** for v1:
- Parallel/orthogonal states
- Invoked actors/services
- Persistence/durability
