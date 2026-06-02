package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// XStateExporter converts a MachineConfig to XState v5 JSON.
//
// The output is consumable by Stately Studio (https://stately.ai/studio)
// for visual editing and round-trip workflows. Re-import via the
// `statekit generate` codegen path closes the loop.
//
// Coverage of the XState v5 format:
//
//   - id, initial, states, type (atomic/compound/parallel/final/history)
//   - on: event → { target, guard, actions } map (single-target form)
//   - after: { "Nms": { target, ... } } map for delayed transitions
//   - entry, exit action arrays
//   - invoke array (services with onDone / onError)
//   - history: shallow | deep
//   - always: eventless transitions (v1.x)
//   - tags: state tags (v1.x)
//   - raise: emitted as xstate.raise action descriptors on a transition (v1.x)
//
// Out-of-scope (kept minimal): internal-vs-external transition flags,
// action argument schemas, sendTo/log built-ins.
type XStateExporter[C any] struct {
	machine *ir.MachineConfig[C]
}

// NewXStateExporter constructs an XStateExporter for the given machine.
func NewXStateExporter[C any](machine *ir.MachineConfig[C]) *XStateExporter[C] {
	return &XStateExporter[C]{machine: machine}
}

// Export returns the XState v5 representation as a generic map ready
// for JSON marshaling.
func (e *XStateExporter[C]) Export() map[string]any {
	root := map[string]any{
		"id": e.machine.ID,
	}
	if e.machine.Initial != "" {
		root["initial"] = string(e.machine.Initial)
	}

	rootStates := e.rootStateIDs()
	if len(rootStates) > 0 {
		states := make(map[string]any, len(rootStates))
		for _, id := range rootStates {
			states[string(id)] = e.exportState(id)
		}
		root["states"] = states
	}
	return root
}

// ExportJSON returns the XState v5 representation as compact JSON.
func (e *XStateExporter[C]) ExportJSON() (string, error) {
	data, err := json.Marshal(e.Export())
	if err != nil {
		return "", fmt.Errorf("marshal xstate: %w", err)
	}
	return string(data), nil
}

// ExportJSONIndent returns the XState v5 representation as indented JSON.
func (e *XStateExporter[C]) ExportJSONIndent(prefix, indent string) (string, error) {
	data, err := json.MarshalIndent(e.Export(), prefix, indent)
	if err != nil {
		return "", fmt.Errorf("marshal xstate: %w", err)
	}
	return string(data), nil
}

// rootStateIDs returns the IDs of states that have no parent, sorted
// for stable output.
func (e *XStateExporter[C]) rootStateIDs() []ir.StateID {
	var ids []ir.StateID
	for id, state := range e.machine.States {
		if state.Parent == "" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (e *XStateExporter[C]) exportState(id ir.StateID) map[string]any {
	state := e.machine.States[id]
	out := map[string]any{}

	switch state.Type {
	case ir.StateTypeFinal:
		out["type"] = "final"
	case ir.StateTypeParallel:
		out["type"] = "parallel"
	case ir.StateTypeHistory:
		out["type"] = "history"
		switch state.HistoryType {
		case ir.HistoryTypeShallow:
			out["history"] = "shallow"
		case ir.HistoryTypeDeep:
			out["history"] = "deep"
		}
		if state.HistoryDefault != "" {
			out["target"] = string(state.HistoryDefault)
		}
	case ir.StateTypeCompound:
		// XState infers compound from "states"; explicit "type" omitted.
	}

	if state.Initial != "" {
		out["initial"] = string(state.Initial)
	}

	// entry / exit actions.
	if len(state.Entry) > 0 {
		out["entry"] = stringifyActions(state.Entry)
	}
	if len(state.Exit) > 0 {
		out["exit"] = stringifyActions(state.Exit)
	}

	// Group transitions by event for `on`; delayed go into `after`.
	if onMap, afterMap := e.exportTransitions(state.Transitions); len(onMap) > 0 || len(afterMap) > 0 {
		if len(onMap) > 0 {
			out["on"] = onMap
		}
		if len(afterMap) > 0 {
			out["after"] = afterMap
		}
	}

	// Eventless ("always") transitions (v1.x).
	if len(state.Always) > 0 {
		group := make([]map[string]any, 0, len(state.Always))
		for _, t := range state.Always {
			group = append(group, transitionEntry(t))
		}
		out["always"] = collapseTransitionGroup(group)
	}

	// State tags (v1.x).
	if len(state.Tags) > 0 {
		out["tags"] = append([]string(nil), state.Tags...)
	}

	// Nested child states.
	if len(state.Children) > 0 {
		children := make(map[string]any, len(state.Children))
		// Sort for stable output.
		sortedChildren := append([]ir.StateID(nil), state.Children...)
		sort.Slice(sortedChildren, func(i, j int) bool { return sortedChildren[i] < sortedChildren[j] })
		for _, childID := range sortedChildren {
			children[string(childID)] = e.exportState(childID)
		}
		out["states"] = children
	}

	// Invoked services.
	if len(state.Invocations) > 0 {
		invokes := make([]map[string]any, 0, len(state.Invocations))
		for _, inv := range state.Invocations {
			entry := map[string]any{"src": string(inv.Src)}
			if inv.ID != "" {
				entry["id"] = inv.ID
			}
			if inv.OnDone != nil {
				entry["onDone"] = string(inv.OnDone.Target)
			}
			if inv.OnError != nil {
				entry["onError"] = string(inv.OnError.Target)
			}
			invokes = append(invokes, entry)
		}
		out["invoke"] = invokes
	}

	return out
}

// exportTransitions splits a state's transitions into the XState `on`
// map (event-driven) and `after` map (delayed). Multiple transitions
// for the same event are emitted as an array per XState's conditional
// transition format.
func (e *XStateExporter[C]) exportTransitions(transitions []*ir.TransitionConfig) (map[string]any, map[string]any) {
	onGroups := make(map[string][]map[string]any)
	afterGroups := make(map[string][]map[string]any)

	for _, t := range transitions {
		entry := transitionEntry(t)
		if t.IsDelayed() {
			delay := strconv.FormatInt(t.Delay.Milliseconds(), 10)
			afterGroups[delay] = append(afterGroups[delay], entry)
		} else {
			onGroups[string(t.Event)] = append(onGroups[string(t.Event)], entry)
		}
	}

	on := make(map[string]any, len(onGroups))
	for event, group := range onGroups {
		on[event] = collapseTransitionGroup(group)
	}
	after := make(map[string]any, len(afterGroups))
	for delay, group := range afterGroups {
		after[delay] = collapseTransitionGroup(group)
	}
	return on, after
}

// collapseTransitionGroup returns a single object if the group has one
// entry, otherwise the array — matches XState's polymorphic shape.
func collapseTransitionGroup(group []map[string]any) any {
	if len(group) == 1 {
		return group[0]
	}
	out := make([]any, 0, len(group))
	for _, entry := range group {
		out = append(out, entry)
	}
	return out
}

// transitionEntry builds the XState entry object for a transition,
// including guard, named actions, and raised events (emitted as
// xstate.raise action descriptors so the raised event survives export).
func transitionEntry(t *ir.TransitionConfig) map[string]any {
	entry := map[string]any{}
	// XState treats a transition without a target as internal; emit the
	// explicit flag too for clarity. Otherwise carry the target.
	if t.Internal {
		entry["internal"] = true
		if t.Target != "" {
			entry["target"] = string(t.Target)
		}
	} else {
		entry["target"] = string(t.Target)
	}
	if t.Guard != "" {
		entry["guard"] = string(t.Guard)
	}

	// Keep the common case (named actions only) as []string for stable
	// output; widen to []any only when raised events must be embedded as
	// xstate.raise action descriptors.
	if len(t.Raise) == 0 {
		if len(t.Actions) > 0 {
			entry["actions"] = stringifyActions(t.Actions)
		}
		return entry
	}

	actions := make([]any, 0, len(t.Actions)+len(t.Raise))
	for _, a := range t.Actions {
		actions = append(actions, string(a))
	}
	for _, r := range t.Raise {
		actions = append(actions, map[string]any{
			"type":  "xstate.raise",
			"event": map[string]any{"type": string(r)},
		})
	}
	entry["actions"] = actions
	return entry
}

func stringifyActions(actions []ir.ActionType) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}
