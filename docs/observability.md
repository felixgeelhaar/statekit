# Observability

Statekit provides built-in observability features for production monitoring through the `metrics` and `health` packages.

## Installation

The observability packages are included with statekit:

```bash
go get go.klarlabs.de/statekit
```

Import the packages:

```go
import (
    "go.klarlabs.de/statekit/metrics"
    "go.klarlabs.de/statekit/health"
)
```

## Prometheus Metrics

The `metrics` package provides Prometheus integration for monitoring state machine behavior.

### Quick Start

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/metrics"
)

func main() {
    // Create metrics with default Prometheus registry
    m := metrics.DefaultMetrics()

    // Build your state machine
    machine, _ := statekit.NewMachine[Context]("order").
        WithInitial("pending").
        // ... state definitions
        Build()

    // Create interpreter with metrics wrapper
    interp := statekit.NewInterpreter(machine)
    mi := metrics.NewMetricsInterpreter(interp, "order-123", m)

    // Use the metrics interpreter
    mi.Start()
    mi.Send(statekit.Event{Type: "SUBMIT"})

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":9090", nil)
}
```

### Available Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `statekit_transitions_total` | Counter | machine, from_state, to_state, event | Total state transitions |
| `statekit_events_total` | Counter | machine, event, transitioned | Total events processed |
| `statekit_transition_duration_seconds` | Histogram | machine, from_state, to_state | Transition processing time |
| `statekit_current_state` | Gauge | machine, state | Current state (1 = active) |
| `statekit_errors_total` | Counter | machine, error_type | Errors during event processing |
| `statekit_machines_active` | Gauge | machine | Number of active machines |
| `statekit_machines_completed_total` | Counter | machine, final_state | Machines that reached final state |

### Custom Registry

Use a custom registry for testing or multi-tenant scenarios:

```go
reg := prometheus.NewRegistry()
m := metrics.NewMetrics(reg)

// Create multiple interpreters sharing the same metrics
mi1 := metrics.NewMetricsInterpreter(interp1, "order-1", m)
mi2 := metrics.NewMetricsInterpreter(interp2, "order-2", m)
```

### Error Recording

Record custom errors for monitoring:

```go
mi := metrics.NewMetricsInterpreter(interp, "order-123", m)
mi.Start()

// Record application errors
if err := validatePayment(); err != nil {
    mi.RecordError("payment_validation_failed")
}
```

### Accessing the Underlying Interpreter

```go
mi := metrics.NewMetricsInterpreter(interp, "order-123", m)

// Get the underlying interpreter for advanced operations
underlying := mi.Interpreter()
underlying.UpdateContext(func(ctx *Context) {
    ctx.LastModified = time.Now()
})
```

## Health Checks

The `health` package provides Kubernetes-compatible health probes for state machines.

### Quick Start

```go
import (
    "net/http"
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/health"
)

func main() {
    // Create health checker
    checker := health.NewChecker[Context]()

    // Register interpreters
    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()
    checker.Register("order-processor", interp)

    // Mount HTTP handlers
    http.Handle("/livez", checker.LivenessHandler())
    http.Handle("/readyz", checker.ReadinessHandler())
    http.Handle("/healthz", checker.HealthHandler())

    http.ListenAndServe(":8080", nil)
}
```

### Health Status Types

| Status | HTTP Code | Description |
|--------|-----------|-------------|
| `healthy` | 200 | All checks passed |
| `degraded` | 200 | Some checks failed but system is operational |
| `unhealthy` | 503 | System is not operational |

### Liveness vs Readiness

- **Liveness** (`/livez`): Is the machine alive? Checks that interpreters are initialized and not nil.
- **Readiness** (`/readyz`): Can the machine accept events? Checks that machines are not in final states.
- **Health** (`/healthz`): Combined liveness and readiness check.

### Custom Ready States

Configure which states are considered "ready":

```go
checker := health.NewChecker[Context]()
checker.Register("order-processor", interp)

// Only consider "processing" state as ready
// (useful when machine must be in specific states to accept work)
checker.SetReadyStates("order-processor", "processing", "waiting")
```

### Check Individual Machines

```go
result := checker.CheckMachine("order-processor")
fmt.Println(result.Status)   // healthy, degraded, or unhealthy
fmt.Println(result.Message)  // Human-readable status
fmt.Println(result.Details)  // Map with state, done, ready info
```

### Response Format

Health check responses are JSON:

```json
{
  "status": "healthy",
  "message": "all interpreters ready",
  "details": {
    "order-processor": "processing",
    "payment-handler": "idle"
  }
}
```

### Multiple Machines

```go
checker := health.NewChecker[Context]()

// Register multiple interpreters
checker.Register("orders", orderInterp)
checker.Register("payments", paymentInterp)
checker.Register("inventory", inventoryInterp)

// Readiness checks all registered machines
// - healthy: all ready
// - degraded: some ready, some not
// - unhealthy: none ready
```

### Lifecycle Management

```go
checker := health.NewChecker[Context]()
checker.Register("worker", interp)

// Query registered machines
count := checker.MachineCount()      // 1
ids := checker.MachineIDs()          // ["worker"]

// Unregister when done
checker.Unregister("worker")
```

## Kubernetes Integration

Example Kubernetes deployment with health probes:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
spec:
  template:
    spec:
      containers:
      - name: order-service
        ports:
        - containerPort: 8080
        - containerPort: 9090  # metrics
        livenessProbe:
          httpGet:
            path: /livez
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Combining Metrics and Health

Use both packages together for comprehensive observability:

```go
func main() {
    // Metrics
    m := metrics.DefaultMetrics()

    // Health checker
    checker := health.NewChecker[Context]()

    // Create and register machines
    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)

    // Wrap with metrics
    mi := metrics.NewMetricsInterpreter(interp, "order-1", m)
    mi.Start()

    // Register underlying interpreter with health checker
    checker.Register("order-1", mi.Interpreter())

    // Mount endpoints
    http.Handle("/metrics", promhttp.Handler())
    http.Handle("/livez", checker.LivenessHandler())
    http.Handle("/readyz", checker.ReadinessHandler())

    http.ListenAndServe(":8080", nil)
}
```

## Best Practices

1. **Use meaningful machine IDs**: Include entity IDs (e.g., "order-123") for debugging.

2. **Set appropriate ready states**: Define which states indicate the machine can accept work.

3. **Monitor transition duration**: Use histograms to detect performance degradation.

4. **Track completion rates**: Monitor `machines_completed_total` to ensure workflows complete.

5. **Alert on errors**: Set up alerts on `errors_total` for early problem detection.

6. **Graceful shutdown**: Call `mi.Stop()` when shutting down to update metrics correctly.

```go
// Graceful shutdown
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

go func() {
    <-sigCh
    mi.Stop()  // Updates metrics before shutdown
    checker.Unregister("order-1")
    os.Exit(0)
}()
```
