# Text Editor Example

A text editor toolbar demonstrating parallel (orthogonal) states for independent formatting options.

## Features Demonstrated

- **Parallel regions** executing simultaneously
- **Independent state management** per region
- **Event handling** within specific regions
- **Region-specific state queries** via `ActiveInParallel`
- **Exiting parallel states** to final states

## State Hierarchy

```
text_editor
├── editing (parallel)
│   ├── bold (region)
│   │   ├── bold_off (initial)
│   │   └── bold_on
│   ├── italic (region)
│   │   ├── italic_off (initial)
│   │   └── italic_on
│   ├── underline (region)
│   │   ├── underline_off (initial)
│   │   └── underline_on
│   ├── alignment (region)
│   │   ├── align_left (initial)
│   │   ├── align_center
│   │   └── align_right
│   └── fontSize (region)
│       ├── size_small
│       ├── size_medium (initial)
│       └── size_large
└── saved (final)
```

## Parallel State Behavior

Each region operates independently. When in the `editing` state:

- All 5 regions are active simultaneously
- Events are matched against all regions
- Only the region with a matching transition responds
- Other regions maintain their current state

```
Initial State:
  editing
  ├─ bold: bold_off
  ├─ italic: italic_off
  ├─ underline: underline_off
  ├─ alignment: align_left
  └─ fontSize: size_medium

After TOGGLE_BOLD:
  editing
  ├─ bold: bold_on        ← changed
  ├─ italic: italic_off   ← unchanged
  ├─ underline: underline_off
  ├─ alignment: align_left
  └─ fontSize: size_medium
```

## Usage

### Run the Example

```bash
go run ./examples/text_editor/
```

### Programmatic Usage

```go
package main

import (
    "github.com/felixgeelhaar/statekit"
)

func main() {
    machine, _ := statekit.NewMachine[EditorContext]("editor").
        WithInitial("editing").
        State("editing").Parallel().
            Region("bold").
                WithInitial("off").
                State("off").On("TOGGLE_BOLD").Target("on").EndState().
                State("on").On("TOGGLE_BOLD").Target("off").EndState().
            EndRegion().
            Region("italic").
                WithInitial("off").
                State("off").On("TOGGLE_ITALIC").Target("on").EndState().
                State("on").On("TOGGLE_ITALIC").Target("off").EndState().
            EndRegion().
        Done().
        Build()

    interp := statekit.NewInterpreter(machine)
    interp.Start()

    // Check active states in each region
    state := interp.State()
    fmt.Println(state.ActiveInParallel["bold"])   // "off"
    fmt.Println(state.ActiveInParallel["italic"]) // "off"

    // Toggle bold (only affects bold region)
    interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})

    state = interp.State()
    fmt.Println(state.ActiveInParallel["bold"])   // "on"
    fmt.Println(state.ActiveInParallel["italic"]) // "off" (unchanged)
}
```

## Run Tests

```bash
go test -v ./examples/text_editor/...
```

## Key Concepts

### Defining Parallel States

```go
State("parent").Parallel().
    Region("region1").
        WithInitial("idle").
        State("idle").On("GO").Target("active").EndState().
        State("active").EndState().
    EndRegion().
    Region("region2").
        WithInitial("off").
        State("off").On("TOGGLE").Target("on").EndState().
        State("on").On("TOGGLE").Target("off").EndState().
    EndRegion().
Done()
```

### Accessing Region States

```go
state := interp.State()

// Parent parallel state name
fmt.Println(state.Value) // "parent"

// Individual region states
fmt.Println(state.ActiveInParallel["region1"]) // "idle" or "active"
fmt.Println(state.ActiveInParallel["region2"]) // "off" or "on"
```

### Matching States in Regions

```go
// Matches the parallel parent
interp.Matches("parent")  // true

// Matches any active region state
interp.Matches("idle")    // true if region1 is in "idle"
interp.Matches("on")      // true if region2 is in "on"
```

### Exiting Parallel States

A transition from the parallel state exits ALL regions:

```go
State("editing").Parallel().
    On("SAVE").Target("saved").End().  // Exit from parallel state
    Region(...).EndRegion().
    Region(...).EndRegion().
Done()
```

### Common Use Cases

| Scenario | Regions |
|----------|---------|
| Text editor | bold, italic, underline, alignment, font |
| Media player | playback, volume, shuffle, repeat |
| Game character | movement, action, inventory, status |
| Form validation | field1_valid, field2_valid, form_valid |
| Dashboard | data_loading, filters_active, view_mode |
