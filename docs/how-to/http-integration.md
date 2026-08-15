# HTTP Integration

The `http` package provides framework-agnostic HTTP handlers and middleware for exposing state machines via REST APIs.

## Installation

```go
import statekithttp "go.klarlabs.de/statekit/http"
```

## Quick Start

```go
package main

import (
    "net/http"

    "go.klarlabs.de/statekit"
    statekithttp "go.klarlabs.de/statekit/http"
)

func main() {
    // Create your state machine
    machine, _ := statekit.NewMachine[struct{}]("order").
        WithInitial("pending").
        State("pending").On("SUBMIT").Target("processing").Done().
        State("processing").On("COMPLETE").Target("done").Done().
        State("done").Final().Done().
        Build()

    // Create and start interpreter
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Create HTTP handler
    handler := statekithttp.NewMachineHandler(interp)

    // Mount on your server
    http.Handle("/api/order/", http.StripPrefix("/api/order", handler))
    http.ListenAndServe(":8080", nil)
}
```

## Endpoints

`MachineHandler` provides three endpoints:

### GET /state

Returns the current state of the machine.

**Response:**
```json
{
  "currentState": "pending",
  "done": false,
  "machineId": "order"
}
```

### POST /event

Sends an event to the state machine.

**Request:**
```json
{
  "type": "SUBMIT",
  "payload": {
    "orderId": "12345",
    "items": ["item1", "item2"]
  }
}
```

**Response:**
```json
{
  "previousState": "pending",
  "currentState": "processing",
  "transitioned": true,
  "done": false
}
```

### GET /context

Returns the current machine context.

**Response:**
```json
{
  "orderId": "12345",
  "items": ["item1", "item2"],
  "total": 99.99
}
```

## Machine Registry

Manage multiple state machine instances with `MachineRegistry`:

```go
// Factory function creates new interpreters on demand
factory := func(id string) (*statekit.Interpreter[OrderContext], error) {
    machine, err := BuildOrderMachine()
    if err != nil {
        return nil, err
    }

    interp := statekit.NewInterpreter(machine)
    interp.Start()
    return interp, nil
}

// Create registry
registry := statekithttp.NewMachineRegistry(factory)

// Get or create interpreter by ID
interp, err := registry.Get("order-123")
if err != nil {
    log.Fatal(err)
}

// List all active machine IDs
ids := registry.List()
fmt.Println(ids) // ["order-123", "order-456", ...]

// Remove and stop a machine
registry.Remove("order-123")
```

### Registry with HTTP

```go
func main() {
    registry := statekithttp.NewMachineRegistry(orderFactory)

    http.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
        // Extract order ID from path
        orderID := strings.TrimPrefix(r.URL.Path, "/orders/")
        orderID = strings.Split(orderID, "/")[0]

        // Get or create interpreter
        interp, err := registry.Get(orderID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        // Create handler and serve
        handler := statekithttp.NewMachineHandler(interp)

        // Strip /orders/{id} prefix
        r.URL.Path = strings.TrimPrefix(r.URL.Path, "/orders/"+orderID)
        handler.ServeHTTP(w, r)
    })

    http.ListenAndServe(":8080", nil)
}
```

## Middleware

### MachineMiddleware

Inject a single interpreter into request context:

```go
interp := statekit.NewInterpreter(machine)
interp.Start()

middleware := statekithttp.MachineMiddleware(interp)

// Wrap your handler
http.Handle("/", middleware(myHandler))

// Access in handler
func myHandler(w http.ResponseWriter, r *http.Request) {
    interp, ok := statekithttp.MachineFromContext[MyContext](r.Context())
    if !ok {
        http.Error(w, "no machine in context", 500)
        return
    }

    // Use interpreter
    state := interp.State()
    fmt.Fprintf(w, "Current state: %s", state.Value)
}
```

### RegistryMiddleware

Look up machines by ID from requests:

```go
registry := statekithttp.NewMachineRegistry(factory)

// Define how to extract machine ID from request
idExtractor := func(r *http.Request) string {
    // From query parameter
    return r.URL.Query().Get("machine_id")

    // Or from header
    // return r.Header.Get("X-Machine-ID")

    // Or from path
    // return mux.Vars(r)["id"]
}

middleware := statekithttp.RegistryMiddleware(registry, idExtractor)

http.Handle("/", middleware(myHandler))
```

**Behavior:**
- Returns 400 Bad Request if ID extractor returns empty string
- Returns 500 Internal Server Error if factory fails
- Otherwise, injects interpreter into context

## Helper Functions

### NewServeMux

Create a pre-configured mux with all endpoints:

```go
interp := statekit.NewInterpreter(machine)
interp.Start()

// Creates mux with:
// - GET  /api/machine/state
// - POST /api/machine/event
// - GET  /api/machine/context
mux := statekithttp.NewServeMux(interp, "/api/machine")

http.ListenAndServe(":8080", mux)
```

### Context Helpers

```go
// Add interpreter to context
ctx := statekithttp.WithMachine(r.Context(), interp)

// Retrieve from context
interp, ok := statekithttp.MachineFromContext[MyContext](ctx)
```

## Framework Integration

### Standard Library

```go
mux := http.NewServeMux()
handler := statekithttp.NewMachineHandler(interp)
mux.Handle("/machine/", http.StripPrefix("/machine", handler))
```

### Chi

```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()
handler := statekithttp.NewMachineHandler(interp)

r.Route("/orders/{orderID}", func(r chi.Router) {
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            orderID := chi.URLParam(r, "orderID")
            interp, _ := registry.Get(orderID)
            ctx := statekithttp.WithMachine(r.Context(), interp)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    })
    r.Get("/state", handler.HandleGetState)
    r.Post("/event", handler.HandleSendEvent)
    r.Get("/context", handler.HandleGetContext)
})
```

### Gin

```go
import "github.com/gin-gonic/gin"

r := gin.Default()

r.GET("/machine/state", func(c *gin.Context) {
    handler := statekithttp.NewMachineHandler(interp)
    handler.HandleGetState(c.Writer, c.Request)
})

r.POST("/machine/event", func(c *gin.Context) {
    handler := statekithttp.NewMachineHandler(interp)
    handler.HandleSendEvent(c.Writer, c.Request)
})
```

### Echo

```go
import "github.com/labstack/echo/v4"

e := echo.New()
handler := statekithttp.NewMachineHandler(interp)

e.GET("/machine/state", echo.WrapHandler(http.HandlerFunc(handler.HandleGetState)))
e.POST("/machine/event", echo.WrapHandler(http.HandlerFunc(handler.HandleSendEvent)))
```

### Fiber

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/adaptor"
)

app := fiber.New()
handler := statekithttp.NewMachineHandler(interp)

app.Get("/machine/state", adaptor.HTTPHandlerFunc(handler.HandleGetState))
app.Post("/machine/event", adaptor.HTTPHandlerFunc(handler.HandleSendEvent))
```

## Complete Example

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "strings"

    "go.klarlabs.de/statekit"
    statekithttp "go.klarlabs.de/statekit/http"
)

type OrderContext struct {
    OrderID string   `json:"orderId"`
    Items   []string `json:"items"`
    Status  string   `json:"status"`
}

func main() {
    // Build machine
    machine, _ := statekit.NewMachine[OrderContext]("order").
        WithInitial("pending").
        WithAction("setProcessing", func(ctx *OrderContext, e statekit.Event) {
            ctx.Status = "processing"
        }).
        WithAction("setCompleted", func(ctx *OrderContext, e statekit.Event) {
            ctx.Status = "completed"
        }).
        State("pending").
            On("SUBMIT").Target("processing").Do("setProcessing").
        Done().
        State("processing").
            On("COMPLETE").Target("done").Do("setCompleted").
        Done().
        State("done").Final().Done().
        Build()

    // Create registry
    factory := func(id string) (*statekit.Interpreter[OrderContext], error) {
        interp := statekit.NewInterpreter(machine)
        interp.UpdateContext(func(ctx *OrderContext) {
            ctx.OrderID = id
        })
        interp.Start()
        return interp, nil
    }
    registry := statekithttp.NewMachineRegistry(factory)

    // ID extractor from path: /orders/{id}/...
    idExtractor := func(r *http.Request) string {
        path := strings.TrimPrefix(r.URL.Path, "/orders/")
        parts := strings.Split(path, "/")
        if len(parts) > 0 {
            return parts[0]
        }
        return ""
    }

    // Create middleware and handler
    middleware := statekithttp.RegistryMiddleware(registry, idExtractor)

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        interp, ok := statekithttp.MachineFromContext[OrderContext](r.Context())
        if !ok {
            http.Error(w, "no machine", 500)
            return
        }

        machineHandler := statekithttp.NewMachineHandler(interp)

        // Route to appropriate handler
        path := r.URL.Path
        switch {
        case strings.HasSuffix(path, "/state") && r.Method == "GET":
            machineHandler.HandleGetState(w, r)
        case strings.HasSuffix(path, "/event") && r.Method == "POST":
            machineHandler.HandleSendEvent(w, r)
        case strings.HasSuffix(path, "/context") && r.Method == "GET":
            machineHandler.HandleGetContext(w, r)
        default:
            http.NotFound(w, r)
        }
    })

    // List endpoint
    http.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
        ids := registry.List()
        json.NewEncoder(w).Encode(ids)
    })

    // Machine endpoints
    http.Handle("/orders/", middleware(handler))

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Usage:**

```bash
# Create an order (first request creates the machine)
curl -X POST http://localhost:8080/orders/order-123/event \
  -H "Content-Type: application/json" \
  -d '{"type": "SUBMIT"}'

# Check state
curl http://localhost:8080/orders/order-123/state

# Get context
curl http://localhost:8080/orders/order-123/context

# Complete the order
curl -X POST http://localhost:8080/orders/order-123/event \
  -H "Content-Type: application/json" \
  -d '{"type": "COMPLETE"}'

# List all orders
curl http://localhost:8080/orders
```

## Error Handling

The handler returns appropriate HTTP status codes:

| Scenario | Status Code |
|----------|-------------|
| Success | 200 OK |
| Invalid JSON body | 400 Bad Request |
| Missing event type | 400 Bad Request |
| Machine ID not found (registry) | 400 Bad Request |
| Factory error (registry) | 500 Internal Server Error |
| Unknown endpoint | 404 Not Found |

Error responses are JSON:

```json
{
  "error": "event type is required"
}
```

## Thread Safety

All handlers use `sync.RWMutex` for thread-safe access:

- `HandleGetState` and `HandleGetContext` use read locks
- `HandleSendEvent` uses a write lock
- `MachineRegistry` is fully thread-safe

This allows concurrent reads while ensuring exclusive access during state transitions.
