package mcp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/export"
	"go.klarlabs.de/statekit/lint"
	"go.klarlabs.de/statekit/viz"
	"go.klarlabs.de/statekit/viz/ascii"
	"go.klarlabs.de/statekit/viz/mermaid"
)

// Tool input types

// CreateMachineInput is the input for the create_machine tool.
type CreateMachineInput struct {
	Definition map[string]any `json:"definition" jsonschema:"description=JSON object with id (optional, auto-generated if empty), initial (string), and states (object mapping state IDs to state definitions)"`
}

// MachineIDInput identifies a machine by ID.
type MachineIDInput struct {
	MachineID string `json:"machine_id" jsonschema:"description=Machine instance ID"`
}

// SendEventInput sends an event to a machine.
type SendEventInput struct {
	MachineID string         `json:"machine_id" jsonschema:"description=Machine instance ID"`
	Event     string         `json:"event" jsonschema:"description=Event type to send"`
	Payload   map[string]any `json:"payload,omitempty" jsonschema:"description=Optional event payload"`
}

// ValidateMachineInput validates a machine definition.
type ValidateMachineInput struct {
	Definition map[string]any `json:"definition" jsonschema:"description=JSON object with id, initial (string), and states (object mapping state IDs to state definitions)"`
}

// ExportMachineInput exports a machine in a given format.
type ExportMachineInput struct {
	MachineID string `json:"machine_id" jsonschema:"description=Machine instance ID"`
	Format    string `json:"format" jsonschema:"description=Export format: json, mermaid, or ascii"`
}

// ListMachinesInput is empty — lists all machines.
type ListMachinesInput struct{}

// Tool output types

// MachineListOutput envelopes the machine list so the result is a JSON
// object (required for structuredContent) rather than a bare array.
type MachineListOutput struct {
	Items []MachineInfo `json:"items"`
	Total int           `json:"total"`
}

// CreateMachineOutput is returned after creating a machine.
type CreateMachineOutput struct {
	ID           string `json:"id"`
	CurrentState string `json:"currentState"`
}

// StateOutput describes the current state of a machine.
type StateOutput struct {
	CurrentState string   `json:"currentState"`
	Done         bool     `json:"done"`
	Path         []string `json:"path"`
}

// SendEventOutput describes the result of sending an event.
type SendEventOutput struct {
	PreviousState string `json:"previousState"`
	CurrentState  string `json:"currentState"`
	Transitioned  bool   `json:"transitioned"`
	Done          bool   `json:"done"`
}

// ValidateOutput describes validation results.
type ValidateOutput struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Tool handlers

// DeleteMachineOutput is returned after deleting a machine.
type DeleteMachineOutput struct {
	Deleted bool `json:"deleted"`
}

func handleCreateMachine(reg *Registry) func(CreateMachineInput) (CreateMachineOutput, error) {
	return func(input CreateMachineInput) (CreateMachineOutput, error) {
		// Auto-generate ID if missing or empty
		if id, _ := input.Definition["id"].(string); id == "" {
			input.Definition["id"] = generateUUID()
		}

		data, err := json.Marshal(input.Definition)
		if err != nil {
			return CreateMachineOutput{}, fmt.Errorf("marshal definition: %w", err)
		}
		vm, err := viz.ParseNativeJSON(data)
		if err != nil {
			return CreateMachineOutput{}, fmt.Errorf("parse definition: %w", err)
		}

		if err := reg.Create(vm); err != nil {
			return CreateMachineOutput{}, err
		}

		inst, _ := reg.Get(vm.ID)
		return CreateMachineOutput{
			ID:           vm.ID,
			CurrentState: string(inst.interp.State().Value),
		}, nil
	}
}

// generateUUID returns a random UUID v4 string.
func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func handleListMachines(reg *Registry) func(ListMachinesInput) (MachineListOutput, error) {
	return func(_ ListMachinesInput) (MachineListOutput, error) {
		machines := reg.List()
		return MachineListOutput{Items: machines, Total: len(machines)}, nil
	}
}

func handleGetState(reg *Registry) func(MachineIDInput) (StateOutput, error) {
	return func(input MachineIDInput) (StateOutput, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return StateOutput{}, fmt.Errorf("machine %q not found", input.MachineID)
		}

		state := inst.interp.State()
		path := buildStatePath(inst.vizData, string(state.Value))

		return StateOutput{
			CurrentState: string(state.Value),
			Done:         inst.interp.Done(),
			Path:         path,
		}, nil
	}
}

func handleSendEvent(reg *Registry) func(SendEventInput) (SendEventOutput, error) {
	return func(input SendEventInput) (SendEventOutput, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return SendEventOutput{}, fmt.Errorf("machine %q not found", input.MachineID)
		}

		prev := string(inst.interp.State().Value)

		evt := statekit.Event{
			Type: statekit.EventType(input.Event),
		}
		if input.Payload != nil {
			evt.Payload = input.Payload
		}

		inst.interp.Send(evt)

		current := string(inst.interp.State().Value)

		return SendEventOutput{
			PreviousState: prev,
			CurrentState:  current,
			Transitioned:  prev != current,
			Done:          inst.interp.Done(),
		}, nil
	}
}

func handleGetContext(reg *Registry) func(MachineIDInput) (Ctx, error) {
	return func(input MachineIDInput) (Ctx, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return nil, fmt.Errorf("machine %q not found", input.MachineID)
		}
		return inst.interp.State().Context, nil
	}
}

func handleGetMachineData(reg *Registry) func(MachineIDInput) (*viz.VizMachine, error) {
	return func(input MachineIDInput) (*viz.VizMachine, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return nil, fmt.Errorf("machine %q not found", input.MachineID)
		}

		// Re-export from the running machine to get current state overlay.
		// Returning the typed VizMachine lets the tool advertise an
		// output schema and emit structuredContent; it marshals to the
		// same JSON object as before.
		exporter := export.NewNativeExporter(inst.machine)
		return exporter.Export(), nil
	}
}

func handleValidateMachine(_ *Registry) func(ValidateMachineInput) (ValidateOutput, error) {
	return func(input ValidateMachineInput) (ValidateOutput, error) {
		data, err := json.Marshal(input.Definition)
		if err != nil {
			return ValidateOutput{
				Valid:  false,
				Errors: []string{fmt.Sprintf("marshal error: %v", err)},
			}, nil
		}
		vm, err := viz.ParseNativeJSON(data)
		if err != nil {
			return ValidateOutput{
				Valid:  false,
				Errors: []string{fmt.Sprintf("parse error: %v", err)},
			}, nil
		}

		// Build to trigger validation
		machine, buildErr := buildFromViz(vm)
		if buildErr != nil {
			return ValidateOutput{
				Valid:  false,
				Errors: []string{buildErr.Error()},
			}, nil
		}

		// Run lint rules
		result := lint.Lint(machine)

		out := ValidateOutput{Valid: !result.HasErrors()}
		for _, d := range result.Errors() {
			out.Errors = append(out.Errors, d.String())
		}
		for _, d := range result.Warnings() {
			out.Warnings = append(out.Warnings, d.String())
		}
		return out, nil
	}
}

func handleExportMachine(reg *Registry) func(ExportMachineInput) (string, error) {
	return func(input ExportMachineInput) (string, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return "", fmt.Errorf("machine %q not found", input.MachineID)
		}

		exporter := export.NewNativeExporter(inst.machine)
		vm := exporter.Export()

		switch input.Format {
		case "json":
			data, err := json.MarshalIndent(vm, "", "  ")
			if err != nil {
				return "", err
			}
			return string(data), nil
		case "mermaid":
			r := mermaid.NewRenderer()
			return r.Render(vm), nil
		case "ascii":
			r := ascii.NewRenderer()
			return r.Render(vm), nil
		default:
			return "", fmt.Errorf("unknown format %q (supported: json, mermaid, ascii)", input.Format)
		}
	}
}

func handleResetMachine(reg *Registry) func(MachineIDInput) (CreateMachineOutput, error) {
	return func(input MachineIDInput) (CreateMachineOutput, error) {
		if err := reg.Reset(input.MachineID); err != nil {
			return CreateMachineOutput{}, err
		}
		inst, _ := reg.Get(input.MachineID)
		return CreateMachineOutput{
			ID:           input.MachineID,
			CurrentState: string(inst.interp.State().Value),
		}, nil
	}
}

func handleDeleteMachine(reg *Registry) func(MachineIDInput) (DeleteMachineOutput, error) {
	return func(input MachineIDInput) (DeleteMachineOutput, error) {
		deleted := reg.Delete(input.MachineID)
		if !deleted {
			return DeleteMachineOutput{}, fmt.Errorf("machine %q not found", input.MachineID)
		}
		return DeleteMachineOutput{Deleted: true}, nil
	}
}

// buildStatePath returns the ancestor path from root to the given state.
func buildStatePath(vm *viz.VizMachine, stateID string) []string {
	var path []string
	current := stateID
	for current != "" {
		path = append([]string{current}, path...)
		s := vm.States[current]
		if s == nil {
			break
		}
		current = s.Parent
	}
	return path
}
