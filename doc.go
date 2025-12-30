// Package statekit provides a Go-native statechart execution engine with
// XState JSON compatibility for visualization.
//
// Statekit enables backend engineers to define, execute, and visualize
// statecharts using existing XState tooling (Stately Visualizer, XState Inspect).
//
// # Basic Usage
//
// Define a state machine using the fluent builder API:
//
//	type Context struct {
//	    Count int
//	}
//
//	machine, err := statekit.NewMachine[Context]("counter").
//	    WithInitial("idle").
//	    WithContext(Context{Count: 0}).
//	    WithAction("increment", func(ctx *Context, e statekit.Event) {
//	        ctx.Count++
//	    }).
//	    WithGuard("hasCount", func(ctx Context, e statekit.Event) bool {
//	        return ctx.Count > 0
//	    }).
//	    State("idle").
//	        OnEntry("increment").
//	        On("START").Target("running").Guard("hasCount").
//	        Done().
//	    State("running").
//	        On("STOP").Target("idle").
//	        Done().
//	    Build()
//
//	interp := statekit.NewInterpreter(machine)
//	interp.Start()
//	interp.Send(statekit.Event{Type: "START"})
//
// # Features
//
// Core features:
//   - Fluent builder API with generics for type-safe context
//   - Synchronous interpreter with guards and actions
//   - Build-time validation
//   - Final states
//
// Hierarchical states:
//   - Compound/nested states with parent-child relationships
//   - Event bubbling from child to parent states
//   - Proper entry/exit action ordering
//
// Advanced features (v2.0):
//   - History states (shallow and deep)
//   - Delayed transitions with timers
//   - Parallel states with orthogonal regions
//
// Visualization:
//   - XState JSON export for use with stately.ai/viz
//   - Compatible with XState Inspector
//
// # XState Export
//
// Export machines for visualization:
//
//	exporter := export.NewXStateExporter(machine)
//	jsonStr, err := exporter.ExportJSONIndent("", "  ")
//	// Use with stately.ai/viz or XState Inspector
//
// For more examples, see the examples directory.
package statekit
