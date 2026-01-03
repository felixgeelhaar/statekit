package main

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/plugin"
)

// TestPlugin is a simple test plugin that tracks calls
type TestPlugin[C any] struct {
	StartCalls      int
	StopCalls       int
	EnterCalls      int
	ExitCalls       int
	EventCalls      int
	TransitionCalls int
	ActionCalls     int
}

func (p *TestPlugin[C]) Name() string { return "test" }

func (p *TestPlugin[C]) OnStart(_ plugin.Context[C]) {
	p.StartCalls++
}

func (p *TestPlugin[C]) OnStop(_ plugin.Context[C]) {
	p.StopCalls++
}

func (p *TestPlugin[C]) OnEnter(_ plugin.Context[C], _ plugin.StateID) {
	p.EnterCalls++
}

func (p *TestPlugin[C]) OnExit(_ plugin.Context[C], _ plugin.StateID) {
	p.ExitCalls++
}

func (p *TestPlugin[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	p.EventCalls++
	return event
}

func (p *TestPlugin[C]) BeforeTransition(_ plugin.Context[C], _, _ plugin.StateID, _ plugin.Event) {
	p.TransitionCalls++
}

func (p *TestPlugin[C]) AfterTransition(_ plugin.Context[C], _, _ plugin.StateID, _ plugin.Event) {
}

func (p *TestPlugin[C]) BeforeAction(_ plugin.Context[C], _ plugin.ActionType, _ plugin.Event) {
	p.ActionCalls++
}

func (p *TestPlugin[C]) AfterAction(_ plugin.Context[C], _ plugin.ActionType, _ plugin.Event) {
}

func TestPlugin_HooksAreCalled(t *testing.T) {
	machine := buildOrderMachine()
	interp := statekit.NewInterpreter(machine)

	tp := &TestPlugin[OrderContext]{}
	interp.Use(tp)

	interp.Start()

	if tp.StartCalls != 1 {
		t.Errorf("Expected 1 OnStart call, got %d", tp.StartCalls)
	}

	// Should have entered initial state
	if tp.EnterCalls < 1 {
		t.Errorf("Expected at least 1 OnEnter call, got %d", tp.EnterCalls)
	}

	// Send an event
	interp.Send(statekit.Event{Type: "SUBMIT"})

	if tp.EventCalls != 1 {
		t.Errorf("Expected 1 OnEvent call, got %d", tp.EventCalls)
	}

	if tp.TransitionCalls < 1 {
		t.Errorf("Expected at least 1 transition, got %d", tp.TransitionCalls)
	}

	interp.Stop()

	if tp.StopCalls != 1 {
		t.Errorf("Expected 1 OnStop call, got %d", tp.StopCalls)
	}
}

func TestPlugin_MultiplePlugins(t *testing.T) {
	machine := buildOrderMachine()
	interp := statekit.NewInterpreter(machine)

	tp1 := &TestPlugin[OrderContext]{}
	tp2 := &TestPlugin[OrderContext]{}

	interp.Use(tp1)
	interp.Use(tp2)

	interp.Start()
	interp.Send(statekit.Event{Type: "SUBMIT"})
	interp.Stop()

	// Both plugins should receive all hooks
	if tp1.StartCalls != 1 || tp2.StartCalls != 1 {
		t.Error("Both plugins should receive OnStart")
	}

	if tp1.EventCalls != 1 || tp2.EventCalls != 1 {
		t.Error("Both plugins should receive OnEvent")
	}

	if tp1.StopCalls != 1 || tp2.StopCalls != 1 {
		t.Error("Both plugins should receive OnStop")
	}
}

func TestPlugin_EventTransformation(t *testing.T) {
	// Create a simple machine that accepts uppercase events
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)

	// Transformer that uppercases events
	transformer := NewEventTransformer[struct{}]()
	interp.Use(transformer)

	interp.Start()

	// Send lowercase event - should be transformed
	interp.Send(statekit.Event{Type: "go"}) // lowercase

	// Should have transitioned via transformed event
	if interp.State().Value != "done" {
		t.Errorf("Expected 'done', got %s (event transformation may have failed)", interp.State().Value)
	}
}

func TestPlugin_MetricsTracking(t *testing.T) {
	machine := buildOrderMachine()
	interp := statekit.NewInterpreter(machine)

	metrics := NewMetricsPlugin[OrderContext]()
	interp.Use(metrics)

	interp.Start()
	interp.Send(statekit.Event{Type: "SUBMIT"})
	interp.Send(statekit.Event{Type: "APPROVE"})
	interp.Stop()

	if metrics.eventCount != 2 {
		t.Errorf("Expected 2 events, got %d", metrics.eventCount)
	}

	if metrics.transitionCount != 2 {
		t.Errorf("Expected 2 transitions, got %d", metrics.transitionCount)
	}

	// Should have tracked time in states
	if len(metrics.stateTime) == 0 {
		t.Error("Expected state time tracking")
	}
}

func TestLoggingPlugin_Name(t *testing.T) {
	lp := NewLoggingPlugin[OrderContext]()
	if lp.Name() != "logging" {
		t.Errorf("Expected name 'logging', got %s", lp.Name())
	}
}

func TestMetricsPlugin_Name(t *testing.T) {
	mp := NewMetricsPlugin[OrderContext]()
	if mp.Name() != "metrics" {
		t.Errorf("Expected name 'metrics', got %s", mp.Name())
	}
}

func TestEventTransformer_Name(t *testing.T) {
	et := NewEventTransformer[OrderContext]()
	if et.Name() != "event-transformer" {
		t.Errorf("Expected name 'event-transformer', got %s", et.Name())
	}
}
