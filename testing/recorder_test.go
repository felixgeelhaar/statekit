package testing_test

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
	statetesting "github.com/felixgeelhaar/statekit/testing"
)

type TestContext struct {
	Counter int
	Value   string
}

func buildTestMachine() *statekit.MachineConfig[TestContext] {
	machine, err := statekit.NewMachine[TestContext]("test").
		WithInitial("idle").
		WithAction("increment", func(ctx *TestContext, e statekit.Event) {
			ctx.Counter++
		}).
		State("idle").
		OnEntry("increment").
		On("START").Target("running").
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

func TestRecorder_Start(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()

	if rec.State().Value != "idle" {
		t.Errorf("expected state 'idle', got %q", rec.State().Value)
	}

	// Should have recorded the start transition
	transitions := rec.Transitions()
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}

	if transitions[0].Event.Type != "__START__" {
		t.Errorf("expected __START__ event, got %q", transitions[0].Event.Type)
	}
	if transitions[0].ToState != "idle" {
		t.Errorf("expected to-state 'idle', got %q", transitions[0].ToState)
	}
}

func TestRecorder_Send(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.Send(statekit.Event{Type: "START"})

	if rec.State().Value != "running" {
		t.Errorf("expected state 'running', got %q", rec.State().Value)
	}

	transitions := rec.Transitions()
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}

	// Check the START transition
	startTr := transitions[1]
	if startTr.FromState != "idle" {
		t.Errorf("expected from-state 'idle', got %q", startTr.FromState)
	}
	if startTr.ToState != "running" {
		t.Errorf("expected to-state 'running', got %q", startTr.ToState)
	}
	if !startTr.Transitioned {
		t.Error("expected transition to have occurred")
	}
}

func TestRecorder_SendAll(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
		statekit.Event{Type: "RESUME"},
	)

	if rec.State().Value != "running" {
		t.Errorf("expected state 'running', got %q", rec.State().Value)
	}

	// Should have 4 transitions (start + 3 events)
	if rec.TransitionCount() != 4 {
		t.Errorf("expected 4 transitions, got %d", rec.TransitionCount())
	}
}

func TestRecorder_States(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	states := rec.States()
	expected := []statekit.StateID{"idle", "running", "paused"}

	if len(states) != len(expected) {
		t.Fatalf("expected %d states, got %d", len(expected), len(states))
	}

	for i, s := range expected {
		if states[i] != s {
			t.Errorf("state[%d]: expected %q, got %q", i, s, states[i])
		}
	}
}

func TestRecorder_UniqueStates(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
		statekit.Event{Type: "RESUME"}, // Back to running
	)

	unique := rec.UniqueStates()
	expected := []statekit.StateID{"idle", "running", "paused"}

	if len(unique) != len(expected) {
		t.Fatalf("expected %d unique states, got %d", len(expected), len(unique))
	}

	for i, s := range expected {
		if unique[i] != s {
			t.Errorf("unique[%d]: expected %q, got %q", i, s, unique[i])
		}
	}
}

func TestRecorder_Events(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	events := rec.Events()

	// Should not include __START__
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != "START" {
		t.Errorf("events[0]: expected 'START', got %q", events[0].Type)
	}
	if events[1].Type != "PAUSE" {
		t.Errorf("events[1]: expected 'PAUSE', got %q", events[1].Type)
	}
}

func TestRecorder_EventTypes(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	types := rec.EventTypes()
	expected := []statekit.EventType{"START", "PAUSE"}

	if len(types) != len(expected) {
		t.Fatalf("expected %d event types, got %d", len(expected), len(types))
	}

	for i, e := range expected {
		if types[i] != e {
			t.Errorf("types[%d]: expected %q, got %q", i, e, types[i])
		}
	}
}

func TestRecorder_LastTransition(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	// Before start, should be nil
	if rec.LastTransition() != nil {
		t.Error("expected nil before any transitions")
	}

	rec.Start()
	rec.Send(statekit.Event{Type: "START"})

	last := rec.LastTransition()
	if last == nil {
		t.Fatal("expected non-nil last transition")
	}

	if last.ToState != "running" {
		t.Errorf("expected last to-state 'running', got %q", last.ToState)
	}
}

func TestRecorder_FindTransition(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	found := rec.FindTransition("PAUSE")
	if found == nil {
		t.Fatal("expected to find PAUSE transition")
	}

	if found.FromState != "running" {
		t.Errorf("expected from-state 'running', got %q", found.FromState)
	}

	notFound := rec.FindTransition("NONEXISTENT")
	if notFound != nil {
		t.Error("expected nil for nonexistent event")
	}
}

func TestRecorder_TransitionsFrom(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
		statekit.Event{Type: "RESUME"},
	)

	fromRunning := rec.TransitionsFrom("running")
	if len(fromRunning) != 1 {
		t.Errorf("expected 1 transition from 'running', got %d", len(fromRunning))
	}
}

func TestRecorder_ActualTransitions(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "INVALID_EVENT"}, // Won't cause transition
		statekit.Event{Type: "PAUSE"},
	)

	actual := rec.ActualTransitions()
	// Start (__START__), START, and PAUSE should transition
	if len(actual) != 3 {
		t.Errorf("expected 3 actual transitions, got %d", len(actual))
	}
}

func TestRecorder_Reset(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.Send(statekit.Event{Type: "START"})

	rec.Reset()

	if rec.TransitionCount() != 0 {
		t.Errorf("expected 0 transitions after reset, got %d", rec.TransitionCount())
	}
}

func TestRecorder_ContextCapture(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()

	transitions := rec.Transitions()
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}

	// Entry action should have incremented counter
	if transitions[0].ContextAfter.Counter != 1 {
		t.Errorf("expected counter=1 after entry, got %d", transitions[0].ContextAfter.Counter)
	}
}

func TestRecorder_Matches(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()

	if !rec.Matches("idle") {
		t.Error("expected to match 'idle'")
	}

	rec.Send(statekit.Event{Type: "START"})

	if !rec.Matches("running") {
		t.Error("expected to match 'running'")
	}
}

func TestRecorder_Done(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()

	if rec.Done() {
		t.Error("should not be done initially")
	}

	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "STOP"})

	if !rec.Done() {
		t.Error("should be done after reaching final state")
	}
}

func TestRecorder_TotalDuration(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	rec.Start()
	rec.Send(statekit.Event{Type: "START"})

	total := rec.TotalDuration()
	if total <= 0 {
		t.Error("expected positive total duration")
	}
}

func TestRecorder_Interpreter(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetesting.NewRecorder(interp)

	if rec.Interpreter() != interp {
		t.Error("expected Interpreter() to return the wrapped interpreter")
	}
}
