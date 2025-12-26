package ir

// StateType represents the kind of state node
type StateType int

const (
	// StateTypeAtomic is a leaf state with no children
	StateTypeAtomic StateType = iota
	// StateTypeCompound has child states (v0.2)
	StateTypeCompound
	// StateTypeFinal is a terminal state
	StateTypeFinal
	// StateTypeHistory remembers the last active child (v2.0)
	StateTypeHistory
	// StateTypeParallel has multiple active regions (v2.0)
	StateTypeParallel
)

// HistoryType specifies how history states remember previous states
type HistoryType int

const (
	// HistoryTypeShallow remembers only the immediate child state
	HistoryTypeShallow HistoryType = iota
	// HistoryTypeDeep remembers the full leaf state path
	HistoryTypeDeep
)

// String returns the string representation of StateType
func (s StateType) String() string {
	switch s {
	case StateTypeAtomic:
		return "atomic"
	case StateTypeCompound:
		return "compound"
	case StateTypeFinal:
		return "final"
	case StateTypeHistory:
		return "history"
	case StateTypeParallel:
		return "parallel"
	default:
		return "unknown"
	}
}

// String returns the string representation of HistoryType
func (h HistoryType) String() string {
	switch h {
	case HistoryTypeShallow:
		return "shallow"
	case HistoryTypeDeep:
		return "deep"
	default:
		return "unknown"
	}
}

// EventType is a named event identifier
type EventType string

// StateID uniquely identifies a state within a machine
type StateID string

// ActionType identifies a named action
type ActionType string

// GuardType identifies a named guard
type GuardType string

// Event represents a runtime event with optional payload
type Event struct {
	Type    EventType
	Payload any
}

// Action is a side-effect function executed during transitions
type Action[C any] func(ctx *C, event Event)

// Guard is a predicate that determines if a transition should occur
type Guard[C any] func(ctx C, event Event) bool
