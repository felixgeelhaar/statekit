// Package export provides exporters for converting state machine configurations
// to external formats like XState JSON.
package export

import (
	"encoding/json"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// XStateExporter converts a MachineConfig to XState-compatible JSON format.
// The exported JSON can be used with:
// - XState Visualizer (stately.ai/viz)
// - XState Inspector
// - XState v5 compatible tools
type XStateExporter[C any] struct {
	machine *ir.MachineConfig[C]
}

// NewXStateExporter creates a new exporter for the given machine configuration
func NewXStateExporter[C any](machine *ir.MachineConfig[C]) *XStateExporter[C] {
	return &XStateExporter[C]{machine: machine}
}

// XStateMachine represents an XState machine configuration
type XStateMachine struct {
	ID      string                `json:"id"`
	Initial string                `json:"initial,omitempty"`
	States  map[string]XStateNode `json:"states"`
}

// XStateNode represents a single state in XState format
type XStateNode struct {
	Type    string                      `json:"type,omitempty"`    // "final", "compound", "atomic"
	Initial string                      `json:"initial,omitempty"` // For compound states
	States  map[string]XStateNode       `json:"states,omitempty"`  // For compound states
	Entry   []string                    `json:"entry,omitempty"`
	Exit    []string                    `json:"exit,omitempty"`
	On      map[string]XStateTransition `json:"on,omitempty"`
}

// XStateTransition represents a transition in XState format
type XStateTransition struct {
	Target  string   `json:"target,omitempty"`
	Actions []string `json:"actions,omitempty"`
	Guard   string   `json:"guard,omitempty"` // XState v5 uses "guard", v4 uses "cond"
}

// Export converts the machine configuration to XState JSON format
func (e *XStateExporter[C]) Export() (*XStateMachine, error) {
	machine := &XStateMachine{
		ID:      e.machine.ID,
		Initial: string(e.machine.Initial),
		States:  make(map[string]XStateNode),
	}

	// Find root-level states (states without parents)
	rootStates := e.findRootStates()

	// Build state tree for each root state
	for _, stateID := range rootStates {
		machine.States[string(stateID)] = e.buildStateNode(stateID)
	}

	return machine, nil
}

// ExportJSON returns the machine configuration as a JSON string
func (e *XStateExporter[C]) ExportJSON() (string, error) {
	machine, err := e.Export()
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(machine)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ExportJSONIndent returns the machine configuration as a formatted JSON string
func (e *XStateExporter[C]) ExportJSONIndent(prefix, indent string) (string, error) {
	machine, err := e.Export()
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(machine, prefix, indent)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// findRootStates returns all states that don't have a parent
func (e *XStateExporter[C]) findRootStates() []ir.StateID {
	var roots []ir.StateID
	for id, state := range e.machine.States {
		if state.Parent == "" {
			roots = append(roots, id)
		}
	}
	return roots
}

// buildStateNode recursively builds an XState node for the given state
func (e *XStateExporter[C]) buildStateNode(stateID ir.StateID) XStateNode {
	state := e.machine.States[stateID]
	if state == nil {
		return XStateNode{}
	}

	node := XStateNode{}

	// Set state type
	switch state.Type {
	case ir.StateTypeFinal:
		node.Type = "final"
	case ir.StateTypeCompound:
		// XState infers compound from having nested states
		// but we can explicitly set it for clarity
		if len(state.Children) > 0 {
			node.Initial = string(state.Initial)
			node.States = make(map[string]XStateNode)
			for _, childID := range state.Children {
				node.States[string(childID)] = e.buildStateNode(childID)
			}
		}
	}
	// StateTypeAtomic is the default, no need to set type

	// Entry actions
	if len(state.Entry) > 0 {
		node.Entry = make([]string, len(state.Entry))
		for i, action := range state.Entry {
			node.Entry[i] = string(action)
		}
	}

	// Exit actions
	if len(state.Exit) > 0 {
		node.Exit = make([]string, len(state.Exit))
		for i, action := range state.Exit {
			node.Exit[i] = string(action)
		}
	}

	// Transitions
	if len(state.Transitions) > 0 {
		node.On = make(map[string]XStateTransition)
		for _, trans := range state.Transitions {
			transition := XStateTransition{
				Target: string(trans.Target),
			}

			if len(trans.Actions) > 0 {
				transition.Actions = make([]string, len(trans.Actions))
				for i, action := range trans.Actions {
					transition.Actions[i] = string(action)
				}
			}

			if trans.Guard != "" {
				transition.Guard = string(trans.Guard)
			}

			node.On[string(trans.Event)] = transition
		}
	}

	return node
}
