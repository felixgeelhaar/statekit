# Design: Persistence and Serialization

## Overview

State machine persistence enables:
- **Durability**: Survive process restarts
- **Distribution**: Share state across services
- **Debugging**: Inspect and replay state history
- **Auditing**: Track state changes over time

## Use Cases

1. **Workflow engines**: Long-running business processes (orders, approvals)
2. **Saga patterns**: Distributed transaction coordination
3. **Session management**: User session state persistence
4. **Process resume**: Continue interrupted processes

## API Design

### Snapshot API

```go
// Snapshot captures the current interpreter state
type Snapshot[C any] struct {
    MachineID   string              `json:"machine_id"`
    Version     string              `json:"version"`      // Machine version for compatibility
    CurrentState StateID            `json:"current_state"`
    Context      C                  `json:"context"`
    History      map[StateID]StateID `json:"history,omitempty"`      // History state memory
    ActiveRegions map[StateID]StateID `json:"active_regions,omitempty"` // Parallel state tracking
    CreatedAt    time.Time          `json:"created_at"`
}

// Interpreter methods
func (i *Interpreter[C]) Snapshot() Snapshot[C]
func (i *Interpreter[C]) Restore(snapshot Snapshot[C]) error
```

### Usage Example

```go
// Save state
snapshot := interp.Snapshot()
data, _ := json.Marshal(snapshot)
db.Save("workflow:123", data)

// Restore state
var snapshot statekit.Snapshot[OrderContext]
json.Unmarshal(db.Get("workflow:123"), &snapshot)
interp.Restore(snapshot)
```

### Persistence Interface

```go
// Persister is implemented by storage backends
type Persister[C any] interface {
    Save(ctx context.Context, id string, snapshot Snapshot[C]) error
    Load(ctx context.Context, id string) (Snapshot[C], error)
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter Filter) ([]string, error)
}

// Filter for listing snapshots
type Filter struct {
    MachineID  string     // Filter by machine type
    CreatedAfter time.Time
    CreatedBefore time.Time
    Limit      int
}
```

### Built-in Persisters

```go
// In-memory (testing)
persist := statekit.NewMemoryPersister[Context]()

// File-based (development)
persist := statekit.NewFilePersister[Context]("/var/lib/statekit")

// SQL (production)
persist := statekit.NewSQLPersister[Context](db, "state_snapshots")
```

## Storage Schema

### SQL Schema

```sql
CREATE TABLE state_snapshots (
    id          VARCHAR(255) PRIMARY KEY,
    machine_id  VARCHAR(255) NOT NULL,
    version     VARCHAR(50) NOT NULL,
    state       VARCHAR(255) NOT NULL,
    context     JSONB NOT NULL,
    history     JSONB,
    regions     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    INDEX idx_machine_id (machine_id),
    INDEX idx_state (state),
    INDEX idx_created_at (created_at)
);
```

### Redis Schema

```
Key: statekit:{machine_id}:{instance_id}
Value: JSON snapshot
TTL: Configurable per machine type
```

## Event Sourcing (Optional)

For full auditability, events can be stored:

```go
// Event log entry
type EventLog struct {
    ID          string    `json:"id"`
    SnapshotID  string    `json:"snapshot_id"`
    Event       Event     `json:"event"`
    FromState   StateID   `json:"from_state"`
    ToState     StateID   `json:"to_state"`
    Timestamp   time.Time `json:"timestamp"`
}

// Event store interface
type EventStore[C any] interface {
    Append(ctx context.Context, id string, log EventLog) error
    GetHistory(ctx context.Context, id string) ([]EventLog, error)
    Replay(ctx context.Context, id string, until time.Time) (*Interpreter[C], error)
}
```

## Version Compatibility

Handle machine definition changes:

```go
// Version check on restore
type Migrator[C any] interface {
    CanMigrate(fromVersion, toVersion string) bool
    Migrate(snapshot Snapshot[C]) (Snapshot[C], error)
}

// Register migrations
machine, _ := statekit.NewMachine[Context]("order").
    WithVersion("2.0").
    WithMigration("1.0", "2.0", func(old Snapshot[Context]) Snapshot[Context] {
        // Handle state renames, context changes, etc.
        return migrated
    }).
    // ... states
    Build()
```

## Implementation Details

### Snapshot Structure

```go
type Snapshot[C any] struct {
    MachineID     string                `json:"machine_id"`
    Version       string                `json:"version"`
    CurrentState  StateID               `json:"current_state"`
    Context       C                     `json:"context"`
    History       map[StateID]StateID   `json:"history,omitempty"`
    ActiveRegions map[StateID]StateID   `json:"active_regions,omitempty"`
    CreatedAt     time.Time             `json:"created_at"`

    // Pending timers (for delayed transitions)
    PendingTimers []PendingTimer        `json:"pending_timers,omitempty"`
}

type PendingTimer struct {
    StateID   StateID       `json:"state_id"`
    Target    StateID       `json:"target"`
    Remaining time.Duration `json:"remaining"`
}
```

### Interpreter Snapshot Implementation

```go
func (i *Interpreter[C]) Snapshot() Snapshot[C] {
    i.mu.RLock()
    defer i.mu.RUnlock()

    return Snapshot[C]{
        MachineID:     i.machine.ID,
        Version:       i.machine.Version,
        CurrentState:  i.currentState,
        Context:       i.context,
        History:       copyMap(i.history),
        ActiveRegions: copyMap(i.activeRegions),
        CreatedAt:     time.Now(),
        PendingTimers: i.snapshotTimers(),
    }
}

func (i *Interpreter[C]) Restore(s Snapshot[C]) error {
    // Validate machine ID
    if s.MachineID != i.machine.ID {
        return fmt.Errorf("machine ID mismatch: %s vs %s", s.MachineID, i.machine.ID)
    }

    // Check version compatibility
    if s.Version != i.machine.Version {
        if !i.machine.CanMigrate(s.Version) {
            return fmt.Errorf("incompatible version: %s", s.Version)
        }
        s = i.machine.Migrate(s)
    }

    // Validate current state exists
    if i.machine.GetState(s.CurrentState) == nil {
        return fmt.Errorf("state %q not found in machine", s.CurrentState)
    }

    i.mu.Lock()
    defer i.mu.Unlock()

    i.currentState = s.CurrentState
    i.context = s.Context
    i.history = copyMap(s.History)
    i.activeRegions = copyMap(s.ActiveRegions)

    // Restore timers
    i.restoreTimers(s.PendingTimers)

    return nil
}
```

## XState Compatibility

XState persists state differently:

```javascript
// XState v5 persisted state
{
  "value": "loading",
  "context": { "count": 5 },
  "historyValue": { "parent": "child1" },
  "status": "active"
}
```

Map to statekit format:

```go
func ToXStateSnapshot(s Snapshot) map[string]any {
    return map[string]any{
        "value":        s.CurrentState,
        "context":      s.Context,
        "historyValue": s.History,
        "status":       "active",
    }
}
```

## Validation Rules

```go
ErrCodeSnapshotInvalidState   = "SNAPSHOT_INVALID_STATE"
ErrCodeSnapshotVersionMismatch = "SNAPSHOT_VERSION_MISMATCH"
ErrCodeSnapshotMachineMismatch = "SNAPSHOT_MACHINE_MISMATCH"
ErrCodeSnapshotCorrupted       = "SNAPSHOT_CORRUPTED"
```

## Thread Safety

- Snapshot creation is thread-safe (read lock)
- Restore is thread-safe (write lock)
- Storage operations use context for cancellation
- Timer restoration respects remaining duration

## Testing Helpers

```go
// Create snapshot for testing
snapshot := statekit.NewTestSnapshot[Context]().
    WithState("processing").
    WithContext(Context{OrderID: "123"}).
    WithHistory("parent", "child2").
    Build()

// Assert snapshot equality
statekit.AssertSnapshotEqual(t, expected, actual)
```

## Implementation Phases

### Phase 1: Core Snapshot
- `Snapshot()` / `Restore()` methods
- JSON serialization
- Basic validation

### Phase 2: Storage Backends
- Memory persister (testing)
- File persister (development)
- SQL persister interface

### Phase 3: Event Sourcing
- Event logging
- Replay functionality
- History queries

### Phase 4: Version Migration
- Version tracking
- Migration registration
- Compatibility checking

## Open Questions

1. **Context serialization**: How to handle non-JSON-serializable contexts?
   - Option A: Require `json.Marshaler` implementation
   - Option B: Provide custom serializer option
   - Option C: Only support basic types

2. **Timer precision**: How precise should timer restoration be?
   - Option A: Exact remaining duration
   - Option B: Round to nearest second
   - Option C: Reset timers on restore

3. **Active services**: Should running services be snapshotted?
   - Option A: Cancel and mark as interrupted
   - Option B: Not supported (document limitation)

4. **Compression**: Should snapshots be compressed?
   - Option A: Optional gzip compression
   - Option B: Leave to storage layer

## Recommendation

Start with **Phase 1: Core Snapshot** as it:
- Enables basic persistence use cases
- Has minimal API surface
- Is testable in isolation
- Provides foundation for advanced features

Storage backends (Phase 2) can be community-contributed or added based on demand.
