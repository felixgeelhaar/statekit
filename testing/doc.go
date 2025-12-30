// Package testing provides utilities for testing statekit state machines.
//
// This package offers helpers for:
//   - Recording state transitions for later assertion
//   - Asserting state, transitions, and event sequences
//   - Convenience functions for sending events in tests
//
// Example usage:
//
//	func TestOrderWorkflow(t *testing.T) {
//	    machine, _ := statekit.NewMachine[OrderCtx]("order").
//	        WithInitial("pending").
//	        State("pending").On("SUBMIT").Target("processing").Done().
//	        State("processing").On("COMPLETE").Target("done").Done().
//	        State("done").Final().Done().
//	        Build()
//
//	    interp := statekit.NewInterpreter(machine)
//	    rec := statetesting.NewRecorder(interp)
//
//	    rec.Start()
//	    rec.Send(statekit.Event{Type: "SUBMIT"})
//	    rec.Send(statekit.Event{Type: "COMPLETE"})
//
//	    statetesting.AssertStateSequence(t, rec, "pending", "processing", "done")
//	    statetesting.AssertDone(t, rec.Interpreter())
//	}
package testing
