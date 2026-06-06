package debug_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/debug"
	"go.klarlabs.de/statekit/internal/ir"
)

type TestContext struct {
	Counter int
	Allowed bool
}

func buildTestMachine() *ir.MachineConfig[TestContext] {
	machine, err := statekit.NewMachine[TestContext]("test").
		WithInitial("idle").
		WithContext(TestContext{Allowed: true}).
		WithAction("increment", func(ctx *TestContext, e statekit.Event) {
			ctx.Counter++
		}).
		WithGuard("isAllowed", func(ctx TestContext, e statekit.Event) bool {
			return ctx.Allowed
		}).
		State("idle").
		OnEntry("increment").
		On("START").Target("running").Guard("isAllowed").
		On("SKIP").Target("done").
		Done().
		State("running").
		On("PAUSE").Target("paused").
		On("STOP").Target("done").
		Done().
		State("paused").
		On("RESUME").Target("running").
		On("STOP").Target("done").
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}

func TestInspector_CurrentState(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	if inspector.CurrentState() != "idle" {
		t.Errorf("expected 'idle', got %q", inspector.CurrentState())
	}

	interp.Send(statekit.Event{Type: "START"})

	if inspector.CurrentState() != "running" {
		t.Errorf("expected 'running', got %q", inspector.CurrentState())
	}
}

func TestInspector_CurrentContext(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	ctx := inspector.CurrentContext()

	// Entry action should have incremented counter
	if ctx.Counter != 1 {
		t.Errorf("expected counter=1, got %d", ctx.Counter)
	}
}

func TestInspector_IsDone(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	if inspector.IsDone() {
		t.Error("should not be done initially")
	}

	interp.Send(statekit.Event{Type: "SKIP"})

	if !inspector.IsDone() {
		t.Error("should be done after SKIP")
	}
}

func TestInspector_MachineInfo(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	if inspector.MachineID() != "test" {
		t.Errorf("expected 'test', got %q", inspector.MachineID())
	}

	if inspector.InitialState() != "idle" {
		t.Errorf("expected 'idle', got %q", inspector.InitialState())
	}
}

func TestInspector_AllStates(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	states := inspector.AllStates()

	if len(states) != 4 {
		t.Errorf("expected 4 states, got %d", len(states))
	}

	// States should be sorted
	expected := []statekit.StateID{"done", "idle", "paused", "running"}
	for i, s := range expected {
		if states[i] != s {
			t.Errorf("states[%d]: expected %q, got %q", i, s, states[i])
		}
	}
}

func TestInspector_StateInfo(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	info := inspector.StateInfo("idle")
	if info == nil {
		t.Fatal("expected state info for 'idle'")
	}

	if info.Type != "atomic" {
		t.Errorf("expected type 'atomic', got %q", info.Type)
	}

	if len(info.Entry) != 1 || info.Entry[0] != "increment" {
		t.Errorf("expected entry action 'increment', got %v", info.Entry)
	}

	if len(info.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(info.Transitions))
	}

	// Non-existent state
	if inspector.StateInfo("nonexistent") != nil {
		t.Error("expected nil for nonexistent state")
	}
}

func TestInspector_AvailableEvents(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	events := inspector.AvailableEvents()

	if len(events) != 2 {
		t.Errorf("expected 2 events from idle, got %d: %v", len(events), events)
	}

	// Events should be sorted
	if events[0] != "SKIP" || events[1] != "START" {
		t.Errorf("expected [SKIP, START], got %v", events)
	}
}

func TestInspector_CanTransition(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	// Should be able to START (guard passes)
	if !inspector.CanTransition("START") {
		t.Error("expected to be able to START")
	}

	// Block the guard
	interp.UpdateContext(func(ctx *TestContext) {
		ctx.Allowed = false
	})

	// Should not be able to START (guard fails)
	if inspector.CanTransition("START") {
		t.Error("expected not to be able to START with guard blocked")
	}

	// SKIP has no guard
	if !inspector.CanTransition("SKIP") {
		t.Error("expected to be able to SKIP")
	}
}

func TestInspector_SimulateTransition(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	// Simulate START
	target, willTransition := inspector.SimulateTransition(statekit.Event{Type: "START"})
	if !willTransition {
		t.Error("expected START to cause transition")
	}
	if target != "running" {
		t.Errorf("expected target 'running', got %q", target)
	}

	// Verify actual state hasn't changed
	if inspector.CurrentState() != "idle" {
		t.Error("simulation should not change actual state")
	}

	// Simulate unknown event
	target, willTransition = inspector.SimulateTransition(statekit.Event{Type: "UNKNOWN"})
	if willTransition {
		t.Error("expected UNKNOWN not to cause transition")
	}
	if target != "idle" {
		t.Errorf("expected current state 'idle', got %q", target)
	}
}

func TestInspector_TransitionsFrom(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	transitions := inspector.TransitionsFrom("running")

	if len(transitions) != 2 {
		t.Errorf("expected 2 transitions from 'running', got %d", len(transitions))
	}
}

func TestInspector_Path(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	path := inspector.Path()

	// For non-hierarchical, path is just the current state
	if len(path) != 1 || path[0] != "idle" {
		t.Errorf("expected path [idle], got %v", path)
	}
}

func TestInspector_Dump(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	dump := inspector.Dump()

	if !strings.Contains(dump, "Machine: test") {
		t.Error("dump should contain machine ID")
	}

	if !strings.Contains(dump, "Current State: idle") {
		t.Error("dump should contain current state")
	}

	if !strings.Contains(dump, "Available Events:") {
		t.Error("dump should contain available events")
	}
}

func TestInspector_DumpMachine(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)
	dump := inspector.DumpMachine()

	if !strings.Contains(dump, "Machine: test") {
		t.Error("dump should contain machine ID")
	}

	if !strings.Contains(dump, "Initial: idle") {
		t.Error("dump should contain initial state")
	}

	if !strings.Contains(dump, "States: 4") {
		t.Error("dump should contain state count")
	}

	if !strings.Contains(dump, "State: idle") {
		t.Error("dump should contain idle state")
	}

	if !strings.Contains(dump, "START -> running") {
		t.Error("dump should contain transitions")
	}
}

func TestInspector_HierarchicalMachine(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("nested").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		On("GLOBAL_EXIT").Target("done").End(). // TransitionBuilder.End() → StateBuilder[active]
		State("idle").
		On("START").Target("running").
		End(). // TransitionBuilder.End() → StateBuilder[idle]
		End(). // StateBuilder[idle].End() → StateBuilder[active]
		State("running").
		On("STOP").Target("idle").
		End(). // TransitionBuilder.End() → StateBuilder[running]
		End(). // StateBuilder[running].End() → StateBuilder[active]
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	inspector := debug.NewInspector(interp, machine)

	// Should be in leaf state
	if inspector.CurrentState() != "idle" {
		t.Errorf("expected 'idle', got %q", inspector.CurrentState())
	}

	// Path should include hierarchy
	path := inspector.Path()
	if len(path) != 2 || path[0] != "active" || path[1] != "idle" {
		t.Errorf("expected path [active, idle], got %v", path)
	}

	// Available events should include parent's events
	events := inspector.AvailableEvents()
	hasGlobalExit := false
	for _, e := range events {
		if e == "GLOBAL_EXIT" {
			hasGlobalExit = true
		}
	}
	if !hasGlobalExit {
		t.Error("expected GLOBAL_EXIT to be available (from parent)")
	}
}
