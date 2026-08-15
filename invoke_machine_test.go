package statekit_test

import (
	"testing"
	"time"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/internal/ir"
)

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	const timeout = time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// Helper to create a child machine that processes work
func createWorkerChildMachine() *ir.MachineConfig[struct{}] {
	machine, err := statekit.NewMachine[struct{}]("worker").
		WithInitial("working").
		State("working").
		On("FINISH").Target("completed").End().
		Done().
		State("completed").Final().
		Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}

func TestInvokeMachine_Basic(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	// Create child machine
	childMachine := createWorkerChildMachine()

	// Create parent machine that invokes child
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("idle").
		WithChildMachine("worker", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			interp := statekit.NewInterpreter(childMachine)
			return interp
		}).
		State("idle").
		On("START").Target("processing").End().
		Done().
		State("processing").
		InvokeMachine("worker").
		OnDone("completed").
		End().
		Done().
		State("completed").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	if interp.State().Value != "idle" {
		t.Errorf("expected idle, got %s", interp.State().Value)
	}

	// Transition to processing - this starts the child machine
	interp.Send(statekit.Event{Type: "START"})

	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	// The child machine should have started; parent must stay in processing.
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if interp.State().Value != "processing" {
			t.Fatalf("expected processing, got %s", interp.State().Value)
		}
		time.Sleep(time.Millisecond)
	}

	interp.Stop()
}

func TestInvokeMachine_OnDone(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	// Create a child machine that starts in "working" and can transition to "done" (final)
	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("working").
		State("working").
		On("COMPLETE").Target("done").End().
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	var childInterp *statekit.Interpreter[ctx]

	// Create parent machine
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		WithChildMachine("worker", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			childInterp = statekit.NewInterpreter(childMachine)
			return childInterp
		}).
		State("processing").
		InvokeMachine("worker").
		OnDone("completed").
		End().
		Done().
		State("completed").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	// Should start in processing and child should be running
	waitUntil(t, func() bool {
		return childInterp != nil && string(childInterp.State().Value) == "working"
	})

	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	// Send COMPLETE to child to make it reach final state
	if childInterp != nil {
		childInterp.Send(statekit.Event{Type: "COMPLETE"})
	}

	// Wait for OnDone transition
	waitUntil(t, func() bool { return interp.State().Value == "completed" })

	// Parent should have transitioned to completed
	if interp.State().Value != "completed" {
		t.Errorf("expected completed, got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("expected parent to be in final state")
	}
}

func TestInvokeMachine_StopOnParentExit(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("working").
		State("working").
		On("FINISH").Target("done").End().
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Create parent machine
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		WithChildMachine("worker", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("processing").
		InvokeMachine("worker").End().
		On("CANCEL").Target("cancelled").End().
		Done().
		State("cancelled").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	// Give child time to start
	waitUntil(t, func() bool { return interp.State().Value == "processing" })

	// Exit the processing state - this should stop the child
	interp.Send(statekit.Event{Type: "CANCEL"})

	waitUntil(t, func() bool { return interp.State().Value == "cancelled" })

	// Child should have been stopped (exit action called)
	// Note: We can't directly verify the child was stopped, but we verify
	// the parent transitioned correctly
	if interp.State().Value != "cancelled" {
		t.Errorf("expected cancelled, got %s", interp.State().Value)
	}
}

func TestInvokeMachine_WithID(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("active").
		State("active").
		On("DONE").Target("finished").End().
		Done().
		State("finished").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Create parent machine with explicit invoke ID
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		WithChildMachine("childRef", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("processing").
		InvokeMachine("childRef").
		ID("myWorker").
		OnDone("completed").
		End().
		Done().
		State("completed").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	interp.Stop()
}

func TestInvokeMachine_MultipleInvocations(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("active").
		State("active").
		On("DONE").Target("finished").End().
		Done().
		State("finished").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Create parent with multiple child invocations
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		WithChildMachine("worker1", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		WithChildMachine("worker2", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("processing").
		InvokeMachine("worker1").ID("w1").End().
		InvokeMachine("worker2").ID("w2").End().
		On("STOP").Target("stopped").End().
		Done().
		State("stopped").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	// Give children time to start
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if interp.State().Value != "processing" {
			t.Fatalf("expected processing, got %s", interp.State().Value)
		}
		time.Sleep(time.Millisecond)
	}

	// Stop should clean up both children
	interp.Send(statekit.Event{Type: "STOP"})

	waitUntil(t, func() bool { return interp.State().Value == "stopped" })

	if interp.State().Value != "stopped" {
		t.Errorf("expected stopped, got %s", interp.State().Value)
	}
}

func TestInvokeMachine_BuilderValidation(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	// Build should succeed even if child machine ref doesn't exist
	// (validation happens at runtime when invoking)
	machine, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		State("processing").
		InvokeMachine("nonexistent").OnDone("done").End().
		Done().
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("build should succeed: %v", err)
	}

	// Machine should be created (validation is at runtime)
	if machine == nil {
		t.Error("expected machine to be non-nil")
	}
}
