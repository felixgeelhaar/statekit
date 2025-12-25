package statekit

import (
	"testing"
)

type counterContext struct {
	Count       int
	Transitions []string
}

func TestInterpreter_Start(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)

	// Before start, state should be empty
	state := interp.State()
	if state.Value != "" {
		t.Errorf("expected empty state before start, got %v", state.Value)
	}

	// Start the interpreter
	interp.Start()

	// After start, should be in initial state
	state = interp.State()
	if state.Value != "idle" {
		t.Errorf("expected state 'idle', got %v", state.Value)
	}
}

func TestInterpreter_Send_BasicTransition(t *testing.T) {
	machine, err := NewMachine[counterContext]("trafficLight").
		WithInitial("green").
		State("green").
		On("TIMER").Target("yellow").
		Done().
		State("yellow").
		On("TIMER").Target("red").
		Done().
		State("red").
		On("TIMER").Target("green").
		Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Initial state
	if interp.State().Value != "green" {
		t.Errorf("expected 'green', got %v", interp.State().Value)
	}

	// Transition to yellow
	interp.Send(Event{Type: "TIMER"})
	if interp.State().Value != "yellow" {
		t.Errorf("expected 'yellow', got %v", interp.State().Value)
	}

	// Transition to red
	interp.Send(Event{Type: "TIMER"})
	if interp.State().Value != "red" {
		t.Errorf("expected 'red', got %v", interp.State().Value)
	}

	// Transition back to green
	interp.Send(Event{Type: "TIMER"})
	if interp.State().Value != "green" {
		t.Errorf("expected 'green', got %v", interp.State().Value)
	}
}

func TestInterpreter_Send_UnknownEvent(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Send unknown event - should stay in current state
	interp.Send(Event{Type: "UNKNOWN"})
	if interp.State().Value != "idle" {
		t.Errorf("expected to stay in 'idle', got %v", interp.State().Value)
	}
}

func TestInterpreter_Send_WithGuard(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		WithGuard("hasCount", func(ctx counterContext, e Event) bool {
			return ctx.Count > 0
		}).
		State("idle").
		On("START").Target("running").Guard("hasCount").
		Done().
		State("running").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Guard should block transition (Count is 0)
	interp.Send(Event{Type: "START"})
	if interp.State().Value != "idle" {
		t.Errorf("expected guard to block transition, got %v", interp.State().Value)
	}

	// Update context and try again
	interp.UpdateContext(func(ctx *counterContext) {
		ctx.Count = 1
	})
	interp.Send(Event{Type: "START"})
	if interp.State().Value != "running" {
		t.Errorf("expected guard to allow transition, got %v", interp.State().Value)
	}
}

func TestInterpreter_Send_WithActions(t *testing.T) {
	var entryLog, exitLog, transitionLog []string

	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		WithAction("logEntry", func(ctx *counterContext, e Event) {
			entryLog = append(entryLog, ctx.Transitions[len(ctx.Transitions)-1]+"_entry")
		}).
		WithAction("logExit", func(ctx *counterContext, e Event) {
			exitLog = append(exitLog, "exit")
		}).
		WithAction("logTransition", func(ctx *counterContext, e Event) {
			transitionLog = append(transitionLog, "transition")
		}).
		WithAction("recordState", func(ctx *counterContext, e Event) {
			ctx.Transitions = append(ctx.Transitions, "idle")
		}).
		WithAction("recordRunning", func(ctx *counterContext, e Event) {
			ctx.Transitions = append(ctx.Transitions, "running")
		}).
		State("idle").
		OnEntry("recordState").
		OnExit("logExit").
		On("START").Target("running").Do("logTransition").
		Done().
		State("running").
		OnEntry("recordRunning").
		Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Entry action should have fired
	ctx := interp.State().Context
	if len(ctx.Transitions) != 1 || ctx.Transitions[0] != "idle" {
		t.Errorf("expected idle entry action to fire, got %v", ctx.Transitions)
	}

	// Transition
	interp.Send(Event{Type: "START"})

	// Check exit action fired
	if len(exitLog) != 1 {
		t.Errorf("expected exit action to fire once, got %d", len(exitLog))
	}

	// Check transition action fired
	if len(transitionLog) != 1 {
		t.Errorf("expected transition action to fire once, got %d", len(transitionLog))
	}

	// Check running entry action fired
	ctx = interp.State().Context
	if len(ctx.Transitions) != 2 || ctx.Transitions[1] != "running" {
		t.Errorf("expected running entry action to fire, got %v", ctx.Transitions)
	}
}

func TestInterpreter_Matches(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if !interp.Matches("idle") {
		t.Error("expected to match 'idle'")
	}
	if interp.Matches("running") {
		t.Error("expected not to match 'running'")
	}

	interp.Send(Event{Type: "START"})

	if interp.Matches("idle") {
		t.Error("expected not to match 'idle'")
	}
	if !interp.Matches("running") {
		t.Error("expected to match 'running'")
	}
}

func TestInterpreter_Done(t *testing.T) {
	machine, err := NewMachine[counterContext]("workflow").
		WithInitial("active").
		State("active").
		On("COMPLETE").Target("done").
		Done().
		State("done").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if interp.Done() {
		t.Error("expected not to be done initially")
	}

	interp.Send(Event{Type: "COMPLETE"})

	if !interp.Done() {
		t.Error("expected to be done after reaching final state")
	}
}

func TestInterpreter_Context(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		WithContext(counterContext{Count: 5}).
		State("idle").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	ctx := interp.State().Context
	if ctx.Count != 5 {
		t.Errorf("expected Count 5, got %v", ctx.Count)
	}
}

func TestInterpreter_MultipleTransitionsOnState(t *testing.T) {
	machine, err := NewMachine[counterContext]("test").
		WithInitial("idle").
		State("idle").
		On("GO_A").Target("stateA").
		On("GO_B").Target("stateB").
		Done().
		State("stateA").Done().
		State("stateB").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	interp.Send(Event{Type: "GO_B"})
	if interp.State().Value != "stateB" {
		t.Errorf("expected 'stateB', got %v", interp.State().Value)
	}
}
