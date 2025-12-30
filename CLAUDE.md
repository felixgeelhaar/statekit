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
│   └── statekit/         # CLI tool for visualization
│       └── commands/     # viz, version commands
├── viz/                  # Visualization package
│   ├── model.go          # VizMachine, VizState models
│   ├── parser.go         # XState JSON parser
│   ├── ascii/            # ASCII box diagram renderer
│   ├── mermaid/          # Mermaid stateDiagram renderer
│   ├── goparser/         # Go source code parser
│   └── tui/              # Interactive terminal UI (Bubble Tea)
├── internal/
│   ├── ir/               # Immutable machine representation
│   │   ├── types.go      # Core type definitions
│   │   ├── machine.go    # MachineConfig, StateConfig, TransitionConfig
│   │   └── validate.go   # Build-time validation
│   └── parser/           # Struct tag parsing for reflection DSL
├── export/
│   ├── xstate.go         # XState JSON exporter
│   └── xstate_test.go    # Exporter tests
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

## Current Status (v0.8)

All planned features implemented:

**Core (v0.2)**
- ✅ Fluent builder API with generics
- ✅ Synchronous interpreter with guards and actions
- ✅ Build-time validation
- ✅ Final states
- ✅ Hierarchical (nested) states with event bubbling
- ✅ XState JSON exporter for visualization

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

Visualize state machines from XState JSON or Go source code:

```bash
# From XState JSON file
statekit viz machine.json

# With Mermaid output
statekit viz machine.json --format mermaid -o diagram.md

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

## Future Enhancements

Potential features for future versions:

**Developer Experience**
- Go code generation from XState JSON
- Integration with popular Go web frameworks
- OpenTelemetry tracing for state transitions
