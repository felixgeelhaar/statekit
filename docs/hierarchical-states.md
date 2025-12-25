# Hierarchical States

Statekit supports hierarchical (nested/compound) states, allowing you to model complex state machines with parent-child relationships.

## What Are Hierarchical States?

Hierarchical states allow you to:
- Group related states under a parent state
- Share transitions across child states
- Model "is-a" relationships (e.g., "Walking is an Active state")

## Creating Nested States

### Fluent Builder API

```go
machine, _ := statekit.NewMachine[Context]("pedestrian_signal").
    WithInitial("active").
    State("active").
        WithInitial("dont_walk").            // Initial child state
        On("MAINTENANCE").Target("offline"). // Shared transition
        State("dont_walk").
            On("BUTTON").Target("walk").
        End().                               // Return to parent
        State("walk").
            On("TIMER").Target("countdown").
        End().
        State("countdown").
            On("TIMER").Target("dont_walk").
        End().
    Done().
    State("offline").
        On("RESUME").Target("active").
    Done().
    Build()
```

### Reflection DSL

```go
type WalkState struct {
    statekit.StateNode `on:"TIMER->countdown"`
}

type DontWalkState struct {
    statekit.StateNode `on:"BUTTON->walk"`
}

type CountdownState struct {
    statekit.StateNode `on:"TIMER->dont_walk"`
}

type ActiveState struct {
    statekit.CompoundNode `initial:"dont_walk" on:"MAINTENANCE->offline"`
    DontWalk DontWalkState
    Walk     WalkState
    Countdown CountdownState
}

type PedestrianSignal struct {
    statekit.MachineDef `id:"pedestrian_signal" initial:"active"`
    Active  ActiveState
    Offline statekit.StateNode `on:"RESUME->active"`
}
```

## Key Concepts

### 1. Initial Child State

Compound states must specify an initial child:

```go
State("parent").
    WithInitial("child1").  // Required for compound states
    State("child1"). ... .End().
    State("child2"). ... .End().
Done()
```

### 2. Event Bubbling

Events bubble up from child to parent. If a child doesn't handle an event, the parent gets a chance:

```go
State("active").
    On("RESET").Target("initial").    // Handles RESET for all children
    State("working").
        On("PAUSE").Target("paused"). // Only handles PAUSE
    End().
    State("paused").
        On("RESUME").Target("working").
    End().
Done()
```

When in `working` state:
- `PAUSE` → handled by `working`, transitions to `paused`
- `RESET` → not handled by `working`, bubbles to `active`, transitions to `initial`

### 3. Entry/Exit Order

When transitioning between states, actions execute in a specific order:

**Exit order**: Leaf → Root (children exit before parents)
**Entry order**: Root → Leaf (parents enter before children)

```go
State("parent").
    OnEntry("enterParent").
    OnExit("exitParent").
    State("child").
        OnEntry("enterChild").
        OnExit("exitChild").
    End().
Done()
```

Entering `child`: `enterParent` → `enterChild`
Exiting `child`: `exitChild` → `exitParent`

### 4. Lowest Common Ancestor (LCA)

Transitions only exit/enter states up to the LCA:

```
       root
      /    \
   active   done
   /   \
 idle  working
```

Transitioning from `idle` to `working`:
- LCA is `active`
- Exit: `idle`
- Enter: `working`
- `active` and `root` are NOT exited/entered

## The Matches() Method

Use `Matches()` to check if the machine is in a state or any of its ancestors:

```go
interp.Start() // Enters "active" → "dont_walk"

interp.State().Value          // "dont_walk" (leaf state)
interp.Matches("dont_walk")   // true
interp.Matches("active")      // true (ancestor)
interp.Matches("walk")        // false
```

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
)

type Context struct {
    ButtonPresses int
}

func main() {
    machine, _ := statekit.NewMachine[Context]("pedestrian_signal").
        WithInitial("active").
        WithAction("countPress", func(ctx *Context, e statekit.Event) {
            ctx.ButtonPresses++
        }).
        State("active").
            WithInitial("dont_walk").
            On("MAINTENANCE").Target("offline").
            State("dont_walk").
                On("BUTTON").Target("walk").Do("countPress").
            End().
            State("walk").
                On("TIMER").Target("countdown").
            End().
            State("countdown").
                On("TIMER").Target("dont_walk").
            End().
        Done().
        State("offline").
            On("RESUME").Target("active").
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    fmt.Println("State:", interp.State().Value)      // "dont_walk"
    fmt.Println("In active?", interp.Matches("active")) // true

    interp.Send(statekit.Event{Type: "BUTTON"})
    fmt.Println("State:", interp.State().Value)      // "walk"

    // MAINTENANCE bubbles from walk to active
    interp.Send(statekit.Event{Type: "MAINTENANCE"})
    fmt.Println("State:", interp.State().Value)      // "offline"
    fmt.Println("In active?", interp.Matches("active")) // false
}
```

## Best Practices

1. **Use compound states for related substates** - Group states that share common transitions or represent a logical unit.

2. **Keep hierarchies shallow** - Deep nesting (3+ levels) becomes hard to reason about.

3. **Use event bubbling for "escape" transitions** - Define global handlers on parent states for events like RESET, CANCEL, ERROR.

4. **Name states descriptively** - Use names that describe the state, not the transition that led to it.

## See Also

- [Getting Started](getting-started.md)
- [Guards and Actions](guards-actions.md)
- [XState Export](xstate-export.md) - Visualize hierarchical machines
