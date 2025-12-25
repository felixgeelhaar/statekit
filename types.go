package statekit

import "github.com/felixgeelhaar/statekit/internal/ir"

// Re-export non-generic types from internal/ir for public API
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
)

// Action is a side-effect function executed during transitions.
// It receives a pointer to the context for modification and the triggering event.
type Action[C any] func(ctx *C, event Event)

// Guard is a predicate that determines if a transition should occur.
// It receives the current context (by value) and the triggering event.
type Guard[C any] func(ctx C, event Event) bool

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
