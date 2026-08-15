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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	parent.Stop()
}

func TestActorRef_Stop(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	parentMachine := buildParentMachine()
	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	_, _ = Spawn(parent, "worker", childMachine)

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
	t.Parallel()
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
	t.Parallel()
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

	waitUntil(t, time.Second, func() bool {
		return parentInterp.GetActor(spawnedActorID) != nil
	})

	// Transition to other state - actor should be stopped
	parentInterp.Send(Event{Type: "NEXT"})

	waitUntil(t, time.Second, func() bool {
		return parentInterp.GetActor(spawnedActorID) == nil
	})

	parentInterp.Stop()
}

func TestAutoForward(t *testing.T) {
	t.Parallel()
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
	_, _ = Spawn(parent, "worker", childMachine,
		WithAutoForward("FORWARD_ME"),
	)

	// Send events to parent
	parent.Send(Event{Type: "FORWARD_ME"})
	parent.Send(Event{Type: "DONT_FORWARD"})

	waitUntil(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, et := range receivedEvents {
			if et == "FORWARD_ME" {
				return true
			}
		}
		return false
	})

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	_ = ref.Send(Event{Type: "COMPLETE"})

	waitUntil(t, time.Second, func() bool {
		return parent.State().Value == "completed"
	})

	parent.Stop()
}

func TestConcurrentSpawning(t *testing.T) {
	t.Parallel()
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

func TestSpawn_WithOnError(t *testing.T) {
	t.Parallel()
	// Test that WithOnError option can be set (error path reserved for future use)
	parentMachine, _ := NewMachine[parentContext]("parent").
		WithInitial("active").
		State("active").
		On("xstate.error.actor.worker").Target("failed").
		Done().
		State("failed").Final().
		Done().
		Build()

	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	// Spawn with OnError configured
	ref, err := Spawn(parent, "worker", childMachine,
		WithOnError("failed"),
		WithSupervision(SupervisionEscalate),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Verify actor was spawned successfully
	if ref == nil {
		t.Fatal("expected non-nil actor ref")
	}
	if ref.ID() != "worker" {
		t.Errorf("expected actor ID 'worker', got %s", ref.ID())
	}

	// Actor should be running
	select {
	case <-ref.Done():
		t.Error("actor should not be done yet")
	default:
		// Expected - actor is still running
	}

	parent.Stop()
}

func TestSpawn_AllOptions(t *testing.T) {
	t.Parallel()
	// Test spawning with all options combined
	parentMachine, _ := NewMachine[parentContext]("parent").
		WithInitial("active").
		State("active").
		On("xstate.done.actor.worker").Target("completed").
		On("xstate.error.actor.worker").Target("failed").
		On("TASK").Target("active"). // For auto-forward testing
		Done().
		State("completed").Final().
		Done().
		State("failed").Final().
		Done().
		Build()

	childMachine := buildSimpleChildMachine()

	parent := NewInterpreter(parentMachine)
	parent.Start()

	ref, err := Spawn(parent, "worker", childMachine,
		WithSupervision(SupervisionRecover),
		WithAutoForward("TASK", "DATA"),
		WithOnDone("completed"),
		WithOnError("failed"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if ref.ID() != "worker" {
		t.Errorf("expected actor ID 'worker', got %s", ref.ID())
	}

	parent.Stop()
}
