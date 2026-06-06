# Incident Lifecycle Example

An SRE incident management workflow demonstrating hierarchical states with guards.

## Features Demonstrated

- Hierarchical states for incident phases
- Event bubbling for escalation
- Guards for state-based decisions
- Actions for notifications and logging
- XState export for visualization

## State Hierarchy

```
incident_lifecycle
├── active (compound, initial)
│   ├── triggered (initial)
│   ├── investigating
│   └── resolved
├── closed (final)
└── cancelled (final)
```

## State Diagram

```
                    ┌─────────────────────────────────────────┐
                    │                active                   │
                    │  ┌───────────┐                          │
      ┌─────────────┼──│ triggered │◄─────────────────────┐   │
      │             │  └─────┬─────┘                      │   │
      │             │        │ ACK                        │   │
      │             │        ▼              ESCALATE      │   │
      │             │  ┌───────────────┐──────────────────┘   │
      │             │  │ investigating │◄─────────┐           │
      │             │  └───────┬───────┘          │ REOPEN    │
      │             │          │ RESOLVE          │           │
      │             │          ▼                  │           │
      │  CANCEL     │  ┌──────────────┐           │           │
      │             │  │   resolved   │───────────┘           │
      │             │  └───────┬──────┘                       │
      │             │          │ CLOSE (hasPostmortem guard)  │
      │             └──────────┼──────────────────────────────┘
      │                        │
      │                        ▼
      │                ┌───────────┐
      │                │  closed   │
      │                └───────────┘
      ▼
┌───────────┐
│ cancelled │
└───────────┘
```

## Usage

### Run the Example

```bash
go run ./examples/incident_lifecycle/
```

### Programmatic Usage

```go
package main

import (
    "go.klarlabs.de/statekit"
)

func main() {
    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)

    // Set initial context
    interp.UpdateContext(func(c *IncidentContext) {
        c.IncidentID = "INC-001"
        c.Title = "Database connection pool exhausted"
        c.Severity = "P1"
    })

    interp.Start() // In "triggered" state

    // Escalate before acknowledgment
    interp.Send(statekit.Event{Type: "ESCALATE"})

    // Acknowledge incident
    interp.Send(statekit.Event{
        Type: "ACK",
        Payload: "alice@example.com",
    })

    // Resolve
    interp.Send(statekit.Event{Type: "RESOLVE"})

    // Must schedule postmortem before closing
    interp.Send(statekit.Event{Type: "CLOSE"}) // Blocked!
    interp.Send(statekit.Event{Type: "SCHEDULE_POSTMORTEM"})
    interp.Send(statekit.Event{Type: "CLOSE"}) // Now works
}
```

## Run Tests

```bash
go test -v ./examples/incident_lifecycle/...
```

## Key Concepts

### Event Bubbling

The `CANCEL` event is handled at the `active` parent level, allowing cancellation from any child state:

```go
State("active").
    WithInitial("triggered").
    On("CANCEL").Target("cancelled").End()
```

### Guards

The `CLOSE` transition requires a postmortem to be scheduled:

```go
WithGuard("hasPostmortem", func(ctx IncidentContext, e statekit.Event) bool {
    return ctx.PostmortemID != ""
})

State("resolved").
    On("CLOSE").Target("closed").Guard("hasPostmortem")
```

### Self-Transitions

Escalations are self-transitions that increment the escalation counter:

```go
State("triggered").
    On("ESCALATE").Target("triggered").Do("escalate")
```

### Context Management

```go
type IncidentContext struct {
    IncidentID   string
    Severity     string
    AssignedTo   string
    Escalations  int
    PostmortemID string
    // ... timestamps
}
```

### XState Visualization

Export to JSON and paste at [stately.ai/viz](https://stately.ai/viz):

```go
exporter := export.NewXStateExporter(machine)
json, _ := exporter.ExportJSONIndent("", "  ")
```
