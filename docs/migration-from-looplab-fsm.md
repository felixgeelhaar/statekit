# Migrating from looplab/fsm to statekit

[looplab/fsm](https://github.com/looplab/fsm) is the most popular Go FSM library. This guide shows how to translate a looplab/fsm machine to statekit, what you gain, and what changes.

## Why migrate

| Concern | looplab/fsm | statekit |
|---|---|---|
| Type safety | `interface{}` events, untyped callbacks | Generic `[C any]` context, typed actions and guards |
| Hierarchy | None — flat FSM only | Compound + parallel + history states |
| Visualization | Graphviz dot only | ASCII, Mermaid, interactive HTML, TUI |
| Static analysis | None | Built-in lint (`unreachable`, `dead-end`, `non-determinism`, `invoke-missing-onerror`, more) |
| Async work | Manual goroutines | First-class `Invoke` services with cancellation |
| Persistence | None | `Snapshot`/`Restore` + event sourcing |
| Determinism for tests | Wall-clock timers | Injectable `Clock` with `FakeClock` |
| API surface | One `NewFSM(initial, events, callbacks)` | Fluent builder, reflection DSL, codegen |

## Mapping concepts

| looplab/fsm | statekit |
|---|---|
| `EventDesc{Name, Src, Dst}` | `.State(src).On(event).Target(dst)` |
| `Callbacks{"before_event": fn}` | `.WithAction("name", fn)` + `.Do("name")` |
| `Callbacks{"enter_state": fn}` | `.OnEntry("name")` |
| `Callbacks{"leave_state": fn}` | `.OnExit("name")` |
| `fsm.Event(ctx, "name")` | `interp.Send(statekit.Event{Type: "name"})` |
| `fsm.Current()` | `interp.State().Value` |
| `fsm.Can("event")` | `interp.Matches(...)` + check transitions (see below) |
| `metadata` map | Typed `Context` struct |

## Worked example: door

### Before — looplab/fsm

```go
package main

import (
    "context"
    "fmt"

    "github.com/looplab/fsm"
)

func main() {
    door := fsm.NewFSM(
        "closed",
        fsm.Events{
            {Name: "open", Src: []string{"closed"}, Dst: "open"},
            {Name: "close", Src: []string{"open"}, Dst: "closed"},
        },
        fsm.Callbacks{
            "enter_open": func(_ context.Context, e *fsm.Event) {
                fmt.Println("door opened")
            },
            "enter_closed": func(_ context.Context, e *fsm.Event) {
                fmt.Println("door closed")
            },
        },
    )

    _ = door.Event(context.Background(), "open")
    fmt.Println(door.Current()) // "open"
}
```

### After — statekit

```go
package main

import (
    "fmt"

    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine, err := statekit.NewMachine[struct{}]("door").
        WithInitial("closed").
        WithAction("logOpened", func(_ *struct{}, _ statekit.Event) {
            fmt.Println("door opened")
        }).
        WithAction("logClosed", func(_ *struct{}, _ statekit.Event) {
            fmt.Println("door closed")
        }).
        State("closed").
        OnEntry("logClosed").
        On("open").Target("open").
        Done().
        State("open").
        OnEntry("logOpened").
        On("close").Target("closed").
        Done().
        Build()
    if err != nil {
        panic(err)
    }

    interp := statekit.NewInterpreter(machine)
    defer interp.Close()
    interp.Start()

    interp.Send(statekit.Event{Type: "open"})
    fmt.Println(interp.State().Value) // "open"
}
```

## Replacing common patterns

### Typed context replaces `metadata`

looplab/fsm gives you a `metadata map[string]interface{}` for state. statekit gives you a typed struct.

```go
// Before
door.SetMetadata("times_opened", 5)
v, _ := door.Metadata("times_opened")
times := v.(int)

// After
type DoorCtx struct{ TimesOpened int }
machine, _ := statekit.NewMachine[DoorCtx]("door").
    WithAction("incr", func(c *DoorCtx, _ statekit.Event) { c.TimesOpened++ }).
    // ...
    Build()

times := interp.State().Context.TimesOpened // int, no cast
```

### Guards replace conditional `before_event` callbacks

looplab/fsm uses callback-returning-error to block transitions. statekit has guards.

```go
// Before
"before_open": func(_ context.Context, e *fsm.Event) {
    if e.FSM.Metadata("locked") == true {
        e.Cancel(fmt.Errorf("door is locked"))
    }
}

// After
.WithGuard("notLocked", func(c DoorCtx, _ statekit.Event) bool {
    return !c.Locked
}).
State("closed").
On("open").Target("open").Guard("notLocked").
Done()
```

### Async work replaces goroutine spaghetti

looplab/fsm has no story for cancellable async work. statekit has `Invoke`.

```go
machine, _ := statekit.NewMachine[Ctx]("loader").
    WithService("fetch", func(ctx statekit.ServiceContext[Ctx]) error {
        return fetchData(ctx.Context)
    }).
    State("loading").
    Invoke("fetch").ID("fetcher").OnDone("ready").OnError("failed").End().
    Done().
    // ...
```

When you exit `loading` (e.g., user cancels), the service's `context.Context` is canceled automatically.

## Things you don't have to give up

- **Callbacks** still work — they're called Actions, attached via `OnEntry` / `OnExit` / `.Do()` on transitions.
- **Multi-source transitions** (`Src: []string{"a", "b"}`) — declare the transition on each source state. `lint` will warn if you forget.
- **Event payloads** — `interp.Send(statekit.Event{Type: "x", Payload: anything})`. Action / guard signatures get the event including payload.

## Things you gain

- **Hierarchical states** — collapse 12 sibling "loading*" states into one compound `loading` with sub-states.
- **History states** — `RESUME` event takes you back to the exact sub-state you left.
- **Delayed transitions** — `.After(30 * time.Second).Target("timeout")`. No goroutine bookkeeping.
- **Visualization** — `statekit viz door.json` (or via Go package) gives you a Mermaid / HTML / TUI diagram.
- **Lint** — `lint.Lint(machine)` reports unreachable states, dead ends, non-determinism, missing OnError on Invoke, etc.
- **Snapshots** — pause/resume long-running workflows, persist them to a database.
- **Test determinism** — `WithClock(NewFakeClock(...))` removes timer flake.

## Step-by-step migration plan

1. Replace `fsm.NewFSM(initial, events, callbacks)` with `statekit.NewMachine[C](id).WithInitial(initial)...Build()`.
2. Translate each `EventDesc` to a `.State(src).On(event).Target(dst)` chain.
3. Move `before_event` callbacks that conditionally cancel into `.Guard()` predicates.
4. Move `enter_state` / `leave_state` callbacks into `.OnEntry()` / `.OnExit()`.
5. Replace `metadata` access with a typed `Context` struct + `.UpdateContext(fn)`.
6. Replace `fsm.Event(ctx, name)` calls with `interp.Send(statekit.Event{Type: name})`.
7. Add `defer interp.Close()` after `interp.Start()`.
8. Run `statekit viz` on the new machine to confirm it matches the original behavior.
9. Run `lint.Lint(machine)` and address any warnings.

## Gotchas

- **Action signature**: looplab passes `*fsm.Event` (which includes the source FSM). statekit passes `*Context` (mutable) and `Event` (the event). Read the event payload via `e.Payload`.
- **Guard order**: statekit evaluates guards *before* actions on a transition. Inspect the event payload in the guard if you need to gate on payload data — context isn't yet mutated.
- **Self-transitions** are external by default — they exit and re-enter the state, running entry/exit actions. statekit's `lint` flags unguarded self-transitions on states with entry actions.
- **Final states** are explicit: `.State("done").Final()` — `interp.Done()` returns `true` only when the machine is in a final state.

## See also

- [Getting Started](./getting-started.md)
- [Hierarchical States](./hierarchical-states.md)
- [Guards & Actions](./guards-actions.md)
- [Static Analysis (Lint)](./lint.md)
- [XState Migration](./xstate-migration.md) — if your team also uses XState on the JS side
