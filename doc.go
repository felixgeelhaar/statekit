// Package statekit provides a Go-native statechart execution engine.
//
// Define and execute statecharts in Go — visualize them with built-in tools.
//
// # Features
//
//   - Fluent Builder API
//   - Hierarchical States (Compound/Nested)
//   - History States (Shallow and Deep)
//   - Delayed Transitions (Timers)
//   - Parallel States (Orthogonal Regions)
//   - Reflection DSL (Struct tags)
//   - Native Visualization (HTML simulator, Mermaid, ASCII, TUI)
//
// # Quick Start
//
//	machine, _ := statekit.NewMachine[struct{}]("traffic").
//		WithInitial("green").
//		State("green").On("TIMER").Target("yellow").Done().
//		State("yellow").On("TIMER").Target("red").Done().
//		State("red").On("TIMER").Target("green").Done().
//		Build()
//
//	interp := statekit.NewInterpreter(machine)
//	interp.Start()
//
//	interp.Send(statekit.Event{Type: "TIMER"})
//
// # Visualization
//
// Export your machine to Statekit Native JSON for visualization:
//
//	exporter := export.NewNativeExporter(machine)
//	jsonStr, _ := exporter.ExportJSONIndent("", "  ")
//	fmt.Println(jsonStr)
//
// Or use the CLI:
//
//	statekit viz --go-package . --format html -o machine.html
package statekit
