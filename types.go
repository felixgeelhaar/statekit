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
	// HistoryType specifies how history states remember previous states (v2.0)
	HistoryType = ir.HistoryType
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
	StateTypeHistory  = ir.StateTypeHistory  // v2.0
	StateTypeParallel = ir.StateTypeParallel // v2.0

	HistoryTypeShallow = ir.HistoryTypeShallow // v2.0
	HistoryTypeDeep    = ir.HistoryTypeDeep    // v2.0
)

// State represents the current runtime state of an interpreter
type State[C any] struct {
	Value   StateID // Current state ID (leaf state, or parallel state when in parallel)
	Context C       // Current context

	// Parallel state tracking (v2.0)
	// When inside a parallel state, maps region ID to its current leaf state
	// Empty when not in a parallel state
	ActiveInParallel map[StateID]StateID
}

// Matches checks if the current state matches the given state ID
// For parallel states, also checks if any region's current state matches
func (s State[C]) Matches(id StateID) bool {
	if s.Value == id {
		return true
	}
	// Check parallel regions
	for _, leafID := range s.ActiveInParallel {
		if leafID == id {
			return true
		}
	}
	return false
}
