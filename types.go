package statekit

import "github.com/felixgeelhaar/statekit/internal/ir"

// Re-export types from internal/ir for public API
type (
	// StateType represents the kind of state node
	StateType = ir.StateType
	// EventType is a named event identifier
	EventType = ir.EventType
	// StateID uniquely identifies a state within a machine
	StateID = ir.StateID
	// ActionType identifies a named action
	ActionType = ir.ActionType
	// GuardType identifies a named guard
	GuardType = ir.GuardType
	// Event represents a runtime event with optional payload
	Event = ir.Event
	// Action is a side-effect function executed during transitions
	Action[C any] = ir.Action[C]
	// Guard is a predicate that determines if a transition should occur
	Guard[C any] = ir.Guard[C]
)

// Re-export constants
const (
	StateTypeAtomic   = ir.StateTypeAtomic
	StateTypeCompound = ir.StateTypeCompound
	StateTypeFinal    = ir.StateTypeFinal
)

// State represents the current runtime state of an interpreter
type State[C any] struct {
	Value   StateID // Current state ID (leaf state)
	Context C       // Current context
}

// Matches checks if the current state matches the given state ID
func (s State[C]) Matches(id StateID) bool {
	return s.Value == id
}
