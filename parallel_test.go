package statekit

import (
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/statekit/export"
)

// TestParallelState_Basic tests basic parallel state entry
func TestParallelState_Basic(t *testing.T) {
	machine, err := NewMachine[struct{}]("parallel_basic").
		WithInitial("active").
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_idle").
		State("r1_idle").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_idle").
		State("r2_idle").EndState().
		EndRegion().
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should be in the parallel state
	if interp.State().Value != "active" {
		t.Errorf("Expected state 'active', got %s", interp.State().Value)
	}

	// Both regions should have their initial states active
	if len(interp.State().ActiveInParallel) != 2 {
		t.Errorf("Expected 2 active regions, got %d", len(interp.State().ActiveInParallel))
	}

	// Check specific region states
	if interp.State().ActiveInParallel["region1"] != "r1_idle" {
		t.Errorf("Expected region1 state 'r1_idle', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_idle" {
		t.Errorf("Expected region2 state 'r2_idle', got %s", interp.State().ActiveInParallel["region2"])
	}

	interp.Stop()
}

// TestParallelState_Matches tests the Matches function with parallel states
func TestParallelState_Matches(t *testing.T) {
	machine, err := NewMachine[struct{}]("parallel_matches").
		WithInitial("active").
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_idle").
		State("r1_idle").EndState().
		State("r1_working").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_idle").
		State("r2_idle").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should match the parallel state
	if !interp.Matches("active") {
		t.Error("Expected to match 'active'")
	}

	// Should match states in regions
	if !interp.Matches("r1_idle") {
		t.Error("Expected to match 'r1_idle'")
	}
	if !interp.Matches("r2_idle") {
		t.Error("Expected to match 'r2_idle'")
	}

	// Should not match non-active states
	if interp.Matches("r1_working") {
		t.Error("Should not match 'r1_working'")
	}

	interp.Stop()
}

// TestParallelState_EventBroadcast tests event broadcasting to regions
func TestParallelState_EventBroadcast(t *testing.T) {
	type Context struct {
		Region1Events int
		Region2Events int
	}

	machine, err := NewMachine[Context]("parallel_broadcast").
		WithInitial("active").
		WithAction("incR1", func(ctx *Context, e Event) {
			ctx.Region1Events++
		}).
		WithAction("incR2", func(ctx *Context, e Event) {
			ctx.Region2Events++
		}).
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_idle").
		State("r1_idle").
		On("GO").Target("r1_working").Do("incR1").
		EndState().
		State("r1_working").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_idle").
		State("r2_idle").
		On("GO").Target("r2_working").Do("incR2").
		EndState().
		State("r2_working").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Both regions should be in idle
	if interp.State().ActiveInParallel["region1"] != "r1_idle" {
		t.Errorf("Expected region1 'r1_idle', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_idle" {
		t.Errorf("Expected region2 'r2_idle', got %s", interp.State().ActiveInParallel["region2"])
	}

	// Send event - should be broadcast to both regions
	interp.Send(Event{Type: "GO"})

	// Both regions should transition
	if interp.State().ActiveInParallel["region1"] != "r1_working" {
		t.Errorf("Expected region1 'r1_working', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_working" {
		t.Errorf("Expected region2 'r2_working', got %s", interp.State().ActiveInParallel["region2"])
	}

	// Both actions should have executed
	if interp.State().Context.Region1Events != 1 {
		t.Errorf("Expected Region1Events 1, got %d", interp.State().Context.Region1Events)
	}
	if interp.State().Context.Region2Events != 1 {
		t.Errorf("Expected Region2Events 1, got %d", interp.State().Context.Region2Events)
	}

	interp.Stop()
}

// TestParallelState_IndependentTransitions tests regions transitioning independently
func TestParallelState_IndependentTransitions(t *testing.T) {
	machine, err := NewMachine[struct{}]("parallel_independent").
		WithInitial("active").
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_idle").
		State("r1_idle").
		On("R1_GO").Target("r1_working").
		EndState().
		State("r1_working").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_idle").
		State("r2_idle").
		On("R2_GO").Target("r2_working").
		EndState().
		State("r2_working").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Send event only to region1
	interp.Send(Event{Type: "R1_GO"})

	// Only region1 should transition
	if interp.State().ActiveInParallel["region1"] != "r1_working" {
		t.Errorf("Expected region1 'r1_working', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_idle" {
		t.Errorf("Expected region2 still 'r2_idle', got %s", interp.State().ActiveInParallel["region2"])
	}

	// Now send event to region2
	interp.Send(Event{Type: "R2_GO"})

	// Both should now be working
	if interp.State().ActiveInParallel["region1"] != "r1_working" {
		t.Errorf("Expected region1 still 'r1_working', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_working" {
		t.Errorf("Expected region2 'r2_working', got %s", interp.State().ActiveInParallel["region2"])
	}

	interp.Stop()
}

// TestParallelState_ExitOnParentTransition tests exiting parallel via parent transition
func TestParallelState_ExitOnParentTransition(t *testing.T) {
	type Context struct {
		EntryCount int
		ExitCount  int
	}

	machine, err := NewMachine[Context]("parallel_exit").
		WithInitial("active").
		WithAction("incEntry", func(ctx *Context, e Event) {
			ctx.EntryCount++
		}).
		WithAction("incExit", func(ctx *Context, e Event) {
			ctx.ExitCount++
		}).
		State("active").Parallel().
		OnEntry("incEntry").
		OnExit("incExit").
		On("CANCEL").Target("cancelled").End().
		Region("region1").
		WithInitial("r1_working").
		State("r1_working").
		OnEntry("incEntry").
		OnExit("incExit").
		EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_working").
		State("r2_working").
		OnEntry("incEntry").
		OnExit("incExit").
		EndState().
		EndRegion().
		Done().
		State("cancelled").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Entry actions: parallel + r1_working + r2_working = 3
	// (regions are compound states but only have their children's entry actions)
	if interp.State().Context.EntryCount != 3 {
		t.Errorf("Expected EntryCount 3, got %d", interp.State().Context.EntryCount)
	}

	// Send CANCEL to exit parallel state
	interp.Send(Event{Type: "CANCEL"})

	// Should now be in cancelled
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected state 'cancelled', got %s", interp.State().Value)
	}

	// Exit actions: r1_working + region1 + r2_working + region2 = 4, then parallel exit is not in the filtered list
	// The exitRegion filters to states within the region, which includes region1/region2 themselves
	// Total: r1_working + r2_working + parallel = 4 (regions don't have exit actions)
	if interp.State().Context.ExitCount != 4 {
		t.Errorf("Expected ExitCount 4, got %d", interp.State().Context.ExitCount)
	}

	// Parallel tracking should be cleared
	if len(interp.State().ActiveInParallel) != 0 {
		t.Errorf("Expected empty ActiveInParallel, got %d entries", len(interp.State().ActiveInParallel))
	}

	interp.Stop()
}

// TestParallelState_EntryOrder tests entry action ordering
func TestParallelState_EntryOrder(t *testing.T) {
	type Context struct {
		Order []string
	}

	machine, err := NewMachine[Context]("parallel_entry_order").
		WithInitial("active").
		WithAction("enterActive", func(ctx *Context, e Event) {
			ctx.Order = append(ctx.Order, "active")
		}).
		WithAction("enterR1", func(ctx *Context, e Event) {
			ctx.Order = append(ctx.Order, "region1")
		}).
		WithAction("enterR1Idle", func(ctx *Context, e Event) {
			ctx.Order = append(ctx.Order, "r1_idle")
		}).
		WithAction("enterR2", func(ctx *Context, e Event) {
			ctx.Order = append(ctx.Order, "region2")
		}).
		WithAction("enterR2Idle", func(ctx *Context, e Event) {
			ctx.Order = append(ctx.Order, "r2_idle")
		}).
		State("active").Parallel().
		OnEntry("enterActive").
		Region("region1").
		WithInitial("r1_idle").
		State("r1_idle").OnEntry("enterR1Idle").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_idle").
		State("r2_idle").OnEntry("enterR2Idle").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Parallel state entry should come first
	if len(interp.State().Context.Order) < 1 || interp.State().Context.Order[0] != "active" {
		t.Errorf("Expected 'active' to be first entry, got %v", interp.State().Context.Order)
	}

	interp.Stop()
}

// TestParallelState_XStateExport tests XState JSON export of parallel states
func TestParallelState_XStateExport(t *testing.T) {
	machine, err := NewMachine[struct{}]("export_parallel").
		WithInitial("active").
		State("active").Parallel().
		Region("upload").
		WithInitial("pending").
		State("pending").
		On("START").Target("uploading").
		EndState().
		State("uploading").EndState().
		State("complete").Final().EndState().
		EndRegion().
		Region("download").
		WithInitial("waiting").
		State("waiting").
		On("START").Target("downloading").
		EndState().
		State("downloading").EndState().
		State("finished").Final().EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	exporter := export.NewXStateExporter(machine)
	exported, err := exporter.Export()
	if err != nil {
		t.Fatalf("Failed to export: %v", err)
	}

	// Check the active state has type "parallel"
	activeState := exported.States["active"]
	if activeState.Type != "parallel" {
		t.Errorf("Expected type 'parallel', got '%s'", activeState.Type)
	}

	// Check it has nested states (regions)
	if activeState.States == nil {
		t.Fatal("Expected nested states in parallel state")
	}
	if _, ok := activeState.States["upload"]; !ok {
		t.Error("Expected 'upload' region")
	}
	if _, ok := activeState.States["download"]; !ok {
		t.Error("Expected 'download' region")
	}

	// Check regions have their nested states
	uploadRegion := activeState.States["upload"]
	if uploadRegion.Initial != "pending" {
		t.Errorf("Expected upload initial 'pending', got '%s'", uploadRegion.Initial)
	}
	if _, ok := uploadRegion.States["pending"]; !ok {
		t.Error("Expected 'pending' state in upload region")
	}

	// Verify JSON structure
	jsonStr, err := exporter.ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("Failed to export JSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}

	states := parsed["states"].(map[string]any)
	active := states["active"].(map[string]any)

	if active["type"] != "parallel" {
		t.Errorf("Expected JSON type 'parallel', got '%v'", active["type"])
	}
}

// TestParallelState_Validation tests validation rules for parallel states
func TestParallelState_Validation(t *testing.T) {
	t.Run("parallel with no regions fails", func(t *testing.T) {
		_, err := NewMachine[struct{}]("no_regions").
			WithInitial("active").
			State("active").Parallel().
			Done().
			Build()

		if err == nil {
			t.Error("Expected validation error for parallel state with no regions")
		}
	})

	t.Run("parallel with valid regions succeeds", func(t *testing.T) {
		_, err := NewMachine[struct{}]("valid_parallel").
			WithInitial("active").
			State("active").Parallel().
			Region("r1").
			WithInitial("s1").
			State("s1").EndState().
			EndRegion().
			Done().
			Build()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

// TestParallelState_TransitionToParallel tests transitioning into a parallel state
func TestParallelState_TransitionToParallel(t *testing.T) {
	machine, err := NewMachine[struct{}]("transition_to_parallel").
		WithInitial("idle").
		State("idle").
		On("START").Target("active").
		Done().
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_working").
		State("r1_working").EndState().
		EndRegion().
		Region("region2").
		WithInitial("r2_working").
		State("r2_working").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in idle
	if interp.State().Value != "idle" {
		t.Errorf("Expected state 'idle', got %s", interp.State().Value)
	}

	// Transition to parallel state
	interp.Send(Event{Type: "START"})

	// Should now be in parallel state
	if interp.State().Value != "active" {
		t.Errorf("Expected state 'active', got %s", interp.State().Value)
	}

	// Both regions should be active
	if interp.State().ActiveInParallel["region1"] != "r1_working" {
		t.Errorf("Expected region1 'r1_working', got %s", interp.State().ActiveInParallel["region1"])
	}
	if interp.State().ActiveInParallel["region2"] != "r2_working" {
		t.Errorf("Expected region2 'r2_working', got %s", interp.State().ActiveInParallel["region2"])
	}

	interp.Stop()
}

// TestParallelState_SimpleWithTransitions tests parallel state with basic transitions
func TestParallelState_SimpleWithTransitions(t *testing.T) {
	machine, err := NewMachine[struct{}]("parallel_simple").
		WithInitial("active").
		State("active").Parallel().
		Region("region1").
		WithInitial("r1_a").
		State("r1_a").
		On("ADVANCE").Target("r1_b").
		EndState().
		State("r1_b").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should be in the initial state
	if interp.State().ActiveInParallel["region1"] != "r1_a" {
		t.Errorf("Expected region1 'r1_a', got %s", interp.State().ActiveInParallel["region1"])
	}

	// Transition within region
	interp.Send(Event{Type: "ADVANCE"})

	if interp.State().ActiveInParallel["region1"] != "r1_b" {
		t.Errorf("Expected region1 'r1_b', got %s", interp.State().ActiveInParallel["region1"])
	}

	interp.Stop()
}
