# Session Timeout Example

A session management system demonstrating delayed (timed) transitions.

## Features Demonstrated

- **Automatic transitions** after a duration
- **Timer cancellation** via manual transitions
- **Guards** on delayed transitions
- **Actions** on delayed transitions
- **Multiple delayed transitions** (first matching wins)

## State Diagram

```
                   ACTIVITY
                 ┌──────────┐
                 │          │
                 ▼          │
            ┌─────────┐     │
            │  active │─────┘
            └────┬────┘
                 │
      ┌──────────┼──────────┐
      │          │          │
  LOGOUT    After(warn)  After(warn)
  (notVIP)  + isVIP guard + notVIP guard
      │          │          │
      │          ▼          ▼
      │    ┌──────────┐ ┌─────────┐
      │    │  active  │ │ warning │
      │    │  (VIP    │ └────┬────┘
      │    │ extended)│      │
      │    └──────────┘      │
      │                      │
      │     ┌────────────────┼──────────┐
      │     │                │          │
      │  ACTIVITY/STAY   After(exp)   LOGOUT
      │     │                │          │
      │     └───► active     ▼          ▼
      │                ┌─────────┐ ┌─────────┐
      │                │ expired │ │  ended  │
      │                └─────────┘ └─────────┘
      │                     ▲           ▲
      └─────────────────────┴───────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/session_timeout/
```

### Programmatic Usage

```go
package main

import (
    "time"
    "go.klarlabs.de/statekit"
)

func main() {
    machine, _ := statekit.NewMachine[SessionContext]("session").
        WithInitial("active").
        State("active").
            On("ACTIVITY").Target("active").
            On("LOGOUT").Target("ended").
            After(5 * time.Minute).Target("warning").   // Warn after 5 min
        Done().
        State("warning").
            On("ACTIVITY").Target("active").            // Activity resets
            After(1 * time.Minute).Target("expired").   // Expire after 1 more min
        Done().
        State("expired").Final().Done().
        State("ended").Final().Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // User activity resets the timer
    interp.Send(statekit.Event{Type: "ACTIVITY"})

    // Without activity, session will:
    // 1. Move to "warning" after 5 minutes
    // 2. Move to "expired" after 1 more minute (6 total)
}
```

## Run Tests

```bash
go test -v ./examples/session_timeout/...
```

## Key Concepts

### Defining Delayed Transitions

```go
State("waiting").
    After(5 * time.Second).Target("timeout").
Done()
```

### Multiple Delayed Transitions

First matching transition wins:

```go
State("active").
    After(5 * time.Minute).Target("warning").Guard("notVIP").
    After(5 * time.Minute).Target("active").Guard("isVIP").Do("extend").
Done()
```

### Timer Cancellation

Any transition out of the state cancels pending timers:

```go
State("waiting").
    On("CANCEL").Target("cancelled").       // Manual cancel
    After(10 * time.Second).Target("done"). // Auto-timeout
Done()
```

### Actions on Delayed Transitions

```go
State("waiting").
    After(5 * time.Second).Target("timeout").Do("logTimeout").
Done()
```

### Guards on Delayed Transitions

```go
WithGuard("canTimeout", func(ctx Context, e Event) bool {
    return !ctx.PreventTimeout
}).
State("active").
    After(5 * time.Minute).Target("expired").Guard("canTimeout").
Done()
```

### Common Use Cases

| Scenario | Implementation |
|----------|----------------|
| Session timeout | `After(15*time.Minute).Target("expired")` |
| Auto-save | `After(30*time.Second).Target("saving")` |
| Retry with backoff | Chain states with increasing delays |
| Debounce | Reset timer on each input event |
| Animation timing | Sequence states with precise delays |

## Implementation Notes

- Timers are managed per-interpreter (not global)
- Exiting a state cancels all pending timers for that state
- Multiple `After()` on the same state creates independent timers
- Guards are evaluated when the timer fires, not when set
- `interp.Stop()` cancels all pending timers
