# Traffic Light Example

A simple finite state machine (FSM) demonstrating the basics of Statekit.

## Features Demonstrated

- Basic state transitions
- Entry actions
- Context management
- Event handling

## State Diagram

```
   ┌─────────┐
   │  green  │◄──────────────┐
   └────┬────┘               │
        │ TIMER              │
        ▼                    │
   ┌─────────┐               │
   │ yellow  │               │
   └────┬────┘               │
        │ TIMER              │ TIMER (incrementCycle)
        ▼                    │
   ┌─────────┐               │
   │   red   │───────────────┘
   └─────────┘
```

## Usage

```go
package main

import (
    "fmt"
    trafficlight "github.com/felixgeelhaar/statekit/examples/traffic_light"
    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine, _ := trafficlight.NewTrafficLight()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    fmt.Println(interp.State().Value) // "green"

    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "yellow"

    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "red"

    interp.Send(statekit.Event{Type: "TIMER"})
    fmt.Println(interp.State().Value) // "green"

    ctx := interp.State().Context
    fmt.Println(ctx.CycleCount) // 1
}
```

## Run Tests

```bash
go test -v ./examples/traffic_light/...
```

## Key Concepts

### Context

The `Context` struct tracks state across transitions:

```go
type Context struct {
    CycleCount int    // Tracks complete cycles
    Log        []string // Entry action logs
}
```

### Entry Actions

Each state logs its entry:

```go
WithAction("logGreen", func(ctx *Context, e statekit.Event) {
    ctx.Log = append(ctx.Log, "Entered GREEN")
})
```

### Transition Actions

The `incrementCycle` action fires when transitioning from red back to green:

```go
State(StateRed).
    On(EventTimer).Target(StateGreen).Do("incrementCycle")
```
