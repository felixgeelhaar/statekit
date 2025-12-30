# Design: Invoked Services

## Overview

Invoked services allow states to spawn child actors (machines, promises, callbacks) that run concurrently with the parent machine. This enables composition and encapsulation of complex behavior.

## XState Reference

XState's invoke pattern:
```javascript
{
  states: {
    loading: {
      invoke: {
        id: 'fetchData',
        src: 'dataFetcher',
        onDone: { target: 'success', actions: 'assignData' },
        onError: { target: 'failure' }
      }
    }
  }
}
```

## Go API Design

### Option 1: Callback-based Services

```go
type Service[C any] func(ctx context.Context, machineCtx C, send func(Event)) error

machine, _ := statekit.NewMachine[Context]("parent").
    WithInitial("loading").
    WithService("fetchData", func(ctx context.Context, machineCtx Context, send func(Event)) error {
        data, err := fetchFromAPI(ctx)
        if err != nil {
            return err // triggers onError
        }
        send(Event{Type: "DATA_LOADED", Payload: data})
        return nil // triggers onDone
    }).
    State("loading").
        Invoke("fetchData").
            OnDone("success").WithAction("assignData").
            OnError("failure").
        End().
    Done().
    State("success").Final().Done().
    State("failure").Done().
    Build()
```

### Option 2: Child Machine Spawning

```go
childMachine, _ := statekit.NewMachine[ChildContext]("child").
    WithInitial("working").
    State("working").On("COMPLETE").Target("done").Done().
    State("done").Final().Done().
    Build()

machine, _ := statekit.NewMachine[ParentContext]("parent").
    WithInitial("active").
    WithChildMachine("worker", childMachine).
    State("active").
        SpawnChild("worker").
            OnDone("completed").
            OnError("failed").
        End().
        On("CANCEL").Target("cancelled").
    Done().
    State("completed").Final().Done().
    Build()
```

### Option 3: Promise-like Pattern

```go
type Promise[T any] func(ctx context.Context) (T, error)

machine, _ := statekit.NewMachine[Context]("fetcher").
    WithInitial("loading").
    WithPromise("getData", func(ctx context.Context) (Data, error) {
        return api.FetchData(ctx)
    }).
    State("loading").
        InvokePromise("getData").
            OnResolve("success").Assign(func(ctx *Context, data Data) {
                ctx.Data = data
            }).
            OnReject("error").
        End().
    Done().
    Build()
```

## Internal Representation

```go
// internal/ir/machine.go

type InvokeConfig struct {
    ID        string        // Unique identifier for this invocation
    Type      InvokeType    // Service, Machine, Promise
    Source    string        // Reference to registered service/machine
    OnDone    *TransitionConfig
    OnError   *TransitionConfig
    AutoForward bool        // Forward events to child
}

const (
    InvokeTypeService InvokeType = iota
    InvokeTypeMachine
    InvokeTypePromise
)

// Extend StateConfig
type StateConfig struct {
    // ... existing fields
    Invoke []*InvokeConfig  // Active invocations while in this state
}
```

## Interpreter Changes

```go
type Interpreter[C any] struct {
    // ... existing fields
    activeServices map[string]context.CancelFunc  // Track running services
    childMachines  map[string]*Interpreter[any]   // Track spawned machines
}

// Service lifecycle
func (i *Interpreter[C]) enterState(state *ir.StateConfig) {
    // ... existing entry logic

    // Start invoked services
    for _, invoke := range state.Invoke {
        switch invoke.Type {
        case ir.InvokeTypeService:
            i.startService(invoke)
        case ir.InvokeTypeMachine:
            i.spawnChildMachine(invoke)
        case ir.InvokeTypePromise:
            i.startPromise(invoke)
        }
    }
}

func (i *Interpreter[C]) exitState(state *ir.StateConfig) {
    // Cancel running services when exiting state
    for _, invoke := range state.Invoke {
        if cancel, ok := i.activeServices[invoke.ID]; ok {
            cancel()
            delete(i.activeServices, invoke.ID)
        }
    }

    // ... existing exit logic
}
```

## Communication Patterns

### Parent → Child
- **Send events**: `interp.SendToChild("worker", Event{Type: "PAUSE"})`
- **Auto-forward**: Forward all events to child machines

### Child → Parent
- **Completion events**: `onDone` / `onError` transitions
- **Custom events**: Child calls `sendParent(Event{Type: "PROGRESS", Payload: 50})`

### Synchronization
- **Serial**: Wait for service completion before continuing
- **Parallel**: Multiple services run concurrently
- **Race**: First service to complete wins

## XState Export

```json
{
  "states": {
    "loading": {
      "invoke": {
        "id": "fetchData",
        "src": "dataFetcher",
        "onDone": {
          "target": "success",
          "actions": ["assignData"]
        },
        "onError": {
          "target": "failure"
        }
      }
    }
  }
}
```

## Validation Rules

```go
ErrCodeInvokeNoSource       = "INVOKE_NO_SOURCE"       // Missing service reference
ErrCodeInvokeSourceNotFound = "INVOKE_SOURCE_NOT_FOUND" // Referenced service not defined
ErrCodeInvokeNoOnDone       = "INVOKE_NO_ON_DONE"      // Missing completion handler
ErrCodeInvokeInFinal        = "INVOKE_IN_FINAL"        // Cannot invoke from final state
```

## Thread Safety Considerations

1. **Service goroutines**: Each service runs in its own goroutine
2. **Event sending**: Use channel to send events back to parent
3. **Cancellation**: Use context.Context for clean shutdown
4. **State access**: Protect context with mutex during service execution

## Implementation Phases

### Phase 1: Callback Services
- Basic `Service[C]` type
- `Invoke()` builder method
- `onDone` / `onError` handlers
- Cancellation on state exit

### Phase 2: Child Machines
- Spawn typed child machines
- Bidirectional communication
- Event forwarding

### Phase 3: Promise Pattern
- Generic promise resolution
- Type-safe result assignment
- Error handling

## Open Questions

1. **Type safety**: How to handle context type differences between parent and child?
2. **Error propagation**: Should errors in child machines bubble to parent?
3. **Lifecycle**: When parent stops, should children be force-stopped or allowed to complete?
4. **Event routing**: How to namespace events from multiple child machines?

## Alternatives Considered

1. **External orchestration**: Keep machines simple, orchestrate externally
   - Pro: Simpler library
   - Con: Common pattern not supported

2. **Actor model**: Full actor system with mailboxes
   - Pro: More powerful
   - Con: Significant complexity increase

3. **Go channels**: Use channels for inter-machine communication
   - Pro: Idiomatic Go
   - Con: Less portable to XState visualization

## Recommendation

Start with **Phase 1: Callback Services** as they:
- Cover the most common use case (async operations)
- Don't require complex type system changes
- Map cleanly to XState's invoke pattern
- Are simpler to implement and test

Child machines (Phase 2) can be added later if there's demand.
