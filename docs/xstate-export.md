# XState Export

Statekit can export machines to XState JSON format for visualization with [Stately Visualizer](https://stately.ai/viz) and other XState-compatible tools.

## Basic Export

```go
import (
    "fmt"
    "github.com/felixgeelhaar/statekit"
    "github.com/felixgeelhaar/statekit/export"
)

// Build your machine
machine, _ := statekit.NewMachine[Context]("traffic_light").
    WithInitial("green").
    State("green").On("TIMER").Target("yellow").Done().
    State("yellow").On("TIMER").Target("red").Done().
    State("red").On("TIMER").Target("green").Done().
    Build()

// Create exporter
exporter := export.NewXStateExporter(machine)

// Export to JSON
json, err := exporter.ExportJSON()
if err != nil {
    panic(err)
}
fmt.Println(json)
```

Output:
```json
{"id":"traffic_light","initial":"green","states":{"green":{"on":{"TIMER":{"target":"yellow"}}},"red":{"on":{"TIMER":{"target":"green"}}},"yellow":{"on":{"TIMER":{"target":"red"}}}}}
```

## Pretty-Printed Output

```go
json, err := exporter.ExportJSONIndent("", "  ")
```

Output:
```json
{
  "id": "traffic_light",
  "initial": "green",
  "states": {
    "green": {
      "on": {
        "TIMER": {
          "target": "yellow"
        }
      }
    },
    "yellow": {
      "on": {
        "TIMER": {
          "target": "red"
        }
      }
    },
    "red": {
      "on": {
        "TIMER": {
          "target": "green"
        }
      }
    }
  }
}
```

## Using the CLI Helper

For batch exports, use the CLI helper:

```go
//go:build ignore

package main

import (
    "log"
    "os"
    "github.com/felixgeelhaar/statekit/export"
)

func main() {
    machines := map[string]export.MachineExporter{
        "traffic":  export.NewXStateExporter(buildTrafficMachine()),
        "order":    export.NewXStateExporter(buildOrderMachine()),
    }

    if err := export.RunCLI(machines, os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```

CLI usage:
```bash
go run export.go -list                    # List machines
go run export.go -pretty                  # Export all, pretty-printed
go run export.go -machine=traffic -pretty # Export specific machine
go run export.go -o machines.json         # Export to file
```

## Visualization with Stately

1. Export your machine to JSON
2. Go to [stately.ai/viz](https://stately.ai/viz)
3. Click "Import JSON" or paste into the code panel
4. See your machine visualized!

## Exported Features

The exporter preserves:

| Feature | XState JSON |
|---------|-------------|
| States | `states: {}` |
| Initial state | `initial: "..."` |
| Transitions | `on: { EVENT: { target: "..." } }` |
| Entry actions | `entry: ["action1", "action2"]` |
| Exit actions | `exit: ["action1", "action2"]` |
| Transition actions | `actions: ["action1"]` |
| Guards | `guard: "guardName"` |
| Final states | `type: "final"` |
| Nested states | `states: { child: { ... } }` |
| Initial child | `initial: "childState"` |

## Hierarchical State Export

Nested states are properly represented:

```go
machine, _ := statekit.NewMachine[Context]("nested").
    WithInitial("active").
    State("active").
        WithInitial("idle").
        State("idle").On("START").Target("working").End().
        State("working").On("STOP").Target("idle").End().
    Done().
    Build()
```

Exports as:
```json
{
  "id": "nested",
  "initial": "active",
  "states": {
    "active": {
      "initial": "idle",
      "states": {
        "idle": {
          "on": {
            "START": { "target": "working" }
          }
        },
        "working": {
          "on": {
            "STOP": { "target": "idle" }
          }
        }
      }
    }
  }
}
```

## Programmatic Export API

### Export Struct

```go
type XStateMachine struct {
    ID      string                `json:"id"`
    Initial string                `json:"initial,omitempty"`
    States  map[string]XStateNode `json:"states"`
}

type XStateNode struct {
    Type    string                      `json:"type,omitempty"`
    Initial string                      `json:"initial,omitempty"`
    States  map[string]XStateNode       `json:"states,omitempty"`
    Entry   []string                    `json:"entry,omitempty"`
    Exit    []string                    `json:"exit,omitempty"`
    On      map[string]XStateTransition `json:"on,omitempty"`
}
```

### Direct Access

```go
exporter := export.NewXStateExporter(machine)

// Get the struct directly
xstate, err := exporter.Export()
if err != nil {
    panic(err)
}

// Access fields
fmt.Println(xstate.ID)
fmt.Println(xstate.Initial)
for name, state := range xstate.States {
    fmt.Printf("State: %s, Type: %s\n", name, state.Type)
}
```

## CLI Helper API

### ExportOptions

```go
type ExportOptions struct {
    PrettyPrint bool       // Enable indented JSON
    Indent      string     // Indentation string (default: "  ")
    Output      io.Writer  // Output destination (default: os.Stdout)
    MachineID   string     // Export only this machine (empty = all)
}
```

### Functions

```go
// Export single machine
func ExportMachine(exporter MachineExporter, opts ExportOptions) error

// Export multiple machines
func ExportAll(machines map[string]MachineExporter, opts ExportOptions) error

// CLI wrapper
func RunCLI(machines map[string]MachineExporter, args []string) error
```

## Complete Export Tool Example

See `examples/export_tool/` for a complete example:

```bash
cd examples/export_tool
go run main.go -list
go run main.go -pretty -machine=traffic
go run main.go -o output.json
```

## Best Practices

1. **Export regularly during development** - Use visualization to catch design issues early.

2. **Version your exports** - Save JSON exports alongside code changes.

3. **Use meaningful IDs** - Machine and state IDs appear in visualizations.

4. **Test round-trips** - Ensure exported JSON validates in XState tools.

## Limitations

Currently not exported:
- Invoke/spawn actors (not supported in statekit)
- Parallel states (not supported in statekit)
- History states (not supported in statekit)
- Delayed transitions (not supported in statekit)

## See Also

- [Getting Started](getting-started.md)
- [Hierarchical States](hierarchical-states.md)
- [XState Documentation](https://stately.ai/docs/xstate)
