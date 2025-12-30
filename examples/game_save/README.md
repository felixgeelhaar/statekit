# Game Save Example

A game save/load system demonstrating snapshot persistence.

## Features Demonstrated

- **Snapshot capture** - `Snapshot()` to capture full interpreter state
- **State restoration** - `Restore()` to restore from snapshot
- **JSON serialization** - Snapshots as portable JSON
- **Context preservation** - Game data persists across save/load
- **History preservation** - History states work after restore

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Game Session 1                            │
│  ┌──────────────┐    ┌───────────────┐                      │
│  │  Interpreter │───►│   Snapshot()   │                      │
│  └──────────────┘    └───────┬───────┘                      │
│                              │                               │
│                              ▼                               │
│                     ┌───────────────┐                       │
│                     │  JSON Export   │                       │
│                     └───────┬───────┘                       │
│                              │                               │
└──────────────────────────────┼───────────────────────────────┘
                               │
                        Save to File
                               │
┌──────────────────────────────┼───────────────────────────────┐
│                    Game Session 2                            │
│                              │                               │
│                     ┌───────▼───────┐                       │
│                     │  JSON Import   │                       │
│                     └───────┬───────┘                       │
│                              │                               │
│  ┌──────────────┐    ┌──────▼────────┐                      │
│  │  Interpreter │◄───│   Restore()   │                      │
│  └──────────────┘    └───────────────┘                      │
│                                                              │
│         Continue from exact saved state                     │
└──────────────────────────────────────────────────────────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/game_save/
```

### Programmatic Usage

```go
package main

import (
    "encoding/json"
    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine := buildGameMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Play the game...
    interp.Send(statekit.Event{Type: "START"})
    interp.Send(statekit.Event{Type: "PLAY"})

    // Save game
    snapshot := interp.Snapshot()
    saveData, _ := json.Marshal(snapshot)
    // Write saveData to file...

    // Later: Load game
    var loadedSnapshot statekit.Snapshot[GameContext]
    json.Unmarshal(saveData, &loadedSnapshot)

    newInterp := statekit.NewInterpreter(machine)
    newInterp.Restore(loadedSnapshot)

    // Continue playing from saved state
    newInterp.Send(statekit.Event{Type: "CONTINUE"})
}
```

## Run Tests

```bash
go test -v ./examples/game_save/...
```

## Snapshot Contents

```go
type Snapshot[C any] struct {
    MachineID        string              // Machine identifier
    Version          string              // Optional version for compatibility
    CurrentState     StateID             // Current leaf state
    Context          C                   // User-defined context (game state)
    ShallowHistory   map[StateID]StateID // Last active child per compound
    DeepHistory      map[StateID]StateID // Last active leaf per compound
    ActiveInParallel map[StateID]StateID // Region states if parallel
    CurrentParallel  StateID             // Parallel state ID if applicable
    PendingTimers    []PendingTimer      // Active delayed transitions
    CreatedAt        time.Time           // Snapshot timestamp
}
```

## Key Concepts

### Creating Snapshots

```go
// Capture current state
snapshot := interp.Snapshot()

// Serialize to JSON
data, _ := json.Marshal(snapshot)

// Or with pretty printing
data, _ := json.MarshalIndent(snapshot, "", "  ")
```

### Restoring from Snapshots

```go
// Deserialize from JSON
var snapshot statekit.Snapshot[MyContext]
json.Unmarshal(data, &snapshot)

// Restore interpreter state
newInterp := statekit.NewInterpreter(machine)
err := newInterp.Restore(snapshot)
if err != nil {
    // Handle restore error (machine mismatch, invalid state, etc.)
}
```

### Validation

Restore validates:
- Machine ID matches
- Current state exists in machine
- Parallel regions are valid
- History targets exist

```go
err := interp.Restore(snapshot)
if restoreErr, ok := err.(*statekit.RestoreError); ok {
    switch restoreErr.Code {
    case statekit.ErrCodeSnapshotMachineMismatch:
        // Wrong machine type
    case statekit.ErrCodeSnapshotInvalidState:
        // State doesn't exist (schema changed?)
    }
}
```

## What Gets Preserved

| Data | Preserved | Notes |
|------|-----------|-------|
| Current state | ✅ | Exact leaf state |
| Context data | ✅ | Full user-defined context |
| Shallow history | ✅ | Last child of each compound |
| Deep history | ✅ | Last leaf of each compound |
| Parallel regions | ✅ | All active region states |
| Pending timers | ✅ | Approximate remaining time |
| Actor metadata | ✅ | IDs and config (not state) |

## What Requires Manual Handling

| Data | Notes |
|------|-------|
| Actor internal state | Respawn actors manually |
| Running goroutines | Restart async operations |
| External connections | Re-establish connections |
| Time-sensitive data | Update stale timestamps |

## Best Practices

1. **Version your schemas** - Include version in Context for migration
2. **Validate on load** - Check data integrity after restore
3. **Handle missing actors** - Respawn child actors manually
4. **Update timestamps** - Refresh time-sensitive context data
5. **Test migrations** - Ensure old saves work with new code
