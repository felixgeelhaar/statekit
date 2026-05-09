package statekit

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit/export"
)

// fixedAnchor is the anchor time used by every FakeClock in this file.
// Using a constant keeps tests deterministic and easy to reason about.
var fixedAnchor = time.Unix(1_700_000_000, 0)

// TestDelayedTransition_Basic tests a simple delayed transition
func TestDelayedTransition_Basic(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer interp.Stop()
	interp.Start()

	if interp.State().Value != "loading" {
		t.Errorf("Expected initial state 'loading', got %s", interp.State().Value)
	}

	clk.Advance(49 * time.Millisecond)
	if interp.State().Value != "loading" {
		t.Errorf("Before deadline: expected 'loading', got %s", interp.State().Value)
	}

	clk.Advance(2 * time.Millisecond)
	if interp.State().Value != "ready" {
		t.Errorf("Past deadline: expected 'ready', got %s", interp.State().Value)
	}
}

// TestDelayedTransition_CancelOnExit tests that timers are canceled when exiting state
func TestDelayedTransition_CancelOnExit(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer interp.Stop()
	interp.Start()

	if interp.State().Value != "waiting" {
		t.Errorf("Expected initial state 'waiting', got %s", interp.State().Value)
	}

	// Cancel before the 100ms timeout.
	clk.Advance(30 * time.Millisecond)
	interp.Send(Event{Type: "CANCEL"})
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected state 'cancelled', got %s", interp.State().Value)
	}

	// Advance past the original timeout — timer should have been canceled
	// when we left the waiting state.
	clk.Advance(time.Hour)
	if interp.State().Value != "cancelled" {
		t.Errorf("After cancel + advance: expected still 'cancelled', got %s", interp.State().Value)
	}
}

// TestDelayedTransition_WithGuard tests delayed transitions with guards
func TestDelayedTransition_WithGuard(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[Context](clk))
	defer interp.Stop()
	interp.Start()

	clk.Advance(time.Hour)
	if interp.State().Value != "waiting" {
		t.Errorf("Guard blocked transition: expected 'waiting', got %s", interp.State().Value)
	}
}

// TestDelayedTransition_WithAction tests delayed transitions with actions
func TestDelayedTransition_WithAction(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[Context](clk))
	defer interp.Stop()
	interp.Start()

	if interp.State().Context.ActionExecuted {
		t.Error("Action should not have executed yet")
	}

	clk.Advance(60 * time.Millisecond)
	if !interp.State().Context.ActionExecuted {
		t.Error("Action should have executed after delay")
	}
}

// TestDelayedTransition_Multiple tests multiple delayed transitions from same state
func TestDelayedTransition_Multiple(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer interp.Stop()
	interp.Start()

	clk.Advance(35 * time.Millisecond)
	if interp.State().Value != "first" {
		t.Errorf("Expected 'first' after shorter delay fires, got %s", interp.State().Value)
	}

	clk.Advance(time.Hour)
	if interp.State().Value != "first" {
		t.Errorf("Second timer should have been canceled on state exit, got %s", interp.State().Value)
	}
}

// TestDelayedTransition_InHierarchy tests delayed transitions in nested states
func TestDelayedTransition_InHierarchy(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer interp.Stop()
	interp.Start()

	if interp.State().Value != "child" {
		t.Errorf("Expected initial state 'child', got %s", interp.State().Value)
	}

	clk.Advance(60 * time.Millisecond)
	if interp.State().Value != "done" {
		t.Errorf("Expected 'done' after delay, got %s", interp.State().Value)
	}
}

// TestDelayedTransition_Stop tests that Stop cancels all timers
func TestDelayedTransition_Stop(t *testing.T) {
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	interp.Start()
	interp.Stop()

	clk.Advance(time.Hour)
	if transitioned.Load() {
		t.Error("Transition should not have happened after Stop()")
	}
}

// TestDelayedTransition_NativeExport tests Native JSON export of delayed transitions
func TestDelayedTransition_NativeExport(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[struct{}]("timeout").
		WithInitial("active").
		State("active").
		After(5 * time.Second).Target("inactive").
		Done().
		State("inactive").Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := export.NewNativeExporter(machine)
	jsonStr, err := exporter.ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	if !strings.Contains(jsonStr, `"isDelayed": true`) {
		t.Error("expected isDelayed: true in output")
	}
	if !strings.Contains(jsonStr, `"delayMs": 5000`) {
		t.Error("expected delayMs: 5000 in output")
	}
}

// TestDelayedTransition_Validation tests validation of delayed transitions
func TestDelayedTransition_Validation(t *testing.T) {
	t.Parallel()
	t.Run("zero delay is valid (not a delayed transition)", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()
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

	clk := NewFakeClock(fixedAnchor)
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer interp.Stop()
	interp.Start()

	interp.Send(Event{Type: "GO"})
	if interp.State().Value != "middle" {
		t.Errorf("Expected 'middle', got %s", interp.State().Value)
	}

	clk.Advance(60 * time.Millisecond)
	if interp.State().Value != "end" {
		t.Errorf("Expected 'end' after delay, got %s", interp.State().Value)
	}
}
