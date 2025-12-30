# Pedestrian Light Example

A pedestrian crossing signal demonstrating hierarchical (nested) states with event bubbling.

## Features Demonstrated

- Compound/nested states
- Event bubbling to parent states
- Proper entry/exit action ordering
- Initial state resolution in nested hierarchies

## State Hierarchy

```
pedestrian_signal
├── active (compound, initial)
│   ├── dont_walk (initial)
│   ├── walk
│   └── countdown (compound)
│       ├── flashing (initial)
│       └── warning
└── maintenance
```

## State Diagram

```
                    ENTER_MAINTENANCE
    ┌──────────────────────────────────────────┐
    │                                          ▼
┌───┴───────────────────────────────────┐  ┌──────────────┐
│              active                   │  │ maintenance  │
│  ┌───────────┐    PEDESTRIAN_BUTTON   │  └──────┬───────┘
│  │ dont_walk │─────────────────────┐  │         │
│  └───────────┘                     ▼  │         │
│        ▲                     ┌──────┐ │         │
│        │ TIMER               │ walk │ │         │
│        │                     └──┬───┘ │         │
│        │                        │     │   EXIT_MAINTENANCE
│        │                  TIMER │     │         │
│        │                        ▼     │         │
│        │              ┌─────────────┐ │         │
│        │              │  countdown  │ │         │
│        │              │  ┌────────┐ │ │         │
│        │              │  │flashing│ │ │         │
│        │              │  └───┬────┘ │ │         │
│        │              │      │TIMER │ │         │
│        │              │      ▼      │ │         │
│        │              │  ┌────────┐ │ │         │
│        └──────────────┼──│warning │ │ │         │
│                       │  └────────┘ │ │         │
│                       └─────────────┘ │         │
└───────────────────────────────────────┘◄────────┘
```

## Usage

```go
package main

import (
    "fmt"
    pedestrianlight "github.com/felixgeelhaar/statekit/examples/pedestrian_light"
    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine, _ := pedestrianlight.NewPedestrianLight()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Initial state is the leaf of the initial hierarchy
    fmt.Println(interp.State().Value)     // "dont_walk"
    fmt.Println(interp.Matches("active")) // true (ancestor match)

    // Pedestrian presses button
    interp.Send(statekit.Event{Type: "PEDESTRIAN_BUTTON"})
    fmt.Println(interp.State().Value) // "walk"

    // Timer expires - enter countdown
    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "flashing"

    // Timer in countdown
    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "warning"

    // Complete cycle
    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "dont_walk"

    // Check crossing count
    fmt.Println(interp.State().Context.CrossingCount) // 1
}
```

## Run Tests

```bash
go test -v ./examples/pedestrian_light/...
```

## Key Concepts

### Event Bubbling

Events that aren't handled by the current state bubble up to ancestors:

```go
// In dont_walk, ENTER_MAINTENANCE bubbles up to "active" parent
interp.Send(statekit.Event{Type: "ENTER_MAINTENANCE"})
// Transitions from active to maintenance
```

### Entry/Exit Ordering

Entry actions execute root → leaf, exit actions execute leaf → root:

```
Exit:  dont_walk → active
Enter: maintenance
```

### Hierarchical Initial States

When entering `active`, the machine automatically descends to its initial leaf state `dont_walk`.

### Context Tracking

```go
type Context struct {
    CrossingCount    int      // Complete crossing cycles
    CountdownSeconds int      // Remaining countdown time
    Log              []string // Action execution log
    InMaintenance    bool     // Maintenance mode flag
}
```
