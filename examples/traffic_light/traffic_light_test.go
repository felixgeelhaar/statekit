package trafficlight

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestTrafficLight_FullCycle(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should start in green
	if !interp.Matches(StateGreen) {
		t.Errorf("expected to start in green, got %v", interp.State().Value)
	}

	// Green -> Yellow
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateYellow) {
		t.Errorf("expected yellow after timer, got %v", interp.State().Value)
	}

	// Yellow -> Red
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateRed) {
		t.Errorf("expected red after timer, got %v", interp.State().Value)
	}

	// Red -> Green (completes cycle)
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateGreen) {
		t.Errorf("expected green after timer, got %v", interp.State().Value)
	}

	// Verify cycle count incremented
	ctx := interp.State().Context
	if ctx.CycleCount != 1 {
		t.Errorf("expected cycle count 1, got %v", ctx.CycleCount)
	}
}

func TestTrafficLight_MultipleCycles(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Run 3 complete cycles
	for cycle := 0; cycle < 3; cycle++ {
		interp.Send(statekit.Event{Type: EventTimer}) // green -> yellow
		interp.Send(statekit.Event{Type: EventTimer}) // yellow -> red
		interp.Send(statekit.Event{Type: EventTimer}) // red -> green
	}

	ctx := interp.State().Context
	if ctx.CycleCount != 3 {
		t.Errorf("expected 3 cycles, got %v", ctx.CycleCount)
	}
}

func TestTrafficLight_EntryActions(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Initial entry to green
	ctx := interp.State().Context
	if len(ctx.Log) != 1 || ctx.Log[0] != "Entered GREEN" {
		t.Errorf("expected 'Entered GREEN' in log, got %v", ctx.Log)
	}

	// Transition to yellow
	interp.Send(statekit.Event{Type: EventTimer})
	ctx = interp.State().Context
	if len(ctx.Log) != 2 || ctx.Log[1] != "Entered YELLOW" {
		t.Errorf("expected 'Entered YELLOW' in log, got %v", ctx.Log)
	}

	// Transition to red
	interp.Send(statekit.Event{Type: EventTimer})
	ctx = interp.State().Context
	if len(ctx.Log) != 3 || ctx.Log[2] != "Entered RED" {
		t.Errorf("expected 'Entered RED' in log, got %v", ctx.Log)
	}
}

func TestTrafficLight_ResetFromGreen(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Reset while in green should stay in green (self-transition)
	interp.Send(statekit.Event{Type: EventReset})
	if !interp.Matches(StateGreen) {
		t.Errorf("expected green after reset, got %v", interp.State().Value)
	}

	// Log should show two green entries (initial + reset)
	ctx := interp.State().Context
	if len(ctx.Log) != 2 {
		t.Errorf("expected 2 log entries after reset, got %v", len(ctx.Log))
	}
}

func TestTrafficLight_IgnoreUnknownEvents(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Send unknown event - should be ignored
	interp.Send(statekit.Event{Type: "UNKNOWN"})
	if !interp.Matches(StateGreen) {
		t.Errorf("expected to stay in green, got %v", interp.State().Value)
	}
}

func TestTrafficLight_NotDone(t *testing.T) {
	machine, err := NewTrafficLight()
	if err != nil {
		t.Fatalf("failed to create traffic light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Traffic light has no final state, should never be done
	if interp.Done() {
		t.Error("traffic light should never be done (no final state)")
	}

	interp.Send(statekit.Event{Type: EventTimer})
	if interp.Done() {
		t.Error("traffic light should never be done (no final state)")
	}
}
