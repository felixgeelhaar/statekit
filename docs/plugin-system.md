# Plugin System

The plugin system allows you to extend interpreter behavior with lifecycle hooks without modifying core code.

## Overview

Plugins implement one or more hook interfaces to receive callbacks at specific points in the interpreter lifecycle:

- **Interpreter lifecycle**: `OnStart`, `OnStop`
- **Event processing**: `OnEvent` (can modify events)
- **State transitions**: `OnEnter`, `OnExit`, `BeforeTransition`, `AfterTransition`
- **Action execution**: `BeforeAction`, `AfterAction`
- **Error handling**: `OnError`

## Basic Usage

```go
import "github.com/felixgeelhaar/statekit/plugin"

// Create a plugin by implementing the hooks you need
type LoggingPlugin[C any] struct {
    logger *slog.Logger
}

func (p *LoggingPlugin[C]) Name() string {
    return "logging"
}

func (p *LoggingPlugin[C]) OnStart(ctx plugin.Context[C]) {
    p.logger.Info("interpreter started")
}

func (p *LoggingPlugin[C]) OnStop(ctx plugin.Context[C]) {
    p.logger.Info("interpreter stopped")
}

func (p *LoggingPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
    p.logger.Info("entered state", "state", state)
}

func (p *LoggingPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
    p.logger.Info("exited state", "state", state)
}

// Register with interpreter
interp := statekit.NewInterpreter(machine)
interp.Use(&LoggingPlugin[MyContext]{logger: slog.Default()})
interp.Start()
```

## Hook Interfaces

### Core Plugin Interface

All plugins must implement the base `Plugin` interface:

```go
type Plugin[C any] interface {
    Name() string
}
```

### Lifecycle Hooks

```go
type OnStartPlugin[C any] interface {
    Plugin[C]
    OnStart(ctx Context[C])
}

type OnStopPlugin[C any] interface {
    Plugin[C]
    OnStop(ctx Context[C])
}
```

### Event Hook

The event hook can intercept and modify events before processing:

```go
type OnEventPlugin[C any] interface {
    Plugin[C]
    OnEvent(ctx Context[C], event Event) Event
}
```

Example - adding metadata to events:

```go
func (p *AuditPlugin[C]) OnEvent(ctx plugin.Context[C], event plugin.Event) plugin.Event {
    event.Payload = map[string]any{
        "original": event.Payload,
        "timestamp": time.Now(),
        "user": ctx.MachineContext.UserID,
    }
    return event
}
```

### State Hooks

```go
type OnStatePlugin[C any] interface {
    Plugin[C]
    OnEnter(ctx Context[C], state StateID)
    OnExit(ctx Context[C], state StateID)
}
```

### Transition Hooks

```go
type OnTransitionPlugin[C any] interface {
    Plugin[C]
    BeforeTransition(ctx Context[C], from, to StateID, event Event)
    AfterTransition(ctx Context[C], from, to StateID, event Event)
}
```

### Action Hooks

```go
type OnActionPlugin[C any] interface {
    Plugin[C]
    BeforeAction(ctx Context[C], action ActionType, event Event)
    AfterAction(ctx Context[C], action ActionType, event Event)
}
```

### Error Hook

```go
type OnErrorPlugin[C any] interface {
    Plugin[C]
    OnError(ctx Context[C], err error)
}
```

## Plugin Context

All hooks receive a `plugin.Context[C]` with access to interpreter state:

```go
type Context[C any] struct {
    MachineID      string
    MachineContext C
    CurrentState   StateID
}
```

## Combining Plugins

Use `Composite` to combine multiple plugins:

```go
loggingPlugin := &LoggingPlugin[MyContext]{}
metricsPlugin := &MetricsPlugin[MyContext]{}
auditPlugin := &AuditPlugin[MyContext]{}

composite := plugin.NewComposite[MyContext](loggingPlugin, metricsPlugin, auditPlugin)
interp.Use(composite)
```

Plugins are called in registration order.

## Multiple Plugins

You can also register plugins individually:

```go
interp := statekit.NewInterpreter(machine)
interp.Use(loggingPlugin)
interp.Use(metricsPlugin)
interp.Use(auditPlugin)
```

## Example: Metrics Plugin

```go
type MetricsPlugin[C any] struct {
    transitionCounter prometheus.Counter
    stateGauge        prometheus.GaugeVec
}

func (p *MetricsPlugin[C]) Name() string {
    return "metrics"
}

func (p *MetricsPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
    p.transitionCounter.Inc()
    p.stateGauge.WithLabelValues(string(to)).Set(1)
    p.stateGauge.WithLabelValues(string(from)).Set(0)
}
```

## Example: Audit Trail Plugin

```go
type AuditPlugin[C any] struct {
    store AuditStore
}

func (p *AuditPlugin[C]) Name() string {
    return "audit"
}

func (p *AuditPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
    p.store.Record(AuditEntry{
        Timestamp: time.Now(),
        MachineID: ctx.MachineID,
        From:      string(from),
        To:        string(to),
        Event:     string(event.Type),
    })
}

func (p *AuditPlugin[C]) OnError(ctx plugin.Context[C], err error) {
    p.store.RecordError(ctx.MachineID, err)
}
```

## Error Handling

Plugin errors are isolated from core interpreter execution:

- Plugin panics are recovered and don't crash the interpreter
- Errors in plugins are logged but don't affect state transitions
- Use `OnError` hook to capture and handle errors from actions

## Best Practices

1. **Keep plugins focused**: Each plugin should have a single responsibility
2. **Avoid side effects in OnEvent**: Modifying events can make debugging harder
3. **Use composite for organization**: Group related plugins together
4. **Handle errors gracefully**: Plugins should not throw panics
5. **Be mindful of performance**: Hooks are called synchronously
