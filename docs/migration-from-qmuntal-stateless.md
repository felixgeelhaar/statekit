# Migrating from qmuntal/stateless to statekit

[qmuntal/stateless](https://github.com/qmuntal/stateless) is a Go port of the .NET Stateless library. It supports hierarchy and is closer to statekit than looplab/fsm. This guide focuses on what's different.

## At a glance

| Concern | qmuntal/stateless | statekit |
|---|---|---|
| Type safety | Reflection-based (`interface{}` triggers) | Generic `[C any]` context, typed events |
| Hierarchy | `Substate(parent)` | `WithInitial` + nested `State()` blocks |
| Parallel states | None | Yes (orthogonal regions) |
| History | None | Shallow + deep history |
| Visualization | DOT export | ASCII, Mermaid, HTML, TUI |
| Static analysis | None | Built-in lint with 9 rules |
| Async work | Manual | `Invoke` services with cancellation |
| Persistence | Manual via `OnEnter` callback | Snapshot + event sourcing |
| Test determinism | Wall-clock | Injectable `Clock` + `FakeClock` |

## Concept mapping

| qmuntal/stateless | statekit |
|---|---|
| `sm := stateless.NewStateMachine(initial)` | `statekit.NewMachine[C](id).WithInitial(initial)...` |
| `Configure(state).Permit(trigger, dst)` | `.State(src).On(trigger).Target(dst)` |
| `Configure(state).PermitIf(trigger, dst, guard)` | `.State(src).On(trigger).Target(dst).Guard("name")` |
| `Configure(state).Substate(parent)` | Nested `.State()` inside parent's compound |
| `Configure(state).OnEntry(fn)` | `.OnEntry("name")` (named action) |
| `Configure(state).OnExit(fn)` | `.OnExit("name")` |
| `sm.Fire(trigger)` | `interp.Send(statekit.Event{Type: "trigger"})` |
| `sm.State()` | `interp.State().Value` |
| `sm.IsInState(s)` | `interp.Matches(s)` |
| `sm.CanFire(trigger)` | (manually inspect — see below) |

## Key difference: actions are *named*

qmuntal/stateless lets you pass a closure directly. statekit registers actions and guards by name and references them in the machine. Named actions enable lint analysis (`unused-action`, `unused-guard`), Native JSON export, and visualization tooling.

```go
// qmuntal/stateless
sm.Configure("running").
    OnEntry(func(_ context.Context, args ...interface{}) error {
        fmt.Println("started")
        return nil
    })

// statekit
machine, _ := statekit.NewMachine[Ctx]("m").
    WithInitial("idle").
    WithAction("logStarted", func(_ *Ctx, _ statekit.Event) {
        fmt.Println("started")
    }).
    State("running").
    OnEntry("logStarted").
    Done().
    Build()
```

If you want closure-style anonymous actions, the reflection DSL or the codegen path (XState/Native JSON → Go) avoids the registration boilerplate.

## Worked example: phone call

### Before — qmuntal/stateless

```go
package main

import (
    "context"
    "fmt"

    "github.com/qmuntal/stateless"
)

const (
    StateIdle      = "Idle"
    StateRinging   = "Ringing"
    StateConnected = "Connected"

    TriggerCallDialed     = "CallDialed"
    TriggerCallConnected  = "CallConnected"
    TriggerCallTerminated = "CallTerminated"
)

func main() {
    phone := stateless.NewStateMachine(StateIdle)

    phone.Configure(StateIdle).
        Permit(TriggerCallDialed, StateRinging)

    phone.Configure(StateRinging).
        OnEntry(func(_ context.Context, _ ...interface{}) error {
            fmt.Println("ring ring")
            return nil
        }).
        Permit(TriggerCallConnected, StateConnected).
        Permit(TriggerCallTerminated, StateIdle)

    phone.Configure(StateConnected).
        Permit(TriggerCallTerminated, StateIdle)

    _ = phone.Fire(context.Background(), TriggerCallDialed)
    _ = phone.Fire(context.Background(), TriggerCallConnected)
    fmt.Println(phone.State()) // "Connected"
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
    machine, err := statekit.NewMachine[struct{}]("phone").
        WithInitial("idle").
        WithAction("logRinging", func(_ *struct{}, _ statekit.Event) {
            fmt.Println("ring ring")
        }).
        State("idle").
        On("CallDialed").Target("ringing").
        Done().
        State("ringing").
        OnEntry("logRinging").
        On("CallConnected").Target("connected").
        On("CallTerminated").Target("idle").
        Done().
        State("connected").
        On("CallTerminated").Target("idle").
        Done().
        Build()
    if err != nil {
        panic(err)
    }

    interp := statekit.NewInterpreter(machine)
    defer interp.Close()
    interp.Start()

    interp.Send(statekit.Event{Type: "CallDialed"})
    interp.Send(statekit.Event{Type: "CallConnected"})
    fmt.Println(interp.State().Value) // "connected"
}
```

## Hierarchy: substates → nested States

qmuntal/stateless uses `Substate(parent)`. statekit uses nested `State()` blocks inside a compound state.

```go
// qmuntal
phone.Configure("OffHook").
    Substate("Connected")

phone.Configure("OnHold").
    Substate("Connected").
    Permit("Resume", "Conversation")

// statekit
.State("connected").
    WithInitial("conversation").
    State("conversation").
    On("Hold").Target("onHold").
    End().
    State("onHold").
    On("Resume").Target("conversation").
    End().
Done().
```

## Things qmuntal/stateless doesn't have

- **Parallel/orthogonal regions** — model `Bluetooth` and `Network` as concurrent regions in the same machine instead of two separate FSMs.
- **History states** — return to the exact sub-state you left when re-entering a compound.
- **Delayed transitions** — `.After(30*time.Second).Target("timeout")` without goroutine bookkeeping.
- **Invoke** — declarative async services with automatic cancellation on state exit.
- **Snapshot/Restore + event sourcing** — full machine state persistence.
- **Lint** — catch structural bugs at build time.
- **Visualization formats** — Mermaid, HTML, TUI, ASCII (not just DOT).
- **MCP integration** — drive a running machine from a Claude / agent via tools.

## Migration plan

1. Translate each `Configure(state).Permit(...)` chain into a `.State(s).On(e).Target(d)` block.
2. Convert anonymous closures to named actions registered with `.WithAction("name", fn)`.
3. Convert `PermitIf` predicates to named guards via `.WithGuard("name", fn)` + `.Guard("name")`.
4. Replace `Substate(parent)` with nested `.State()` blocks inside the parent's compound.
5. Replace `Fire(ctx, trigger)` with `Send(Event{Type: trigger})`.
6. Replace `IsInState(s)` with `Matches(s)`.
7. Add `defer interp.Close()` after `Start()`.
8. Run `lint.Lint(machine)` to surface structural issues the original library would have missed.

## Gotchas

- **Trigger payloads**: qmuntal/stateless passes variadic `args ...interface{}` to callbacks. statekit packages this in `Event.Payload`. Read it via `e.Payload`.
- **Guards see context, not args**: statekit guards take `(C, Event)`. Payload-based gating reads `e.Payload`; context-based gating reads `c`.
- **OnEntry called once on entry**: same semantics as qmuntal. Self-transitions count as exit + re-entry.

## See also

- [Migration from looplab/fsm](./migration-from-looplab-fsm.md)
- [XState Migration](./xstate-migration.md)
- [Hierarchical States](./hierarchical-states.md)
- [Static Analysis (Lint)](./lint.md)
