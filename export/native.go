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
func (e *NativeExporter[C]) Export() *viz.VizMachine {
	vm := &viz.VizMachine{
		ID:      e.machine.ID,
		Initial: string(e.machine.Initial),
		States:  make(map[string]*viz.VizState),
	}

	for id, state := range e.machine.States {
		vm.States[string(id)] = e.convertState(state)
	}

	// Compute depths
	for _, state := range vm.States {
		state.Depth = len(e.machine.GetAncestors(ir.StateID(state.ID)))
	}

	return vm
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

func (e *NativeExporter[C]) convertState(state *ir.StateConfig) *viz.VizState {
	vs := &viz.VizState{
		ID:          string(state.ID),
		Type:        viz.VizStateType(state.Type.String()),
		Initial:     string(state.Initial),
		Parent:      string(state.Parent),
		Children:    make([]string, len(state.Children)),
		Entry:       make([]string, len(state.Entry)),
		Exit:        make([]string, len(state.Exit)),
		Transitions: make([]viz.VizTransition, 0, len(state.Transitions)),
	}

	for i, child := range state.Children {
		vs.Children[i] = string(child)
	}

	for i, action := range state.Entry {
		vs.Entry[i] = string(action)
	}
	for i, action := range state.Exit {
		vs.Exit[i] = string(action)
	}

	if state.Type == ir.StateTypeHistory {
		vs.HistoryType = state.HistoryType.String()
		vs.HistoryDefault = string(state.HistoryDefault)
	}

	for _, t := range state.Transitions {
		vs.Transitions = append(vs.Transitions, convertTransition(t))
	}

	// Eventless ("always") transitions and tags (v1.x)
	for _, t := range state.Always {
		vs.Always = append(vs.Always, convertTransition(t))
	}
	if len(state.Tags) > 0 {
		vs.Tags = append(vs.Tags, state.Tags...)
	}

	for _, inv := range state.Invocations {
		vi := viz.VizInvoke{
			ID:  inv.ID,
			Src: string(inv.Src),
		}
		if inv.OnDone != nil {
			vi.OnDoneTarget = string(inv.OnDone.Target)
		}
		if inv.OnError != nil {
			vi.OnErrTarget = string(inv.OnError.Target)
		}
		vs.Invocations = append(vs.Invocations, vi)
	}

	return vs
}

// convertTransition maps an IR transition to its visualization form,
// including delayed metadata and raised internal events (v1.x).
func convertTransition(t *ir.TransitionConfig) viz.VizTransition {
	vt := viz.VizTransition{
		Event:   string(t.Event),
		Target:  string(t.Target),
		Guard:   string(t.Guard),
		Actions: make([]string, len(t.Actions)),
	}
	for i, a := range t.Actions {
		vt.Actions[i] = string(a)
	}
	if t.IsDelayed() {
		vt.IsDelayed = true
		vt.DelayMs = t.Delay.Milliseconds()
	}
	for _, r := range t.Raise {
		vt.Raise = append(vt.Raise, string(r))
	}
	vt.Internal = t.Internal
	return vt
}
