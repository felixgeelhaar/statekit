package debug

import (
	"fmt"
	"sort"
	"strings"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// Inspector provides debugging and inspection capabilities for a state machine.
// It allows examining the machine configuration, available transitions, and
// simulating transitions without side effects.
type Inspector[C any] struct {
	interp  *statekit.Interpreter[C]
	machine *ir.MachineConfig[C]
}

// NewInspector creates a new Inspector for the given interpreter.
// The machine parameter is required for accessing machine configuration.
func NewInspector[C any](interp *statekit.Interpreter[C], machine *ir.MachineConfig[C]) *Inspector[C] {
	return &Inspector[C]{
		interp:  interp,
		machine: machine,
	}
}

// CurrentState returns the current state ID of the interpreter.
func (i *Inspector[C]) CurrentState() statekit.StateID {
	return i.interp.State().Value
}

// CurrentContext returns the current context.
func (i *Inspector[C]) CurrentContext() C {
	return i.interp.State().Context
}

// IsDone returns true if the machine is in a final state.
func (i *Inspector[C]) IsDone() bool {
	return i.interp.Done()
}

// MachineID returns the machine's ID.
func (i *Inspector[C]) MachineID() string {
	return i.machine.ID
}

// InitialState returns the machine's initial state.
func (i *Inspector[C]) InitialState() statekit.StateID {
	return i.machine.Initial
}

// AllStates returns all state IDs defined in the machine.
func (i *Inspector[C]) AllStates() []statekit.StateID {
	states := make([]statekit.StateID, 0, len(i.machine.States))
	for id := range i.machine.States {
		states = append(states, id)
	}
	sort.Slice(states, func(a, b int) bool {
		return states[a] < states[b]
	})
	return states
}

// StateInfo returns detailed information about a specific state.
func (i *Inspector[C]) StateInfo(stateID statekit.StateID) *StateInfo {
	config := i.machine.GetState(stateID)
	if config == nil {
		return nil
	}

	info := &StateInfo{
		ID:       config.ID,
		Type:     config.Type.String(),
		Parent:   config.Parent,
		Initial:  config.Initial,
		Children: config.Children,
		Entry:    actionsToStrings(config.Entry),
		Exit:     actionsToStrings(config.Exit),
	}

	// Collect transitions
	for _, t := range config.Transitions {
		info.Transitions = append(info.Transitions, TransitionInfo{
			Event:   t.Event,
			Target:  t.Target,
			Guard:   t.Guard,
			Actions: actionsToStrings(t.Actions),
			Delay:   t.Delay.String(),
		})
	}

	return info
}

// AvailableEvents returns all events that the current state can handle.
// This includes events handled by ancestor states (due to event bubbling).
func (i *Inspector[C]) AvailableEvents() []statekit.EventType {
	currentState := i.interp.State().Value
	eventSet := make(map[statekit.EventType]bool)

	// Collect events from current state and all ancestors
	stateID := currentState
	for stateID != "" {
		config := i.machine.GetState(stateID)
		if config == nil {
			break
		}

		for _, t := range config.Transitions {
			// Skip delayed transitions (they fire automatically)
			if t.Delay > 0 {
				continue
			}
			eventSet[t.Event] = true
		}

		stateID = config.Parent
	}

	events := make([]statekit.EventType, 0, len(eventSet))
	for e := range eventSet {
		events = append(events, e)
	}
	sort.Slice(events, func(a, b int) bool {
		return events[a] < events[b]
	})
	return events
}

// CanTransition checks if the given event would cause a transition.
// This evaluates guards but does not execute actions.
func (i *Inspector[C]) CanTransition(eventType statekit.EventType) bool {
	_, canTransition := i.SimulateTransition(statekit.Event{Type: eventType})
	return canTransition
}

// CanTransitionWithPayload checks if the given event with payload would cause a transition.
func (i *Inspector[C]) CanTransitionWithPayload(event statekit.Event) bool {
	_, canTransition := i.SimulateTransition(event)
	return canTransition
}

// SimulateTransition simulates sending an event without executing actions.
// Returns the target state and whether a transition would occur.
func (i *Inspector[C]) SimulateTransition(event statekit.Event) (statekit.StateID, bool) {
	currentState := i.interp.State().Value
	ctx := i.interp.State().Context

	// Find matching transition starting from current state
	stateID := currentState
	for stateID != "" {
		config := i.machine.GetState(stateID)
		if config == nil {
			break
		}

		for _, t := range config.Transitions {
			if t.Event != event.Type {
				continue
			}

			// Check guard if present
			if t.Guard != "" {
				guard := i.machine.GetGuard(t.Guard)
				if guard != nil && !guard(ctx, event) {
					continue // Guard failed, try next
				}
			}

			// Found matching transition
			target := t.Target
			if target == "" {
				// Self-transition with no explicit target
				return currentState, false
			}

			// Resolve to leaf state
			resolvedTarget := i.machine.GetInitialLeaf(target)
			return resolvedTarget, true
		}

		stateID = config.Parent
	}

	return currentState, false
}

// TransitionsFrom returns all possible transitions from the given state.
func (i *Inspector[C]) TransitionsFrom(stateID statekit.StateID) []TransitionInfo {
	config := i.machine.GetState(stateID)
	if config == nil {
		return nil
	}

	var transitions []TransitionInfo
	for _, t := range config.Transitions {
		transitions = append(transitions, TransitionInfo{
			Event:   t.Event,
			Target:  t.Target,
			Guard:   t.Guard,
			Actions: actionsToStrings(t.Actions),
			Delay:   t.Delay.String(),
		})
	}
	return transitions
}

// Ancestors returns all ancestor states of the current state.
func (i *Inspector[C]) Ancestors() []statekit.StateID {
	return i.machine.GetAncestors(i.interp.State().Value)
}

// Path returns the full path from root to current state.
func (i *Inspector[C]) Path() []statekit.StateID {
	return i.machine.GetPath(i.interp.State().Value)
}

// Dump returns a human-readable string representation of the current machine state.
func (i *Inspector[C]) Dump() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Machine: %s\n", i.machine.ID))
	sb.WriteString(fmt.Sprintf("Current State: %s\n", i.interp.State().Value))
	sb.WriteString(fmt.Sprintf("Is Done: %v\n", i.interp.Done()))
	sb.WriteString("\n")

	// Path
	path := i.Path()
	sb.WriteString(fmt.Sprintf("Path: %s\n", strings.Join(toStringSlice(path), " -> ")))
	sb.WriteString("\n")

	// Available events
	events := i.AvailableEvents()
	sb.WriteString("Available Events:\n")
	if len(events) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, e := range events {
		canTransition := i.CanTransition(e)
		target, _ := i.SimulateTransition(statekit.Event{Type: e})
		marker := ""
		if canTransition {
			marker = fmt.Sprintf(" -> %s", target)
		} else {
			marker = " (blocked by guard)"
		}
		sb.WriteString(fmt.Sprintf("  - %s%s\n", e, marker))
	}

	return sb.String()
}

// DumpMachine returns a complete dump of the machine configuration.
func (i *Inspector[C]) DumpMachine() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Machine: %s\n", i.machine.ID))
	sb.WriteString(fmt.Sprintf("Initial: %s\n", i.machine.Initial))
	sb.WriteString(fmt.Sprintf("States: %d\n", len(i.machine.States)))
	sb.WriteString("\n")

	// List all states
	states := i.AllStates()
	for _, stateID := range states {
		info := i.StateInfo(stateID)
		sb.WriteString(fmt.Sprintf("State: %s\n", info.ID))
		sb.WriteString(fmt.Sprintf("  Type: %s\n", info.Type))
		if info.Parent != "" {
			sb.WriteString(fmt.Sprintf("  Parent: %s\n", info.Parent))
		}
		if info.Initial != "" {
			sb.WriteString(fmt.Sprintf("  Initial: %s\n", info.Initial))
		}
		if len(info.Children) > 0 {
			sb.WriteString(fmt.Sprintf("  Children: %s\n", strings.Join(toStringSlice(info.Children), ", ")))
		}
		if len(info.Entry) > 0 {
			sb.WriteString(fmt.Sprintf("  Entry: %s\n", strings.Join(info.Entry, ", ")))
		}
		if len(info.Exit) > 0 {
			sb.WriteString(fmt.Sprintf("  Exit: %s\n", strings.Join(info.Exit, ", ")))
		}
		if len(info.Transitions) > 0 {
			sb.WriteString("  Transitions:\n")
			for _, t := range info.Transitions {
				sb.WriteString(fmt.Sprintf("    %s -> %s", t.Event, t.Target))
				if t.Guard != "" {
					sb.WriteString(fmt.Sprintf(" [%s]", t.Guard))
				}
				if len(t.Actions) > 0 {
					sb.WriteString(fmt.Sprintf(" / %s", strings.Join(t.Actions, ", ")))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// StateInfo contains detailed information about a state.
type StateInfo struct {
	ID          statekit.StateID
	Type        string
	Parent      statekit.StateID
	Initial     statekit.StateID
	Children    []statekit.StateID
	Entry       []string
	Exit        []string
	Transitions []TransitionInfo
}

// TransitionInfo contains information about a transition.
type TransitionInfo struct {
	Event   statekit.EventType
	Target  statekit.StateID
	Guard   statekit.GuardType
	Actions []string
	Delay   string
}

// Helper functions

func actionsToStrings(actions []ir.ActionType) []string {
	result := make([]string, len(actions))
	for i, a := range actions {
		result[i] = string(a)
	}
	return result
}

func toStringSlice[T ~string](slice []T) []string {
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = string(s)
	}
	return result
}
