package viz

import (
	"encoding/json"
	"fmt"
)

// ParseNativeJSON converts Native JSON to VizMachine.
func ParseNativeJSON(data []byte) (*VizMachine, error) {
	var vm VizMachine
	if err := json.Unmarshal(data, &vm); err != nil {
		return nil, fmt.Errorf("unmarshal Native JSON: %w", err)
	}

	// Ensure map is initialized
	if vm.States == nil {
		vm.States = make(map[string]*VizState)
	}

	// Calculate depths just in case
	calculateDepths(&vm)

	return &vm, nil
}

// calculateDepths sets the Depth field for all states.
func calculateDepths(vm *VizMachine) {
	for id, state := range vm.States {
		state.Depth = getDepth(vm, id)
	}
}

func getDepth(vm *VizMachine, stateID string) int {
	state := vm.States[stateID]
	if state == nil || state.Parent == "" {
		return 0
	}
	return 1 + getDepth(vm, state.Parent)
}