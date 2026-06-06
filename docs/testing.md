# Testing State Machines

Statekit provides dedicated testing and debugging packages to help you write comprehensive tests for your state machines.

## Installation

The testing and debug packages are included with statekit:

```bash
go get go.klarlabs.de/statekit
```

Import the packages in your test files:

```go
import (
    "go.klarlabs.de/statekit/statetest"
    "go.klarlabs.de/statekit/debug"
)
```

## Testing Package

The `statetest` package provides utilities for testing state machines with transition recording, assertions, and test helpers.

### Transition Recorder

The `Recorder` captures all transitions for later verification:

```go
func TestOrderWorkflow(t *testing.T) {
    machine := buildOrderMachine()
    interp := statekit.NewInterpreter(machine)

    // Wrap the interpreter with a recorder
    rec := statetest.NewRecorder(interp)
    rec.Start()

    // Send events
    rec.Send(statekit.Event{Type: "SUBMIT"})
    rec.Send(statekit.Event{Type: "PAY"})
    rec.Send(statekit.Event{Type: "SHIP"})

    // Verify recorded transitions
    transitions := rec.Transitions()
    if len(transitions) != 3 {
        t.Errorf("expected 3 transitions, got %d", len(transitions))
    }

    // Each transition captures full details
    first := transitions[0]
    fmt.Println(first.FromState)     // "pending"
    fmt.Println(first.ToState)       // "submitted"
    fmt.Println(first.Transitioned)  // true
    fmt.Println(first.Duration)      // execution time
}
```

The recorder captures:
- `Event` - The event that triggered the transition
- `FromState` - State before the transition
- `ToState` - State after the transition
- `Transitioned` - Whether a transition actually occurred
- `ContextBefore` - Context snapshot before transition
- `ContextAfter` - Context snapshot after transition
- `Timestamp` - When the event was sent
- `Duration` - How long the transition took

### Assertions

Use assertion helpers for cleaner test code:

```go
func TestCheckout(t *testing.T) {
    machine := buildCheckoutMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Assert initial state
    statetest.AssertState(t, interp, "cart")
    statetest.AssertNotDone(t, interp)

    // Send events and verify
    interp.Send(statekit.Event{Type: "CHECKOUT"})
    statetest.AssertState(t, interp, "payment")

    interp.Send(statekit.Event{Type: "PAY"})
    statetest.AssertState(t, interp, "confirmation")
    statetest.AssertDone(t, interp)
}
```

Available assertions:

| Function | Description |
|----------|-------------|
| `AssertState` | Assert current state equals expected |
| `AssertMatches` | Assert current state matches (including ancestors) |
| `AssertDone` | Assert machine is in a final state |
| `AssertNotDone` | Assert machine is not in a final state |
| `AssertTransitioned` | Assert recorder captured a specific transition |
| `AssertNoTransition` | Assert event did not cause a transition |
| `AssertEventSequence` | Assert recorder captured events in order |
| `AssertStateSequence` | Assert recorder captured states in order |
| `AssertContext` | Assert context satisfies a predicate |

### Fluent Assertions

For more expressive tests, use the fluent assertion API:

```go
func TestWorkflow(t *testing.T) {
    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)
    rec := statetest.NewRecorder(interp)
    rec.Start()

    // Send events
    rec.Send(statekit.Event{Type: "START"})
    rec.Send(statekit.Event{Type: "COMPLETE"})

    // Fluent state assertions
    statetest.NewStateAssertion(t, interp).
        IsIn("done").
        IsDone()

    // Fluent recorder assertions
    statetest.NewRecorderAssertion(t, rec).
        StateSequence("idle", "running", "done").
        EventSequence("START", "COMPLETE")
}
```

### Test Helpers

Convenience functions for common testing scenarios:

```go
// Send multiple events
statetest.SendEvents(interp,
    statekit.Event{Type: "START"},
    statekit.Event{Type: "PAUSE"},
    statekit.Event{Type: "RESUME"},
)

// Send events by type only (no payload)
statetest.SendEventTypes(interp, "START", "PAUSE", "RESUME")
```

### Quick Machine Builders

Create simple machines for testing:

```go
// Linear machine: idle -> running -> done (NEXT transitions)
machine := statetest.QuickMachine[Context]("idle", "running", "done")

// Toggle machine: off <-> on
machine := statetest.ToggleMachine[Context]("off", "on", "ON", "OFF")

// Cycle machine: a -> b -> c -> a (NEXT event)
machine := statetest.CycleMachine[Context]("a", "b", "c")

// Branch machine: start -> (a, b) based on event
machine := statetest.BranchMachine[Context]("start", "a", "b", "GO_A", "GO_B")
```

### Action and Guard Test Helpers

Control actions and guards in tests:

```go
func TestWithCounter(t *testing.T) {
    counter := statetest.NewActionCounter()

    machine, _ := statekit.NewMachine[struct{}]("test").
        WithInitial("idle").
        WithAction("count", statetest.ActionFor[struct{}](counter, "count")).
        State("idle").
            OnEntry("count").
            On("NEXT").Target("done").
        Done().
        State("done").Final().
            OnEntry("count").
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()
    interp.Send(statekit.Event{Type: "NEXT"})

    if counter.Count("count") != 2 {
        t.Errorf("expected 2 action calls, got %d", counter.Count("count"))
    }
}

func TestWithGuard(t *testing.T) {
    guards := statetest.NewGuardResult()
    guards.Set("check", true) // Initially passes

    machine, _ := statekit.NewMachine[struct{}]("test").
        WithInitial("idle").
        WithGuard("check", statetest.GuardFor[struct{}](guards, "check")).
        State("idle").
            On("GO").Target("done").Guard("check").
        Done().
        State("done").Final().Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Guard passes
    interp.Send(statekit.Event{Type: "GO"})
    statetest.AssertState(t, interp, "done")

    // Reset and block
    interp2 := statekit.NewInterpreter(machine)
    interp2.Start()
    guards.Set("check", false)
    interp2.Send(statekit.Event{Type: "GO"})
    statetest.AssertState(t, interp2, "idle") // Blocked
}
```

## Debug Package

The `debug` package provides runtime inspection and visualization tools.

### Inspector

The `Inspector` provides read-only access to machine state and configuration:

```go
func DebugMachine(interp *statekit.Interpreter[Context], machine *statekit.MachineConfig[Context]) {
    inspector := debug.NewInspector(interp, machine)

    // Current state information
    fmt.Println("Current:", inspector.CurrentState())
    fmt.Println("Done:", inspector.IsDone())
    fmt.Println("Path:", inspector.Path())  // For hierarchical states

    // Available events from current state
    events := inspector.AvailableEvents()
    for _, e := range events {
        canTransition := inspector.CanTransition(e)
        fmt.Printf("  %s: can transition = %v\n", e, canTransition)
    }

    // Simulate transitions without side effects
    target, willTransition := inspector.SimulateTransition(
        statekit.Event{Type: "SUBMIT"},
    )
    fmt.Printf("SUBMIT would go to: %s (transition: %v)\n", target, willTransition)
}
```

Inspector methods:

| Method | Description |
|--------|-------------|
| `CurrentState()` | Get current state ID |
| `CurrentContext()` | Get current context |
| `IsDone()` | Check if in final state |
| `MachineID()` | Get machine ID |
| `InitialState()` | Get initial state ID |
| `AllStates()` | List all state IDs |
| `StateInfo(id)` | Get detailed state information |
| `AvailableEvents()` | List events from current state |
| `CanTransition(event)` | Check if event would cause transition |
| `SimulateTransition(event)` | Simulate without side effects |
| `TransitionsFrom(id)` | List transitions from a state |
| `Path()` | Get path from root to current state |
| `Dump()` | Get human-readable state dump |
| `DumpMachine()` | Get complete machine dump |

### State Graph

The `StateGraph` represents the machine as a navigable graph:

```go
graph := debug.NewStateGraph(machine)

// Traverse the graph
for _, node := range graph.RootNodes() {
    fmt.Printf("State: %s (type: %s)\n", node.ID, node.Type)
}

// Find edges
edges := graph.GetEdgesFrom("idle")
for _, edge := range edges {
    fmt.Printf("  %s -> %s on %s\n", edge.From, edge.To, edge.Event)
}

// Analysis
analysis := graph.Analyze()
fmt.Println(analysis.String())
```

Graph analysis detects:
- Unreachable states
- Dead-end states (non-final with no outgoing transitions)
- Cycles in the transition graph
- Maximum hierarchy depth

### Visualization Export

Export machines for visualization:

```go
graph := debug.NewStateGraph(machine)

// Mermaid diagram (for documentation)
mermaid := graph.ToMermaid()
// Output:
// stateDiagram-v2
//     [*] --> idle
//     idle --> running : START
//     running --> done : COMPLETE
//     done --> [*]

// GraphViz DOT (for visualization tools)
dot := graph.ToDOT()
// Output:
// digraph example {
//     rankdir=LR;
//     node [shape=box, style=rounded];
//     __start__ [shape=point, width=0.1];
//     __start__ -> idle;
//     done [shape=doublecircle];
//     idle -> running [label="START"];
//     running -> done [label="COMPLETE"];
// }
```

Use the exports with:
- **Mermaid**: Embed in Markdown documentation, GitHub READMEs
- **DOT**: Render with GraphViz (`dot -Tpng machine.dot -o machine.png`)

## Table-Driven Tests

Combine statekit testing utilities with Go's table-driven test pattern:

```go
func TestOrderStates(t *testing.T) {
    tests := []struct {
        name     string
        events   []statekit.EventType
        expected statekit.StateID
        done     bool
    }{
        {
            name:     "new order stays pending",
            events:   nil,
            expected: "pending",
            done:     false,
        },
        {
            name:     "submit order",
            events:   []statekit.EventType{"SUBMIT"},
            expected: "submitted",
            done:     false,
        },
        {
            name:     "full order flow",
            events:   []statekit.EventType{"SUBMIT", "PAY", "SHIP", "DELIVER"},
            expected: "delivered",
            done:     true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            machine := buildOrderMachine()
            interp := statekit.NewInterpreter(machine)
            interp.Start()

            statetest.SendEventTypes(interp, tt.events...)
            statetest.AssertState(t, interp, tt.expected)

            if tt.done {
                statetest.AssertDone(t, interp)
            } else {
                statetest.AssertNotDone(t, interp)
            }
        })
    }
}
```

## Integration with Testing Frameworks

### With testify

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWithTestify(t *testing.T) {
    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    assert.Equal(t, statekit.StateID("idle"), interp.State().Value)

    interp.Send(statekit.Event{Type: "START"})
    require.Equal(t, statekit.StateID("running"), interp.State().Value)
}
```

### With gomega

```go
import (
    . "github.com/onsi/gomega"
)

func TestWithGomega(t *testing.T) {
    g := NewGomegaWithT(t)

    machine := buildMachine()
    interp := statekit.NewInterpreter(machine)
    interp.Start()

    g.Expect(interp.State().Value).To(Equal(statekit.StateID("idle")))

    interp.Send(statekit.Event{Type: "START"})
    g.Expect(interp.State().Value).To(Equal(statekit.StateID("running")))
}
```

## Best Practices

1. **Test state transitions, not implementation details**: Focus on verifying that events cause expected state changes.

2. **Use the recorder for integration tests**: Capture the full transition history to verify complex workflows.

3. **Use assertions for unit tests**: Quick state checks are ideal for focused unit tests.

4. **Simulate before asserting guards**: Use `SimulateTransition` to verify guard behavior without side effects.

5. **Test edge cases**: Verify behavior when guards block transitions or unknown events are sent.

6. **Use table-driven tests**: Cover multiple scenarios efficiently with Go's table-driven test pattern.

7. **Export graphs for documentation**: Use Mermaid export to include machine diagrams in documentation.

8. **Analyze for issues**: Run `graph.Analyze()` to detect unreachable states and dead ends during testing.
