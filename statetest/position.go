package statetest

import (
	"fmt"
	"testing"
	"time"

	"go.klarlabs.de/statekit"
)

// InterpreterAt returns a started interpreter positioned at the given state,
// without driving the machine there event by event. It is the way to test a
// property of one state — "a final state accepts nothing", "this state ignores
// CANCEL" — against the machine that actually ships, rather than against a
// variant machine rebuilt for the test.
//
//	interp := statetest.InterpreterAt(BuildLifecycle(), "archived")
//	defer interp.Close()
//
//	statetest.AssertTerminal(t, interp, "SUBMIT", "APPROVE", "PUBLISH")
//
// Final states are allowed, and are the main reason this exists. Note that
// starting a machine in a final state is possible without this helper too —
// point WithInitial at one and Start lands there with Done reporting true —
// but that changes the machine, which defeats the purpose when the test is
// meant to prove the machine and its specification agree.
//
// It is built on Interpreter.Restore, the same mechanism used for persistence
// and recovery. Two consequences follow from that, and both matter when
// reading a test that uses it:
//
//   - Entry actions for the state are not executed, and no transition into it
//     is recorded. The interpreter is placed there, not moved there. Use
//     RunMachine or the recorder when the path matters.
//   - The context starts as the machine's configured initial context, not
//     whatever a real run would have accumulated. Set up any context the
//     assertion depends on with Interpreter.UpdateContext.
//
// Compound states resolve to their initial leaf, matching what Start would do.
//
// InterpreterAt panics rather than returning an error, in keeping with the
// other constructors in this package (MustBuild, QuickMachine): in a test, a
// state ID that does not exist is a broken test, not a condition to handle.
// Parallel states are not supported — restoring one needs per-region leaves
// that a single state ID cannot express.
func InterpreterAt[C any](machine *statekit.MachineConfig[C], state statekit.StateID) *statekit.Interpreter[C] {
	if machine == nil {
		panic("InterpreterAt: machine is nil")
	}

	target := machine.GetState(state)
	if target == nil {
		panic(fmt.Sprintf("InterpreterAt: state %q not found in machine %q", state, machine.ID))
	}
	if target.IsParallel() {
		panic(fmt.Sprintf("InterpreterAt: state %q is a parallel state; "+
			"restore it with Interpreter.Restore and an explicit ActiveInParallel map", state))
	}

	leaf := machine.GetInitialLeaf(state)

	interp := statekit.NewInterpreter(machine)
	if err := interp.Restore(statekit.Snapshot[C]{
		MachineID:    machine.ID,
		CurrentState: leaf,
		Context:      machine.Context,
		CreatedAt:    time.Now(),
	}); err != nil {
		panic(fmt.Sprintf("InterpreterAt: positioning at %q: %v", state, err))
	}

	return interp
}

// AssertTerminal asserts that the interpreter is in a final state and that
// none of the given events move it out — the negative property that a final
// state accepts nothing.
//
// Pass every event the machine defines anywhere, not only the ones plausible
// for this state; the point is that none of them apply.
//
//	interp := statetest.InterpreterAt(BuildLifecycle(), "archived")
//	defer interp.Close()
//	statetest.AssertTerminal(t, interp, "SUBMIT", "APPROVE", "PUBLISH", "ARCHIVE")
//
// Calling it with no events checks only that the interpreter is done, which
// AssertDone already says more directly.
func AssertTerminal[C any](t testing.TB, interp *statekit.Interpreter[C], events ...statekit.EventType) {
	t.Helper()

	start := interp.State().Value
	if !interp.Done() {
		t.Errorf("expected state %q to be final, but the interpreter is not done", start)
		return
	}

	for _, e := range events {
		interp.Send(statekit.Event{Type: e})
		if got := interp.State().Value; got != start {
			t.Errorf("final state %q accepted event %q and moved to %q", start, e, got)
			return
		}
		if !interp.Done() {
			t.Errorf("final state %q stopped being done after event %q", start, e)
			return
		}
	}
}
