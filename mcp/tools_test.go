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

func trafficLightDef() map[string]any {
	var m map[string]any
	_ = json.Unmarshal([]byte(trafficLightJSON), &m)
	return m
}

func setupRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	handler := handleCreateMachine(reg)
	_, err := handler(CreateMachineInput{Definition: trafficLightDef()})
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	return reg
}

func TestHandleCreateMachine(t *testing.T) {
	reg := NewRegistry()
	handler := handleCreateMachine(reg)

	out, err := handler(CreateMachineInput{Definition: trafficLightDef()})
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
	if out.Total != 1 {
		t.Fatalf("expected total 1, got %d", out.Total)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(out.Items))
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
	// Context should be nil map for default — just verify no error
	_ = ctx
}

func TestHandleGetMachineData(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleGetMachineData(reg)

	data, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil viz data")
	}
	if data.ID != "traffic-light" {
		t.Errorf("ID = %q, want traffic-light", data.ID)
	}
	if len(data.States) == 0 {
		t.Error("expected non-empty states")
	}
}

func TestHandleValidateMachine(t *testing.T) {
	handler := handleValidateMachine(nil)

	out, err := handler(ValidateMachineInput{Definition: trafficLightDef()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Valid {
		t.Errorf("expected valid, got errors: %v", out.Errors)
	}
}

func TestHandleValidateMachine_Invalid(t *testing.T) {
	handler := handleValidateMachine(nil)

	// Pass a map that won't produce a valid machine (missing id, initial, states)
	out, err := handler(ValidateMachineInput{Definition: map[string]any{"foo": "bar"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Valid {
		t.Error("expected invalid for bad definition")
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

func TestHandleResetMachine(t *testing.T) {
	reg := setupRegistry(t)

	// Advance state
	sendHandler := handleSendEvent(reg)
	_, _ = sendHandler(SendEventInput{MachineID: "traffic-light", Event: "TIMER"})

	// Verify not in initial state
	inst, _ := reg.Get("traffic-light")
	if string(inst.interp.State().Value) == "green" {
		t.Fatal("expected state to have changed from green")
	}

	// Reset
	handler := handleResetMachine(reg)
	out, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.CurrentState != "green" {
		t.Errorf("state = %q, want green", out.CurrentState)
	}
}

func TestHandleResetMachine_NotFound(t *testing.T) {
	reg := NewRegistry()
	handler := handleResetMachine(reg)

	_, err := handler(MachineIDInput{MachineID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing machine")
	}
}

func TestHandleDeleteMachine(t *testing.T) {
	reg := setupRegistry(t)
	handler := handleDeleteMachine(reg)

	out, err := handler(MachineIDInput{MachineID: "traffic-light"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Deleted {
		t.Error("expected deleted = true")
	}

	// Verify gone
	_, ok := reg.Get("traffic-light")
	if ok {
		t.Error("expected machine to be removed")
	}
}

func TestHandleDeleteMachine_NotFound(t *testing.T) {
	reg := NewRegistry()
	handler := handleDeleteMachine(reg)

	_, err := handler(MachineIDInput{MachineID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing machine")
	}
}

func TestHandleCreateMachine_AutoGenerateID(t *testing.T) {
	reg := NewRegistry()
	handler := handleCreateMachine(reg)

	def := map[string]any{
		"initial": "on",
		"states": map[string]any{
			"on":  map[string]any{"id": "on", "type": "atomic", "transitions": []any{}},
			"off": map[string]any{"id": "off", "type": "atomic", "transitions": []any{}},
		},
	}

	out, err := handler(CreateMachineInput{Definition: def})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if len(out.ID) < 10 {
		t.Errorf("ID %q looks too short for a UUID", out.ID)
	}
	if out.CurrentState != "on" {
		t.Errorf("state = %q, want on", out.CurrentState)
	}
}
