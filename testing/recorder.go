package testing

import (
	"sync"
	"time"

	"github.com/felixgeelhaar/statekit"
)

// Transition represents a recorded state transition.
type Transition[C any] struct {
	// Event that triggered the transition
	Event statekit.Event
	// FromState is the state before the event was processed
	FromState statekit.StateID
	// ToState is the state after the event was processed
	ToState statekit.StateID
	// Transitioned indicates whether the state actually changed
	Transitioned bool
	// ContextBefore is a copy of the context before the transition
	ContextBefore C
	// ContextAfter is a copy of the context after the transition
	ContextAfter C
	// Timestamp when the transition occurred
	Timestamp time.Time
	// Duration of the transition processing
	Duration time.Duration
}

// Recorder wraps an interpreter and records all transitions for testing.
// It provides methods to query the recorded history for assertions.
type Recorder[C any] struct {
	interp      *statekit.Interpreter[C]
	transitions []Transition[C]
	mu          sync.RWMutex
}

// NewRecorder creates a new Recorder wrapping the given interpreter.
// The recorder will capture all transitions when events are sent through it.
func NewRecorder[C any](interp *statekit.Interpreter[C]) *Recorder[C] {
	return &Recorder[C]{
		interp:      interp,
		transitions: make([]Transition[C], 0),
	}
}

// Interpreter returns the underlying interpreter.
func (r *Recorder[C]) Interpreter() *statekit.Interpreter[C] {
	return r.interp
}

// Start starts the interpreter and records the initial state entry.
func (r *Recorder[C]) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()
	var zeroCtx C
	r.interp.Start()
	duration := time.Since(start)

	// Record the initial state entry as a transition from empty state
	r.transitions = append(r.transitions, Transition[C]{
		Event:         statekit.Event{Type: "__START__"},
		FromState:     "",
		ToState:       r.interp.State().Value,
		Transitioned:  true,
		ContextBefore: zeroCtx,
		ContextAfter:  r.interp.State().Context,
		Timestamp:     start,
		Duration:      duration,
	})
}

// Send processes an event and records the transition.
func (r *Recorder[C]) Send(event statekit.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stateBefore := r.interp.State()
	start := time.Now()
	r.interp.Send(event)
	duration := time.Since(start)
	stateAfter := r.interp.State()

	r.transitions = append(r.transitions, Transition[C]{
		Event:         event,
		FromState:     stateBefore.Value,
		ToState:       stateAfter.Value,
		Transitioned:  stateBefore.Value != stateAfter.Value,
		ContextBefore: stateBefore.Context,
		ContextAfter:  stateAfter.Context,
		Timestamp:     start,
		Duration:      duration,
	})
}

// SendAll processes multiple events in sequence, recording each transition.
func (r *Recorder[C]) SendAll(events ...statekit.Event) {
	for _, event := range events {
		r.Send(event)
	}
}

// Transitions returns a copy of all recorded transitions.
func (r *Recorder[C]) Transitions() []Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Transition[C], len(r.transitions))
	copy(result, r.transitions)
	return result
}

// TransitionCount returns the number of recorded transitions.
func (r *Recorder[C]) TransitionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.transitions)
}

// LastTransition returns the most recent transition, or nil if none recorded.
func (r *Recorder[C]) LastTransition() *Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.transitions) == 0 {
		return nil
	}
	t := r.transitions[len(r.transitions)-1]
	return &t
}

// States returns all states visited in order (including duplicates).
func (r *Recorder[C]) States() []statekit.StateID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make([]statekit.StateID, 0, len(r.transitions))
	for i, t := range r.transitions {
		// Add FromState only for the first transition if it's not empty
		if i == 0 && t.FromState != "" {
			states = append(states, t.FromState)
		}
		states = append(states, t.ToState)
	}
	return states
}

// UniqueStates returns all unique states visited (no duplicates).
func (r *Recorder[C]) UniqueStates() []statekit.StateID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[statekit.StateID]bool)
	states := make([]statekit.StateID, 0)

	for i, t := range r.transitions {
		if i == 0 && t.FromState != "" && !seen[t.FromState] {
			seen[t.FromState] = true
			states = append(states, t.FromState)
		}
		if !seen[t.ToState] {
			seen[t.ToState] = true
			states = append(states, t.ToState)
		}
	}
	return states
}

// Events returns all events that were sent (in order).
func (r *Recorder[C]) Events() []statekit.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]statekit.Event, 0, len(r.transitions))
	for _, t := range r.transitions {
		// Skip the synthetic __START__ event
		if t.Event.Type != "__START__" {
			events = append(events, t.Event)
		}
	}
	return events
}

// EventTypes returns all event types that were sent (in order).
func (r *Recorder[C]) EventTypes() []statekit.EventType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]statekit.EventType, 0, len(r.transitions))
	for _, t := range r.transitions {
		// Skip the synthetic __START__ event
		if t.Event.Type != "__START__" {
			types = append(types, t.Event.Type)
		}
	}
	return types
}

// Reset clears all recorded transitions.
func (r *Recorder[C]) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transitions = make([]Transition[C], 0)
}

// State returns the current state from the underlying interpreter.
func (r *Recorder[C]) State() statekit.State[C] {
	return r.interp.State()
}

// Matches delegates to the underlying interpreter's Matches method.
func (r *Recorder[C]) Matches(id statekit.StateID) bool {
	return r.interp.Matches(id)
}

// Done returns true if the interpreter is in a final state.
func (r *Recorder[C]) Done() bool {
	return r.interp.Done()
}

// FindTransition finds the first transition matching the given criteria.
// Returns nil if no matching transition is found.
func (r *Recorder[C]) FindTransition(eventType statekit.EventType) *Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.transitions {
		if t.Event.Type == eventType {
			return &t
		}
	}
	return nil
}

// FindTransitions finds all transitions matching the given event type.
func (r *Recorder[C]) FindTransitions(eventType statekit.EventType) []Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Transition[C], 0)
	for _, t := range r.transitions {
		if t.Event.Type == eventType {
			result = append(result, t)
		}
	}
	return result
}

// TransitionsFrom returns all transitions that started from the given state.
func (r *Recorder[C]) TransitionsFrom(state statekit.StateID) []Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Transition[C], 0)
	for _, t := range r.transitions {
		if t.FromState == state {
			result = append(result, t)
		}
	}
	return result
}

// TransitionsTo returns all transitions that ended in the given state.
func (r *Recorder[C]) TransitionsTo(state statekit.StateID) []Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Transition[C], 0)
	for _, t := range r.transitions {
		if t.ToState == state {
			result = append(result, t)
		}
	}
	return result
}

// ActualTransitions returns only transitions where state actually changed.
func (r *Recorder[C]) ActualTransitions() []Transition[C] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Transition[C], 0)
	for _, t := range r.transitions {
		if t.Transitioned {
			result = append(result, t)
		}
	}
	return result
}

// TotalDuration returns the sum of all transition durations.
func (r *Recorder[C]) TotalDuration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total time.Duration
	for _, t := range r.transitions {
		total += t.Duration
	}
	return total
}
