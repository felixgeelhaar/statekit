# Form Wizard Example

A multi-step form wizard demonstrating history states for resuming interrupted workflows.

## Features Demonstrated

- **Shallow history** - Remembers the immediate child state
- **Deep history** - Remembers the full leaf state path
- **History defaults** - Fallback when no history exists
- **Compound states** - Nested steps within a parent section
- **Guards** - Validation before submission

## State Hierarchy

```
form_wizard
├── filling (compound, initial)
│   ├── personal (initial)
│   ├── address
│   ├── payment (compound)
│   │   ├── card_type (initial)
│   │   └── card_details
│   ├── review
│   ├── hist (shallow history)
│   └── deepHist (deep history)
├── previewing
├── submitted (final)
└── cancelled (final)
```

## History State Behavior

### Shallow vs Deep History

When the user is at `card_details` (nested inside `payment`) and goes to preview:

- **Shallow history (`hist`)**: Returns to `payment`, then enters its initial (`card_type`)
- **Deep history (`deepHist`)**: Returns directly to `card_details`

```
Before:      filling → payment → card_details
After PREVIEW: previewing

BACK_SHALLOW: filling → payment → card_type   (lost nested position)
BACK_DEEP:    filling → payment → card_details (exact position restored)
```

### History Default

If no history is recorded (never entered the parent state), the default target is used:

```go
History("hist").Shallow().Default("personal").End()
```

## Usage

### Run the Example

```bash
go run ./examples/form_wizard/
```

### Programmatic Usage

```go
package main

import (
    "go.klarlabs.de/statekit"
)

func main() {
    machine, _ := statekit.NewMachine[FormContext]("wizard").
        WithInitial("steps").
        State("steps").
            WithInitial("step1").
            On("INTERRUPT").Target("paused").End().
            History("hist").Shallow().Default("step1").End().
            History("deepHist").Deep().Default("step1").End().
            State("step1").On("NEXT").Target("step2").End().End().
            State("step2").On("NEXT").Target("step3").End().End().
            State("step3").End().
        Done().
        State("paused").
            On("RESUME_SHALLOW").Target("hist").
            On("RESUME_DEEP").Target("deepHist").
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()           // In step1

    interp.Send(statekit.Event{Type: "NEXT"})       // step2
    interp.Send(statekit.Event{Type: "NEXT"})       // step3
    interp.Send(statekit.Event{Type: "INTERRUPT"}) // paused

    // Shallow: goes to steps initial (step1)
    // Deep: goes to exact leaf (step3)
    interp.Send(statekit.Event{Type: "RESUME_DEEP"}) // step3
}
```

## Run Tests

```bash
go test -v ./examples/form_wizard/...
```

## Key Concepts

### Defining History States

```go
State("parent").
    WithInitial("child1").
    History("hist").Shallow().Default("child1").End().
    History("deepHist").Deep().Default("child1").End().
    State("child1").End().
    State("child2").End().
Done()
```

### Transitioning to History

```go
State("paused").
    On("RESUME").Target("hist").  // Target the history state
Done()
```

### When to Use Each Type

| Scenario | Use |
|----------|-----|
| Simple flat child states | Shallow |
| Nested compound states | Deep |
| Resume exact position | Deep |
| Resume section, not exact step | Shallow |

### Common Use Cases

1. **Form wizards** - Resume where user left off
2. **Tab navigation** - Remember last viewed tab in each section
3. **Game states** - Return to exact game position after pause
4. **Multi-step workflows** - Resume interrupted processes
