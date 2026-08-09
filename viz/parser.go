package viz

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// rawState mirrors VizState but adds fields for nested and XState shorthand input.
type rawState struct {
	ID          string          `json:"id"`
	Type        VizStateType    `json:"type"`
	Initial     string          `json:"initial,omitempty"`
	Parent      string          `json:"parent,omitempty"`
	Children    []string        `json:"children,omitempty"`
	Transitions []VizTransition `json:"transitions,omitempty"`
	// Always is deliberately raw, for the same reason On is: the exporter's
	// collapseTransitionGroup writes an object for a group of one and an
	// array otherwise, so neither []VizTransition nor a single object can
	// read both. A concrete type here does not merely skip the shape it
	// cannot read — it fails the whole ParseNativeJSON call.
	Always         json.RawMessage      `json:"always,omitempty"`
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
	// Delayed transitions, keyed by delay: {"after": {"5000": {...}}}. Raw for
	// the same reason On and Always are — the value collapses to an object for
	// a group of one and an array otherwise.
	After map[string]json.RawMessage `json:"after,omitempty"`
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
		transitions = append(transitions, parseTransitionGroup(event, raw)...)
	}
	// Delayed transitions. The exporter writes these under "after" keyed by
	// milliseconds; nothing read them, so a state whose only edge was a
	// timeout parsed as terminal.
	for delay, raw := range rs.After {
		for _, t := range parseTransitionGroup("", raw) {
			t.IsDelayed = true
			if ms, err := strconv.ParseInt(delay, 10, 64); err == nil {
				t.DelayMs = ms
			} else {
				// A non-numeric key is an XState delay reference, not a
				// duration. Keep the edge and carry the name — dropping it
				// would misreport the state as terminal, which is the bug
				// this fixes.
				t.Event = delay
			}
			transitions = append(transitions, t)
		}
	}

	vs := &VizState{
		ID:             id,
		Type:           rs.Type,
		Initial:        rs.Initial,
		Parent:         rs.Parent,
		Children:       rs.Children,
		Transitions:    transitions,
		Always:         parseTransitionGroup("", rs.Always),
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

// parseTransitionGroup reads one transition group — the value of an "on" event
// or of "always" — in every shape the XState exporter emits for it:
//
//	"GO": "b"                                  bare target
//	"GO": {"target": "b", "guard": "ok"}       single transition
//	"GO": [{"target": "b"}, {"target": "c"}]   guarded alternatives
//	"always": {"target": "b"}                  a group of one, collapsed
//	"always": [{"target": "b"}, ...]           two or more
//
// The array form is not exotic: export.collapseTransitionGroup writes an
// object for exactly one entry and an array for anything else, so a second
// guarded transition on the same event changes the shape. Reading only one of
// them meant those transitions parsed to nothing at all, with no error — a
// diagram missing an edge is indistinguishable from a machine without one.
//
// event is "" for eventless ("always") transitions.
func parseTransitionGroup(event string, raw json.RawMessage) []VizTransition {
	if len(raw) == 0 {
		return nil
	}

	// An array of entries, each of which is itself a target string or object.
	var group []json.RawMessage
	if err := json.Unmarshal(raw, &group); err == nil {
		out := make([]VizTransition, 0, len(group))
		for _, entry := range group {
			// One malformed entry must not discard its siblings.
			if t := parseTransitionEntry(event, entry); t != nil {
				out = append(out, *t)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}

	if t := parseTransitionEntry(event, raw); t != nil {
		return []VizTransition{*t}
	}
	return nil
}

// parseTransitionEntry reads a single transition: a bare target string, or the
// object export.transitionEntry builds.
func parseTransitionEntry(event string, raw json.RawMessage) *VizTransition {
	var target string
	if err := json.Unmarshal(raw, &target); err == nil {
		return &VizTransition{Event: event, Target: target}
	}

	// actions is []any, not []string: transitionEntry widens it to embed
	// raised events as xstate.raise descriptors alongside named actions.
	// Typing it []string here did not drop the raise — it failed the
	// unmarshal, and took the whole transition with it.
	var obj struct {
		Target   string `json:"target"`
		Guard    string `json:"guard,omitempty"`
		Actions  []json.RawMessage
		Internal bool `json:"internal,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}

	actions, raise := splitActions(obj.Actions)

	// A transition with no target is internal — it runs actions and stays put.
	// Dropping it loses the only thing it exists to do. Require some content,
	// so that an empty object still parses to nothing.
	if obj.Target == "" && !obj.Internal && len(actions) == 0 && len(raise) == 0 {
		return nil
	}

	return &VizTransition{
		Event:   event,
		Target:  obj.Target,
		Guard:   obj.Guard,
		Actions: actions,
		Raise:   raise,
	}
}

// splitActions separates named actions from xstate.raise descriptors, the two
// things export.transitionEntry packs into one "actions" array.
func splitActions(raw []json.RawMessage) (actions, raise []string) {
	for _, a := range raw {
		var name string
		if err := json.Unmarshal(a, &name); err == nil {
			actions = append(actions, name)
			continue
		}
		var desc struct {
			Type  string `json:"type"`
			Event struct {
				Type string `json:"type"`
			} `json:"event"`
		}
		if err := json.Unmarshal(a, &desc); err != nil {
			continue
		}
		if desc.Type == "xstate.raise" && desc.Event.Type != "" {
			raise = append(raise, desc.Event.Type)
		}
	}
	return actions, raise
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
