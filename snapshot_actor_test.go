package statekit_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

func TestSnapshot_CapturesActorMetadata(t *testing.T) {
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

	// Create parent machine
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		State("processing").
		On("STOP").Target("done").End().
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	// Spawn an actor
	_, err = statekit.Spawn(interp, "worker-1", childMachine,
		statekit.WithSupervision(statekit.SupervisionRecover),
		statekit.WithAutoForward("DATA"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Give actor time to start
	time.Sleep(50 * time.Millisecond)

	// Take snapshot
	snap := interp.Snapshot()

	// Verify actor metadata is captured
	if len(snap.SpawnedActors) != 1 {
		t.Fatalf("expected 1 spawned actor, got %d", len(snap.SpawnedActors))
	}

	actor := snap.SpawnedActors[0]
	if actor.ID != "worker-1" {
		t.Errorf("expected actor ID worker-1, got %s", actor.ID)
	}
	if actor.SpawnedInState != "processing" {
		t.Errorf("expected spawned in processing, got %s", actor.SpawnedInState)
	}
	if actor.Supervision != statekit.SupervisionRecover {
		t.Errorf("expected SupervisionRecover, got %v", actor.Supervision)
	}
	if len(actor.AutoForward) != 1 || actor.AutoForward[0] != "DATA" {
		t.Errorf("expected AutoForward = [DATA], got %v", actor.AutoForward)
	}

	interp.Stop()
}

func TestSnapshot_MultipleActors(t *testing.T) {
	type ctx struct{}

	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("active").
		State("active").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		State("processing").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	// Spawn multiple actors
	_, err = statekit.Spawn(interp, "worker-1", childMachine)
	if err != nil {
		t.Fatal(err)
	}
	_, err = statekit.Spawn(interp, "worker-2", childMachine)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	snap := interp.Snapshot()

	if len(snap.SpawnedActors) != 2 {
		t.Fatalf("expected 2 spawned actors, got %d", len(snap.SpawnedActors))
	}

	// Check both actors are captured (order may vary)
	foundIDs := make(map[statekit.ActorID]bool)
	for _, actor := range snap.SpawnedActors {
		foundIDs[actor.ID] = true
	}

	if !foundIDs["worker-1"] || !foundIDs["worker-2"] {
		t.Errorf("expected worker-1 and worker-2, got %v", foundIDs)
	}

	interp.Stop()
}

func TestSnapshot_NoActors(t *testing.T) {
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	snap := interp.Snapshot()

	if snap.SpawnedActors != nil {
		t.Errorf("expected nil SpawnedActors, got %v", snap.SpawnedActors)
	}

	interp.Stop()
}

func TestSnapshot_ActorMetadataSerializable(t *testing.T) {
	type ctx struct{}

	childMachine, err := statekit.NewMachine[ctx]("child").
		WithInitial("active").
		State("active").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		State("processing").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(parent)
	interp.Start()

	_, err = statekit.Spawn(interp, "worker-1", childMachine,
		statekit.WithSupervision(statekit.SupervisionRestart),
		statekit.WithAutoForward("EVENT_A", "EVENT_B"),
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	snap := interp.Snapshot()

	// Serialize to JSON
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}

	// Deserialize back
	var restored statekit.Snapshot[ctx]
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	// Verify actor metadata is preserved
	if len(restored.SpawnedActors) != 1 {
		t.Fatalf("expected 1 actor after round-trip, got %d", len(restored.SpawnedActors))
	}

	actor := restored.SpawnedActors[0]
	if actor.ID != "worker-1" {
		t.Errorf("expected actor ID worker-1, got %s", actor.ID)
	}
	if actor.Supervision != statekit.SupervisionRestart {
		t.Errorf("expected SupervisionRestart, got %v", actor.Supervision)
	}
	if len(actor.AutoForward) != 2 {
		t.Errorf("expected 2 auto-forward events, got %d", len(actor.AutoForward))
	}

	interp.Stop()
}

func TestSnapshot_InvokedMachineMetadata(t *testing.T) {
	type ctx struct{}

	// Create child machine
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

	// Create parent that invokes child
	parent, err := statekit.NewMachine[ctx]("parent").
		WithInitial("processing").
		WithChildMachine("worker", func(parentCtx ctx, parentSend func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("processing").
		InvokeMachine("worker").ID("w1").End().
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

	// Give invoked machine time to start
	time.Sleep(50 * time.Millisecond)

	snap := interp.Snapshot()

	// Invoked machines are tracked separately from manually spawned actors
	// This test just verifies the snapshot doesn't break with invoked machines
	if snap.CurrentState != "processing" {
		t.Errorf("expected processing, got %s", snap.CurrentState)
	}

	interp.Stop()
}
