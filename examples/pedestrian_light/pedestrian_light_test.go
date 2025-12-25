package pedestrianlight

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestPedestrianLight_InitialState(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should start in the initial leaf state: dont_walk
	if !interp.Matches(StateDontWalk) {
		t.Errorf("expected to start in 'dont_walk', got %v", interp.State().Value)
	}

	// Should also match the parent compound state
	if !interp.Matches(StateActive) {
		t.Error("expected to match 'active' (parent state)")
	}

	// Check entry actions were called in order
	ctx := interp.State().Context
	expected := []string{
		"Entered ACTIVE mode",
		"DON'T WALK - Hand symbol displayed",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
	for i, exp := range expected {
		if ctx.Log[i] != exp {
			t.Errorf("expected log[%d] = %q, got %q", i, exp, ctx.Log[i])
		}
	}
}

func TestPedestrianLight_FullCrossingCycle(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Press button: dont_walk -> walk
	interp.Send(statekit.Event{Type: EventPedestrianButton})
	if !interp.Matches(StateWalk) {
		t.Errorf("expected 'walk' after button press, got %v", interp.State().Value)
	}

	// Timer: walk -> countdown/flashing
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateFlashing) {
		t.Errorf("expected 'flashing' after first timer, got %v", interp.State().Value)
	}
	if !interp.Matches(StateCountdown) {
		t.Error("expected to match 'countdown' (parent state)")
	}

	// Timer: flashing -> warning
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateWarning) {
		t.Errorf("expected 'warning' after second timer, got %v", interp.State().Value)
	}

	// Timer: warning -> dont_walk
	interp.Send(statekit.Event{Type: EventTimer})
	if !interp.Matches(StateDontWalk) {
		t.Errorf("expected 'dont_walk' after third timer, got %v", interp.State().Value)
	}

	// Verify crossing count incremented
	ctx := interp.State().Context
	if ctx.CrossingCount != 1 {
		t.Errorf("expected crossing count 1, got %d", ctx.CrossingCount)
	}
}

func TestPedestrianLight_EntryExitOrdering(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Clear log and press button to go to walk
	interp.UpdateContext(func(c *Context) {
		c.Log = nil
	})

	interp.Send(statekit.Event{Type: EventPedestrianButton})

	ctx := interp.State().Context
	expected := []string{
		"DON'T WALK ended",
		"WALK - Walking figure displayed",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
	for i, exp := range expected {
		if ctx.Log[i] != exp {
			t.Errorf("expected log[%d] = %q, got %q", i, exp, ctx.Log[i])
		}
	}

	// Clear log and go to countdown
	interp.UpdateContext(func(c *Context) {
		c.Log = nil
	})

	interp.Send(statekit.Event{Type: EventTimer})

	ctx = interp.State().Context
	expected = []string{
		"WALK ended",
		"Countdown started",
		"Flashing hand symbol",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
	for i, exp := range expected {
		if ctx.Log[i] != exp {
			t.Errorf("expected log[%d] = %q, got %q", i, exp, ctx.Log[i])
		}
	}
}

func TestPedestrianLight_MaintenanceMode(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go to walk state first
	interp.Send(statekit.Event{Type: EventPedestrianButton})
	if !interp.Matches(StateWalk) {
		t.Error("expected to be in 'walk'")
	}

	// Clear log
	interp.UpdateContext(func(c *Context) {
		c.Log = nil
	})

	// Enter maintenance - should bubble up to active's transition
	interp.Send(statekit.Event{Type: EventEnterMaintenance})

	if !interp.Matches(StateMaintenance) {
		t.Errorf("expected 'maintenance' after ENTER_MAINTENANCE, got %v", interp.State().Value)
	}

	ctx := interp.State().Context
	if !ctx.InMaintenance {
		t.Error("expected InMaintenance to be true")
	}

	// Check exit/entry order: exit walk, exit active, transition action, enter maintenance
	expected := []string{
		"WALK ended",
		"Exited ACTIVE mode",
		"Transition action executed",
		"Entered MAINTENANCE mode - all lights off",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
	for i, exp := range expected {
		if ctx.Log[i] != exp {
			t.Errorf("expected log[%d] = %q, got %q", i, exp, ctx.Log[i])
		}
	}

	// Exit maintenance
	interp.UpdateContext(func(c *Context) {
		c.Log = nil
	})

	interp.Send(statekit.Event{Type: EventExitMaintenance})

	if !interp.Matches(StateDontWalk) {
		t.Errorf("expected 'dont_walk' after EXIT_MAINTENANCE, got %v", interp.State().Value)
	}

	ctx = interp.State().Context
	if ctx.InMaintenance {
		t.Error("expected InMaintenance to be false")
	}

	// Check we entered active and dont_walk
	expected = []string{
		"Exited MAINTENANCE mode",
		"Entered ACTIVE mode",
		"DON'T WALK - Hand symbol displayed",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
}

func TestPedestrianLight_EventBubblingFromDeepState(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go to countdown/flashing (deeply nested)
	interp.Send(statekit.Event{Type: EventPedestrianButton}) // -> walk
	interp.Send(statekit.Event{Type: EventTimer})            // -> countdown/flashing

	if !interp.Matches(StateFlashing) {
		t.Errorf("expected 'flashing', got %v", interp.State().Value)
	}

	// Clear log
	interp.UpdateContext(func(c *Context) {
		c.Log = nil
	})

	// Enter maintenance from deep state - event bubbles up to active
	interp.Send(statekit.Event{Type: EventEnterMaintenance})

	if !interp.Matches(StateMaintenance) {
		t.Errorf("expected 'maintenance', got %v", interp.State().Value)
	}

	// Should have exited flashing, countdown, and active
	ctx := interp.State().Context
	expected := []string{
		"Countdown ended, crossing complete",
		"Exited ACTIVE mode",
		"Transition action executed",
		"Entered MAINTENANCE mode - all lights off",
	}
	if len(ctx.Log) != len(expected) {
		t.Fatalf("expected %d log entries, got %d: %v", len(expected), len(ctx.Log), ctx.Log)
	}
}

func TestPedestrianLight_MultipleCycles(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Run 3 complete crossing cycles
	for i := 0; i < 3; i++ {
		interp.Send(statekit.Event{Type: EventPedestrianButton}) // -> walk
		interp.Send(statekit.Event{Type: EventTimer})            // -> countdown/flashing
		interp.Send(statekit.Event{Type: EventTimer})            // -> warning
		interp.Send(statekit.Event{Type: EventTimer})            // -> dont_walk
	}

	ctx := interp.State().Context
	if ctx.CrossingCount != 3 {
		t.Errorf("expected 3 crossing cycles, got %d", ctx.CrossingCount)
	}
}

func TestPedestrianLight_IgnoreButtonInWalk(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go to walk
	interp.Send(statekit.Event{Type: EventPedestrianButton})
	if !interp.Matches(StateWalk) {
		t.Error("expected to be in 'walk'")
	}

	// Button should be ignored while walking
	interp.Send(statekit.Event{Type: EventPedestrianButton})
	if !interp.Matches(StateWalk) {
		t.Error("expected to still be in 'walk' after ignored button press")
	}
}

func TestPedestrianLight_MatchesCompoundStates(t *testing.T) {
	machine, err := NewPedestrianLight()
	if err != nil {
		t.Fatalf("failed to create pedestrian light: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go to flashing (deep nested)
	interp.Send(statekit.Event{Type: EventPedestrianButton})
	interp.Send(statekit.Event{Type: EventTimer})

	// Should match all ancestors
	if !interp.Matches(StateFlashing) {
		t.Error("expected to match 'flashing'")
	}
	if !interp.Matches(StateCountdown) {
		t.Error("expected to match 'countdown'")
	}
	if !interp.Matches(StateActive) {
		t.Error("expected to match 'active'")
	}

	// Should not match siblings or other branches
	if interp.Matches(StateWarning) {
		t.Error("should not match 'warning' (sibling)")
	}
	if interp.Matches(StateWalk) {
		t.Error("should not match 'walk' (different branch)")
	}
	if interp.Matches(StateMaintenance) {
		t.Error("should not match 'maintenance' (different branch)")
	}
}
