package statekit

import (
	"testing"
)

// Test context for reflection tests
type ReflectTestContext struct {
	Count int
}

// Simple machine definition for testing
type SimpleReflectMachine struct {
	MachineDef `id:"simple" initial:"idle"`
	Idle       StateNode `on:"START->running"`
	Running    StateNode `on:"STOP->idle"`
}

func TestFromStruct_Simple(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	machine, err := FromStruct[SimpleReflectMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine.ID != "simple" {
		t.Errorf("expected ID 'simple', got %q", machine.ID)
	}
	if string(machine.Initial) != "idle" {
		t.Errorf("expected Initial 'idle', got %q", machine.Initial)
	}
	if len(machine.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(machine.States))
	}

	// Verify states exist
	if _, ok := machine.States["idle"]; !ok {
		t.Error("missing 'idle' state")
	}
	if _, ok := machine.States["running"]; !ok {
		t.Error("missing 'running' state")
	}

	// Verify transitions
	idle := machine.States["idle"]
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition on idle, got %d", len(idle.Transitions))
	}
	if string(idle.Transitions[0].Event) != "START" {
		t.Errorf("expected event 'START', got %q", idle.Transitions[0].Event)
	}
	if string(idle.Transitions[0].Target) != "running" {
		t.Errorf("expected target 'running', got %q", idle.Transitions[0].Target)
	}
}

// Machine with actions for testing
type ActionReflectMachine struct {
	MachineDef `id:"actions" initial:"idle"`
	Idle       StateNode `on:"START->running" entry:"onEnterIdle" exit:"onExitIdle"`
	Running    StateNode `on:"STOP->idle" entry:"onEnterRunning"`
}

func TestFromStruct_WithActions(t *testing.T) {
	enterIdleCalled := false
	exitIdleCalled := false
	enterRunningCalled := false

	registry := NewActionRegistry[ReflectTestContext]().
		WithAction("onEnterIdle", func(ctx *ReflectTestContext, e Event) {
			enterIdleCalled = true
		}).
		WithAction("onExitIdle", func(ctx *ReflectTestContext, e Event) {
			exitIdleCalled = true
		}).
		WithAction("onEnterRunning", func(ctx *ReflectTestContext, e Event) {
			enterRunningCalled = true
		})

	machine, err := FromStruct[ActionReflectMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify actions are registered
	if len(machine.Actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(machine.Actions))
	}

	// Verify state entry/exit actions
	idle := machine.States["idle"]
	if len(idle.Entry) != 1 || string(idle.Entry[0]) != "onEnterIdle" {
		t.Errorf("unexpected idle entry actions: %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || string(idle.Exit[0]) != "onExitIdle" {
		t.Errorf("unexpected idle exit actions: %v", idle.Exit)
	}

	// Test with interpreter
	interp := NewInterpreter(machine)
	interp.Start()

	if !enterIdleCalled {
		t.Error("expected onEnterIdle to be called on start")
	}

	interp.Send(Event{Type: "START"})

	if !exitIdleCalled {
		t.Error("expected onExitIdle to be called on transition")
	}
	if !enterRunningCalled {
		t.Error("expected onEnterRunning to be called on transition")
	}
}

// Machine with guards for testing
type GuardReflectMachine struct {
	MachineDef `id:"guards" initial:"idle"`
	Idle       StateNode `on:"START->running:canStart"`
	Running    StateNode `on:"STOP->idle"`
}

func TestFromStruct_WithGuards(t *testing.T) {
	canStart := false

	registry := NewActionRegistry[ReflectTestContext]().
		WithGuard("canStart", func(ctx ReflectTestContext, e Event) bool {
			return canStart
		})

	machine, err := FromStruct[GuardReflectMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify guard is registered
	if len(machine.Guards) != 1 {
		t.Errorf("expected 1 guard, got %d", len(machine.Guards))
	}

	// Test guard behavior
	interp := NewInterpreter(machine)
	interp.Start()

	// Guard returns false, should not transition
	interp.Send(Event{Type: "START"})
	if interp.State().Value != "idle" {
		t.Errorf("expected to stay in 'idle' when guard false, got %q", interp.State().Value)
	}

	// Enable guard
	canStart = true
	interp.Send(Event{Type: "START"})
	if interp.State().Value != "running" {
		t.Errorf("expected to transition to 'running' when guard true, got %q", interp.State().Value)
	}
}

// Machine with final state for testing
type FinalReflectMachine struct {
	MachineDef `id:"final" initial:"active"`
	Active     StateNode `on:"COMPLETE->done"`
	Done       FinalNode
}

func TestFromStruct_WithFinalState(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	machine, err := FromStruct[FinalReflectMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify final state
	done := machine.States["done"]
	if !done.IsFinal() {
		t.Error("expected 'done' to be a final state")
	}

	// Test with interpreter
	interp := NewInterpreter(machine)
	interp.Start()

	if interp.Done() {
		t.Error("should not be done initially")
	}

	interp.Send(Event{Type: "COMPLETE"})

	if !interp.Done() {
		t.Error("should be done after transitioning to final state")
	}
}

// Hierarchical state definitions
type ChildState struct {
	StateNode `on:"NEXT->sibling"`
}

type SiblingState struct {
	StateNode `on:"BACK->child"`
}

type ParentState struct {
	CompoundNode `initial:"child" on:"RESET->done"`
	Child        ChildState
	Sibling      SiblingState
}

type HierarchicalReflectMachine struct {
	MachineDef `id:"hierarchical" initial:"parent"`
	Parent     ParentState
	Done       FinalNode
}

func TestFromStruct_Hierarchical(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	machine, err := FromStruct[HierarchicalReflectMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify structure
	if len(machine.States) != 4 { // parent, child, sibling, done
		t.Errorf("expected 4 states, got %d", len(machine.States))
	}

	parent := machine.States["parent"]
	if !parent.IsCompound() {
		t.Error("expected 'parent' to be compound state")
	}
	if string(parent.Initial) != "child" {
		t.Errorf("expected parent initial 'child', got %q", parent.Initial)
	}
	if len(parent.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(parent.Children))
	}

	child := machine.States["child"]
	if string(child.Parent) != "parent" {
		t.Errorf("expected child parent 'parent', got %q", child.Parent)
	}

	// Test with interpreter
	interp := NewInterpreter(machine)
	interp.Start()

	// Should start in the leaf state
	if interp.State().Value != "child" {
		t.Errorf("expected to start in 'child', got %q", interp.State().Value)
	}

	// Matches should work for ancestors
	if !interp.Matches("parent") {
		t.Error("expected Matches('parent') to be true")
	}

	// Transition within compound state
	interp.Send(Event{Type: "NEXT"})
	if interp.State().Value != "sibling" {
		t.Errorf("expected to be in 'sibling', got %q", interp.State().Value)
	}

	// Event bubbling to parent
	interp.Send(Event{Type: "RESET"})
	if interp.State().Value != "done" {
		t.Errorf("expected to be in 'done' after RESET, got %q", interp.State().Value)
	}
}

// Test with context
type ContextReflectMachine struct {
	MachineDef `id:"context" initial:"counting"`
	Counting   StateNode `on:"INCREMENT->counting" entry:"incrementCount"`
	Done       FinalNode
}

func TestFromStructWithContext(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]().
		WithAction("incrementCount", func(ctx *ReflectTestContext, e Event) {
			ctx.Count++
		})

	initialCtx := ReflectTestContext{Count: 10}

	machine, err := FromStructWithContext[ContextReflectMachine, ReflectTestContext](registry, initialCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine.Context.Count != 10 {
		t.Errorf("expected initial count 10, got %d", machine.Context.Count)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Entry action should increment
	if interp.State().Context.Count != 11 {
		t.Errorf("expected count 11 after start, got %d", interp.State().Context.Count)
	}

	interp.Send(Event{Type: "INCREMENT"})
	if interp.State().Context.Count != 12 {
		t.Errorf("expected count 12 after INCREMENT, got %d", interp.State().Context.Count)
	}
}

// Test parity with fluent builder
func TestFromStruct_ParityWithBuilder(t *testing.T) {
	// Build with fluent API
	builderMachine, err := NewMachine[ReflectTestContext]("traffic").
		WithInitial("green").
		WithAction("logGreen", func(ctx *ReflectTestContext, e Event) {}).
		WithAction("logYellow", func(ctx *ReflectTestContext, e Event) {}).
		WithAction("logRed", func(ctx *ReflectTestContext, e Event) {}).
		State("green").
		OnEntry("logGreen").
		On("TIMER").Target("yellow").
		Done().
		State("yellow").
		OnEntry("logYellow").
		On("TIMER").Target("red").
		Done().
		State("red").
		OnEntry("logRed").
		On("TIMER").Target("green").
		Done().
		Build()

	if err != nil {
		t.Fatalf("builder error: %v", err)
	}

	// Build with reflection
	type TrafficMachine struct {
		MachineDef `id:"traffic" initial:"green"`
		Green      StateNode `on:"TIMER->yellow" entry:"logGreen"`
		Yellow     StateNode `on:"TIMER->red" entry:"logYellow"`
		Red        StateNode `on:"TIMER->green" entry:"logRed"`
	}

	registry := NewActionRegistry[ReflectTestContext]().
		WithAction("logGreen", func(ctx *ReflectTestContext, e Event) {}).
		WithAction("logYellow", func(ctx *ReflectTestContext, e Event) {}).
		WithAction("logRed", func(ctx *ReflectTestContext, e Event) {})

	reflectMachine, err := FromStruct[TrafficMachine, ReflectTestContext](registry)
	if err != nil {
		t.Fatalf("reflect error: %v", err)
	}

	// Compare machines
	if builderMachine.ID != reflectMachine.ID {
		t.Errorf("ID mismatch: builder=%q, reflect=%q", builderMachine.ID, reflectMachine.ID)
	}
	if builderMachine.Initial != reflectMachine.Initial {
		t.Errorf("Initial mismatch: builder=%q, reflect=%q", builderMachine.Initial, reflectMachine.Initial)
	}
	if len(builderMachine.States) != len(reflectMachine.States) {
		t.Errorf("States count mismatch: builder=%d, reflect=%d", len(builderMachine.States), len(reflectMachine.States))
	}

	// Verify behavior is identical
	builderInterp := NewInterpreter(builderMachine)
	reflectInterp := NewInterpreter(reflectMachine)

	builderInterp.Start()
	reflectInterp.Start()

	for _, event := range []string{"TIMER", "TIMER", "TIMER", "TIMER"} {
		builderInterp.Send(Event{Type: EventType(event)})
		reflectInterp.Send(Event{Type: EventType(event)})

		if builderInterp.State().Value != reflectInterp.State().Value {
			t.Errorf("state mismatch after %s: builder=%q, reflect=%q",
				event, builderInterp.State().Value, reflectInterp.State().Value)
		}
	}
}

// Test validation errors
type InvalidMachine struct {
	MachineDef `id:"invalid" initial:"nonexistent"`
	Idle       StateNode
}

func TestFromStruct_ValidationError(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	_, err := FromStruct[InvalidMachine, ReflectTestContext](registry)
	if err == nil {
		t.Fatal("expected validation error for nonexistent initial state")
	}
}

// Test missing action error
type MissingActionMachine struct {
	MachineDef `id:"missing" initial:"idle"`
	Idle       StateNode `entry:"nonexistentAction"`
}

func TestFromStruct_MissingAction(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	_, err := FromStruct[MissingActionMachine, ReflectTestContext](registry)
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

// Test missing guard error
type MissingGuardMachine struct {
	MachineDef `id:"missing" initial:"idle"`
	Idle       StateNode `on:"START->running:nonexistentGuard"`
	Running    StateNode
}

func TestFromStruct_MissingGuard(t *testing.T) {
	registry := NewActionRegistry[ReflectTestContext]()

	_, err := FromStruct[MissingGuardMachine, ReflectTestContext](registry)
	if err == nil {
		t.Fatal("expected error for missing guard")
	}
}
