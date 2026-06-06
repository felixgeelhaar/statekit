# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Statekit is a Go-native statechart execution engine. It enables backend engineers to define, execute, and visualize statecharts.

**One-liner:** Define and execute statecharts in Go — visualize them with built-in tools.

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

## Release Process

Before releasing a new version, always run the following checks:

```bash
# 1. Format all code
go fmt ./...

# 2. Run static analysis
go vet ./...

# 3. Run linter (fix any issues)
golangci-lint run

# 4. Run all tests
go test ./...

# 5. Check coverage meets thresholds
coverctl check

# 6. Use relicta for release management
relicta plan      # Analyze commits and plan release
relicta bump      # Set next version
relicta notes     # Generate release notes
relicta evaluate  # Evaluate risk
relicta approve   # Approve release
relicta publish   # Create tags and GitHub release
```

## Project Structure

```
statekit/
├── types.go              # Public types (Event, State, StateID, ActorID, etc.)
├── builder.go            # Fluent API (NewMachine, StateBuilder, TransitionBuilder, InvokeBuilder)
├── interpreter.go        # Runtime execution (Start, Send, State, Matches, Done, Stop)
├── actor.go              # Actor model (Spawn, ActorRef, supervision strategies)
├── snapshot.go           # Snapshot/Restore (interpreter state persistence)
├── eventstore.go         # Event sourcing interfaces (EventStore, SnapshotStore)
├── persist.go            # PersistentInterpreter (event sourcing wrapper)
├── distributed.go        # Distributed execution (StreamLock, DistributedInterpreter)
├── reflect.go            # Reflection DSL (FromStruct, MachineDef, ActionRegistry)
├── *_test.go             # Comprehensive tests for all features
├── cmd/
│   └── statekit/         # CLI tool
│       └── commands/     # viz, generate, version commands
├── generate/             # Go code generation from Native JSON
│   ├── generate.go       # Generator and Native JSON parsing
│   └── generate_test.go  # Generator tests
├── http/                 # HTTP integration for web frameworks
│   ├── http.go           # MachineHandler, Registry, Middleware
│   └── http_test.go      # HTTP handler tests
├── otel/                 # OpenTelemetry tracing integration
│   ├── otel.go           # TracingInterpreter, TracingHook
│   └── otel_test.go      # Tracing tests
├── viz/                  # Visualization package
│   ├── model.go          # VizMachine, VizState models
│   ├── parser.go         # Native JSON parser
│   ├── ascii/            # ASCII box diagram renderer
│   ├── mermaid/          # Mermaid stateDiagram renderer
│   ├── html/             # Interactive HTML renderer
│   ├── goparser/         # Go source code parser
│   └── tui/              # Interactive terminal UI (Bubble Tea)
├── internal/
│   ├── ir/               # Immutable machine representation
│   │   ├── types.go      # Core type definitions
│   │   ├── machine.go    # MachineConfig, StateConfig, TransitionConfig
│   │   └── validate.go   # Build-time validation
│   └── parser/           # Struct tag parsing for reflection DSL
├── export/
│   ├── native.go         # Statekit Native JSON exporter
│   └── native_test.go    # Exporter tests
├── statetest/            # Testing utilities
│   ├── recorder.go       # Transition recorder
│   ├── assert.go         # Test assertions
│   └── helpers.go        # Convenience functions
├── debug/                # Debugging tools
│   ├── inspector.go      # Runtime inspection
│   └── graph.go          # State graph analysis
├── metrics/              # Prometheus integration
│   └── metrics.go        # MetricsInterpreter wrapper
├── health/               # Health checks
│   └── health.go         # Kubernetes probes
├── lint/                 # Static analysis
│   └── lint.go           # Lint rules and diagnostics
├── plugin/               # Plugin system for extensibility
│   ├── plugin.go         # Plugin interfaces and composite
│   └── doc.go            # Package documentation
├── examples/
│   ├── traffic_light/    # Simple FSM example
│   ├── pedestrian_light/ # Hierarchical states example
│   ├── order_workflow/   # Reflection DSL example
│   ├── incident_lifecycle/ # Complex workflow example
│   └── actor_supervisor/ # Actor model example
└── docs/                 # Documentation
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

4. **Native Exporter** (`export/native.go`) - Visualization export
   - `NewNativeExporter(machine)` creates exporter
   - `.Export()` returns VizMachine struct
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

### Visualization Example

```go
exporter := export.NewNativeExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
fmt.Println(jsonStr)
// Use with statekit viz command
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
- **Visualization as a feature** - Built-in visualization tools
- **Small surface area** - Fewer features, better guarantees

## Current Status (v1.0)

**v1.0 is the first stable release** with API stability commitment. All planned features implemented:

**Core (v0.2)**
- ✅ Fluent builder API with generics
- ✅ Synchronous interpreter with guards and actions
- ✅ Build-time validation
- ✅ Final states
- ✅ Hierarchical (nested) states with event bubbling
- ✅ Native JSON exporter for visualization

**Reflection DSL (v0.3)**
- ✅ Struct-based machine definitions with tags
- ✅ ActionRegistry for named actions/guards

**Advanced Statecharts (v0.4)**
- ✅ History states (shallow and deep)
- ✅ Delayed transitions with timers
- ✅ Parallel/orthogonal states with regions

**Production Features (v0.5)**
- ✅ Invoked services (async operations with cancellation)
- ✅ Snapshot/Restore (interpreter state persistence)

**Actor Model (v0.6)**
- ✅ Dynamic actor spawning with `Spawn()` and `SpawnWithContext()`
- ✅ State-scoped lifecycle (actors stop when parent exits spawning state)
- ✅ Bidirectional communication (`SendTo`, `SendParent`, `ActorRef.Send`)
- ✅ Supervision strategies (Escalate, Recover, Restart, Stop)
- ✅ Auto-forwarding of events to child actors
- ✅ XState-compatible done/error events

**CLI Visualization Tool (v0.6)**
- ✅ `statekit viz` command with multiple output formats
- ✅ ASCII box diagrams for terminal
- ✅ Mermaid stateDiagram-v2 markdown output
- ✅ Interactive TUI with keyboard navigation
- ✅ Go package parser for extracting machine definitions

**Event Persistence (v0.7)**
- ✅ Event sourcing with `PersistentInterpreter`
- ✅ `EventStore` interface for pluggable storage backends
- ✅ `SnapshotStore` interface for periodic snapshots
- ✅ Optimistic concurrency control (version-based conflict detection)
- ✅ Automatic state hydration from events + snapshots
- ✅ Configurable snapshot strategies (by interval, on final, by time)
- ✅ Event replay for debugging and read models
- ✅ In-memory implementations for testing

**Distributed Execution (v0.8)**
- ✅ `StreamLock` interface for distributed locking (Redis, etcd, PostgreSQL, etc.)
- ✅ `DistributedInterpreter` with automatic lock management
- ✅ Lock TTL with automatic renewal
- ✅ Graceful lock release and failover detection
- ✅ `ClusterMembership` interface for cluster coordination
- ✅ `StreamRouter` with consistent hashing for load distribution
- ✅ In-memory implementations for testing

**Developer Experience (v0.9)**
- ✅ Go code generation from XState JSON (`generate` package)
- ✅ HTTP handlers and middleware for web frameworks (`http` package)
- ✅ OpenTelemetry tracing for state transitions (`otel` package)

**Testing & Debugging (v0.10)**
- ✅ Test assertions and helpers (`statetest` package)
- ✅ Transition recorder for test verification
- ✅ Debug inspector for runtime state inspection (`debug` package)

**Observability (v0.11)**
- ✅ Prometheus metrics for state machine monitoring (`metrics` package)
- ✅ Health check endpoints for Kubernetes probes (`health` package)

**Static Analysis (v0.12)**
- ✅ Lint package for detecting structural issues (`lint` package)
- ✅ Rules: unreachable, dead-end, non-determinism, compound-initial, self-transition, unused-action, unused-guard

**Eventless Transitions (v0.13)**
- ✅ Always transitions (auto-trigger on state entry)
- ✅ Conditional always transitions with guards
- ✅ Priority ordering for multiple always transitions

**Advanced Composition (v0.14)**
- ✅ Plugin system with lifecycle hooks (`plugin` package)
- ✅ Machine composition via `InvokeMachine()` builder
- ✅ Actor metadata persistence in snapshots

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

## Invoked Services

Invoke async operations that are started on state entry and cancelled on exit:

```go
machine, _ := statekit.NewMachine[struct{}]("data_loader").
    WithInitial("loading").
    WithService("fetchData", func(ctx ir.ServiceContext[struct{}]) error {
        // Perform async work
        resp, err := http.Get("https://api.example.com/data")
        if err != nil {
            return err  // Triggers OnError transition
        }
        defer resp.Body.Close()
        return nil  // Triggers OnDone transition
    }).
    WithAction("handleSuccess", func(ctx *struct{}, e statekit.Event) {}).
    State("loading").
        Invoke("fetchData").
            ID("fetchData").
            OnDone("success").
            OnDoneAction("handleSuccess").
            OnError("failure").
        End().
    Done().
    State("success").Final().Done().
    State("failure").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
// Service starts automatically

// Services can send events back to the machine
// ctx.Send(statekit.Event{Type: "DATA_RECEIVED"})

// Always call Stop() to cancel active services
interp.Stop()
```

Key behaviors:
- Services start when entering a state with `Invoke()`
- Services are cancelled when exiting the state
- `OnDone` transition triggers on successful completion
- `OnError` transition triggers on error return
- Services receive a context with `Context` (for cancellation), `MachineContext`, and `Send` function
- Multiple services can be invoked per state
- `interp.Stop()` cancels all active services

## Snapshot/Restore

Persist and restore interpreter state for durability:

```go
// Create and run interpreter
machine, _ := statekit.NewMachine[MyContext]("workflow").
    // ... machine definition
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
interp.Send(statekit.Event{Type: "PROCESS"})

// Take a snapshot
snapshot := interp.Snapshot()

// Serialize for storage (JSON, database, etc.)
data, _ := json.Marshal(snapshot)
// Store data...

// Later: restore from snapshot
var restored statekit.InterpreterSnapshot[MyContext]
json.Unmarshal(data, &restored)

newInterp := statekit.NewInterpreter(machine)
if err := newInterp.Restore(restored); err != nil {
    // Handle error
}

// Continue from where we left off
newInterp.Send(statekit.Event{Type: "NEXT"})
```

Snapshot includes:
- Current state path (for hierarchical states)
- Machine context
- History state memory (shallow and deep)
- Parallel region states

## Actor Model

Spawn child state machines with state-scoped lifecycle and supervision:

```go
// Define child machine
childMachine, _ := statekit.NewMachine[ChildCtx]("worker").
    WithInitial("idle").
    State("idle").On("TASK").Target("working").Done().
    State("working").On("COMPLETE").Target("done").Done().
    State("done").Final().Done().
    Build()

// Define parent machine
parentMachine, _ := statekit.NewMachine[ParentCtx]("supervisor").
    WithInitial("active").
    State("active").
        On("xstate.done.actor.worker").Target("completed").
    Done().
    State("completed").Final().Done().
    Build()

// Create and start parent
parent := statekit.NewInterpreter(parentMachine)
parent.Start()

// Spawn child actor
ref, err := statekit.Spawn(parent, "worker", childMachine,
    statekit.WithSupervision(statekit.SupervisionRecover),
    statekit.WithAutoForward("TASK"),
    statekit.WithOnDone("completed"),
)

// Send events to child
ref.Send(statekit.Event{Type: "TASK", Payload: "data"})

// Or via parent
parent.SendTo("worker", statekit.Event{Type: "COMPLETE"})

// Child can send to parent (from within child actions)
// childInterp.SendParent(statekit.Event{Type: "RESULT"})

// Wait for child completion
<-ref.Done()

// Clean up
parent.Stop()
```

Key behaviors:
- **State-scoped lifecycle**: Actors spawned in a state are automatically stopped when that state exits
- **Supervision strategies**:
  - `SupervisionEscalate`: Bubble error to parent via `xstate.error.actor.<id>` event
  - `SupervisionRecover`: Log and continue
  - `SupervisionRestart`: Stop and allow respawn
  - `SupervisionStop`: Stop silently
- **Auto-forwarding**: Events matching configured types are automatically forwarded to child
- **Done/Error events**: XState-compatible events (`xstate.done.actor.<id>`, `xstate.error.actor.<id>`)
- **Concurrent spawning**: Thread-safe spawn and communication

## CLI Visualization Tool

Visualize state machines from Statekit JSON or Go source code:

```bash
# From Statekit JSON file
statekit viz machine.json

# With Mermaid output
statekit viz machine.json --format mermaid -o diagram.md

# Interactive HTML
statekit viz machine.json --format html -o machine.html

# Interactive TUI
statekit viz machine.json --format tui

# From Go package
statekit viz --go-package ./examples/order_workflow

# Filter by type
statekit viz --go-package ./... --go-type OrderMachine

# Pipe from stdin
cat machine.json | statekit viz
```

Output formats:
- `ascii` (default): Terminal-friendly box diagrams
- `mermaid`: Mermaid stateDiagram-v2 markdown
- `html`: Interactive simulator
- `tui`: Interactive terminal UI with keyboard navigation

## Event Persistence

Event sourcing for durable state machine execution:

```go
// Create event and snapshot stores
eventStore := statekit.NewMemoryEventStore()
snapshotStore := statekit.NewMemorySnapshotStore[MyContext]()

// Create persistent interpreter
pi, err := statekit.NewPersistentInterpreter(
    ctx,
    "order-123",  // Stream ID (identifies this instance)
    machine,
    eventStore,
    statekit.WithSnapshotStore[MyContext](snapshotStore),
    statekit.WithSnapshotConfig[MyContext](statekit.SnapshotConfig{
        Strategy: statekit.SnapshotByInterval,
        Interval: 10,  // Snapshot every 10 events
    }),
)
if err != nil {
    return err
}
defer pi.Stop()

// Send events (recorded but not yet persisted)
pi.Send(statekit.Event{Type: "SUBMIT"})
pi.Send(statekit.Event{Type: "PAY", Payload: 99.99})

// Persist uncommitted events
committed, err := pi.Commit(ctx)
if err != nil {
    if _, ok := err.(*statekit.ErrConcurrencyConflict); ok {
        // Handle concurrent modification
    }
}

// State is automatically hydrated on restart
pi2, _ := statekit.NewPersistentInterpreter(ctx, "order-123", machine, eventStore,
    statekit.WithSnapshotStore[MyContext](snapshotStore),
)
// pi2 is now in the same state as pi was after Commit
```

Key behaviors:
- **Event sourcing**: All state-changing events are recorded
- **Optimistic concurrency**: Version-based conflict detection prevents lost updates
- **Automatic hydration**: State is reconstructed from snapshot + events on startup
- **Snapshot strategies**:
  - `SnapshotNever`: Manual snapshots only
  - `SnapshotByInterval`: Snapshot every N events
  - `SnapshotOnFinal`: Snapshot when reaching a final state
  - `SnapshotByTime`: Snapshot after a time duration
- **Replay functions**: `ReplayEvents` and `ReplayToVersion` for debugging/read models

Implementing custom stores:
```go
type EventStore interface {
    AppendEvents(ctx context.Context, streamID string, expectedVersion int, events []PersistedEvent) error
    LoadEvents(ctx context.Context, streamID string, fromVersion int) ([]PersistedEvent, error)
    GetStreamVersion(ctx context.Context, streamID string) (int, error)
}

type SnapshotStore[C any] interface {
    SaveSnapshot(ctx context.Context, streamID string, version int, snapshot *MachineSnapshot[C]) error
    LoadSnapshot(ctx context.Context, streamID string, maxVersion int) (*MachineSnapshot[C], int, error)
}
```

## Distributed Execution

Run state machines across multiple nodes with distributed locking and coordination:

```go
// Create stores and lock
eventStore := statekit.NewMemoryEventStore()
snapshotStore := statekit.NewMemorySnapshotStore[MyContext]()
streamLock := statekit.NewMemoryStreamLock()  // In production: use Redis, etcd, etc.

// Create distributed interpreter
// Automatically acquires lock and hydrates state
di, err := statekit.NewDistributedInterpreter(
    ctx,
    "order-123",  // Stream ID
    machine,
    eventStore,
    streamLock,
    statekit.WithDistributedSnapshotStore[MyContext](snapshotStore),
    statekit.WithDistributedSnapshotConfig[MyContext](statekit.SnapshotConfig{
        Strategy: statekit.SnapshotByInterval,
        Interval: 10,
    }),
    statekit.WithLockTTL[MyContext](30*time.Second),  // Lock TTL with auto-renewal
)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Another node holds the lock
    }
    return err
}
defer di.Stop(ctx)

// Process events (lock is automatically renewed)
if err := di.Send(statekit.Event{Type: "SUBMIT"}); err != nil {
    if err == statekit.ErrLockLost {
        // Lock was lost - stop processing
    }
}

// Commit persists events
committed, err := di.Commit(ctx)

// Monitor lock health
select {
case <-di.LockLost():
    // Lock was lost - handle failover
default:
    // Lock still held
}
```

Key behaviors:
- **Exclusive access**: Only one node can process a stream at a time
- **Automatic renewal**: Lock TTL is renewed periodically (default: every 1/3 of TTL)
- **Lock loss detection**: `LockLost()` channel and `ErrLockLost` error for failover
- **Graceful release**: `Stop()` releases lock and cleans up resources

### Implementing Custom Lock Backends

```go
type StreamLock interface {
    // Acquire blocks until lock is acquired or context cancelled
    Acquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error)

    // TryAcquire returns immediately with ErrLockHeld if lock is unavailable
    TryAcquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error)
}

type Lock interface {
    Renew(ctx context.Context, ttl time.Duration) error
    Release(ctx context.Context) error
    Done() <-chan struct{}  // Closed when lock is lost
}
```

Example Redis implementation (pseudocode):
```go
type RedisStreamLock struct {
    client *redis.Client
}

func (r *RedisStreamLock) TryAcquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error) {
    ok, err := r.client.SetNX(ctx, "lock:"+streamID, nodeID, ttl).Result()
    if err != nil {
        return nil, err
    }
    if !ok {
        return nil, statekit.ErrLockHeld
    }
    return &redisLock{client: r.client, streamID: streamID, nodeID: nodeID}, nil
}
```

### Cluster Coordination

For advanced use cases, implement `ClusterMembership` for node discovery:

```go
type ClusterMembership interface {
    Join(ctx context.Context, node ClusterNode) error
    Leave(ctx context.Context) error
    Members(ctx context.Context) ([]ClusterNode, error)
    Watch(ctx context.Context) (<-chan MembershipEvent, error)
}
```

Use `StreamRouter` for consistent hashing to distribute streams across nodes:

```go
router := statekit.NewConsistentHashRouter(100)  // 100 virtual nodes per physical node
members := getMemberList()

// Check if this node should handle the stream
if router.IsLocal(streamID, members, localNodeID) {
    // Acquire lock and process
    di, _ := statekit.NewDistributedInterpreter(ctx, streamID, machine, eventStore, streamLock)
    // ...
}
```

## Go Code Generation

Generate Go code from Statekit JSON definitions:

```bash
# Generate from Statekit JSON file
statekit generate machine.json -o machine.go

# With custom package and type name
statekit generate machine.json -p mypackage -t OrderWorkflow -c OrderContext

# From stdin
cat machine.json | statekit generate -o machine.go
```

Generated code includes:
- `Build<TypeName>()` function that constructs the state machine
- Stub functions for all actions and guards
- Type alias for the context type

Example output:
```go
// Code generated by statekit generate. DO NOT EDIT.
package main

import "go.klarlabs.de/statekit"

type OrderMachineContext = struct{}

func actionLogOrder(ctx *OrderMachineContext, e statekit.Event) {
    // TODO: Implement logOrder action
}

func guardHasItems(ctx OrderMachineContext, e statekit.Event) bool {
    // TODO: Implement hasItems guard
    return true
}

func BuildOrderMachine() (*statekit.MachineConfig[OrderMachineContext], error) {
    return statekit.NewMachine[OrderMachineContext]("order").
        WithInitial("pending").
        WithAction("logOrder", actionLogOrder).
        WithGuard("hasItems", guardHasItems).
        State("pending").On("SUBMIT").Target("processing").Guard("hasItems").Done().
        State("processing").On("COMPLETE").Target("done").Done().
        State("done").Final().Done().
        Build()
}
```

## HTTP Integration

Framework-agnostic HTTP handlers and middleware for state machines:

```go
import statekithttp "go.klarlabs.de/statekit/http"

// Create handler for a single machine
handler := statekithttp.NewMachineHandler(interp)

// Endpoints available:
// GET  /state   - Returns current state
// POST /event   - Send an event
// GET  /context - Returns machine context

// Use with standard library
mux := http.NewServeMux()
mux.Handle("/machine/", http.StripPrefix("/machine", handler))

// Or use the helper function
mux := statekithttp.NewServeMux(interp, "/api/machine")
```

### Machine Registry

Manage multiple machine instances:

```go
// Factory creates new interpreters
factory := func(id string) (*statekit.Interpreter[MyContext], error) {
    interp := statekit.NewInterpreter(machine)
    interp.Start()
    return interp, nil
}

registry := statekithttp.NewMachineRegistry(factory)

// Get or create interpreter by ID
interp, err := registry.Get("order-123")

// Remove (stops interpreter)
registry.Remove("order-123")

// List all IDs
ids := registry.List()
```

### Middleware

Inject interpreters into request context:

```go
// Single machine middleware
middleware := statekithttp.MachineMiddleware(interp)
http.Handle("/", middleware(myHandler))

// Registry middleware (extracts ID from request)
idExtractor := func(r *http.Request) string {
    return r.URL.Query().Get("machine_id")
}
middleware := statekithttp.RegistryMiddleware(registry, idExtractor)
http.Handle("/", middleware(myHandler))

// Access in handler
func myHandler(w http.ResponseWriter, r *http.Request) {
    interp, ok := statekithttp.MachineFromContext[MyContext](r.Context())
    if !ok {
        http.Error(w, "no machine", 500)
        return
    }
    // Use interp...
}
```

### Response Types

```go
// GET /state response
type StateResponse struct {
    CurrentState string `json:"currentState"`
    Done         bool   `json:"done"`
    MachineID    string `json:"machineId,omitempty"`
}

// POST /event request
type EventRequest struct {
    Type    string         `json:"type"`
    Payload map[string]any `json:"payload,omitempty"`
}

// POST /event response
type EventResponse struct {
    PreviousState string `json:"previousState"`
    CurrentState  string `json:"currentState"`
    Transitioned  bool   `json:"transitioned"`
    Done          bool   `json:"done"`
}
```

## OpenTelemetry Tracing

Instrument state machines with OpenTelemetry for observability:

```go
import statekotel "go.klarlabs.de/statekit/otel"

// Wrap interpreter with tracing
interp := statekit.NewInterpreter(machine)
ti := statekotel.NewTracingInterpreter(interp, "order-workflow")

// Start with context (creates root span)
ctx := ti.Start(context.Background())

// Send events (creates child spans)
ti.Send(ctx, statekit.Event{Type: "SUBMIT"})
ti.Send(ctx, statekit.Event{Type: "PAY"})

// Stop ends root span
ti.Stop()
```

### Span Attributes

Each event creates a span with attributes:
- `statekit.machine.id` - Machine identifier
- `statekit.event.type` - Event type
- `statekit.state.before` - State before transition
- `statekit.state.after` - State after transition
- `statekit.transitioned` - Whether state changed

Events recorded:
- `state.entered` - When entering initial state
- `state.transition` - On state transitions
- `state.final` - When reaching final state

### Custom Tracer

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("my-app")
ti := statekotel.NewTracingInterpreter(interp, "my-machine",
    statekotel.WithTracer[MyContext](tracer),
)
```

### Tracing Hook

For simpler integration without wrapping:

```go
hook := statekotel.TracingHook(tracer)

// Call after each transition
hook(ctx, "machine-id", event, beforeState, afterState)
```

### Attribute Helpers

```go
// For custom spans
attrs := statekotel.StateAttributes("machine-1", "running")
attrs := statekotel.EventAttributes("machine-1", event)
attrs := statekotel.TransitionAttributes("machine-1", event, "idle", "running")
```

## Plugin System

Extend interpreter behavior with lifecycle hooks:

```go
import "go.klarlabs.de/statekit/plugin"

// Implement plugin hooks you need
type LoggingPlugin[C any] struct{}

func (p *LoggingPlugin[C]) Name() string { return "logging" }

func (p *LoggingPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
    log.Printf("entered state: %s", state)
}

func (p *LoggingPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
    log.Printf("exited state: %s", state)
}

func (p *LoggingPlugin[C]) BeforeTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
    log.Printf("transitioning %s → %s on %s", from, to, event.Type)
}

func (p *LoggingPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
    log.Printf("transitioned %s → %s", from, to)
}

// Register with interpreter
interp := statekit.NewInterpreter(machine)
interp.Use(&LoggingPlugin[MyContext]{})
interp.Start()
```

Available hook interfaces:
- `OnStart(ctx)` / `OnStop(ctx)` - Interpreter lifecycle
- `OnEvent(ctx, event) Event` - Event interception and modification
- `OnEnter(ctx, state)` / `OnExit(ctx, state)` - State entry/exit
- `BeforeTransition(ctx, from, to, event)` / `AfterTransition(ctx, from, to, event)` - Transition lifecycle
- `BeforeAction(ctx, action, event)` / `AfterAction(ctx, action, event)` - Action execution
- `OnError(ctx, err)` - Error handling

Combine multiple plugins:
```go
composite := plugin.NewComposite[MyContext](loggingPlugin, metricsPlugin, auditPlugin)
interp.Use(composite)
```

## Machine Composition (InvokeMachine)

Invoke child state machines within a state with automatic lifecycle management:

```go
// Define child machine
childMachine, _ := statekit.NewMachine[ChildCtx]("worker").
    WithInitial("working").
    State("working").On("COMPLETE").Target("done").End().Done().
    State("done").Final().Done().
    Build()

// Parent machine invokes child
parent, _ := statekit.NewMachine[ParentCtx]("parent").
    WithInitial("idle").
    // Register child machine factory
    WithChildMachine("worker", func(ctx ParentCtx, send func(statekit.Event) error) ir.ChildInterpreter {
        return statekit.NewInterpreter(childMachine)
    }).
    State("idle").
        On("START").Target("processing").End().
    Done().
    State("processing").
        // Invoke child machine when entering this state
        InvokeMachine("worker").
            ID("w1").
            OnDone("completed").  // Transition when child reaches final state
        End().
    Done().
    State("completed").Final().Done().
    Build()

interp := statekit.NewInterpreter(parent)
interp.Start()
interp.Send(statekit.Event{Type: "START"})
// Child machine starts automatically
// When child reaches "done" (final), parent transitions to "completed"
```

Key behaviors:
- **Automatic lifecycle**: Child starts on state entry, stops on state exit
- **OnDone transition**: Parent transitions when child reaches final state
- **Type-erased**: Child machines can have different context types
- **Multiple invocations**: Multiple children can be invoked per state

## Actor Metadata Persistence

Snapshots now capture spawned actor metadata for serialization:

```go
// Spawn actors with configuration
statekit.Spawn(interp, "worker-1", childMachine,
    statekit.WithSupervision(statekit.SupervisionRecover),
    statekit.WithAutoForward("DATA", "TASK"),
)

// Take snapshot - includes actor metadata
snap := interp.Snapshot()

// Serialize to JSON
data, _ := json.Marshal(snap)

// SpawnedActors field contains:
// - ID: actor identifier
// - SpawnedInState: state where actor was spawned
// - Supervision: supervision strategy
// - AutoForward: event types to auto-forward
```

Note: Actor metadata is captured for informational purposes. Actors must be manually respawned after restoring a snapshot.

## Performance Characteristics

Benchmarks on Apple M1:

| Operation | Time | Allocations |
|-----------|------|-------------|
| Simple transition | ~960ns | 7 |
| Hierarchical bubble | ~1.7μs | 10 |
| No match (fast path) | ~91ns | 0 |
| State query | ~35ns | 0 |
| Context update | ~27ns | 0 |
| Snapshot | ~383ns | 3 |
| Plugin overhead | ~0ns | 0 |

Run benchmarks with: `go test -bench=. -benchmem .`

## Post v1.0 Considerations

The library is feature-complete and API-stable. Future additions (v1.x):

**Formal Verification**
- Model checking for state machine properties
- Deadlock detection
- Liveness verification

**Visual Editor Integration**
- VSCode extension for machine editing
- Real-time visualization
- Enhanced Stately.ai integration

**Advanced Persistence**
- Full actor state persistence
- Automatic actor restoration
- Distributed actor coordination
