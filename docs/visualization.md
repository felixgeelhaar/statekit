# Visualization

Statekit includes powerful visualization tools to help you design, debug, and document your state machines.

## Live Visualizer

You can use the [Statekit Live Visualizer](visualizer.html) to paste your machine JSON and interact with it directly in the browser.

Features:
- **Editor**: Paste Native JSON to see the graph instantly.
- **Simulation**: Trigger events to see how the machine transitions.
- **Hierarchical View**: Supports compound and parallel states.

## Visualization Formats

### HTML Visualizer (Interactive)

The CLI can generate a standalone version of the visualizer for a specific machine:

```bash
statekit viz machine.json --format html -o machine.html
open machine.html
```

Features:
- **Interactive Graph**: Pan, zoom, and explore the state chart.
- **Simulation**: Click buttons to trigger events and see state transitions.
- **State Highlighting**: Current state and active paths are highlighted.
- **Self-contained**: No external servers required (uses CDN for libraries).

### Mermaid Diagrams

Generate [Mermaid](https://mermaid.js.org/) state diagrams for documentation or GitHub comments.

```bash
statekit viz machine.json --format mermaid -o diagram.md
```

### ASCII Diagrams

Generate ASCII box diagrams for terminal output or quick checks.

```bash
statekit viz machine.json --format ascii
```

### TUI (Terminal User Interface)

Explore your state machine interactively in the terminal.

```bash
statekit viz machine.json --format tui
```

## Input Sources

### From Go Source Code

You can visualize machines directly from your Go source code without exporting JSON first. The tool parses your Go code to extract machine definitions.

```bash
# Visualize a specific machine type in a package
statekit viz --go-package ./examples/order_workflow --go-type OrderMachine --format html -o order.html

# Visualize all machines in a package
statekit viz --go-package ./examples/... 
```

### From Statekit Native JSON

You can export your machine to JSON using `export.NewNativeExporter` and visualize the file.

```go
exporter := export.NewNativeExporter(machine)
jsonStr, _ := exporter.ExportJSONIndent("", "  ")
// Save to file...
```

Then visualize it:

```bash
statekit viz machine.json
```

## Marketing / Website Integration

The HTML visualizer uses a standard JSON format that can be easily embedded in websites or documentation portals. You can generate the HTML once and host it anywhere.

Example JSON structure:

```json
{
  "id": "traffic_light",
  "initial": "green",
  "states": {
    "green": {
      "id": "green",
      "type": "atomic",
      "transitions": [
        { "event": "TIMER", "target": "yellow" }
      ]
    },
    ...
  }
}
```
