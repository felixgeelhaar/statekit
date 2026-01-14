package export

import (
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/viz"
)

func TestNativeExporter_SimpleMachine(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("traffic_light").
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
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewNativeExporter(machine)
	result := exporter.Export()

	if result.ID != "traffic_light" {
		t.Errorf("expected ID 'traffic_light', got %s", result.ID)
	}

	if result.Initial != "green" {
		t.Errorf("expected initial 'green', got %s", result.Initial)
	}

	if len(result.States) != 3 {
		t.Errorf("expected 3 states, got %d", len(result.States))
	}

	// Check green state
	green, ok := result.States["green"]
	if !ok {
		t.Fatal("expected 'green' state")
	}
	
	// Find transition
	found := false
	for _, tr := range green.Transitions {
		if tr.Event == "TIMER" && tr.Target == "yellow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected green->yellow transition on TIMER")
	}
}

func TestNativeExporter_WithEntryExitActions(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		WithAction("onEnter", func(ctx *struct{}, e statekit.Event) {}).
		WithAction("onExit", func(ctx *struct{}, e statekit.Event) {}).
		State("idle").
		OnEntry("onEnter").
		OnExit("onExit").
		On("ACTIVATE").Target("active").
		Done().
		State("active").
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewNativeExporter(machine)
	result := exporter.Export()

	idle := result.States["idle"]
	if len(idle.Entry) != 1 || idle.Entry[0] != "onEnter" {
		t.Errorf("expected entry action 'onEnter', got %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || idle.Exit[0] != "onExit" {
		t.Errorf("expected exit action 'onExit', got %v", idle.Exit)
	}
}

func TestNativeExporter_WithTransitionActions(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		WithAction("doAction", func(ctx *struct{}, e statekit.Event) {}).
		State("idle").
		On("GO").Target("active").Do("doAction").
		Done().
		State("active").
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewNativeExporter(machine)
	result := exporter.Export()

	idle := result.States["idle"]
	var transition viz.VizTransition
	found := false
	for _, t := range idle.Transitions {
		if t.Event == "GO" {
			transition = t
			found = true
			break
		}
	}
	
	if !found {
		t.Fatal("expected transition GO")
	}

	if len(transition.Actions) != 1 {
		t.Error("expected transition action")
	}
	if transition.Actions[0] != "doAction" {
		t.Errorf("expected action 'doAction', got %s", transition.Actions[0])
	}
}

func TestNativeExporter_HierarchicalStates(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").
		On("START").Target("working").
		End().
		End().
		State("working").
		On("STOP").Target("idle").
		End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewNativeExporter(machine)
	result := exporter.Export()

	// Check compound state
	active := result.States["active"]
	if active.Initial != "idle" {
		t.Errorf("expected initial 'idle', got %s", active.Initial)
	}
	
	// In native format, States is flat, but Children contains IDs
	if len(active.Children) != 2 {
		t.Fatalf("expected 2 child states, got %d", len(active.Children))
	}

	// Check nested states exist in the flat map
	idle := result.States["idle"]
	if idle == nil {
		t.Fatal("expected idle state")
	}
	if idle.Parent != "active" {
		t.Errorf("expected idle parent 'active', got %s", idle.Parent)
	}
	
	working := result.States["working"]
	if working == nil {
		t.Fatal("expected working state")
	}
}

func TestNativeExporter_JSONOutput(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("active").
		Done().
		State("active").
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewNativeExporter(machine)

	// Test compact JSON
	jsonStr, err := exporter.ExportJSON()
	if err != nil {
		t.Fatalf("failed to export JSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed viz.VizMachine
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("exported JSON is invalid: %v", err)
	}

	if parsed.ID != "test" {
		t.Errorf("expected ID 'test', got %s", parsed.ID)
	}
}
