# Technical Design Document (TDD)

## Project: Statekit - Go Statecharts with XState Compatibility

**Version:** 1.0
**Status:** Draft
**Author:** Statekit Team
**Last Updated:** 2024-12-25

---

## Table of Contents

1. [Overview](#1-overview)
2. [Goals and Non-Goals](#2-goals-and-non-goals)
3. [System Architecture](#3-system-architecture)
4. [Data Structures](#4-data-structures)
5. [API Design](#5-api-design)
6. [Execution Semantics](#6-execution-semantics)
7. [XState JSON Export](#7-xstate-json-export)
8. [Error Handling](#8-error-handling)
9. [Testing Strategy](#9-testing-strategy)
10. [Package Structure](#10-package-structure)
11. [Implementation Phases](#11-implementation-phases)
12. [Open Design Decisions](#12-open-design-decisions)

---

## 1. Overview

### 1.1 Purpose

This document describes the technical design for Statekit, a Go library that enables developers to define, execute, and visualize statecharts. The library exports machine definitions to XState-compatible JSON for visualization in tools like Stately Visualizer.

### 1.2 Background

Statecharts extend finite state machines with:
- **Hierarchical states** (nested states with parent-child relationships)
- **Entry/exit actions** (side effects on state transitions)
- **Guards** (conditional transitions based on context)
- **Final states** (terminal states that signal completion)

XState is the de-facto standard for statecharts in JavaScript/TypeScript. This library brings equivalent capabilities to Go while maintaining interoperability through JSON export.

### 1.3 Terminology

| Term | Definition |
|------|------------|
| **Machine** | A complete statechart definition with states, transitions, and configuration |
| **State** | A node in the statechart (can be atomic, compound, or final) |
| **Transition** | A directed edge from one state to another, triggered by an event |
| **Event** | A named trigger that may cause a state transition |
| **Guard** | A predicate function that must return true for a transition to occur |
| **Action** | A side-effect function executed during transitions or state entry/exit |
| **Context** | User-defined data that accompanies the machine state |
| **Interpreter** | The runtime that executes the statechart |

---

## 2. Goals and Non-Goals

### 2.1 Goals

1. **Type-safe machine definition** via fluent builder API
2. **Deterministic execution** with explicit transition resolution
3. **Hierarchical state support** with proper entry/exit semantics
4. **XState JSON export** compatible with Stately Visualizer
5. **Minimal dependencies** (stdlib only for core)
6. **Testable by design** with mockable actions and guards

### 2.2 Non-Goals

1. Full XState spec compliance (parallel states, history, delays, actors)
2. JSON import / round-trip editing
3. Persistence or durability primitives
4. Distributed execution or coordination
5. Runtime visualization (export only)

---

## 3. System Architecture

### 3.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Code                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Builder API                                 │
│   NewMachine() → State() → On() → Target() → Build()            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              Internal Representation (IR)                        │
│   MachineConfig, StateConfig, TransitionConfig                  │
└─────────────────────────────────────────────────────────────────┘
                    │                   │
                    ▼                   ▼
┌──────────────────────────┐  ┌────────────────────────────────────┐
│    Execution Engine      │  │       XState Exporter              │
│  Interpreter + State     │  │   IR → XState JSON Schema          │
└──────────────────────────┘  └────────────────────────────────────┘
          │                              │
          ▼                              ▼
┌──────────────────────────┐  ┌────────────────────────────────────┐
│   Runtime State Query    │  │   JSON for Stately Visualizer      │
│   Send(event) API        │  │                                    │
└──────────────────────────┘  └────────────────────────────────────┘
```

### 3.2 Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **Builder API** | Fluent, type-safe construction of machine definitions |
| **IR (Internal Representation)** | Immutable, validated machine configuration |
| **Interpreter** | Stateful runtime that processes events and executes transitions |
| **Exporter** | Transforms IR to XState-compatible JSON |

### 3.3 Data Flow

1. User constructs machine using Builder API
2. `Build()` validates and produces immutable IR
3. IR is passed to `NewInterpreter()` to create runtime
4. User calls `Send(event)` to trigger transitions
5. Interpreter resolves transitions, executes actions, updates state
6. IR can be exported to JSON at any time via Exporter

---

## 4. Data Structures

### 4.1 Core Types

```go
// StateType represents the kind of state node
type StateType int

const (
    StateTypeAtomic   StateType = iota // Leaf state, no children
    StateTypeCompound                   // Has child states
    StateTypeFinal                      // Terminal state
)

// EventType is a named event identifier
type EventType string

// StateID uniquely identifies a state within a machine
type StateID string

// ActionType identifies a named action
type ActionType string

// GuardType identifies a named guard
type GuardType string
```

### 4.2 Machine Configuration (IR)

```go
// MachineConfig is the immutable internal representation of a statechart
type MachineConfig[C any] struct {
    ID          string
    Initial     StateID
    Context     C
    States      map[StateID]*StateConfig[C]
    Actions     map[ActionType]Action[C]
    Guards      map[GuardType]Guard[C]
}

// StateConfig represents a single state node
type StateConfig[C any] struct {
    ID          StateID
    Type        StateType
    Parent      StateID              // Empty for root-level states
    Initial     StateID              // For compound states only
    Children    []StateID            // For compound states only
    Entry       []ActionType
    Exit        []ActionType
    Transitions []*TransitionConfig
}

// TransitionConfig represents a single transition
type TransitionConfig struct {
    Event   EventType
    Target  StateID
    Guard   GuardType   // Optional
    Actions []ActionType
}
```

### 4.3 Runtime Types

```go
// Action is a side-effect function executed during transitions
type Action[C any] func(ctx *C, event Event)

// Guard is a predicate that determines if a transition should occur
type Guard[C any] func(ctx C, event Event) bool

// Event represents a runtime event with optional payload
type Event struct {
    Type    EventType
    Payload any
}

// State represents the current runtime state
type State[C any] struct {
    Value   StateID      // Current state ID (leaf state)
    Path    []StateID    // Full path from root to current state
    Context C
}

// Interpreter is the statechart runtime
type Interpreter[C any] struct {
    machine *MachineConfig[C]
    state   *State[C]
    started bool
}
```

### 4.4 Builder Types

```go
// MachineBuilder provides fluent API for machine construction
type MachineBuilder[C any] struct {
    id       string
    initial  StateID
    context  C
    states   []*StateBuilder[C]
    actions  map[ActionType]Action[C]
    guards   map[GuardType]Guard[C]
    errors   []error
}

// StateBuilder provides fluent API for state construction
type StateBuilder[C any] struct {
    id          StateID
    stateType   StateType
    initial     StateID
    entry       []ActionType
    exit        []ActionType
    transitions []*TransitionBuilder
    children    []*StateBuilder[C]
    parent      *StateBuilder[C]
}

// TransitionBuilder provides fluent API for transition construction
type TransitionBuilder struct {
    event   EventType
    target  StateID
    guard   GuardType
    actions []ActionType
}
```

---

## 5. API Design

### 5.1 Builder API

```go
// Machine creation
machine := statekit.NewMachine[Context]("trafficLight").
    WithInitialContext(Context{Count: 0}).
    WithInitial("green").

    // Define actions
    WithAction("logTransition", func(ctx *Context, e statekit.Event) {
        log.Printf("Transition: %s", e.Type)
    }).

    // Define guards
    WithGuard("canProceed", func(ctx Context, e statekit.Event) bool {
        return ctx.Count > 0
    }).

    // Define states
    State("green").
        OnEntry("logTransition").
        On("TIMER").Target("yellow").
        Done().

    State("yellow").
        On("TIMER").Target("red").
        Done().

    State("red").
        On("TIMER").Target("green").Guard("canProceed").
        Done().

    Build()
```

### 5.2 Hierarchical States

```go
machine := statekit.NewMachine[Context]("pedestrianLight").
    WithInitial("active").

    // Compound state with children
    State("active").
        WithInitial("walk").

        State("walk").
            On("COUNTDOWN").Target("wait").
            Done().

        State("wait").
            On("COUNTDOWN").Target("stop").
            Done().

        State("stop").Final().
            Done().

        // Transition from compound state (exits all children)
        On("RESET").Target("active").
        Done().

    Build()
```

### 5.3 Interpreter API

```go
// Create interpreter from machine config
interp := statekit.NewInterpreter(machine)

// Start the interpreter (enters initial state)
interp.Start()

// Query current state
state := interp.State()
fmt.Println(state.Value)    // "green"
fmt.Println(state.Path)     // ["green"]
fmt.Println(state.Context)  // Context{Count: 0}

// Check if in a specific state (handles hierarchy)
interp.Matches("green")           // true
interp.Matches("active.walk")     // true (hierarchical)

// Send events
interp.Send(statekit.Event{Type: "TIMER"})

// Send with payload
interp.Send(statekit.Event{
    Type:    "UPDATE",
    Payload: map[string]any{"value": 42},
})

// Check if machine is in final state
interp.Done() // bool

// Stop interpreter (runs exit actions)
interp.Stop()
```

### 5.4 Export API

```go
// Export to XState-compatible JSON
jsonBytes, err := statekit.ExportJSON(machine)

// Export with options
jsonBytes, err := statekit.ExportJSON(machine,
    statekit.WithPrettyPrint(true),
    statekit.WithVersion("5"),  // XState version
)

// Export to io.Writer
err := statekit.ExportTo(machine, os.Stdout)
```

---

## 6. Execution Semantics

### 6.1 Transition Resolution

When `Send(event)` is called:

1. **Find matching transitions** from current state (and ancestors for hierarchy)
2. **Evaluate guards** in document order; first passing guard wins
3. **Compute exit set** (states to exit, from leaf to root)
4. **Execute exit actions** in exit order
5. **Execute transition actions**
6. **Compute entry set** (states to enter, from root to leaf)
7. **Execute entry actions** in entry order
8. **Update current state** to new leaf state

### 6.2 Hierarchical State Transitions

```
Given machine:
  └─ active (compound)
       ├─ idle (initial)
       └─ working
            ├─ loading (initial)
            └─ processing

Transition from "active.working.loading" to "active.idle":
  1. Exit: loading → working
  2. Enter: idle

Transition from "active.working.loading" to "active.working.processing":
  1. Exit: loading
  2. Enter: processing
```

### 6.3 Event Queue

Events are processed synchronously and sequentially. The interpreter does not queue events - each `Send()` completes before returning.

```go
interp.Send(event1) // Fully processed
interp.Send(event2) // Fully processed after event1
```

### 6.4 Context Updates

Context is passed by pointer to actions, allowing mutations:

```go
WithAction("increment", func(ctx *Context, e statekit.Event) {
    ctx.Count++
})
```

Guards receive context by value (read-only):

```go
WithGuard("hasItems", func(ctx Context, e statekit.Event) bool {
    return len(ctx.Items) > 0
})
```

---

## 7. XState JSON Export

### 7.1 Target Schema

The exporter produces XState v5 compatible JSON:

```json
{
  "id": "trafficLight",
  "initial": "green",
  "context": {},
  "states": {
    "green": {
      "on": {
        "TIMER": {
          "target": "yellow"
        }
      }
    },
    "yellow": {
      "on": {
        "TIMER": {
          "target": "red"
        }
      }
    },
    "red": {
      "type": "final"
    }
  }
}
```

### 7.2 Mapping Rules

| Go Concept | XState JSON |
|------------|-------------|
| `StateTypeAtomic` | `{}` (default) |
| `StateTypeCompound` | `{ "initial": "...", "states": {...} }` |
| `StateTypeFinal` | `{ "type": "final" }` |
| Entry actions | `"entry": ["actionName"]` |
| Exit actions | `"exit": ["actionName"]` |
| Transition actions | `"actions": ["actionName"]` |
| Guards | `"cond": "guardName"` |

### 7.3 Limitations

- Action/guard implementations are not exported (names only)
- Context is exported as initial value snapshot
- Go-specific types in context may not serialize cleanly

---

## 8. Error Handling

### 8.1 Build-Time Validation

The `Build()` method validates the machine configuration:

```go
machine, err := builder.Build()
if err != nil {
    // ValidationError contains all issues
    var valErr *statekit.ValidationError
    if errors.As(err, &valErr) {
        for _, issue := range valErr.Issues {
            log.Printf("Validation: %s", issue)
        }
    }
}
```

Validation checks:
- Initial state exists
- All transition targets exist
- Referenced actions/guards are defined
- Compound states have initial child
- No orphaned states
- No circular initial state references

### 8.2 Runtime Errors

Runtime errors are exceptional and indicate bugs:

```go
// Panics if interpreter not started
interp.Send(event)

// Safe version that returns error
err := interp.TrySend(event)
```

### 8.3 Error Types

```go
// ValidationError contains build-time validation issues
type ValidationError struct {
    Issues []ValidationIssue
}

type ValidationIssue struct {
    Code    string   // e.g., "MISSING_TARGET"
    Message string
    Path    []string // e.g., ["states", "green", "transitions", "0"]
}

// RuntimeError indicates an execution problem
type RuntimeError struct {
    Code    string
    Message string
    State   StateID
    Event   EventType
}
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

Each component has isolated unit tests:

```
internal/ir/          → IR construction and validation
internal/interpreter/ → Transition resolution, action execution
internal/export/      → JSON serialization
builder/              → Fluent API correctness
```

### 9.2 Integration Tests

End-to-end tests with real statecharts:

```go
func TestTrafficLightCycle(t *testing.T) {
    machine := buildTrafficLight()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    assert.Equal(t, StateID("green"), interp.State().Value)

    interp.Send(Event{Type: "TIMER"})
    assert.Equal(t, StateID("yellow"), interp.State().Value)

    interp.Send(Event{Type: "TIMER"})
    assert.Equal(t, StateID("red"), interp.State().Value)
}
```

### 9.3 Property-Based Tests

Use `testing/quick` for invariant verification:

- Transitions always result in valid states
- Entry/exit actions fire in correct order
- Guards are evaluated before transitions

### 9.4 XState Compatibility Tests

Validate exported JSON against XState:

1. Export machine to JSON
2. Load JSON into XState (via test harness)
3. Compare state transitions between Go and XState
4. Verify visualization renders correctly

### 9.5 Benchmark Tests

Performance benchmarks for critical paths:

```go
func BenchmarkSend(b *testing.B) {
    machine := buildLargeMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    event := Event{Type: "NEXT"}
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        interp.Send(event)
    }
}
```

---

## 10. Package Structure

```
statekit/
├── statekit.go           # Public API re-exports
├── builder.go            # MachineBuilder, StateBuilder
├── interpreter.go        # Interpreter runtime
├── export.go             # JSON export functions
├── types.go              # Public types (Event, State, etc.)
├── errors.go             # Error types
│
├── internal/
│   ├── ir/
│   │   ├── machine.go    # MachineConfig
│   │   ├── state.go      # StateConfig
│   │   ├── transition.go # TransitionConfig
│   │   └── validate.go   # Validation logic
│   │
│   ├── interpreter/
│   │   ├── engine.go     # Core execution logic
│   │   ├── resolve.go    # Transition resolution
│   │   └── hierarchy.go  # Hierarchical state handling
│   │
│   └── export/
│       ├── xstate.go     # XState JSON schema
│       └── marshal.go    # Serialization
│
├── examples/
│   ├── traffic_light/
│   ├── order_workflow/
│   └── incident_lifecycle/
│
└── cmd/
    └── statekit/
        └── main.go       # CLI tool (v0.3)
```

---

## 11. Implementation Phases

### Phase 1: Core Foundation (v0.1)

**Goal:** Basic flat state machine with builder API

Deliverables:
- [ ] Core types (`Event`, `State`, `StateID`, etc.)
- [ ] `MachineConfig` IR with flat states only
- [ ] `MachineBuilder` fluent API
- [ ] `Interpreter` with `Start()`, `Send()`, `State()`
- [ ] Build-time validation
- [ ] Unit test suite

Exit Criteria:
- Can define and execute a simple traffic light state machine
- All transitions are deterministic and tested

### Phase 2: Hierarchical States + Export (v0.2)

**Goal:** Full statechart support with XState visualization

Deliverables:
- [ ] Compound state support in IR
- [ ] Hierarchical transition resolution
- [ ] Entry/exit action ordering for hierarchy
- [ ] `StateTypeFinal` support
- [ ] Guards and transition actions
- [ ] XState JSON exporter
- [ ] Integration test suite

Exit Criteria:
- Hierarchical state machine exports to valid XState JSON
- Exported JSON renders in Stately Visualizer
- Entry/exit semantics match XState behavior

### Phase 3: Developer Experience (v0.3)

**Goal:** Polish and alternative authoring

Deliverables:
- [ ] Reflection-based DSL (struct tags)
- [ ] CLI tool for JSON export
- [ ] Comprehensive documentation
- [ ] Example repository
- [ ] Performance benchmarks

Exit Criteria:
- Two authoring styles (builder + struct tags)
- `statekit export` CLI produces valid JSON
- README with getting started guide

---

## 12. Open Design Decisions

### 12.1 Generic Context vs Interface

**Option A:** Generic type parameter `[C any]`
- Pro: Type-safe, no casting
- Con: Viral generics throughout codebase

**Option B:** `context any` with type assertions
- Pro: Simpler API surface
- Con: Runtime type errors

**Decision:** Option A (generics) - type safety is a core principle.

### 12.2 Action Error Handling

**Option A:** Actions return `error`
- Pro: Explicit error handling
- Con: Complicates builder API, unclear semantics for failed actions

**Option B:** Actions panic on error, recover in interpreter
- Pro: Simple action signatures
- Con: Panic-based control flow is controversial

**Option C:** Actions are void, use context for error state
- Pro: Statecharts model errors as states, not exceptions
- Con: Requires careful machine design

**Decision:** TBD - leaning toward Option C (statechart-native error modeling).

### 12.3 Event Payload Typing

**Option A:** `Payload any` with type assertions
- Pro: Flexible, matches XState
- Con: No compile-time safety

**Option B:** Generic events `Event[P any]`
- Pro: Type-safe payloads
- Con: Complex, different event types per transition

**Option C:** Typed event constructors
- Pro: Balance of safety and ergonomics
- Con: More boilerplate

**Decision:** Option A for v1 - simplicity first, can add typed wrappers later.

### 12.4 Thread Safety

**Option A:** Interpreter is not thread-safe (document clearly)
- Pro: No synchronization overhead
- Con: User must synchronize

**Option B:** Interpreter uses mutex internally
- Pro: Safe by default
- Con: Performance overhead, potential deadlocks with actions

**Decision:** Option A - single-threaded by default, users can wrap with mutex if needed.

---

## Appendix A: XState JSON Schema Reference

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "id": { "type": "string" },
    "initial": { "type": "string" },
    "context": { "type": "object" },
    "states": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "type": { "enum": ["atomic", "compound", "final"] },
          "initial": { "type": "string" },
          "entry": { "type": "array", "items": { "type": "string" } },
          "exit": { "type": "array", "items": { "type": "string" } },
          "on": {
            "type": "object",
            "additionalProperties": {
              "oneOf": [
                { "type": "string" },
                {
                  "type": "object",
                  "properties": {
                    "target": { "type": "string" },
                    "cond": { "type": "string" },
                    "actions": { "type": "array", "items": { "type": "string" } }
                  }
                }
              ]
            }
          },
          "states": { "$ref": "#/properties/states" }
        }
      }
    }
  },
  "required": ["id", "initial", "states"]
}
```

---

## Appendix B: Example Machine - Order Workflow

```go
type OrderContext struct {
    OrderID   string
    Items     []Item
    Total     float64
    Error     string
}

machine := statekit.NewMachine[OrderContext]("orderWorkflow").
    WithInitial("pending").

    WithAction("validateOrder", validateOrder).
    WithAction("processPayment", processPayment).
    WithAction("notifyShipping", notifyShipping).
    WithAction("sendConfirmation", sendConfirmation).
    WithAction("logError", logError).

    WithGuard("hasItems", func(ctx OrderContext, e Event) bool {
        return len(ctx.Items) > 0
    }).
    WithGuard("paymentValid", func(ctx OrderContext, e Event) bool {
        return ctx.Total > 0
    }).

    State("pending").
        On("SUBMIT").Target("validating").Guard("hasItems").
        On("CANCEL").Target("cancelled").
        Done().

    State("validating").
        OnEntry("validateOrder").
        On("VALID").Target("processing").
        On("INVALID").Target("failed").
        Done().

    State("processing").
        WithInitial("payment").

        State("payment").
            OnEntry("processPayment").
            On("PAYMENT_SUCCESS").Target("shipping").
            On("PAYMENT_FAILED").Target("failed").
            Done().

        State("shipping").
            OnEntry("notifyShipping").
            On("SHIPPED").Target("completed").
            Done().

        Done().

    State("completed").Final().
        OnEntry("sendConfirmation").
        Done().

    State("failed").Final().
        OnEntry("logError").
        Done().

    State("cancelled").Final().
        Done().

    Build()
```
