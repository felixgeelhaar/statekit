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
