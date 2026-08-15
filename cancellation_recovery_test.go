package statekit

import (
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
)

// TestCancellation_ServiceCancelledOnExit_NoStuckTransition exercises
// the design corner that frustrated users of looplab/fsm in
// https://github.com/looplab/fsm/issues/115 — a cancelled context
// leaving the interpreter in a "transition in progress" stuck state.
//
// statekit does not tie an external context.Context to the public Send
// path — events are queued internally and transitions complete
// synchronously. The closest analog is an Invoke service whose context
// gets cancelled on state exit. After cancellation, subsequent Send
// calls must continue to drive the machine normally.
func TestCancellation_ServiceCancelledOnExit_NoStuckTransition(t *testing.T) {
	t.Parallel()
	// Service that returns an error — exercises the OnError path which
	// is the looplab-equivalent "transition encountered context error"
	// scenario.
	failing := func(_ ir.ServiceContext[struct{}]) error {
		return errors.New("simulated upstream cancellation")
	}

	machine, err := NewMachine[struct{}]("ctx-test").
		WithInitial("idle").
		WithService("flaky", failing).
		State("idle").
		On("GO").Target("working").
		Done().
		State("working").
		Invoke("flaky").ID("inv").OnError("recovery").End().
		Done().
		State("recovery").
		On("RETRY").Target("idle").
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	interp := NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(Event{Type: "GO"})

	// Service runs in a goroutine; give it a moment to surface the
	// error and route to recovery.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if string(interp.State().Value) == "recovery" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := string(interp.State().Value); got != "recovery" {
		t.Fatalf("after service error: state = %q, want recovery (no stuck transition)", got)
	}

	// The interpreter must still accept events — no "InTransitionError"
	// equivalent in statekit. This is the assertion looplab #115 cannot
	// make on their library.
	interp.Send(Event{Type: "RETRY"})
	if got := string(interp.State().Value); got != "idle" {
		t.Errorf("after RETRY: state = %q, want idle (interpreter recovered cleanly)", got)
	}
}

// TestCancellation_StopAndRestart_NewLifecycle confirms that Stop()
// cleanly cancels active services + timers and that a fresh
// interpreter (or Restore) can resume work without inheriting any
// "stuck" state from the previous cycle.
func TestCancellation_StopAndRestart_NewLifecycle(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[struct{}]("restart").
		WithInitial("loading").
		State("loading").
		After(time.Hour).Target("done"). // Long timer.
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	first := NewInterpreter(machine)
	first.Start()
	first.Send(Event{Type: "noop"})
	if err := first.Close(); err != nil {
		t.Fatalf("first.Close: %v", err)
	}

	// A second interpreter on the same MachineConfig must start in a
	// clean slate — no leakage from the first instance.
	second := NewInterpreter(machine)
	defer func() { _ = second.Close() }()
	second.Start()
	if got := string(second.State().Value); got != "loading" {
		t.Errorf("second interpreter state = %q, want loading", got)
	}
}
