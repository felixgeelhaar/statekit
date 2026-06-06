# Logging Plugin Example

A comprehensive example demonstrating the plugin system for extending interpreter behavior.

## Features Demonstrated

- **Logging plugin** - Observes all state machine events
- **Metrics plugin** - Tracks execution statistics
- **Event transformer** - Modifies events before processing
- **Multiple plugins** - Combining multiple plugins
- **All hook types** - OnStart, OnStop, OnEnter, OnExit, OnEvent, etc.

## Plugin Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Interpreter                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Plugin Chain                        │  │
│  │  ┌─────────┐  ┌─────────┐  ┌────────────────────────┐  │  │
│  │  │ Logging │  │ Metrics │  │ Event Transformer      │  │  │
│  │  └────┬────┘  └────┬────┘  └───────────┬────────────┘  │  │
│  │       │            │                    │              │  │
│  │       ▼            ▼                    ▼              │  │
│  │   OnEnter      transitionCount      OnEvent           │  │
│  │   OnExit       actionCount          (modifies)        │  │
│  │   ...          stateTime            ...               │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                  │
│                           ▼                                  │
│                    State Machine                            │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/logging_plugin/
```

### Programmatic Usage

```go
package main

import (
    "fmt"
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/plugin"
)

// Define a custom plugin
type MyPlugin[C any] struct{}

func (p *MyPlugin[C]) Name() string { return "my-plugin" }

func (p *MyPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
    fmt.Printf("Entered: %s\n", state)
}

func (p *MyPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
    fmt.Printf("Exited: %s\n", state)
}

func main() {
    machine, _ := statekit.NewMachine[MyContext]("example").
        WithInitial("idle").
        State("idle").On("GO").Target("running").Done().
        State("running").Done().
        Build()

    interp := statekit.NewInterpreter(machine)

    // Register plugin
    interp.Use(&MyPlugin[MyContext]{})

    interp.Start()
    interp.Send(statekit.Event{Type: "GO"})
    interp.Stop()
}
```

## Run Tests

```bash
go test -v ./examples/logging_plugin/...
```

## Available Hook Interfaces

### OnStartStopHook

Called when interpreter starts and stops:

```go
type OnStartStopHook[C any] interface {
    Plugin[C]
    OnStart(ctx plugin.Context[C])
    OnStop(ctx plugin.Context[C])
}
```

### OnStateHook

Called when entering and exiting states:

```go
type OnStateHook[C any] interface {
    Plugin[C]
    OnEnter(ctx plugin.Context[C], state plugin.StateID)
    OnExit(ctx plugin.Context[C], state plugin.StateID)
}
```

### OnEventHook

Called when events are received (can modify events):

```go
type OnEventHook[C any] interface {
    Plugin[C]
    // Return modified event for processing
    OnEvent(ctx plugin.Context[C], event plugin.Event) plugin.Event
}
```

### OnTransitionHook

Called before and after transitions:

```go
type OnTransitionHook[C any] interface {
    Plugin[C]
    BeforeTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event)
    AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event)
}
```

### OnActionHook

Called before and after action execution:

```go
type OnActionHook[C any] interface {
    Plugin[C]
    BeforeAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event)
    AfterAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event)
}
```

### OnErrorHook

Called when errors occur:

```go
type OnErrorHook[C any] interface {
    Plugin[C]
    OnError(ctx plugin.Context[C], err error)
}
```

## Plugin Context

Every hook receives a `plugin.Context[C]` with:

```go
type Context[C any] struct {
    MachineID    string    // Machine identifier
    CurrentState StateID   // Current state
    Context      C         // Machine context (read-only)
}
```

## Common Plugin Patterns

### Logging Plugin

```go
func (p *LoggingPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
    log.Printf("[%s] Entered: %s", ctx.MachineID, state)
}
```

### Metrics Plugin

```go
func (p *MetricsPlugin[C]) BeforeTransition(...) {
    p.transitionCount++
    p.histogram.Observe(time.Since(p.lastTransition).Seconds())
}
```

### Validation Plugin

```go
func (p *ValidationPlugin[C]) OnEvent(ctx plugin.Context[C], event plugin.Event) plugin.Event {
    if !isValid(event) {
        event.Type = "INVALID" // Transform to invalid event
    }
    return event
}
```

### Audit Plugin

```go
func (p *AuditPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to StateID, event Event) {
    p.auditLog.Record(AuditEntry{
        From:      from,
        To:        to,
        Event:     event.Type,
        Timestamp: time.Now(),
        UserID:    ctx.Context.UserID,
    })
}
```

## Best Practices

1. **Keep plugins focused** - Each plugin should have a single responsibility
2. **Avoid side effects in OnEvent** - If modifying events, document clearly
3. **Handle errors gracefully** - Plugin errors don't stop execution
4. **Use meaningful names** - `Name()` is used for debugging
5. **Consider performance** - Plugins run on every hook call
