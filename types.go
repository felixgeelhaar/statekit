package statekit

import "go.klarlabs.de/statekit/internal/ir"

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
	// ServiceType identifies a named service (v3.0)
	ServiceType = ir.ServiceType
	// Event represents a runtime event with optional payload
	Event = ir.Event
	// HistoryType specifies how history states remember previous states (v2.0)
	HistoryType = ir.HistoryType
	// ServiceContext provides the execution context for a service (v3.0)
	ServiceContext[C any] = ir.ServiceContext[C]
	// MachineConfig is the immutable internal representation of a statechart
	MachineConfig[C any] = ir.MachineConfig[C]
)

// ActorID uniquely identifies a spawned child actor
type ActorID string

// SupervisionStrategy defines how parent handles child actor errors
type SupervisionStrategy int

const (
	// SupervisionEscalate bubbles the error to the parent via xstate.error.actor.<id> event
	SupervisionEscalate SupervisionStrategy = iota
	// SupervisionRecover logs the error and continues without stopping the child
	SupervisionRecover
	// SupervisionRestart stops the child and restarts it with initial state
	SupervisionRestart
	// SupervisionStop stops the child silently without generating an error event
	SupervisionStop
)

// Action is a side-effect function executed during transitions.
// It receives a pointer to the context for modification and the triggering event.
type Action[C any] func(ctx *C, event Event)

// Guard is a predicate that determines if a transition should occur.
// It receives the current context (by value) and the triggering event.
type Guard[C any] func(ctx C, event Event) bool

// Service is an async operation invoked when entering a state.
// It runs in a goroutine and can send events back to the machine.
// The service should respect the context for cancellation.
type Service[C any] = ir.Service[C]

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
