package statekit

import (
	"sync"
	"testing"
	"time"
)

// Test contexts for parent and child machines
type parentContext struct {
	Count     int
	ChildDone bool
}

type childContext struct {
	TaskID int
	Result string
}

// buildSimpleChildMachine creates a simple child machine for testing
func buildSimpleChildMachine() *MachineConfig[childContext] {
	machine, _ := NewMachine[childContext]("child").
		WithInitial("idle").
		State("idle").
		On("TASK").Target("working").
		Done().
		State("working").
		On("COMPLETE").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()
	return machine
}

// buildParentMachine creates a parent machine for testing
func buildParentMachine() *MachineConfig[parentContext] {
	machine, _ := NewMachine[parentContext]("parent").
		WithInitial("active").
		State("active").
		On("SPAWN_CHILD").Target("active"). // Self-transition for spawn trigger
		On("xstate.done.actor.worker").Target("completed").
		Done().
		State("completed").Final().
		Done().
		Build()
	return machine
}

func TestSpawn_Basic(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn a child actor
	ref, err := Spawn(parent, "worker", childMachine)
	if err != nil {
		t.Fatalf("failed to spawn: %v", err)
	}

	if ref == nil {
		t.Fatal("expected non-nil ref")
	}

	if ref.ID() != "worker" {
		t.Errorf("expected ID 'worker', got %s", ref.ID())
	}

	// Verify actor is registered
	foundRef := parent.GetActor("worker")
	if foundRef == nil {
		t.Error("actor not found in registry")
	}

	// Clean up
	parent.Stop()
}

func TestSpawn_DuplicateID(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn first child
	_, err := Spawn(parent, "worker", childMachine)
	if err != nil {
		t.Fatalf("first spawn failed: %v", err)
	}

	// Try to spawn with same ID
	_, err = Spawn(parent, "worker", childMachine)
	if err != ErrActorAlreadyExists {
		t.Errorf("expected ErrActorAlreadyExists, got %v", err)
	}

	parent.Stop()
}

func TestActorRef_Send(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref, _ := Spawn(parent, "worker", childMachine)

	// Send event to child
	err := ref.Send(Event{Type: "TASK"})
	if err != nil {
		t.Errorf("failed to send: %v", err)
	}

	// Give child time to process
	time.Sleep(10 * time.Millisecond)

	parent.Stop()
}

func TestActorRef_Stop(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref, _ := Spawn(parent, "worker", childMachine)

	// Stop the actor
	ref.Stop()

	// Wait for done channel
	select {
	case <-ref.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("done channel not closed after stop")
	}

	// Send should fail after stop
	err := ref.Send(Event{Type: "TASK"})
	if err != ErrActorStopped {
		t.Errorf("expected ErrActorStopped, got %v", err)
	}

	parent.Stop()
}

func TestActorRef_StopIdempotent(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref, _ := Spawn(parent, "worker", childMachine)

	// Stop multiple times should not panic
	ref.Stop()
	ref.Stop()
	ref.Stop()

	parent.Stop()
}

func TestSendTo(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	Spawn(parent, "worker", childMachine)

	// Send via parent's SendTo
	err := parent.SendTo("worker", Event{Type: "TASK"})
	if err != nil {
		t.Errorf("SendTo failed: %v", err)
	}

	// Send to non-existent actor
	err = parent.SendTo("unknown", Event{Type: "TASK"})
	if err != ErrActorNotFound {
		t.Errorf("expected ErrActorNotFound, got %v", err)
	}

	parent.Stop()
}

func TestSendParent_NoParent(t *testing.T) {
	machine, _ := NewMachine[struct{}]("root").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	// SendParent should fail for root interpreter
	err := interp.SendParent(Event{Type: "TEST"})
	if err != ErrNoParent {
		t.Errorf("expected ErrNoParent, got %v", err)
	}

	interp.Stop()
}

func TestStateScopedLifecycle(t *testing.T) {
	// Create a machine that spawns actors in a specific state
	var spawnedActorID ActorID
	var parentInterp *Interpreter[struct{}]

	machine, _ := NewMachine[struct{}]("parent").
		WithInitial("spawning").
		WithAction("spawn", func(ctx *struct{}, e Event) {
			// Spawn actor when entering the state
			if parentInterp != nil {
				childMachine := buildSimpleChildMachine()
				ref, _ := Spawn(parentInterp, "scoped-worker", childMachine)
				if ref != nil {
					spawnedActorID = ref.ID()
				}
			}
		}).
		State("spawning").
		OnEntry("spawn").
		On("NEXT").Target("other").
		Done().
		State("other").Done().
		Build()

	parentInterp = NewInterpreter(machine)
	parentInterp.Start()

	// Give spawn time to complete
	time.Sleep(10 * time.Millisecond)

	// Verify actor exists
	if parentInterp.GetActor(spawnedActorID) == nil {
		t.Error("actor should exist in spawning state")
	}

	// Transition to other state - actor should be stopped
	parentInterp.Send(Event{Type: "NEXT"})

	// Give cleanup time
	time.Sleep(10 * time.Millisecond)

	// Actor should be gone
	if parentInterp.GetActor(spawnedActorID) != nil {
		t.Error("actor should be stopped when exiting spawning state")
	}

	parentInterp.Stop()
}

func TestAutoForward(t *testing.T) {
	parentMachine := buildParentMachine()

	// Child that tracks received events
	var receivedEvents []EventType
	var mu sync.Mutex

	childMachine, _ := NewMachine[struct{}]("child").
		WithInitial("listening").
		WithAction("track", func(ctx *struct{}, e Event) {
			mu.Lock()
			receivedEvents = append(receivedEvents, e.Type)
			mu.Unlock()
		}).
		State("listening").
		On("FORWARD_ME").Target("listening").Do("track").
		On("DONT_FORWARD").Target("listening").Do("track").
		Done().
		Build()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn with auto-forward for FORWARD_ME only
	Spawn(parent, "worker", childMachine,
		WithAutoForward("FORWARD_ME"),
	)

	// Give spawn time
	time.Sleep(10 * time.Millisecond)

	// Send events to parent
	parent.Send(Event{Type: "FORWARD_ME"})
	parent.Send(Event{Type: "DONT_FORWARD"})

	// Give processing time
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	hasForwardMe := false
	hasDontForward := false
	for _, et := range receivedEvents {
		if et == "FORWARD_ME" {
			hasForwardMe = true
		}
		if et == "DONT_FORWARD" {
			hasDontForward = true
		}
	}
	mu.Unlock()

	if !hasForwardMe {
		t.Error("FORWARD_ME should have been forwarded to child")
	}
	if hasDontForward {
		t.Error("DONT_FORWARD should NOT have been forwarded to child")
	}

	parent.Stop()
}

func TestSpawn_NotStarted(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	// Don't start

	_, err := Spawn(parent, "worker", childMachine)
	if err == nil {
		t.Error("expected error when spawning on unstarted interpreter")
	}
}

func TestParentStop_StopsAllActors(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref1, _ := Spawn(parent, "worker1", childMachine)
	ref2, _ := Spawn(parent, "worker2", childMachine)

	// Stop parent
	parent.Stop()

	// Both actors should be stopped
	select {
	case <-ref1.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("worker1 not stopped")
	}

	select {
	case <-ref2.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("worker2 not stopped")
	}
}

func TestSpawnWithContext(t *testing.T) {
	type parentCtx struct {
		BaseValue int
	}
	type childCtx struct {
		DerivedValue int
	}

	parentMachine, _ := NewMachine[parentCtx]("parent").
		WithInitial("active").
		WithContext(parentCtx{BaseValue: 42}).
		State("active").Done().
		Build()

	childMachine, _ := NewMachine[childCtx]("child").
		WithInitial("idle").
		State("idle").Done().
		Build()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn with context derivation
	ref, err := SpawnWithContext(parent, "worker", childMachine,
		func(p parentCtx) childCtx {
			return childCtx{DerivedValue: p.BaseValue * 2}
		},
	)

	if err != nil {
		t.Fatalf("SpawnWithContext failed: %v", err)
	}
	if ref == nil {
		t.Fatal("expected non-nil ref")
	}

	parent.Stop()
}

func TestActorDone_NotifiesParent(t *testing.T) {
	parentMachine, _ := NewMachine[struct{}]("parent").
		WithInitial("active").
		State("active").
		On("xstate.done.actor.worker").Target("completed").
		Done().
		State("completed").Final().
		Done().
		Build()

	childMachine, _ := NewMachine[struct{}]("child").
		WithInitial("idle").
		State("idle").
		On("COMPLETE").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref, _ := Spawn(parent, "worker", childMachine,
		WithOnDone("completed"),
	)

	// Complete the child
	ref.Send(Event{Type: "COMPLETE"})

	// Wait for child to finish and parent to transition
	time.Sleep(50 * time.Millisecond)

	// Parent should now be in completed state
	if parent.State().Value != "completed" {
		t.Errorf("expected parent in 'completed', got %s", parent.State().Value)
	}

	parent.Stop()
}

func TestConcurrentSpawning(t *testing.T) {
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn multiple actors concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			actorID := ActorID("worker-" + string(rune('0'+id)))
			_, err := Spawn(parent, actorID, childMachine)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("spawn error: %v", err)
	}

	parent.Stop()
}
