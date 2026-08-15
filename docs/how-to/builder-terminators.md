# Builder terminators

Every `State(...)` you open must be closed. Which terminator you use depends on **where** the state sits — not on taste.

| Terminator | Returns to | Use for |
|---|---|---|
| `Done()` | `MachineBuilder` | a **top-level** state |
| `End()` | enclosing `StateBuilder` | a **child** of a compound state |
| `EndState()` | enclosing `RegionBuilder` | a state inside a **parallel region** |
| `EndRegion()` | parallel `StateBuilder` | closing a region (not a state) |

`EndMachine()` is a deprecated alias of `Done()`. Prefer `Done()`.

Wrong terminators often still compile because several of them return chainable types. Two hard guard rails catch the common mistakes at call time:

- `End()` on a top-level state **panics** — use `Done()`.
- `EndState()` outside a region **panics** — use `End()` or `Done()`.

## Flat machine — only `Done`

```go
statekit.NewMachine[Ctx]("order").
    WithInitial("cart").
    State("cart").On("CHECKOUT").Target("paid").Done().
    State("paid").Final().Done().
    Build()
```

## Nested machine — `End` per level, then `Done`

```go
statekit.NewMachine[Ctx]("editor").
    WithInitial("editing").
    State("editing").
        WithInitial("idle").
        State("idle").On("TYPE").Target("dirty").End().End().
        //                                       ^      ^ back to "editing"
        //                                       close transition, then state
        State("dirty").On("CLEAR").Target("idle").End().End().
    Done().
    Build()
```

The doubled `End` reads as "close the transition, then close the state."

## Parallel machine — `EndState` → `EndRegion` → `Done`

```go
statekit.NewMachine[Ctx]("editor").
    WithInitial("editing").
    State("editing").
        Parallel().
        Region("bold").WithInitial("off").
            State("off").On("TOGGLE_BOLD").Target("on").EndState().
            State("on").On("TOGGLE_BOLD").Target("off").EndState().
        EndRegion().
    Done().
    Build()
```

## Building states in a loop

When the nesting shape is not visible in the source, close each state with the terminator that matches its depth:

```go
builder := statekit.NewMachine[Ctx]("lifecycle").WithInitial(states[0])
for _, s := range states {
    sb := builder.State(s)
    for _, tr := range transitionsFrom(s) {
        sb = sb.On(tr.Event).Target(tr.Target).End() // close the transition
    }
    builder = sb.Done() // top-level: close the state back to the machine
}
machine, err := builder.Build()
```

For the full table and more examples, see the package godoc (`doc.go`) under "Closing the builder."
