package statekit

import (
	"testing"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

type testContext struct {
	Count int
}

func TestMachineBuilder_Basic(t *testing.T) {
	machine, err := NewMachine[testContext]("trafficLight").
		WithInitial("green").
		State("green").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if machine.ID != "trafficLight" {
		t.Errorf("expected ID 'trafficLight', got %v", machine.ID)
	}
	if machine.Initial != "green" {
		t.Errorf("expected Initial 'green', got %v", machine.Initial)
	}
}

func TestMachineBuilder_WithContext(t *testing.T) {
	ctx := testContext{Count: 42}
	machine, err := NewMachine[testContext]("test").
		WithInitial("idle").
		WithContext(ctx).
		State("idle").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if machine.Context.Count != 42 {
		t.Errorf("expected context Count 42, got %v", machine.Context.Count)
	}
}

func TestMachineBuilder_WithStates(t *testing.T) {
	machine, err := NewMachine[testContext]("trafficLight").
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

	// Check all states exist
	states := []ir.StateID{"green", "yellow", "red"}
	for _, s := range states {
		if machine.States[s] == nil {
			t.Errorf("expected state '%s' to exist", s)
		}
	}

	// Check transitions
	greenState := machine.States["green"]
	if len(greenState.Transitions) != 1 {
		t.Fatalf("expected 1 transition on green, got %d", len(greenState.Transitions))
	}
	if greenState.Transitions[0].Event != "TIMER" {
		t.Errorf("expected event 'TIMER', got %v", greenState.Transitions[0].Event)
	}
	if greenState.Transitions[0].Target != "yellow" {
		t.Errorf("expected target 'yellow', got %v", greenState.Transitions[0].Target)
	}
}

func TestMachineBuilder_FinalState(t *testing.T) {
	machine, err := NewMachine[testContext]("workflow").
		WithInitial("active").
		State("active").
			On("COMPLETE").Target("done").
			Done().
		State("done").Final().
			Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doneState := machine.States["done"]
	if doneState.Type != ir.StateTypeFinal {
		t.Errorf("expected done state to be Final, got %v", doneState.Type)
	}
}

func TestMachineBuilder_WithActions(t *testing.T) {
	actionCalled := false
	action := func(ctx *testContext, e Event) {
		actionCalled = true
		ctx.Count++
	}

	machine, err := NewMachine[testContext]("test").
		WithInitial("idle").
		WithAction("increment", action).
		State("idle").
			OnEntry("increment").
			OnExit("increment").
			On("NEXT").Target("active").Do("increment").
			Done().
		State("active").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify action is registered
	registeredAction := machine.Actions["increment"]
	if registeredAction == nil {
		t.Fatal("expected action to be registered")
	}

	// Call the action and verify
	ctx := testContext{Count: 0}
	registeredAction(&ctx, ir.Event{})
	if !actionCalled {
		t.Error("expected action to be called")
	}
	if ctx.Count != 1 {
		t.Errorf("expected Count 1, got %v", ctx.Count)
	}

	// Verify state entry/exit actions
	idleState := machine.States["idle"]
	if len(idleState.Entry) != 1 || idleState.Entry[0] != "increment" {
		t.Errorf("expected entry action 'increment', got %v", idleState.Entry)
	}
	if len(idleState.Exit) != 1 || idleState.Exit[0] != "increment" {
		t.Errorf("expected exit action 'increment', got %v", idleState.Exit)
	}

	// Verify transition action
	if len(idleState.Transitions[0].Actions) != 1 || idleState.Transitions[0].Actions[0] != "increment" {
		t.Errorf("expected transition action 'increment', got %v", idleState.Transitions[0].Actions)
	}
}

func TestMachineBuilder_WithGuards(t *testing.T) {
	guard := func(ctx testContext, e Event) bool {
		return ctx.Count > 0
	}

	machine, err := NewMachine[testContext]("test").
		WithInitial("idle").
		WithGuard("hasCount", guard).
		State("idle").
			On("NEXT").Target("active").Guard("hasCount").
			Done().
		State("active").Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify guard is registered
	registeredGuard := machine.Guards["hasCount"]
	if registeredGuard == nil {
		t.Fatal("expected guard to be registered")
	}

	// Verify guard works
	if registeredGuard(testContext{Count: 0}, ir.Event{}) {
		t.Error("expected guard to return false for Count 0")
	}
	if !registeredGuard(testContext{Count: 1}, ir.Event{}) {
		t.Error("expected guard to return true for Count 1")
	}

	// Verify transition has guard
	idleState := machine.States["idle"]
	if idleState.Transitions[0].Guard != "hasCount" {
		t.Errorf("expected guard 'hasCount', got %v", idleState.Transitions[0].Guard)
	}
}

func TestMachineBuilder_MultipleTransitions(t *testing.T) {
	machine, err := NewMachine[testContext]("test").
		WithInitial("idle").
		State("idle").
			On("START").Target("running").
			On("SKIP").Target("done").
			Done().
		State("running").
			On("STOP").Target("done").
			Done().
		State("done").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idleState := machine.States["idle"]
	if len(idleState.Transitions) != 2 {
		t.Errorf("expected 2 transitions on idle, got %d", len(idleState.Transitions))
	}
}
