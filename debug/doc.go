// Package debug provides utilities for debugging and inspecting statekit state machines.
//
// This package offers:
//   - Inspector for examining machine configuration and available transitions
//   - State graph representation for visualization
//   - Simulation capabilities to test transitions without side effects
//
// Example usage:
//
//	machine, _ := statekit.NewMachine[Context]("order")....Build()
//	interp := statekit.NewInterpreter(machine)
//	interp.Start()
//
//	inspector := debug.NewInspector(interp, machine)
//
//	// Check available events from current state
//	events := inspector.AvailableEvents()
//	fmt.Println("Available events:", events)
//
//	// Check if a transition is possible
//	if inspector.CanTransition("SUBMIT") {
//	    fmt.Println("Can submit order")
//	}
//
//	// Simulate a transition without actually executing it
//	nextState, willTransition := inspector.SimulateTransition(statekit.Event{Type: "SUBMIT"})
//	fmt.Printf("Would transition to: %s (will transition: %v)\n", nextState, willTransition)
//
//	// Get a visual representation of the machine
//	graph := inspector.StateGraph()
//	fmt.Println(graph.String())
package debug
