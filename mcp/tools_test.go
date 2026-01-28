package mcp

import (
	"encoding/json"
	"testing"
)

var trafficLightJSON = `{
	"id": "traffic-light",
	"initial": "green",
	"states": {
		"green": {"id": "green", "type": "atomic", "transitions": [{"event": "TIMER", "target": "yellow"}]},
		"yellow": {"id": "yellow", "type": "atomic", "transitions": [{"event": "TIMER", "target": "red"}]},
		"red": {"id": "red", "type": "atomic", "transitions": [{"event": "TIMER", "target": "green"}]}
	}
}`

func setupRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	handler := handleCreateMachine(reg)
	_, err := handler(CreateMachineInput{Definition: json.RawMessage(trafficLightJSON)})
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return reg
}

func TestHandleCreateMachine(t *testing.T) {
	reg := NewRegistry()
	handler := handleCreateMachine(reg)

	out, err := handler(CreateMachineInput{Definition: json.RawMessage(trafficLightJSON)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != "traffic-light" {
		t.Errorf("ID = %q, want traffic-light", out.ID)
	}
	if out.CurrentState != "green" {
		t.Errorf("state = %q, want green", out.CurrentState)
	}
}

func TestHandleListMachines(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleListMachines(reg)

	out, err := handler(ListMachinesInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(out))
	}
}

func TestHandleGetState(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleGetState(reg)

	out, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.CurrentState != "green" {
		t.Errorf("state = %q, want green", out.CurrentState)
	}
	if out.Done {
		t.Error("expected not done")
	}
	if len(out.Path) == 0 {
		t.Error("expected non-empty path")
	}
}

func TestHandleGetState_NotFound(t *testing.T) {
	reg := NewRegistry()
	handler := handleGetState(reg)

	_, err := handler(MachineIDInput{MachineID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing machine")
	}
}

func TestHandleSendEvent(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleSendEvent(reg)

	out, err := handler(SendEventInput{MachineID: "traffic-light", Event: "TIMER"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.PreviousState != "green" {
		t.Errorf("prev = %q, want green", out.PreviousState)
	}
	if out.CurrentState != "yellow" {
		t.Errorf("current = %q, want yellow", out.CurrentState)
	}
	if !out.Transitioned {
		t.Error("expected transitioned = true")
	}
}

func TestHandleSendEvent_NoTransition(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleSendEvent(reg)

	out, err := handler(SendEventInput{MachineID: "traffic-light", Event: "UNKNOWN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Transitioned {
		t.Error("expected no transition for unknown event")
	}
}

func TestHandleGetContext(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleGetContext(reg)

	ctx, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Context should be nil map for default
	if ctx == nil {
		// That's fine — default context is nil map
	}
}

func TestHandleVisualizeMachine(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleVisualizeMachine(reg)

	data, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty viz data")
	}
}

func TestHandleValidateMachine(t *testing.T) {
	handler := handleValidateMachine(nil)

	out, err := handler(ValidateMachineInput{Definition: json.RawMessage(trafficLightJSON)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Errorf("expected valid, got errors: %v", out.Errors)
	}
}

func TestHandleValidateMachine_Invalid(t *testing.T) {
	handler := handleValidateMachine(nil)

	out, err := handler(ValidateMachineInput{Definition: json.RawMessage(`{invalid`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Error("expected invalid for bad JSON")
	}
}

func TestHandleExportMachine(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleExportMachine(reg)

	tests := []struct {
		format string
	}{
		{"json"},
		{"mermaid"},
		{"ascii"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			out, err := handler(ExportMachineInput{MachineID: "traffic-light", Format: tt.format})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestHandleExportMachine_UnknownFormat(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleExportMachine(reg)

	_, err := handler(ExportMachineInput{MachineID: "traffic-light", Format: "pdf"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
