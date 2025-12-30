package statekit_test

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

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

	// The child machine should have started
	// Give it time to start (async)
	time.Sleep(50 * time.Millisecond)

	// The child is not yet done, so parent should still be in processing
	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	interp.Stop()
}

func TestInvokeMachine_OnDone(t *testing.T) {
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
	time.Sleep(50 * time.Millisecond)

	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	// Send COMPLETE to child to make it reach final state
	if childInterp != nil {
		childInterp.Send(statekit.Event{Type: "COMPLETE"})
	}

	// Wait for OnDone transition
	time.Sleep(100 * time.Millisecond)

	// Parent should have transitioned to completed
	if interp.State().Value != "completed" {
		t.Errorf("expected completed, got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("expected parent to be in final state")
	}
}

func TestInvokeMachine_StopOnParentExit(t *testing.T) {
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
	time.Sleep(50 * time.Millisecond)

	// Exit the processing state - this should stop the child
	interp.Send(statekit.Event{Type: "CANCEL"})

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	// Child should have been stopped (exit action called)
	// Note: We can't directly verify the child was stopped, but we verify
	// the parent transitioned correctly
	if interp.State().Value != "cancelled" {
		t.Errorf("expected cancelled, got %s", interp.State().Value)
	}
}

func TestInvokeMachine_WithID(t *testing.T) {
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
	time.Sleep(50 * time.Millisecond)

	if interp.State().Value != "processing" {
		t.Errorf("expected processing, got %s", interp.State().Value)
	}

	// Stop should clean up both children
	interp.Send(statekit.Event{Type: "STOP"})

	time.Sleep(50 * time.Millisecond)

	if interp.State().Value != "stopped" {
		t.Errorf("expected stopped, got %s", interp.State().Value)
	}
}

func TestInvokeMachine_BuilderValidation(t *testing.T) {
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
