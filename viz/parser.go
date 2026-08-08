package viz

import (
	"encoding/json"
	"fmt"
)

// rawState mirrors VizState but adds fields for nested and XState shorthand input.
type rawState struct {
	ID             string               `json:"id"`
	Type           VizStateType         `json:"type"`
	Initial        string               `json:"initial,omitempty"`
	Parent         string               `json:"parent,omitempty"`
	Children       []string             `json:"children,omitempty"`
	Transitions    []VizTransition      `json:"transitions,omitempty"`
	Always         []VizTransition      `json:"always,omitempty"`
	Tags           []string             `json:"tags,omitempty"`
	Entry          []string             `json:"entry,omitempty"`
	Exit           []string             `json:"exit,omitempty"`
	HistoryType    string               `json:"historyType,omitempty"`
	HistoryDefault string               `json:"historyDefault,omitempty"`
	Invocations    []VizInvoke          `json:"invocations,omitempty"`
	Depth          int                  `json:"depth,omitempty"`
	States         map[string]*rawState `json:"states,omitempty"`
	// XState shorthand: "on": {"EVENT": "target"} or "on": {"EVENT": {"target": "..."}}
	On map[string]json.RawMessage `json:"on,omitempty"`
}

type rawMachine struct {
	ID      string               `json:"id"`
	Initial string               `json:"initial"`
	States  map[string]*rawState `json:"states"`
}

// ParseNativeJSON converts Native JSON to VizMachine.
// Supports both flat format (with parent/children references) and nested
// format (with inline states maps inside compound states).
func ParseNativeJSON(data []byte) (*VizMachine, error) {
	var raw rawMachine
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal Native JSON: %w", err)
	}

	vm := &VizMachine{
		ID:      raw.ID,
		Initial: raw.Initial,
		States:  make(map[string]*VizState),
	}

	// Flatten nested states into the flat VizMachine.States map
	for id, rs := range raw.States {
		flattenState(vm, id, rs, "")
	}

	calculateDepths(vm)

	return vm, nil
}

// flattenState recursively flattens a rawState and its nested children
// into the VizMachine flat states map.
func flattenState(vm *VizMachine, id string, rs *rawState, parentID string) {
	// Convert XState "on" shorthand to transitions
	transitions := rs.Transitions
	for event, raw := range rs.On {
		t := parseOnTransition(event, raw)
		if t != nil {
			transitions = append(transitions, *t)
		}
	}

	vs := &VizState{
		ID:             id,
		Type:           rs.Type,
		Initial:        rs.Initial,
		Parent:         rs.Parent,
		Children:       rs.Children,
		Transitions:    transitions,
		Always:         rs.Always,
		Tags:           rs.Tags,
		Entry:          rs.Entry,
		Exit:           rs.Exit,
		HistoryType:    rs.HistoryType,
		HistoryDefault: rs.HistoryDefault,
		Invocations:    rs.Invocations,
	}

	// If parent is set, override with the one we computed
	if parentID != "" {
		vs.Parent = parentID
	}

	// Process nested states (XState-style)
	if len(rs.States) > 0 {
		if vs.Type == "" {
			vs.Type = VizStateCompound
		}
		childIDs := make([]string, 0, len(rs.States))
		for childID, childRaw := range rs.States {
			childIDs = append(childIDs, childID)
			flattenState(vm, childID, childRaw, id)
		}
		// Merge with any existing Children
		if len(vs.Children) == 0 {
			vs.Children = childIDs
		}
	}

	vm.States[id] = vs
}

// parseOnTransition converts an XState "on" entry to a VizTransition.
// Supports:
//   - string: "on": {"EVENT": "target"}
//   - object: "on": {"EVENT": {"target": "...", "guard": "..."}}
func parseOnTransition(event string, raw json.RawMessage) *VizTransition {
	// Try string first: "EVENT": "target"
	var target string
	if err := json.Unmarshal(raw, &target); err == nil {
		return &VizTransition{Event: event, Target: target}
	}

	// Try object: "EVENT": {"target": "...", "guard": "...", "actions": [...]}
	var obj struct {
		Target  string   `json:"target"`
		Guard   string   `json:"guard,omitempty"`
		Actions []string `json:"actions,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Target != "" {
		return &VizTransition{
			Event:   event,
			Target:  obj.Target,
			Guard:   obj.Guard,
			Actions: obj.Actions,
		}
	}

	return nil
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
