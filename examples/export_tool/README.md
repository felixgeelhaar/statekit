# Export Tool Example

A CLI tool demonstrating how to export state machines to XState JSON format.

## Features Demonstrated

- CLI argument parsing with `export.RunCLI`
- Multiple machine export
- XState JSON generation for visualization
- Pretty-printing and file output

## Usage

```bash
# List available machines
go run ./examples/export_tool/ -list

# Export all machines (pretty-printed)
go run ./examples/export_tool/ -pretty

# Export specific machine
go run ./examples/export_tool/ -machine=traffic -pretty

# Export to file
go run ./examples/export_tool/ -o machines.json

# Combine options
go run ./examples/export_tool/ -machine=order -pretty -o order.json
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `-list` | List available machine names |
| `-machine=NAME` | Export only the specified machine |
| `-pretty` | Pretty-print JSON output |
| `-o FILE` | Write output to file instead of stdout |

## Programmatic Usage

```go
package main

import (
    "os"
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/export"
)

func main() {
    // Build your machines
    machine1 := buildMachine1()
    machine2 := buildMachine2()

    // Create exporters map
    machines := map[string]export.MachineExporter{
        "machine1": export.NewXStateExporter(machine1),
        "machine2": export.NewXStateExporter(machine2),
    }

    // Run CLI with command-line arguments
    if err := export.RunCLI(machines, os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```

## Key Concepts

### MachineExporter Interface

```go
type MachineExporter interface {
    Export() (XStateMachine, error)
    ExportJSON() (string, error)
    ExportJSONIndent(prefix, indent string) (string, error)
}
```

### Creating an Exporter

```go
exporter := export.NewXStateExporter(machine)

// Get structured data
xstate, _ := exporter.Export()

// Get compact JSON
json, _ := exporter.ExportJSON()

// Get formatted JSON
prettyJSON, _ := exporter.ExportJSONIndent("", "  ")
```

### Visualization

Copy the JSON output and paste it at [stately.ai/viz](https://stately.ai/viz) to visualize your state machine.

## Example Output

```json
{
  "id": "traffic_light",
  "initial": "green",
  "states": {
    "green": {
      "entry": ["incrementCycle"],
      "on": {
        "TIMER": { "target": "yellow" }
      }
    },
    "yellow": {
      "on": {
        "TIMER": { "target": "red" }
      }
    },
    "red": {
      "on": {
        "TIMER": { "target": "green" }
      }
    }
  }
}
```
