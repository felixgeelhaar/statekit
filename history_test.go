package statekit

import (
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/statekit/export"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// TestHistoryState_ShallowWithTransition tests shallow history with proper exit/enter
func TestHistoryState_ShallowWithTransition(t *testing.T) {
	machine, err := NewMachine[struct{}]("shallow_history").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		On("PAUSE").Target("paused").End().              // End returns to active
		History("hist").Shallow().Default("idle").End(). // History.End returns to active
		State("idle").
		On("START").Target("working").
		End(). // End transition, return to idle StateBuilder
		End(). // End idle StateBuilder, return to active StateBuilder
		State("working").
		On("FINISH").Target("done").
		End(). // End transition, return to working StateBuilder
		End(). // End working StateBuilder, return to active StateBuilder
		State("done").
		End().  // End done StateBuilder, return to active StateBuilder
		Done(). // End active, return to machine
		State("paused").
		On("RESUME").Target("hist").
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in idle (initial of active)
	if interp.State().Value != "idle" {
		t.Errorf("Expected initial state 'idle', got %s", interp.State().Value)
	}

	// Move to working
	interp.Send(Event{Type: "START"})
	if interp.State().Value != "working" {
		t.Errorf("Expected state 'working', got %s", interp.State().Value)
	}

	// Pause (exit active, enter paused)
	interp.Send(Event{Type: "PAUSE"})
	if interp.State().Value != "paused" {
		t.Errorf("Expected state 'paused', got %s", interp.State().Value)
	}

	// Resume via history - should go back to 'working' (the last child)
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "working" {
		t.Errorf("Expected state 'working' via history, got %s", interp.State().Value)
	}
}

// TestHistoryState_ShallowDefault tests that default is used when no history exists
func TestHistoryState_ShallowDefault(t *testing.T) {
	machine, err := NewMachine[struct{}]("shallow_default").
		WithInitial("paused").
		State("active").
		WithInitial("idle").
		On("PAUSE").Target("paused").End().
		History("hist").Shallow().Default("idle").End().
		State("idle").
		On("START").Target("working").
		End().
		End().
		State("working").
		End().
		Done().
		State("paused").
		On("RESUME").Target("hist").
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in paused
	if interp.State().Value != "paused" {
		t.Errorf("Expected initial state 'paused', got %s", interp.State().Value)
	}

	// Resume - no history exists, should use default (idle)
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "idle" {
		t.Errorf("Expected state 'idle' (default), got %s", interp.State().Value)
	}
}

// TestHistoryState_DeepBasic tests deep history that remembers the full leaf path
func TestHistoryState_DeepBasic(t *testing.T) {
	machine, err := NewMachine[struct{}]("deep_history").
		WithInitial("active").
		State("active").
		WithInitial("section1").
		On("PAUSE").Target("paused").End().
		History("hist").Deep().Default("section1").End().
		State("section1").
		WithInitial("step1").
		State("step1").
		On("NEXT").Target("step2").
		End().
		End().
		State("step2").
		On("NEXT").Target("step3").
		End().
		End().
		State("step3").
		End().
		End(). // End section1
		Done().
		State("paused").
		On("RESUME").Target("hist").
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in step1 (initial of section1 which is initial of active)
	if interp.State().Value != "step1" {
		t.Errorf("Expected initial state 'step1', got %s", interp.State().Value)
	}

	// Move to step2
	interp.Send(Event{Type: "NEXT"})
	if interp.State().Value != "step2" {
		t.Errorf("Expected state 'step2', got %s", interp.State().Value)
	}

	// Move to step3
	interp.Send(Event{Type: "NEXT"})
	if interp.State().Value != "step3" {
		t.Errorf("Expected state 'step3', got %s", interp.State().Value)
	}

	// Pause
	interp.Send(Event{Type: "PAUSE"})
	if interp.State().Value != "paused" {
		t.Errorf("Expected state 'paused', got %s", interp.State().Value)
	}

	// Resume via deep history - should go back to step3 (the full leaf)
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "step3" {
		t.Errorf("Expected state 'step3' via deep history, got %s", interp.State().Value)
	}
}

// TestHistoryState_Validation tests validation rules for history states
func TestHistoryState_Validation(t *testing.T) {
	t.Run("history state without parent fails", func(t *testing.T) {
		// Create a machine with a history state at root level (invalid)
		machine := ir.NewMachineConfig[struct{}]("test", "hist", struct{}{})
		machine.States["hist"] = &ir.StateConfig{
			ID:             "hist",
			Type:           ir.StateTypeHistory,
			Parent:         "", // No parent - invalid
			HistoryType:    ir.HistoryTypeShallow,
			HistoryDefault: "idle",
		}
		machine.States["idle"] = ir.NewStateConfig("idle", ir.StateTypeAtomic)

		err := ir.Validate(machine)
		if err == nil {
			t.Error("Expected validation error for history state without parent")
		}
	})

	t.Run("history state without default fails", func(t *testing.T) {
		// Create a valid compound state with history but no default
		machine := ir.NewMachineConfig[struct{}]("test", "active", struct{}{})
		machine.States["active"] = &ir.StateConfig{
			ID:       "active",
			Type:     ir.StateTypeCompound,
			Initial:  "idle",
			Children: []ir.StateID{"idle", "hist"},
		}
		machine.States["idle"] = &ir.StateConfig{
			ID:     "idle",
			Type:   ir.StateTypeAtomic,
			Parent: "active",
		}
		machine.States["hist"] = &ir.StateConfig{
			ID:             "hist",
			Type:           ir.StateTypeHistory,
			Parent:         "active",
			HistoryType:    ir.HistoryTypeShallow,
			HistoryDefault: "", // No default - invalid
		}

		err := ir.Validate(machine)
		if err == nil {
			t.Error("Expected validation error for history state without default")
		}
	})

	t.Run("history state with invalid default fails", func(t *testing.T) {
		machine := ir.NewMachineConfig[struct{}]("test", "active", struct{}{})
		machine.States["active"] = &ir.StateConfig{
			ID:       "active",
			Type:     ir.StateTypeCompound,
			Initial:  "idle",
			Children: []ir.StateID{"idle", "hist"},
		}
		machine.States["idle"] = &ir.StateConfig{
			ID:     "idle",
			Type:   ir.StateTypeAtomic,
			Parent: "active",
		}
		machine.States["hist"] = &ir.StateConfig{
			ID:             "hist",
			Type:           ir.StateTypeHistory,
			Parent:         "active",
			HistoryType:    ir.HistoryTypeShallow,
			HistoryDefault: "nonexistent", // Invalid target
		}

		err := ir.Validate(machine)
		if err == nil {
			t.Error("Expected validation error for history state with invalid default")
		}
	})
}

// TestHistoryState_XStateExport tests that history states export correctly to XState JSON
func TestHistoryState_XStateExport(t *testing.T) {
	machine, err := NewMachine[struct{}]("export_test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		History("hist").Shallow().Default("idle").End().
		History("deepHist").Deep().Default("idle").End().
		State("idle").
		On("START").Target("running").
		End().
		End().
		State("running").
		End().
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

	// Check the active state has both history states
	activeState := exported.States["active"]
	if activeState.States == nil {
		t.Fatal("Expected active state to have nested states")
	}

	// Check shallow history
	histState := activeState.States["hist"]
	if histState.Type != "history" {
		t.Errorf("Expected hist type 'history', got '%s'", histState.Type)
	}
	if histState.History != "shallow" {
		t.Errorf("Expected hist history 'shallow', got '%s'", histState.History)
	}
	if histState.Target != "idle" {
		t.Errorf("Expected hist target 'idle', got '%s'", histState.Target)
	}

	// Check deep history
	deepHistState := activeState.States["deepHist"]
	if deepHistState.Type != "history" {
		t.Errorf("Expected deepHist type 'history', got '%s'", deepHistState.Type)
	}
	if deepHistState.History != "deep" {
		t.Errorf("Expected deepHist history 'deep', got '%s'", deepHistState.History)
	}

	// Verify JSON structure
	jsonStr, err := exporter.ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("Failed to export JSON: %v", err)
	}

	// Parse back and verify
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}

	states := parsed["states"].(map[string]any)
	active := states["active"].(map[string]any)
	nestedStates := active["states"].(map[string]any)

	hist := nestedStates["hist"].(map[string]any)
	if hist["type"] != "history" {
		t.Errorf("JSON: Expected hist type 'history', got '%v'", hist["type"])
	}
	if hist["history"] != "shallow" {
		t.Errorf("JSON: Expected hist history 'shallow', got '%v'", hist["history"])
	}
}

// TestHistoryState_MultipleExitsAndReturns tests history across multiple exit/return cycles
func TestHistoryState_MultipleExitsAndReturns(t *testing.T) {
	machine, err := NewMachine[struct{}]("multiple_cycles").
		WithInitial("active").
		State("active").
		WithInitial("a").
		On("PAUSE").Target("paused").End().
		History("hist").Shallow().Default("a").End().
		State("a").
		On("NEXT").Target("b").
		End().
		End().
		State("b").
		On("NEXT").Target("c").
		End().
		End().
		State("c").
		On("NEXT").Target("a").
		End().
		Done().
		State("paused").
		On("RESUME").Target("hist").
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Start at 'a'
	if interp.State().Value != "a" {
		t.Errorf("Expected 'a', got %s", interp.State().Value)
	}

	// Move to 'b'
	interp.Send(Event{Type: "NEXT"})

	// Pause and resume - should return to 'b'
	interp.Send(Event{Type: "PAUSE"})
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "b" {
		t.Errorf("Expected 'b' after first resume, got %s", interp.State().Value)
	}

	// Move to 'c'
	interp.Send(Event{Type: "NEXT"})

	// Pause and resume - should return to 'c'
	interp.Send(Event{Type: "PAUSE"})
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "c" {
		t.Errorf("Expected 'c' after second resume, got %s", interp.State().Value)
	}

	// Move back to 'a'
	interp.Send(Event{Type: "NEXT"})

	// Pause and resume - should return to 'a'
	interp.Send(Event{Type: "PAUSE"})
	interp.Send(Event{Type: "RESUME"})
	if interp.State().Value != "a" {
		t.Errorf("Expected 'a' after third resume, got %s", interp.State().Value)
	}
}

// TestHistoryState_ShallowVsDeep tests the difference between shallow and deep history
func TestHistoryState_ShallowVsDeep(t *testing.T) {
	// Create a machine with nested states to test shallow vs deep
	machine, err := NewMachine[struct{}]("shallow_vs_deep").
		WithInitial("main").
		State("main").
		WithInitial("outer").
		On("EXIT").Target("outside").End().
		History("shallowHist").Shallow().Default("outer").End().
		History("deepHist").Deep().Default("outer").End().
		State("outer").
		WithInitial("inner1").
		State("inner1").
		On("NEXT").Target("inner2").
		End().
		End().
		State("inner2").
		End().
		End(). // End outer
		Done().
		State("outside").
		On("SHALLOW_RESUME").Target("shallowHist").
		On("DEEP_RESUME").Target("deepHist").
		Done().
		Build()
	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	// Test shallow history
	t.Run("shallow history returns to immediate child", func(t *testing.T) {
		interp := NewInterpreter(machine)
		interp.Start()

		// Should be in inner1
		if interp.State().Value != "inner1" {
			t.Errorf("Expected 'inner1', got %s", interp.State().Value)
		}

		// Move to inner2
		interp.Send(Event{Type: "NEXT"})
		if interp.State().Value != "inner2" {
			t.Errorf("Expected 'inner2', got %s", interp.State().Value)
		}

		// Exit to outside
		interp.Send(Event{Type: "EXIT"})
		if interp.State().Value != "outside" {
			t.Errorf("Expected 'outside', got %s", interp.State().Value)
		}

		// Shallow resume - should go to outer, then inner1 (initial of outer)
		interp.Send(Event{Type: "SHALLOW_RESUME"})
		if interp.State().Value != "inner1" {
			t.Errorf("Expected 'inner1' via shallow history, got %s", interp.State().Value)
		}
	})

	// Test deep history
	t.Run("deep history returns to exact leaf", func(t *testing.T) {
		interp := NewInterpreter(machine)
		interp.Start()

		// Should be in inner1
		if interp.State().Value != "inner1" {
			t.Errorf("Expected 'inner1', got %s", interp.State().Value)
		}

		// Move to inner2
		interp.Send(Event{Type: "NEXT"})
		if interp.State().Value != "inner2" {
			t.Errorf("Expected 'inner2', got %s", interp.State().Value)
		}

		// Exit to outside
		interp.Send(Event{Type: "EXIT"})
		if interp.State().Value != "outside" {
			t.Errorf("Expected 'outside', got %s", interp.State().Value)
		}

		// Deep resume - should go directly to inner2 (the exact leaf)
		interp.Send(Event{Type: "DEEP_RESUME"})
		if interp.State().Value != "inner2" {
			t.Errorf("Expected 'inner2' via deep history, got %s", interp.State().Value)
		}
	})
}
