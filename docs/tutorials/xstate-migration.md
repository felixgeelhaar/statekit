# Migrating from XState to Statekit

This guide helps developers familiar with [XState](https://stately.ai/docs/xstate) transition to Statekit. Both libraries implement the statechart specification, but Statekit is designed specifically for Go backends.

## Why Migrate?

| XState | Statekit |
|--------|----------|
| TypeScript/JavaScript | Go |
| Frontend/Node.js focused | Backend focused |
| Runtime-heavy | Compile-time validation |
| Actor model built-in | Explicit actor spawning |
| Dynamic interpretation | Static machine definition |

Statekit provides built-in visualization tools (`statekit viz`), removing the dependency on external services.

## Quick Reference

### Basic Machine Definition

**XState (TypeScript):**
```typescript
import { createMachine, interpret } from 'xstate';

const toggleMachine = createMachine({
  id: 'toggle',
  initial: 'inactive',
  states: {
    inactive: {
      on: { TOGGLE: 'active' }
    },
    active: {
      on: { TOGGLE: 'inactive' }
    }
  }
});

const service = interpret(toggleMachine).start();
service.send({ type: 'TOGGLE' });
console.log(service.getSnapshot().value); // 'active'
```

**Statekit (Go):**
```go
import "go.klarlabs.de/statekit"

type Context struct{}

machine, _ := statekit.NewMachine[Context]("toggle").
    WithInitial("inactive").
    State("inactive").
        On("TOGGLE").Target("active").
    Done().
    State("active").
        On("TOGGLE").Target("inactive").
    Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
interp.Send(statekit.Event{Type: "TOGGLE"})
fmt.Println(interp.State().Value) // "active"
```

### Concept Mapping

| XState Concept | Statekit Equivalent |
|----------------|---------------------|
| `createMachine()` | `statekit.NewMachine[C]()` |
| `interpret()` | `statekit.NewInterpreter()` |
| `service.start()` | `interp.Start()` |
| `service.send()` | `interp.Send()` |
| `service.getSnapshot()` | `interp.State()` |
| `service.stop()` | `interp.Stop()` |
| `state.matches()` | `interp.Matches()` |
| `state.done` | `interp.Done()` |

---

## Context

### XState Context

```typescript
const machine = createMachine({
  id: 'counter',
  initial: 'active',
  context: {
    count: 0,
    user: null
  },
  states: {
    active: {
      on: {
        INCREMENT: {
          actions: assign({ count: (ctx) => ctx.count + 1 })
        }
      }
    }
  }
});
```

### Statekit Context

Context is a type parameter, providing compile-time type safety:

```go
type CounterContext struct {
    Count int
    User  *User
}

machine, _ := statekit.NewMachine[CounterContext]("counter").
    WithInitial("active").
    WithContext(CounterContext{Count: 0}).  // Optional initial context
    WithAction("increment", func(ctx *CounterContext, e statekit.Event) {
        ctx.Count++
    }).
    State("active").
        On("INCREMENT").Target("active").Do("increment").
    Done().
    Build()

// Access context
state := interp.State()
fmt.Println(state.Context.Count)

// Update context directly
interp.UpdateContext(func(ctx *CounterContext) {
    ctx.User = &User{Name: "Alice"}
})
```

**Key difference:** Statekit passes `*Context` to actions (mutable pointer), not a copy.

---

## Actions

### XState Actions

```typescript
const machine = createMachine({
  id: 'order',
  initial: 'pending',
  context: { items: [] },
  states: {
    pending: {
      entry: 'logEntry',
      exit: 'logExit',
      on: {
        SUBMIT: {
          target: 'processing',
          actions: ['validateOrder', 'notifyUser']
        }
      }
    },
    processing: { /* ... */ }
  }
}, {
  actions: {
    logEntry: (ctx, event) => console.log('Entered pending'),
    logExit: (ctx, event) => console.log('Exited pending'),
    validateOrder: (ctx, event) => { /* ... */ },
    notifyUser: (ctx, event) => { /* ... */ }
  }
});
```

### Statekit Actions

```go
machine, _ := statekit.NewMachine[OrderContext]("order").
    WithInitial("pending").
    // Register actions on the machine builder
    WithAction("logEntry", func(ctx *OrderContext, e statekit.Event) {
        fmt.Println("Entered pending")
    }).
    WithAction("logExit", func(ctx *OrderContext, e statekit.Event) {
        fmt.Println("Exited pending")
    }).
    WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {
        // Validation logic
    }).
    WithAction("notifyUser", func(ctx *OrderContext, e statekit.Event) {
        // Notification logic
    }).
    State("pending").
        OnEntry("logEntry").
        OnExit("logExit").
        On("SUBMIT").Target("processing").Do("validateOrder").Do("notifyUser").
    Done().
    State("processing").
        // ...
    Done().
    Build()
```

**Key differences:**
- Actions are registered on the machine builder with `WithAction()`
- Action names are strings, implementations are Go functions
- Chain multiple actions: `.Do("action1").Do("action2")`

---

## Guards

### XState Guards

```typescript
const machine = createMachine({
  id: 'payment',
  initial: 'idle',
  context: { balance: 100, amount: 0 },
  states: {
    idle: {
      on: {
        PAY: [
          { target: 'success', guard: 'hasEnoughBalance' },
          { target: 'failed' }
        ]
      }
    },
    success: { type: 'final' },
    failed: { type: 'final' }
  }
}, {
  guards: {
    hasEnoughBalance: (ctx) => ctx.balance >= ctx.amount
  }
});
```

### Statekit Guards

```go
machine, _ := statekit.NewMachine[PaymentContext]("payment").
    WithInitial("idle").
    WithGuard("hasEnoughBalance", func(ctx PaymentContext, e statekit.Event) bool {
        return ctx.Balance >= ctx.Amount
    }).
    State("idle").
        On("PAY").Target("success").Guard("hasEnoughBalance").
        On("PAY").Target("failed").  // Fallback (no guard = always true)
    Done().
    State("success").Final().Done().
    State("failed").Final().Done().
    Build()
```

**Key differences:**
- Guards receive `Context` by value (not pointer) - they're read-only
- Guard ordering matters - first matching transition wins
- Fallback transitions don't need explicit guard

---

## Hierarchical (Nested) States

### XState Nested States

```typescript
const machine = createMachine({
  id: 'app',
  initial: 'authenticated',
  states: {
    authenticated: {
      initial: 'dashboard',
      on: {
        LOGOUT: 'unauthenticated'  // Applies to all child states
      },
      states: {
        dashboard: {
          on: { VIEW_SETTINGS: 'settings' }
        },
        settings: {
          on: { BACK: 'dashboard' }
        }
      }
    },
    unauthenticated: {
      on: { LOGIN: 'authenticated' }
    }
  }
});
```

### Statekit Nested States

```go
machine, _ := statekit.NewMachine[Context]("app").
    WithInitial("authenticated").
    State("authenticated").
        WithInitial("dashboard").
        On("LOGOUT").Target("unauthenticated").End().  // Parent-level transition
        State("dashboard").
            On("VIEW_SETTINGS").Target("settings").
        End().End().  // End transition, End child state
        State("settings").
            On("BACK").Target("dashboard").
        End().End().
    Done().  // Complete parent compound state
    State("unauthenticated").
        On("LOGIN").Target("authenticated").
    Done().
    Build()
```

**Builder pattern:**
- `.End()` returns to the parent builder (StateBuilder or TransitionBuilder)
- `.Done()` completes a top-level state and returns to MachineBuilder
- For child states: `State("child").On("X").Target("y").End().End()` - first End() ends transition, second ends child state

---

## Parallel States

### XState Parallel States

```typescript
const machine = createMachine({
  id: 'editor',
  type: 'parallel',
  states: {
    bold: {
      initial: 'off',
      states: {
        off: { on: { TOGGLE_BOLD: 'on' } },
        on: { on: { TOGGLE_BOLD: 'off' } }
      }
    },
    italic: {
      initial: 'off',
      states: {
        off: { on: { TOGGLE_ITALIC: 'on' } },
        on: { on: { TOGGLE_ITALIC: 'off' } }
      }
    }
  }
});
```

### Statekit Parallel States

```go
machine, _ := statekit.NewMachine[Context]("editor").
    WithInitial("active").
    State("active").Parallel().
        Region("bold").WithInitial("off").
            State("off").On("TOGGLE_BOLD").Target("on").EndState().
            State("on").On("TOGGLE_BOLD").Target("off").EndState().
        EndRegion().
        Region("italic").WithInitial("off").
            State("off").On("TOGGLE_ITALIC").Target("on").EndState().
            State("on").On("TOGGLE_ITALIC").Target("off").EndState().
        EndRegion().
    Done().
    Build()

// Check region states
state := interp.State()
fmt.Println(state.ActiveInParallel["bold"])   // "on" or "off"
fmt.Println(state.ActiveInParallel["italic"]) // "on" or "off"
```

**Key differences:**
- Regions are explicit with `.Region("name")`
- Use `.EndState()` for states inside regions
- Use `.EndRegion()` to close a region
- Access region states via `ActiveInParallel` map

---

## History States

### XState History States

```typescript
const machine = createMachine({
  id: 'wizard',
  initial: 'form',
  states: {
    form: {
      initial: 'step1',
      states: {
        step1: { on: { NEXT: 'step2' } },
        step2: { on: { NEXT: 'step3', PREV: 'step1' } },
        step3: { on: { PREV: 'step2' } },
        hist: { type: 'history', history: 'shallow' }
      },
      on: {
        HELP: 'help'
      }
    },
    help: {
      on: {
        BACK: 'form.hist'  // Return to last form step
      }
    }
  }
});
```

### Statekit History States

```go
machine, _ := statekit.NewMachine[Context]("wizard").
    WithInitial("form").
    State("form").
        WithInitial("step1").
        On("HELP").Target("help").End().
        History("hist").Shallow().Default("step1").End().
        State("step1").On("NEXT").Target("step2").End().End().
        State("step2").
            On("NEXT").Target("step3").
            On("PREV").Target("step1").
        End().End().
        State("step3").On("PREV").Target("step2").End().End().
    Done().
    State("help").
        On("BACK").Target("hist").  // Target the history pseudo-state
    Done().
    Build()
```

**Key differences:**
- Define history with `.History("name").Shallow().Default("fallback").End()`
- Or use `.Deep()` for deep history
- Target history state by its ID: `.Target("hist")`

---

## Delayed Transitions

### XState Delayed Transitions

```typescript
const machine = createMachine({
  id: 'session',
  initial: 'active',
  states: {
    active: {
      after: {
        5000: 'warning'
      },
      on: {
        ACTIVITY: 'active'
      }
    },
    warning: {
      after: {
        5000: 'expired'
      },
      on: {
        ACTIVITY: 'active'
      }
    },
    expired: { type: 'final' }
  }
});
```

### Statekit Delayed Transitions

```go
import "time"

machine, _ := statekit.NewMachine[Context]("session").
    WithInitial("active").
    State("active").
        After(5 * time.Second).Target("warning").
        On("ACTIVITY").Target("active").
    Done().
    State("warning").
        After(5 * time.Second).Target("expired").
        On("ACTIVITY").Target("active").
    Done().
    State("expired").Final().Done().
    Build()
```

**Key differences:**
- Use Go's `time.Duration` instead of milliseconds
- `.After(duration).Target("state")`
- Timers auto-cancel on state exit
- Guards work: `.After(5*time.Second).Target("x").Guard("condition")`

---

## Final States

### XState Final States

```typescript
const machine = createMachine({
  id: 'payment',
  initial: 'processing',
  states: {
    processing: {
      on: {
        SUCCESS: 'completed',
        FAILURE: 'failed'
      }
    },
    completed: { type: 'final' },
    failed: { type: 'final' }
  }
});
```

### Statekit Final States

```go
machine, _ := statekit.NewMachine[Context]("payment").
    WithInitial("processing").
    State("processing").
        On("SUCCESS").Target("completed").
        On("FAILURE").Target("failed").
    Done().
    State("completed").Final().Done().
    State("failed").Final().Done().
    Build()

interp := statekit.NewInterpreter(machine)
interp.Start()
interp.Send(statekit.Event{Type: "SUCCESS"})

if interp.Done() {
    fmt.Println("Machine completed")
}
```

---

## Event Payload

### XState Event Payload

```typescript
service.send({ type: 'UPDATE', data: { name: 'Alice', age: 30 } });

// Access in action
const machine = createMachine({
  // ...
}, {
  actions: {
    updateUser: (ctx, event) => {
      ctx.user = event.data;
    }
  }
});
```

### Statekit Event Payload

```go
// Event with payload (use Payload field, not Data)
interp.Send(statekit.Event{
    Type: "UPDATE",
    Payload: map[string]any{
        "name": "Alice",
        "age":  30,
    },
})

// Access in action with type assertion
WithAction("updateUser", func(ctx *Context, e statekit.Event) {
    if data, ok := e.Payload.(map[string]any); ok {
        ctx.User.Name = data["name"].(string)
        ctx.User.Age = data["age"].(int)
    }
})
```

**Key difference:** Use `Event.Payload` (type `any`) - requires type assertions in Go.

---

## Invoked Services

### XState Invoked Services

```typescript
const machine = createMachine({
  id: 'fetch',
  initial: 'loading',
  states: {
    loading: {
      invoke: {
        id: 'fetchData',
        src: async () => {
          const response = await fetch('/api/data');
          return response.json();
        },
        onDone: { target: 'success' },
        onError: { target: 'failure' }
      }
    },
    success: { type: 'final' },
    failure: { type: 'final' }
  }
});
```

### Statekit Invoked Services

```go
import (
    "context"
    "net/http"
)

machine, _ := statekit.NewMachine[FetchContext]("fetch").
    WithInitial("loading").
    // Register service with a name
    WithService("fetchData", func(svc statekit.ServiceContext[FetchContext]) error {
        ctx := svc.Context.(context.Context)  // For cancellation

        req, _ := http.NewRequestWithContext(ctx, "GET", "/api/data", nil)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return err  // Triggers OnError
        }
        defer resp.Body.Close()

        // Send result back as event
        svc.Send(statekit.Event{
            Type:    "DATA_LOADED",
            Payload: resp,
        })
        return nil  // Success triggers OnDone
    }).
    State("loading").
        Invoke("fetchData").
            OnDone("success").
            OnError("failure").
        End().
    Done().
    State("success").Final().Done().
    State("failure").Final().Done().
    Build()
```

**Key differences:**
- Register services with `WithService("name", func)`
- Service receives `ServiceContext` with context for cancellation, machine context (read-only), and `Send` function
- Return `error` for failure, `nil` for success
- Use `.Invoke("serviceName")` on state

---

## Persistence (Snapshots)

### XState Persistence

```typescript
const persistedState = JSON.stringify(service.getSnapshot());
localStorage.setItem('machine-state', persistedState);

const restoredState = JSON.parse(localStorage.getItem('machine-state'));
const service = interpret(machine).start(restoredState);
```

### Statekit Persistence

```go
// Save state
snapshot := interp.Snapshot()
data, _ := json.Marshal(snapshot)
// Save to file/database...

// Restore state
var snapshot statekit.Snapshot[MyContext]
json.Unmarshal(data, &snapshot)

newInterp := statekit.NewInterpreter(machine)
newInterp.Restore(snapshot)
// Continue from restored state
```

**Key difference:** `Snapshot` is a typed struct that includes context, history, parallel state, and pending timers.

---

## Plugin System

Statekit provides a plugin system for extending interpreter behavior:

```go
import "go.klarlabs.de/statekit/plugin"

// Implement plugin interfaces
type LoggingPlugin[C any] struct{}

func (p *LoggingPlugin[C]) Name() string { return "logging" }

func (p *LoggingPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
    log.Printf("Entered: %s", state)
}

func (p *LoggingPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
    log.Printf("Exited: %s", state)
}

// Register plugin
interp.Use(&LoggingPlugin[MyContext]{})
```

XState has similar extensibility through the `inspect` API.

---

## Declarative Definition (Reflection DSL)

For a more XState-like declarative syntax:

```go
type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"pending"`

    Pending    statekit.StateNode `on:"SUBMIT->processing"`
    Processing statekit.StateNode `on:"APPROVE->approved,REJECT->rejected"`
    Approved   statekit.FinalNode
    Rejected   statekit.FinalNode
}

registry := statekit.NewActionRegistry[OrderContext]()
machine, _ := statekit.FromStruct[OrderMachine, OrderContext](registry)
```

---

## Summary: Key Differences

| Aspect | XState | Statekit |
|--------|--------|----------|
| **Language** | TypeScript/JavaScript | Go |
| **Type safety** | Runtime (with TS) | Compile-time (generics) |
| **Machine creation** | Object literal | Fluent builder |
| **Context mutation** | `assign()` action | Pointer in action |
| **Guards** | `guard: fn` | `.Guard("name")` |
| **Async** | Promises/async-await | Goroutines |
| **Event payload** | `event.data` | `event.Payload` |
| **Plugins** | `inspect`, middleware | Plugin interfaces |
| **Visualization** | Native | Native (HTML, Mermaid, TUI) |

---

## Migration Checklist

- [ ] Convert machine definition to fluent builder or reflection DSL
- [ ] Define context as a Go struct with proper types
- [ ] Convert actions to `func(*Context, Event)` signature
- [ ] Convert guards to `func(Context, Event) bool` signature
- [ ] Replace `assign()` with direct context pointer mutation
- [ ] Replace promises with goroutines for async (via services)
- [ ] Update event payload: `event.data` → `event.Payload`
- [ ] Use `statekit viz` for visualization
- [ ] Add plugins for logging/metrics if needed

---

## See Also

- [Getting Started](./getting-started.md)
- [Visualization](../how-to/visualization.md)
- [Reflection DSL](../how-to/reflection-dsl.md)
- [Plugin System](../how-to/plugin-system.md)
- [API Reference](../reference/api-reference.md)
