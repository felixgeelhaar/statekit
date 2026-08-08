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
// # Closing the builder
//
// Every state you open with State must be closed again, and which terminator
// closes it depends on where the state sits. There are three destinations:
//
//	Terminator     Returns to            Use for
//	-----------    -------------------   ----------------------------------
//	Done()         MachineBuilder        a top-level state
//	End()          enclosing state       a child of a compound state
//	EndState()     enclosing region      a state inside a parallel region
//
// Plus EndRegion(), which closes a region and returns to the parallel state
// that owns it. EndMachine() is a deprecated second spelling of Done().
//
// The three shapes, side by side. Flat — every state closes with Done:
//
//	statekit.NewMachine[Ctx]("order").
//		WithInitial("cart").
//		State("cart").On("CHECKOUT").Target("paid").Done().
//		State("paid").Final().Done().
//		Build()
//
// Nested — children close with End, one call per level, and the top-level
// parent closes with Done:
//
//	statekit.NewMachine[Ctx]("editor").
//		WithInitial("editing").
//		State("editing").
//			WithInitial("idle").
//			State("idle").On("TYPE").Target("dirty").End().End().
//			State("dirty").On("CLEAR").Target("idle").End().End().
//		Done().
//		Build()
//
// The doubled End reads as "close the transition, then close the state". The
// first returns to the state, the second to its parent.
//
// Parallel — states inside a region close with EndState, the region with
// EndRegion, the parallel state with Done:
//
//	statekit.NewMachine[Ctx]("editor").
//		WithInitial("editing").
//		State("editing").
//			Parallel().
//			Region("bold").WithInitial("off").
//				State("off").On("TOGGLE_BOLD").Target("on").EndState().
//				State("on").On("TOGGLE_BOLD").Target("off").EndState().
//			EndRegion().
//		Done().
//		Build()
//
// Building states in a loop, where the shape is not visible in the source, is
// the case that trips people up most. A flat machine assembled from a table:
//
//	builder := statekit.NewMachine[Ctx]("lifecycle").WithInitial(states[0])
//	for i, s := range states {
//		sb := builder.State(s)
//		if isFinal(s) {
//			sb = sb.Final()
//		}
//		for _, tr := range transitionsFrom(s) {
//			sb = sb.On(tr.Event).Target(tr.Target).End() // close the transition
//		}
//		builder = sb.Done() // close the state
//	}
//	machine, err := builder.Build()
//
// Using the wrong terminator often still compiles, because several of them
// return chainable types. Two guard rails narrow that: End on a top-level
// state and EndState outside a region both panic immediately with a message
// naming the terminator to use instead, rather than returning a nil builder
// that fails somewhere later.
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
