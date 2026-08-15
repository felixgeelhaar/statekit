package statetest_test

import (
	"testing"

	"go.klarlabs.de/statekit"
	statetesting "go.klarlabs.de/statekit/statetest"
)

func TestSendEvents(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	statetesting.SendEvents(interp,
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	if interp.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp.State().Value)
	}
}

func TestSendEventTypes(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	statetesting.SendEventTypes(interp, "START", "PAUSE")

	if interp.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp.State().Value)
	}
}

func TestMakeEvent(t *testing.T) {
	t.Parallel()
	e := statetesting.MakeEvent("TEST")
	if e.Type != "TEST" {
		t.Errorf("expected type 'TEST', got %q", e.Type)
	}
	if e.Payload != nil {
		t.Error("expected nil payload")
	}

	eWithPayload := statetesting.MakeEvent("TEST", "data")
	if eWithPayload.Payload != "data" {
		t.Errorf("expected payload 'data', got %v", eWithPayload.Payload)
	}
}

func TestMakeEvents(t *testing.T) {
	t.Parallel()
	events := statetesting.MakeEvents("A", "B", "C")
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Type != "A" || events[1].Type != "B" || events[2].Type != "C" {
		t.Error("event types don't match")
	}
}

func TestStartAndSend(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)

	statetesting.StartAndSend(interp,
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	if interp.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp.State().Value)
	}
}

func TestStartAndSendTypes(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)

	statetesting.StartAndSendTypes(interp, "START", "PAUSE")

	if interp.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp.State().Value)
	}
}

func TestRunMachine(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()

	interp := statetesting.RunMachine(machine, statekit.Event{Type: "START"})

	if interp.State().Value != "running" {
		t.Errorf("expected 'running', got %q", interp.State().Value)
	}
}

func TestRunMachineTypes(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()

	interp := statetesting.RunMachineTypes(machine, "START", "PAUSE")

	if interp.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp.State().Value)
	}
}

func TestRecordMachine(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()

	rec := statetesting.RecordMachine(machine)

	if rec.State().Value != "idle" {
		t.Errorf("expected 'idle', got %q", rec.State().Value)
	}

	// Should have recorded start transition
	if rec.TransitionCount() != 1 {
		t.Errorf("expected 1 transition, got %d", rec.TransitionCount())
	}
}

func TestRecordAndRun(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()

	rec := statetesting.RecordAndRun(machine,
		statekit.Event{Type: "START"},
		statekit.Event{Type: "PAUSE"},
	)

	if rec.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", rec.State().Value)
	}

	// Start + 2 events = 3 transitions
	if rec.TransitionCount() != 3 {
		t.Errorf("expected 3 transitions, got %d", rec.TransitionCount())
	}
}

func TestRecordAndRunTypes(t *testing.T) {
	t.Parallel()
	machine := buildTestMachine()

	rec := statetesting.RecordAndRunTypes(machine, "START", "STOP")

	if !rec.Done() {
		t.Error("expected to be done")
	}
}

func TestMustBuild(t *testing.T) {
	t.Parallel()
	builder := statekit.NewMachine[struct{}]("test").
		WithInitial("a").
		State("a").Done()

	machine := statetesting.MustBuild(builder)
	if machine == nil {
		t.Error("expected non-nil machine")
	}
}

func TestMustBuild_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustBuild to panic on invalid machine")
		}
	}()

	// Invalid machine - no initial state set
	builder := statekit.NewMachine[struct{}]("test").
		State("a").Done()

	statetesting.MustBuild(builder)
}

func TestQuickMachine(t *testing.T) {
	t.Parallel()
	machine := statetesting.QuickMachine[struct{}]("a", "b", "c")
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "a" {
		t.Errorf("expected 'a', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "b" {
		t.Errorf("expected 'b', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "c" {
		t.Errorf("expected 'c', got %q", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("expected to be done at 'c'")
	}
}

func TestQuickMachine_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected QuickMachine to panic with no states")
		}
	}()

	statetesting.QuickMachine[struct{}]()
}

func TestQuickMachineWithEvents(t *testing.T) {
	t.Parallel()
	machine := statetesting.QuickMachineWithEvents[struct{}](
		[]string{"idle", "loading", "done"},
		[]statekit.EventType{"LOAD", "COMPLETE"},
	)
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	interp.Send(statekit.Event{Type: "LOAD"})
	if interp.State().Value != "loading" {
		t.Errorf("expected 'loading', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "COMPLETE"})
	if interp.State().Value != "done" {
		t.Errorf("expected 'done', got %q", interp.State().Value)
	}
}

func TestToggleMachine(t *testing.T) {
	t.Parallel()
	machine := statetesting.ToggleMachine[struct{}]("off", "on", "TURN_ON", "TURN_OFF")
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "off" {
		t.Errorf("expected 'off', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "TURN_ON"})
	if interp.State().Value != "on" {
		t.Errorf("expected 'on', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "TURN_OFF"})
	if interp.State().Value != "off" {
		t.Errorf("expected 'off', got %q", interp.State().Value)
	}
}

func TestCycleMachine(t *testing.T) {
	t.Parallel()
	machine := statetesting.CycleMachine[struct{}]("red", "yellow", "green")
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go around the cycle
	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "yellow" {
		t.Errorf("expected 'yellow', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "green" {
		t.Errorf("expected 'green', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "red" {
		t.Errorf("expected 'red' (cycled back), got %q", interp.State().Value)
	}
}

func TestCycleMachine_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected CycleMachine to panic with less than 2 states")
		}
	}()

	statetesting.CycleMachine[struct{}]("only")
}

func TestBranchMachine(t *testing.T) {
	t.Parallel()
	machine := statetesting.BranchMachine[struct{}]("deciding", "success", "failure", "APPROVE", "REJECT")
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "deciding" {
		t.Errorf("expected 'deciding', got %q", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "APPROVE"})
	if interp.State().Value != "success" {
		t.Errorf("expected 'success', got %q", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("expected 'success' to be final")
	}
}

func TestActionCounter(t *testing.T) {
	t.Parallel()
	counter := statetesting.NewActionCounter()

	action := statetesting.ActionFor[struct{}](counter, "myAction")
	action(nil, statekit.Event{})
	action(nil, statekit.Event{})

	if counter.Count("myAction") != 2 {
		t.Errorf("expected count 2, got %d", counter.Count("myAction"))
	}

	if counter.Total() != 2 {
		t.Errorf("expected total 2, got %d", counter.Total())
	}

	counter.Reset()
	if counter.Count("myAction") != 0 {
		t.Errorf("expected count 0 after reset, got %d", counter.Count("myAction"))
	}
}

func TestActionCounter_WithMachine(t *testing.T) {
	t.Parallel()
	counter := statetesting.NewActionCounter()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("a").
		WithAction("onEntry", statetesting.ActionFor[struct{}](counter, "onEntry")).
		WithAction("onExit", statetesting.ActionFor[struct{}](counter, "onExit")).
		State("a").
		OnEntry("onEntry").
		OnExit("onExit").
		On("NEXT").Target("b").
		Done().
		State("b").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if counter.Count("onEntry") != 1 {
		t.Errorf("expected 1 onEntry call, got %d", counter.Count("onEntry"))
	}

	interp.Send(statekit.Event{Type: "NEXT"})

	if counter.Count("onExit") != 1 {
		t.Errorf("expected 1 onExit call, got %d", counter.Count("onExit"))
	}
}

func TestGuardResult(t *testing.T) {
	t.Parallel()
	guards := statetesting.NewGuardResult()

	guard := statetesting.GuardFor[struct{}](guards, "canProceed")

	// Default is true
	if !guard(struct{}{}, statekit.Event{}) {
		t.Error("expected default guard result to be true")
	}

	guards.Set("canProceed", false)
	if guard(struct{}{}, statekit.Event{}) {
		t.Error("expected guard result to be false after Set")
	}

	guards.Set("canProceed", true)
	if !guard(struct{}{}, statekit.Event{}) {
		t.Error("expected guard result to be true after Set")
	}
}

func TestGuardResult_WithMachine(t *testing.T) {
	t.Parallel()
	guards := statetesting.NewGuardResult()
	guards.Set("allowed", false)

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("a").
		WithGuard("allowed", statetesting.GuardFor[struct{}](guards, "allowed")).
		State("a").
		On("NEXT").Target("b").Guard("allowed").
		Done().
		State("b").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Guard blocks transition
	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "a" {
		t.Errorf("expected to stay in 'a', got %q", interp.State().Value)
	}

	// Allow transition
	guards.Set("allowed", true)
	interp.Send(statekit.Event{Type: "NEXT"})
	if interp.State().Value != "b" {
		t.Errorf("expected 'b', got %q", interp.State().Value)
	}
}

func TestGuardResult_SetAll(t *testing.T) {
	t.Parallel()
	guards := statetesting.NewGuardResult()
	guards.Set("a", true)
	guards.Set("b", true)

	guards.SetAll(false)

	guardA := statetesting.GuardFor[struct{}](guards, "a")
	guardB := statetesting.GuardFor[struct{}](guards, "b")

	if guardA(struct{}{}, statekit.Event{}) {
		t.Error("expected guard 'a' to be false")
	}
	if guardB(struct{}{}, statekit.Event{}) {
		t.Error("expected guard 'b' to be false")
	}
}

func TestGuardResult_Reset(t *testing.T) {
	t.Parallel()
	guards := statetesting.NewGuardResult()
	guards.Set("test", false)

	guards.Reset()

	guard := statetesting.GuardFor[struct{}](guards, "test")
	if !guard(struct{}{}, statekit.Event{}) {
		t.Error("expected guard to return true after reset (default)")
	}
}
