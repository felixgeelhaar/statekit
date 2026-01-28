package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
	"github.com/felixgeelhaar/statekit/lint"
	"github.com/felixgeelhaar/statekit/viz"
	"github.com/felixgeelhaar/statekit/viz/ascii"
	"github.com/felixgeelhaar/statekit/viz/mermaid"
)

// Tool input types

// CreateMachineInput is the input for the create_machine tool.
type CreateMachineInput struct {
	Definition map[string]any `json:"definition" jsonschema:"description=Statekit Native JSON machine definition"`
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
	Definition map[string]any `json:"definition" jsonschema:"description=Statekit Native JSON machine definition"`
}

// ExportMachineInput exports a machine in a given format.
type ExportMachineInput struct {
	MachineID string `json:"machine_id" jsonschema:"description=Machine instance ID"`
	Format    string `json:"format" jsonschema:"description=Export format: json, mermaid, or ascii"`
}

// ListMachinesInput is empty — lists all machines.
type ListMachinesInput struct{}

// Tool output types

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

func handleCreateMachine(reg *Registry) func(CreateMachineInput) (CreateMachineOutput, error) {
	return func(input CreateMachineInput) (CreateMachineOutput, error) {
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

func handleListMachines(reg *Registry) func(ListMachinesInput) ([]MachineInfo, error) {
	return func(_ ListMachinesInput) ([]MachineInfo, error) {
		return reg.List(), nil
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

func handleVisualizeMachine(reg *Registry) func(MachineIDInput) (json.RawMessage, error) {
	return func(input MachineIDInput) (json.RawMessage, error) {
		inst, ok := reg.Get(input.MachineID)
		if !ok {
			return nil, fmt.Errorf("machine %q not found", input.MachineID)
		}

		// Re-export from the running machine to get current state overlay
		exporter := export.NewNativeExporter(inst.machine)
		vm := exporter.Export()

		data, err := json.Marshal(vm)
		if err != nil {
			return nil, fmt.Errorf("marshal viz data: %w", err)
		}
		return data, nil
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
