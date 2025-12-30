# Actor Supervisor Example

A supervisor pattern demonstrating the actor model features of Statekit.

## Features Demonstrated

- Actor spawning and supervision
- Parent-child machine relationships
- Event forwarding between actors
- Supervision strategies
- Actor lifecycle management

## Architecture

```
┌────────────────────────────────────────────────────┐
│                   Supervisor                       │
│  ┌───────────┐    BEGIN    ┌─────────────┐        │
│  │   idle    │────────────►│ supervising │        │
│  └───────────┘             └──────┬──────┘        │
│                                   │ STOP          │
│                                   ▼               │
│                            ┌───────────┐          │
│                            │  stopped  │          │
│                            └───────────┘          │
│                                                   │
│  Spawned Workers:                                 │
│  ┌─────────────────────────────────────────────┐  │
│  │  worker-1   │   worker-2   │   worker-3     │  │
│  │  ┌──────┐   │   ┌──────┐   │   ┌──────┐    │  │
│  │  │ idle │   │   │ idle │   │   │ idle │    │  │
│  │  └──┬───┘   │   └──┬───┘   │   └──┬───┘    │  │
│  │     │ TASK  │      │ TASK  │      │ TASK   │  │
│  │     ▼       │      ▼       │      ▼        │  │
│  │  ┌────────┐ │   ┌────────┐ │   ┌────────┐  │  │
│  │  │working │ │   │working │ │   │working │  │  │
│  │  └───┬────┘ │   └───┬────┘ │   └───┬────┘  │  │
│  │      │      │       │      │       │       │  │
│  │      ▼      │       ▼      │       ▼       │  │
│  │   ┌──────┐  │    ┌──────┐  │    ┌──────┐   │  │
│  │   │ done │  │    │ done │  │    │ done │   │  │
│  │   └──────┘  │    └──────┘  │    └──────┘   │  │
│  └─────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/actor_supervisor/
```

### Programmatic Usage

```go
package main

import (
    "github.com/felixgeelhaar/statekit"
)

func main() {
    // Create worker machine
    workerMachine, _ := statekit.NewMachine[WorkerContext]("worker").
        WithInitial("idle").
        State("idle").On("TASK").Target("working").Done().
        State("working").On("COMPLETE").Target("done").Done().
        State("done").Final().Done().
        Build()

    // Create and start supervisor
    supervisor, _ := statekit.NewMachine[SupervisorContext]("supervisor").
        WithInitial("idle").
        State("idle").On("BEGIN").Target("supervising").Done().
        State("supervising").On("STOP").Target("stopped").Done().
        State("stopped").Final().Done().
        Build()

    interp := statekit.NewInterpreter(supervisor)
    interp.Start()
    interp.Send(statekit.Event{Type: "BEGIN"})

    // Spawn workers
    ref, _ := statekit.Spawn(interp, "worker-1", workerMachine,
        statekit.WithAutoForward("TASK"),
        statekit.WithSupervision(statekit.SupervisionRecover),
    )

    // Send task to worker
    interp.SendTo(ref.ID(), statekit.Event{Type: "TASK", Payload: "process item"})

    // Complete and cleanup
    interp.SendTo(ref.ID(), statekit.Event{Type: "COMPLETE"})
    interp.Send(statekit.Event{Type: "STOP"})
    interp.Stop()
}
```

## Run Tests

```bash
go test -v ./examples/actor_supervisor/...
```

## Key Concepts

### Spawning Actors

```go
ref, err := statekit.Spawn(interp, "worker-1", workerMachine,
    statekit.WithAutoForward("TASK"),      // Forward TASK events to worker
    statekit.WithSupervision(statekit.SupervisionRecover),
)
```

### Actor Options

- `WithAutoForward(eventTypes...)` - Automatically forward specific events to child
- `WithSupervision(strategy)` - Define error handling strategy
  - `SupervisionStop` - Stop actor on error
  - `SupervisionRecover` - Restart actor on error
  - `SupervisionEscalate` - Propagate error to parent

### Sending to Actors

```go
// Direct send to specific actor
err := interp.SendTo("worker-1", statekit.Event{Type: "TASK"})

// Send to parent from child (if configured)
err := interp.SendToParent(statekit.Event{Type: "DONE"})
```

### Actor Completion Events

When an actor reaches a final state, it sends `xstate.done.actor.<id>` to the parent:

```go
State("supervising").
    On("xstate.done.actor.worker-1").Target("supervising")
```

### Context Separation

Each actor has its own context type:

```go
type WorkerContext struct {
    WorkerID string
    TaskID   int
    Result   string
}

type SupervisorContext struct {
    TotalTasks     int
    CompletedTasks int
    ActiveWorkers  int
}
```
