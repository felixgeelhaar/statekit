# Actor Persistence

> **Tier 2 — experimental.** Actor metadata in snapshots and the `Spawn` APIs may change in a future v1.x minor. Pin a specific minor for production use. See [API Stability Tiers](./stability.md).

Snapshots capture spawned actor metadata for serialization and debugging purposes.

## Overview

When taking a snapshot of an interpreter with spawned actors, the snapshot includes metadata about each actor:
- Actor ID
- State where the actor was spawned
- Supervision strategy
- Auto-forward event types

This metadata is useful for:
- Debugging actor hierarchies
- Logging and monitoring
- Understanding system state
- Potentially recreating actors after restore

## Snapshot Structure

```go
type Snapshot[C any] struct {
    CurrentState    StateID           `json:"current_state"`
    Context         C                 `json:"context"`
    History         map[StateID]StateID `json:"history,omitempty"`
    DeepHistory     map[StateID]StateID `json:"deep_history,omitempty"`
    ActiveInParallel map[string]StateID `json:"active_in_parallel,omitempty"`
    SpawnedActors   []ActorMetadata   `json:"spawned_actors,omitempty"`
}

type ActorMetadata struct {
    ID             ActorID             `json:"id"`
    SpawnedInState StateID             `json:"spawned_in_state"`
    Supervision    SupervisionStrategy `json:"supervision"`
    AutoForward    []EventType         `json:"auto_forward,omitempty"`
}
```

## Basic Usage

```go
// Create parent interpreter
parent := statekit.NewInterpreter(parentMachine)
parent.Start()

// Spawn actors
statekit.Spawn(parent, "worker-1", workerMachine,
    statekit.WithSupervision(statekit.SupervisionRecover),
    statekit.WithAutoForward("TASK", "DATA"),
)

statekit.Spawn(parent, "worker-2", workerMachine,
    statekit.WithSupervision(statekit.SupervisionRestart),
)

// Take snapshot - includes actor metadata
snapshot := parent.Snapshot()

// SpawnedActors contains metadata for both workers
for _, actor := range snapshot.SpawnedActors {
    fmt.Printf("Actor: %s, State: %s, Supervision: %v\n",
        actor.ID, actor.SpawnedInState, actor.Supervision)
}
```

## JSON Serialization

Actor metadata is fully JSON-serializable:

```go
snapshot := parent.Snapshot()

// Serialize
data, err := json.Marshal(snapshot)
if err != nil {
    log.Fatal(err)
}

// Store or transmit...

// Deserialize
var restored statekit.Snapshot[MyContext]
err = json.Unmarshal(data, &restored)
if err != nil {
    log.Fatal(err)
}

// Access actor metadata
for _, actor := range restored.SpawnedActors {
    fmt.Printf("Actor %s was in state %s\n", actor.ID, actor.SpawnedInState)
}
```

## Example Output

```json
{
  "current_state": "processing",
  "context": {
    "order_id": "12345",
    "total": 99.99
  },
  "spawned_actors": [
    {
      "id": "worker-1",
      "spawned_in_state": "processing",
      "supervision": "recover",
      "auto_forward": ["TASK", "DATA"]
    },
    {
      "id": "worker-2",
      "spawned_in_state": "processing",
      "supervision": "restart"
    }
  ]
}
```

## Restoring Actors

Actor metadata is captured for informational purposes. When restoring from a snapshot, actors are **not** automatically recreated.

To restore actors manually:

```go
// Restore interpreter state
newInterp := statekit.NewInterpreter(machine)
err := newInterp.Restore(snapshot)
if err != nil {
    log.Fatal(err)
}

// Manually recreate actors based on metadata
for _, actorMeta := range snapshot.SpawnedActors {
    // Determine which machine to use based on actor ID or other criteria
    childMachine := getChildMachine(actorMeta.ID)

    // Build spawn options from metadata
    opts := []statekit.SpawnOption{
        statekit.WithSupervision(actorMeta.Supervision),
    }
    if len(actorMeta.AutoForward) > 0 {
        opts = append(opts, statekit.WithAutoForward(actorMeta.AutoForward...))
    }

    // Respawn the actor
    _, err := statekit.Spawn(newInterp, actorMeta.ID, childMachine, opts...)
    if err != nil {
        log.Printf("Failed to respawn actor %s: %v", actorMeta.ID, err)
    }
}
```

## Use Cases

### Debugging

Inspect actor hierarchy at any point:

```go
snapshot := interp.Snapshot()
log.Printf("Current state: %s", snapshot.CurrentState)
log.Printf("Active actors: %d", len(snapshot.SpawnedActors))
for _, actor := range snapshot.SpawnedActors {
    log.Printf("  - %s (supervision: %s)", actor.ID, actor.Supervision)
}
```

### Monitoring

Track actor lifecycle for observability:

```go
type ActorMonitor struct {
    previousActors map[statekit.ActorID]bool
}

func (m *ActorMonitor) Check(interp *statekit.Interpreter[MyContext]) {
    snapshot := interp.Snapshot()
    currentActors := make(map[statekit.ActorID]bool)

    for _, actor := range snapshot.SpawnedActors {
        currentActors[actor.ID] = true
        if !m.previousActors[actor.ID] {
            metrics.ActorSpawned.Inc()
        }
    }

    for id := range m.previousActors {
        if !currentActors[id] {
            metrics.ActorStopped.Inc()
        }
    }

    m.previousActors = currentActors
}
```

### Audit Trail

Record actor state for compliance:

```go
func recordAuditSnapshot(interp *statekit.Interpreter[MyContext]) {
    snapshot := interp.Snapshot()

    auditRecord := AuditRecord{
        Timestamp:    time.Now(),
        State:        snapshot.CurrentState,
        ActorCount:   len(snapshot.SpawnedActors),
        ActorDetails: snapshot.SpawnedActors,
    }

    auditStore.Save(auditRecord)
}
```

## Limitations

1. **Metadata only**: Actor snapshots contain metadata, not the full child machine state
2. **Manual restore**: Actors must be manually recreated after restore
3. **No child state**: The internal state of child machines is not captured
4. **Runtime only**: Actor metadata reflects the current runtime state

## Best Practices

1. **Regular snapshots**: Take periodic snapshots for debugging complex systems
2. **Log actor changes**: Monitor spawned actors for unexpected behavior
3. **Plan restoration**: Design your system to handle actor respawning on restore
4. **Use event sourcing**: For full durability, combine with event sourcing patterns
