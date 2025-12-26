package statekit

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit/export"
)

// TestDelayedTransition_Basic tests a simple delayed transition
func TestDelayedTransition_Basic(t *testing.T) {
	machine, err := NewMachine[struct{}]("delayed_basic").
		WithInitial("loading").
		State("loading").
		After(50 * time.Millisecond).Target("ready").
		Done().
		State("ready").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in loading
	if interp.State().Value != "loading" {
		t.Errorf("Expected initial state 'loading', got %s", interp.State().Value)
	}

	// Wait for delayed transition
	time.Sleep(100 * time.Millisecond)

	// Should now be in ready
	if interp.State().Value != "ready" {
		t.Errorf("Expected state 'ready' after delay, got %s", interp.State().Value)
	}

	interp.Stop()
}

// TestDelayedTransition_CancelOnExit tests that timers are canceled when exiting state
func TestDelayedTransition_CancelOnExit(t *testing.T) {
	machine, err := NewMachine[struct{}]("delayed_cancel").
		WithInitial("waiting").
		State("waiting").
		After(100 * time.Millisecond).Target("timeout").
		On("CANCEL").Target("cancelled").
		Done().
		State("timeout").
		Done().
		State("cancelled").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in waiting
	if interp.State().Value != "waiting" {
		t.Errorf("Expected initial state 'waiting', got %s", interp.State().Value)
	}

	// Cancel before timeout fires
	time.Sleep(30 * time.Millisecond)
	interp.Send(Event{Type: "CANCEL"})

	// Should be in cancelled
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected state 'cancelled', got %s", interp.State().Value)
	}

	// Wait past the original timeout
	time.Sleep(100 * time.Millisecond)

	// Should still be in cancelled (timer was canceled)
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected state still 'cancelled', got %s", interp.State().Value)
	}

	interp.Stop()
}

// TestDelayedTransition_WithGuard tests delayed transitions with guards
func TestDelayedTransition_WithGuard(t *testing.T) {
	type Context struct {
		ShouldProceed bool
	}

	machine, err := NewMachine[Context]("delayed_guard").
		WithInitial("waiting").
		WithContext(Context{ShouldProceed: false}).
		WithGuard("canProceed", func(ctx Context, e Event) bool {
			return ctx.ShouldProceed
		}).
		State("waiting").
		After(50 * time.Millisecond).Target("proceeded").Guard("canProceed").
		Done().
		State("proceeded").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Wait for delayed transition (guard will block it)
	time.Sleep(100 * time.Millisecond)

	// Should still be in waiting because guard returned false
	if interp.State().Value != "waiting" {
		t.Errorf("Expected state 'waiting' (guard blocked), got %s", interp.State().Value)
	}

	interp.Stop()
}

// TestDelayedTransition_WithAction tests delayed transitions with actions
func TestDelayedTransition_WithAction(t *testing.T) {
	type Context struct {
		ActionExecuted bool
	}

	machine, err := NewMachine[Context]("delayed_action").
		WithInitial("start").
		WithAction("markExecuted", func(ctx *Context, e Event) {
			ctx.ActionExecuted = true
		}).
		State("start").
		After(50 * time.Millisecond).Target("end").Do("markExecuted").
		Done().
		State("end").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Action should not have executed yet
	if interp.State().Context.ActionExecuted {
		t.Error("Action should not have executed yet")
	}

	// Wait for delayed transition
	time.Sleep(100 * time.Millisecond)

	// Action should have executed
	if !interp.State().Context.ActionExecuted {
		t.Error("Expected action to be executed")
	}

	interp.Stop()
}

// TestDelayedTransition_Multiple tests multiple delayed transitions from same state
func TestDelayedTransition_Multiple(t *testing.T) {
	machine, err := NewMachine[struct{}]("delayed_multiple").
		WithInitial("start").
		State("start").
		After(30 * time.Millisecond).Target("first").
		After(100 * time.Millisecond).Target("second").
		Done().
		State("first").
		Done().
		State("second").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Wait for first delayed transition
	time.Sleep(60 * time.Millisecond)

	// Should be in first (shorter delay fires first)
	if interp.State().Value != "first" {
		t.Errorf("Expected state 'first', got %s", interp.State().Value)
	}

	// Wait past the second delay
	time.Sleep(100 * time.Millisecond)

	// Should still be in first (second timer was canceled when we left start)
	if interp.State().Value != "first" {
		t.Errorf("Expected state still 'first', got %s", interp.State().Value)
	}

	interp.Stop()
}

// TestDelayedTransition_InHierarchy tests delayed transitions in nested states
func TestDelayedTransition_InHierarchy(t *testing.T) {
	machine, err := NewMachine[struct{}]("delayed_hierarchy").
		WithInitial("parent").
		State("parent").
		WithInitial("child").
		State("child").
		After(50 * time.Millisecond).Target("done").
		End().
		End().
		Done().
		State("done").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in child
	if interp.State().Value != "child" {
		t.Errorf("Expected initial state 'child', got %s", interp.State().Value)
	}

	// Wait for delayed transition
	time.Sleep(100 * time.Millisecond)

	// Should now be in done
	if interp.State().Value != "done" {
		t.Errorf("Expected state 'done' after delay, got %s", interp.State().Value)
	}

	interp.Stop()
}

// TestDelayedTransition_Stop tests that Stop cancels all timers
func TestDelayedTransition_Stop(t *testing.T) {
	var transitioned atomic.Bool

	machine, err := NewMachine[struct{}]("delayed_stop").
		WithInitial("waiting").
		WithAction("mark", func(ctx *struct{}, e Event) {
			transitioned.Store(true)
		}).
		State("waiting").
		After(50 * time.Millisecond).Target("done").Do("mark").
		Done().
		State("done").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Stop immediately
	interp.Stop()

	// Wait past the delay
	time.Sleep(100 * time.Millisecond)

	// Transition should not have happened
	if transitioned.Load() {
		t.Error("Transition should not have happened after Stop()")
	}
}

// TestDelayedTransition_XStateExport tests XState JSON export of delayed transitions
func TestDelayedTransition_XStateExport(t *testing.T) {
	machine, err := NewMachine[struct{}]("export_test").
		WithInitial("loading").
		State("loading").
		After(1000 * time.Millisecond).Target("timeout").
		After(5000 * time.Millisecond).Target("error").
		On("LOADED").Target("ready").
		Done().
		State("timeout").
		Done().
		State("error").
		Done().
		State("ready").
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

	// Check the loading state has both "on" and "after"
	loadingState := exported.States["loading"]

	// Check "on" transitions
	if loadingState.On == nil {
		t.Fatal("Expected loading state to have 'on' transitions")
	}
	if _, ok := loadingState.On["LOADED"]; !ok {
		t.Error("Expected 'LOADED' event in 'on' transitions")
	}

	// Check "after" transitions
	if loadingState.After == nil {
		t.Fatal("Expected loading state to have 'after' transitions")
	}
	if _, ok := loadingState.After["1000"]; !ok {
		t.Error("Expected '1000' delay in 'after' transitions")
	}
	if _, ok := loadingState.After["5000"]; !ok {
		t.Error("Expected '5000' delay in 'after' transitions")
	}

	// Check targets
	if loadingState.After["1000"].Target != "timeout" {
		t.Errorf("Expected '1000' after target 'timeout', got '%s'", loadingState.After["1000"].Target)
	}
	if loadingState.After["5000"].Target != "error" {
		t.Errorf("Expected '5000' after target 'error', got '%s'", loadingState.After["5000"].Target)
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
	loading := states["loading"].(map[string]any)

	// Check "after" in JSON
	afterMap, ok := loading["after"].(map[string]any)
	if !ok {
		t.Fatal("Expected 'after' field in loading state JSON")
	}
	if _, ok := afterMap["1000"]; !ok {
		t.Error("Expected '1000' in after map")
	}
}

// TestDelayedTransition_Validation tests validation of delayed transitions
func TestDelayedTransition_Validation(t *testing.T) {
	t.Run("zero delay is valid (not a delayed transition)", func(t *testing.T) {
		_, err := NewMachine[struct{}]("zero_delay").
			WithInitial("start").
			State("start").
			On("GO").Target("end"). // Normal event transition (delay = 0)
			Done().
			State("end").
			Done().
			Build()

		if err != nil {
			t.Errorf("Expected zero delay to be valid, got error: %v", err)
		}
	})

	t.Run("positive delay is valid", func(t *testing.T) {
		_, err := NewMachine[struct{}]("positive_delay").
			WithInitial("start").
			State("start").
			After(time.Second).Target("end").
			Done().
			State("end").
			Done().
			Build()

		if err != nil {
			t.Errorf("Expected positive delay to be valid, got error: %v", err)
		}
	})
}

// TestDelayedTransition_ChainedBuilder tests fluent API chaining
func TestDelayedTransition_ChainedBuilder(t *testing.T) {
	machine, err := NewMachine[struct{}]("chained").
		WithInitial("start").
		State("start").
		On("GO").Target("middle").
		After(50 * time.Millisecond).Target("timeout").
		Done().
		State("middle").
		After(50 * time.Millisecond).Target("end").
		On("SKIP").Target("end").
		Done().
		State("timeout").
		Done().
		State("end").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Transition via event before timeout
	interp.Send(Event{Type: "GO"})
	if interp.State().Value != "middle" {
		t.Errorf("Expected 'middle', got %s", interp.State().Value)
	}

	// Wait for delayed transition from middle
	time.Sleep(100 * time.Millisecond)

	if interp.State().Value != "end" {
		t.Errorf("Expected 'end' after delay, got %s", interp.State().Value)
	}

	interp.Stop()
}
