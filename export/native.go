package export

import (
	"encoding/json"
	"fmt"

	"go.klarlabs.de/statekit/internal/ir"
	"go.klarlabs.de/statekit/viz"
)

// NativeExporter converts a MachineConfig to the native Statekit visualization format.
type NativeExporter[C any] struct {
	machine *ir.MachineConfig[C]
}

// NewNativeExporter creates a new exporter for the given machine configuration.
func NewNativeExporter[C any](machine *ir.MachineConfig[C]) *NativeExporter[C] {
	return &NativeExporter[C]{machine: machine}
}

// Export converts the machine configuration to a VizMachine.
//
// It delegates to viz.FromMachine, which is the same translation without the
// exporter wrapper — call that directly when you want the visualization model
// rather than its JSON encoding.
func (e *NativeExporter[C]) Export() *viz.VizMachine {
	return viz.FromMachine(e.machine)
}

// ExportJSON returns the machine as a JSON string.
func (e *NativeExporter[C]) ExportJSON() (string, error) {
	vm := e.Export()
	b, err := json.Marshal(vm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(b), nil
}

// ExportJSONIndent returns the machine as a formatted JSON string.
func (e *NativeExporter[C]) ExportJSONIndent(prefix, indent string) (string, error) {
	vm := e.Export()
	b, err := json.MarshalIndent(vm, prefix, indent)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(b), nil
}
