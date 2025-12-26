package export

import (
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

func TestXStateExporter_SimpleMachine(t *testing.T) {
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

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

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
	if green.On == nil || green.On["TIMER"].Target != "yellow" {
		t.Error("expected green->yellow transition on TIMER")
	}
}

func TestXStateExporter_WithEntryExitActions(t *testing.T) {
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

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	idle := result.States["idle"]
	if len(idle.Entry) != 1 || idle.Entry[0] != "onEnter" {
		t.Errorf("expected entry action 'onEnter', got %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || idle.Exit[0] != "onExit" {
		t.Errorf("expected exit action 'onExit', got %v", idle.Exit)
	}
}

func TestXStateExporter_WithTransitionActions(t *testing.T) {
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

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	idle := result.States["idle"]
	if idle.On["GO"].Actions == nil || len(idle.On["GO"].Actions) != 1 {
		t.Error("expected transition action")
	}
	if idle.On["GO"].Actions[0] != "doAction" {
		t.Errorf("expected action 'doAction', got %s", idle.On["GO"].Actions[0])
	}
}

func TestXStateExporter_WithGuard(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		WithGuard("canGo", func(ctx struct{}, e ir.Event) bool { return true }).
		State("idle").
		On("GO").Target("active").Guard("canGo").
		Done().
		State("active").
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	idle := result.States["idle"]
	if idle.On["GO"].Guard != "canGo" {
		t.Errorf("expected guard 'canGo', got %s", idle.On["GO"].Guard)
	}
}

func TestXStateExporter_FinalState(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("running").
		State("running").
		On("COMPLETE").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	done := result.States["done"]
	if done.Type != "final" {
		t.Errorf("expected type 'final', got %s", done.Type)
	}
}

func TestXStateExporter_HierarchicalStates(t *testing.T) {
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

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Check compound state
	active := result.States["active"]
	if active.Initial != "idle" {
		t.Errorf("expected initial 'idle', got %s", active.Initial)
	}
	if active.States == nil || len(active.States) != 2 {
		t.Fatalf("expected 2 nested states, got %d", len(active.States))
	}

	// Check nested states
	idle := active.States["idle"]
	if idle.On["START"].Target != "working" {
		t.Error("expected idle->working transition on START")
	}

	working := active.States["working"]
	if working.On["STOP"].Target != "idle" {
		t.Error("expected working->idle transition on STOP")
	}
}

func TestXStateExporter_DeepHierarchy(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("level1").
		State("level1").
		WithInitial("level2").
		State("level2").
		WithInitial("level3").
		State("level3").End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Navigate through hierarchy
	level1 := result.States["level1"]
	if level1.Initial != "level2" {
		t.Error("expected level1.initial = level2")
	}

	level2 := level1.States["level2"]
	if level2.Initial != "level3" {
		t.Error("expected level2.initial = level3")
	}

	_, ok := level2.States["level3"]
	if !ok {
		t.Error("expected level3 state to exist")
	}
}

func TestXStateExporter_JSONOutput(t *testing.T) {
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

	exporter := NewXStateExporter(machine)

	// Test compact JSON
	jsonStr, err := exporter.ExportJSON()
	if err != nil {
		t.Fatalf("failed to export JSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed XStateMachine
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("exported JSON is invalid: %v", err)
	}

	if parsed.ID != "test" {
		t.Errorf("expected ID 'test', got %s", parsed.ID)
	}

	// Test indented JSON
	indentedJSON, err := exporter.ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("failed to export indented JSON: %v", err)
	}

	// Verify it contains newlines (is indented)
	if len(indentedJSON) <= len(jsonStr) {
		t.Error("indented JSON should be longer than compact JSON")
	}
}

func TestXStateExporter_ComplexMachine(t *testing.T) {
	// Build a complex machine similar to pedestrian light
	machine, err := statekit.NewMachine[struct{}]("pedestrian_signal").
		WithInitial("active").
		WithAction("enterActive", func(ctx *struct{}, e statekit.Event) {}).
		WithAction("enterDontWalk", func(ctx *struct{}, e statekit.Event) {}).
		State("active").
		WithInitial("dont_walk").
		OnEntry("enterActive").
		On("MAINTENANCE").Target("maintenance").End().
		State("dont_walk").
		OnEntry("enterDontWalk").
		On("BUTTON").Target("walk").
		End().
		End().
		State("walk").
		On("TIMER").Target("countdown").
		End().
		End().
		State("countdown").
		WithInitial("flashing").
		State("flashing").
		On("TIMER").Target("warning").
		End().
		End().
		State("warning").
		On("TIMER").Target("dont_walk").
		End().
		End().
		End().
		Done().
		State("maintenance").
		On("RESUME").Target("active").
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	exporter := NewXStateExporter(machine)
	result, err := exporter.Export()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Verify structure
	if result.ID != "pedestrian_signal" {
		t.Errorf("expected ID 'pedestrian_signal', got %s", result.ID)
	}
	if result.Initial != "active" {
		t.Errorf("expected initial 'active', got %s", result.Initial)
	}

	// Check active state
	active := result.States["active"]
	if active.Initial != "dont_walk" {
		t.Errorf("expected active.initial 'dont_walk', got %s", active.Initial)
	}
	if len(active.Entry) != 1 || active.Entry[0] != "enterActive" {
		t.Errorf("expected entry action 'enterActive', got %v", active.Entry)
	}

	// Check nested countdown state
	countdown := active.States["countdown"]
	if countdown.Initial != "flashing" {
		t.Errorf("expected countdown.initial 'flashing', got %s", countdown.Initial)
	}
	if len(countdown.States) != 2 {
		t.Errorf("expected 2 countdown children, got %d", len(countdown.States))
	}

	// Verify JSON export works
	jsonStr, err := exporter.ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("failed to export JSON: %v", err)
	}

	t.Logf("Exported XState JSON:\n%s", jsonStr)
}
