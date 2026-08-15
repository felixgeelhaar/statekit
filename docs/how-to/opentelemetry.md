# OpenTelemetry Tracing

The `otel` package provides OpenTelemetry tracing integration for state machines, enabling observability of state transitions in distributed systems.

## Installation

```go
import statekotel "go.klarlabs.de/statekit/otel"
```

You'll also need the OpenTelemetry SDK:

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk
```

## Quick Start

```go
package main

import (
    "context"

    "go.klarlabs.de/statekit"
    statekotel "go.klarlabs.de/statekit/otel"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    // Set up OpenTelemetry (example with stdout exporter)
    exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
    tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
    otel.SetTracerProvider(tp)
    defer tp.Shutdown(context.Background())

    // Create your state machine
    machine, _ := statekit.NewMachine[struct{}]("order").
        WithInitial("pending").
        State("pending").On("SUBMIT").Target("processing").Done().
        State("processing").On("COMPLETE").Target("done").Done().
        State("done").Final().Done().
        Build()

    // Wrap interpreter with tracing
    interp := statekit.NewInterpreter(machine)
    ti := statekotel.NewTracingInterpreter(interp, "order-workflow")

    // Start creates a root span
    ctx := ti.Start(context.Background())

    // Each Send creates a child span
    ti.Send(ctx, statekit.Event{Type: "SUBMIT"})
    ti.Send(ctx, statekit.Event{Type: "COMPLETE"})

    // Stop ends the root span
    ti.Stop()
}
```

## TracingInterpreter

`TracingInterpreter` wraps a standard interpreter and automatically creates spans for state machine operations.

### Creation

```go
interp := statekit.NewInterpreter(machine)

// Basic usage - uses global tracer
ti := statekotel.NewTracingInterpreter(interp, "my-machine")

// With custom tracer
tracer := otel.Tracer("my-service")
ti := statekotel.NewTracingInterpreter(interp, "my-machine",
    statekotel.WithTracer[MyContext](tracer),
)
```

### Methods

| Method | Description |
|--------|-------------|
| `Start(ctx)` | Starts interpreter and creates root span |
| `Send(ctx, event)` | Processes event with a child span |
| `SendAll(ctx, events)` | Processes multiple events, each with a span |
| `State()` | Returns current state |
| `Context()` | Returns current machine context |
| `Done()` | Returns true if in final state |
| `Matches(id)` | Checks if current state matches ID |
| `Stop()` | Stops interpreter and ends root span |
| `Interpreter()` | Returns the underlying interpreter |

### Example with All Methods

```go
ti := statekotel.NewTracingInterpreter(interp, "workflow")

// Start creates root span "statemachine/workflow"
ctx := ti.Start(context.Background())

// Check current state
if ti.Matches("pending") {
    fmt.Println("Waiting for submission")
}

// Send events (each creates a span)
ti.Send(ctx, statekit.Event{Type: "SUBMIT"})

// Send multiple events
ti.SendAll(ctx, []statekit.Event{
    {Type: "VALIDATE"},
    {Type: "PROCESS"},
})

// Check completion
if ti.Done() {
    fmt.Println("Workflow completed")
}

// Get context data
machineCtx := ti.Context()

// Access underlying interpreter if needed
underlying := ti.Interpreter()

// Stop ends root span
ti.Stop()
```

## Span Structure

### Root Span

Created by `Start()`:

```
statemachine/{machineID}
├── Attributes:
│   └── statekit.machine.id: "order-workflow"
└── Events:
    └── state.entered (statekit.state.id: "pending")
```

### Event Spans

Created by `Send()`:

```
event/{eventType}
├── Attributes:
│   ├── statekit.machine.id: "order-workflow"
│   ├── statekit.event.type: "SUBMIT"
│   ├── statekit.state.before: "pending"
│   ├── statekit.state.after: "processing"
│   └── statekit.transitioned: true
└── Events:
    └── state.transition (from: "pending", to: "processing")
```

### Final State

When reaching a final state:

```
event/COMPLETE
├── Attributes: ...
└── Events:
    ├── state.transition (from: "processing", to: "done")
    └── state.final (statekit.state.id: "done")
```

### Root Span Completion

When `Stop()` is called:

```
statemachine/order-workflow
├── Attributes:
│   ├── statekit.machine.id: "order-workflow"
│   ├── statekit.state.final: "done"
│   └── statekit.completed: true
└── Status: OK (if completed)
```

## TracingHook

For simpler integration without wrapping the interpreter:

```go
tracer := otel.Tracer("my-service")
hook := statekotel.TracingHook(tracer)

// Call after each transition
interp.Send(event)
stateBefore := "pending"
stateAfter := string(interp.State().Value)

hook(ctx, "my-machine", event, stateBefore, stateAfter)
```

The hook creates a span:

```
statemachine/{machineID}/event/{eventType}
├── Attributes:
│   ├── statekit.machine.id: "my-machine"
│   ├── statekit.event.type: "SUBMIT"
│   ├── statekit.state.before: "pending"
│   ├── statekit.state.after: "processing"
│   └── statekit.transitioned: true
```

## Attribute Helpers

Create OpenTelemetry attributes for custom spans:

```go
import "go.opentelemetry.io/otel/attribute"

// State attributes
attrs := statekotel.StateAttributes("machine-1", "running")
// Returns:
// - statekit.machine.id: "machine-1"
// - statekit.state.id: "running"

// Event attributes
attrs := statekotel.EventAttributes("machine-1", event)
// Returns:
// - statekit.machine.id: "machine-1"
// - statekit.event.type: "SUBMIT"

// Transition attributes
attrs := statekotel.TransitionAttributes("machine-1", event, "idle", "running")
// Returns:
// - statekit.machine.id: "machine-1"
// - statekit.event.type: "SUBMIT"
// - statekit.state.from: "idle"
// - statekit.state.to: "running"
// - statekit.transitioned: true
```

### Custom Span Example

```go
tracer := otel.Tracer("my-service")

ctx, span := tracer.Start(ctx, "process-order",
    trace.WithAttributes(statekotel.StateAttributes("order-123", "processing")...),
)
defer span.End()

// Do work...
```

## Integration Examples

### With HTTP Handler

```go
import (
    "net/http"

    statekotel "go.klarlabs.de/statekit/otel"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
)

func handler(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from incoming request
    ctx := otel.GetTextMapPropagator().Extract(
        r.Context(),
        propagation.HeaderCarrier(r.Header),
    )

    // Create tracing interpreter
    ti := statekotel.NewTracingInterpreter(interp, "order")
    ti.Start(ctx) // Links to incoming trace

    // Process...
    ti.Send(ctx, statekit.Event{Type: "SUBMIT"})

    ti.Stop()
}
```

### With gRPC

```go
import (
    "context"

    statekotel "go.klarlabs.de/statekit/otel"
    "go.opentelemetry.io/otel"
)

func (s *server) ProcessOrder(ctx context.Context, req *pb.OrderRequest) (*pb.OrderResponse, error) {
    // ctx already has trace context from gRPC interceptor
    ti := statekotel.NewTracingInterpreter(interp, "order")
    ti.Start(ctx)
    defer ti.Stop()

    ti.Send(ctx, statekit.Event{Type: "SUBMIT", Payload: req})

    return &pb.OrderResponse{State: string(ti.State().Value)}, nil
}
```

### With Jaeger

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("my-service"),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### With OTLP Exporter

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(ctx context.Context) (*trace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

## Complete Example

```go
package main

import (
    "context"
    "log"
    "time"

    "go.klarlabs.de/statekit"
    statekotel "go.klarlabs.de/statekit/otel"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

type OrderContext struct {
    OrderID string
    Total   float64
}

func main() {
    // Initialize tracer
    exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
    tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
    otel.SetTracerProvider(tp)
    defer tp.Shutdown(context.Background())

    // Build state machine
    machine, _ := statekit.NewMachine[OrderContext]("order").
        WithInitial("pending").
        WithAction("logTransition", func(ctx *OrderContext, e statekit.Event) {
            log.Printf("Order %s: processing event %s", ctx.OrderID, e.Type)
        }).
        State("pending").
            On("SUBMIT").Target("validating").Do("logTransition").
        Done().
        State("validating").
            On("VALID").Target("processing").Do("logTransition").
            On("INVALID").Target("pending").Do("logTransition").
        Done().
        State("processing").
            On("SHIP").Target("shipped").Do("logTransition").
        Done().
        State("shipped").
            On("DELIVER").Target("delivered").Do("logTransition").
        Done().
        State("delivered").Final().Done().
        Build()

    // Create interpreter with initial context
    interp := statekit.NewInterpreter(machine)
    interp.UpdateContext(func(ctx *OrderContext) {
        ctx.OrderID = "ORD-12345"
        ctx.Total = 99.99
    })

    // Wrap with tracing
    ti := statekotel.NewTracingInterpreter(interp, "order-ORD-12345")

    // Start (creates root span)
    ctx := ti.Start(context.Background())

    // Simulate order lifecycle
    events := []statekit.EventType{
        "SUBMIT",
        "VALID",
        "SHIP",
        "DELIVER",
    }

    for _, eventType := range events {
        time.Sleep(100 * time.Millisecond) // Simulate processing time
        ti.Send(ctx, statekit.Event{Type: eventType})
        log.Printf("State: %s, Done: %v", ti.State().Value, ti.Done())
    }

    // Stop (ends root span)
    ti.Stop()
}
```

**Output traces show:**

```
{
    "Name": "statemachine/order-ORD-12345",
    "SpanContext": { ... },
    "Attributes": [
        { "Key": "statekit.machine.id", "Value": "order-ORD-12345" }
    ],
    "Events": [
        { "Name": "state.entered", "Attributes": [...] }
    ],
    "ChildSpanCount": 4,
    ...
}

{
    "Name": "event/SUBMIT",
    "ParentSpanId": "...",
    "Attributes": [
        { "Key": "statekit.machine.id", "Value": "order-ORD-12345" },
        { "Key": "statekit.event.type", "Value": "SUBMIT" },
        { "Key": "statekit.state.before", "Value": "pending" },
        { "Key": "statekit.state.after", "Value": "validating" },
        { "Key": "statekit.transitioned", "Value": true }
    ],
    ...
}
```

## Semantic Conventions

The package follows OpenTelemetry semantic conventions where applicable:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `statekit.machine.id` | Unique machine identifier | `"order-123"` |
| `statekit.event.type` | Type of event sent | `"SUBMIT"` |
| `statekit.state.id` | Current state ID | `"processing"` |
| `statekit.state.before` | State before transition | `"pending"` |
| `statekit.state.after` | State after transition | `"processing"` |
| `statekit.state.from` | Source state (in events) | `"pending"` |
| `statekit.state.to` | Target state (in events) | `"processing"` |
| `statekit.state.final` | Final state reached | `"completed"` |
| `statekit.transitioned` | Whether state changed | `true` |
| `statekit.completed` | Machine reached final state | `true` |

## Best Practices

1. **Use meaningful machine IDs** - Include entity IDs for correlation: `"order-12345"`, `"user-session-abc"`

2. **Propagate context** - Pass the context from `Start()` to `Send()` for proper parent-child relationships

3. **Always call Stop()** - Ensures the root span is properly ended and flushed

4. **Use with distributed tracing** - Extract/inject trace context in HTTP headers for end-to-end visibility

5. **Set up sampling** - In production, use appropriate sampling to control trace volume:

```go
tp := trace.NewTracerProvider(
    trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
    trace.WithBatcher(exporter),
)
```
