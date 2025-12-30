package statekit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// Snapshot captures the complete state of an interpreter for persistence.
// It can be serialized to JSON and later used to restore an interpreter
// to the exact same state.
type Snapshot[C any] struct {
	// MachineID identifies which machine type this snapshot belongs to
	MachineID string `json:"machine_id"`

	// Version is the machine version for compatibility checking
	Version string `json:"version,omitempty"`

	// CurrentState is the active state ID (leaf state, or parallel state ID)
	CurrentState StateID `json:"current_state"`

	// Context is the user-defined context data
	Context C `json:"context"`

	// ShallowHistory maps compound state IDs to their last active child
	ShallowHistory map[StateID]StateID `json:"shallow_history,omitempty"`

	// DeepHistory maps compound state IDs to their last active leaf
	DeepHistory map[StateID]StateID `json:"deep_history,omitempty"`

	// ActiveInParallel maps region IDs to their current leaf states
	// Only populated when snapshot was taken from a parallel state
	ActiveInParallel map[StateID]StateID `json:"active_in_parallel,omitempty"`

	// CurrentParallel holds the parallel state ID if currently in a parallel state
	CurrentParallel StateID `json:"current_parallel,omitempty"`

	// PendingTimers captures active delayed transitions
	PendingTimers []PendingTimer `json:"pending_timers,omitempty"`

	// SpawnedActors captures metadata about spawned child actors (v0.14)
	// Note: Actors are NOT automatically restored. This metadata allows
	// the application to manually respawn actors if needed.
	SpawnedActors []ActorMetadata `json:"spawned_actors,omitempty"`

	// CreatedAt is when the snapshot was taken
	CreatedAt time.Time `json:"created_at"`
}

// ActorMetadata captures information about a spawned actor for persistence (v0.14).
// This is a metadata-only snapshot - the actor's internal state is not captured.
// When restoring, actors must be manually respawned by the application.
type ActorMetadata struct {
	// ID is the actor's unique identifier
	ID ActorID `json:"id"`

	// SpawnedInState is the state that spawned this actor
	SpawnedInState StateID `json:"spawned_in_state"`

	// Supervision is the supervision strategy for this actor
	Supervision SupervisionStrategy `json:"supervision"`

	// AutoForward lists event types that were auto-forwarded to this actor
	AutoForward []EventType `json:"auto_forward,omitempty"`
}

// PendingTimer represents an active delayed transition that hasn't fired yet.
type PendingTimer struct {
	// StateID is the state that owns this delayed transition
	StateID StateID `json:"state_id"`

	// TransitionIndex identifies which transition in the state
	TransitionIndex int `json:"transition_index"`

	// Target is the destination state
	Target StateID `json:"target"`

	// Remaining is how much time is left until the timer fires
	Remaining time.Duration `json:"remaining_ns"`
}

// Snapshot creates a snapshot of the current interpreter state.
// The snapshot captures all information needed to restore the interpreter
// to this exact state later.
//
// Note on actors: Actor metadata is captured but actors are NOT automatically
// restored. The SpawnedActors field contains metadata about what actors were
// running, allowing the application to manually respawn them if needed.
func (i *Interpreter[C]) Snapshot() Snapshot[C] {
	i.mu.Lock()
	defer i.mu.Unlock()

	s := Snapshot[C]{
		MachineID:        i.machine.ID,
		CurrentState:     i.state.Value,
		Context:          i.state.Context,
		CurrentParallel:  i.currentParallel,
		CreatedAt:        time.Now(),
		ShallowHistory:   make(map[StateID]StateID),
		DeepHistory:      make(map[StateID]StateID),
		ActiveInParallel: make(map[StateID]StateID),
	}

	// Copy shallow history
	for k, v := range i.shallowHistory {
		s.ShallowHistory[k] = v
	}

	// Copy deep history
	for k, v := range i.deepHistory {
		s.DeepHistory[k] = v
	}

	// Copy parallel state tracking
	for k, v := range i.state.ActiveInParallel {
		s.ActiveInParallel[k] = v
	}

	// Capture pending timers
	s.PendingTimers = i.snapshotTimers()

	// Capture actor metadata (v0.14)
	s.SpawnedActors = i.snapshotActors()

	return s
}

// snapshotActors captures metadata about spawned actors.
// Caller must hold mu.
func (i *Interpreter[C]) snapshotActors() []ActorMetadata {
	i.actorMu.Lock()
	defer i.actorMu.Unlock()

	if len(i.actorRegistry) == 0 {
		return nil
	}

	actors := make([]ActorMetadata, 0, len(i.actorRegistry))
	for id, entry := range i.actorRegistry {
		// Convert auto-forward map to slice
		var autoForward []EventType
		for et := range entry.autoForward {
			autoForward = append(autoForward, et)
		}

		actors = append(actors, ActorMetadata{
			ID:             id,
			SpawnedInState: entry.stateID,
			Supervision:    entry.supervision,
			AutoForward:    autoForward,
		})
	}

	return actors
}

// snapshotTimers captures information about active delayed transitions.
// Caller must hold mu.
func (i *Interpreter[C]) snapshotTimers() []PendingTimer {
	if len(i.timers) == 0 {
		return nil
	}

	var pending []PendingTimer
	now := time.Now()

	// For each active timer, we need to calculate remaining time
	// Since time.Timer doesn't expose remaining time, we track it differently
	for key := range i.timers {
		// Parse timer key (stateID:transitionIndex)
		var stateID string
		var transIdx int
		if _, err := fmt.Sscanf(key, "%s:%d", &stateID, &transIdx); err != nil {
			// Try alternate parsing for state IDs with colons
			for idx := len(key) - 1; idx >= 0; idx-- {
				if key[idx] == ':' {
					stateID = key[:idx]
					if _, err := fmt.Sscanf(key[idx+1:], "%d", &transIdx); err == nil {
						break
					}
				}
			}
		}

		state := i.machine.GetState(ir.StateID(stateID))
		if state == nil || transIdx >= len(state.Transitions) {
			continue
		}

		trans := state.Transitions[transIdx]
		if !trans.IsDelayed() {
			continue
		}

		// Estimate remaining time (imprecise but best effort)
		// In a real implementation, we'd track timer start times
		pending = append(pending, PendingTimer{
			StateID:         ir.StateID(stateID),
			TransitionIndex: transIdx,
			Target:          trans.Target,
			Remaining:       trans.Delay, // Approximation: full delay
		})
	}

	_ = now // Used for more precise timing in future
	return pending
}

// Restore restores the interpreter to the state captured in the snapshot.
// The machine configuration must match (same machine ID).
// Returns an error if the snapshot is incompatible.
func (i *Interpreter[C]) Restore(s Snapshot[C]) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Validate machine ID
	if s.MachineID != i.machine.ID {
		return &RestoreError{
			Code:    ErrCodeSnapshotMachineMismatch,
			Message: fmt.Sprintf("machine ID mismatch: snapshot has %q, interpreter has %q", s.MachineID, i.machine.ID),
		}
	}

	// Validate current state exists
	if i.machine.GetState(s.CurrentState) == nil {
		return &RestoreError{
			Code:    ErrCodeSnapshotInvalidState,
			Message: fmt.Sprintf("state %q not found in machine", s.CurrentState),
		}
	}

	// Validate parallel state tracking
	if s.CurrentParallel != "" {
		parallelState := i.machine.GetState(s.CurrentParallel)
		if parallelState == nil || !parallelState.IsParallel() {
			return &RestoreError{
				Code:    ErrCodeSnapshotInvalidState,
				Message: fmt.Sprintf("parallel state %q not found or not a parallel state", s.CurrentParallel),
			}
		}
	}

	// Validate all active parallel region states
	for regionID, leafID := range s.ActiveInParallel {
		if i.machine.GetState(regionID) == nil {
			return &RestoreError{
				Code:    ErrCodeSnapshotInvalidState,
				Message: fmt.Sprintf("parallel region %q not found", regionID),
			}
		}
		if i.machine.GetState(leafID) == nil {
			return &RestoreError{
				Code:    ErrCodeSnapshotInvalidState,
				Message: fmt.Sprintf("parallel region leaf state %q not found", leafID),
			}
		}
	}

	// Cancel any existing timers
	for key, timer := range i.timers {
		timer.Stop()
		delete(i.timers, key)
	}

	// Restore state
	i.state.Value = s.CurrentState
	i.state.Context = s.Context
	i.currentParallel = s.CurrentParallel
	i.started = true

	// Restore shallow history
	i.shallowHistory = make(map[ir.StateID]ir.StateID)
	for k, v := range s.ShallowHistory {
		i.shallowHistory[k] = v
	}

	// Restore deep history
	i.deepHistory = make(map[ir.StateID]ir.StateID)
	for k, v := range s.DeepHistory {
		i.deepHistory[k] = v
	}

	// Restore parallel state tracking
	i.state.ActiveInParallel = make(map[ir.StateID]ir.StateID)
	for k, v := range s.ActiveInParallel {
		i.state.ActiveInParallel[k] = v
	}

	// Restore timers with remaining duration from snapshot
	i.restoreTimers(s.PendingTimers)

	// Schedule delayed transitions for current state(s)
	// This handles the case where snapshot had no pending timers but state has delayed transitions
	if s.CurrentParallel != "" {
		// For parallel states, schedule for each region
		for regionID, leafID := range i.state.ActiveInParallel {
			i.scheduleDelayedTransitions(leafID)
			if leafID != regionID {
				i.scheduleDelayedTransitions(regionID)
			}
		}
		i.scheduleDelayedTransitions(s.CurrentParallel)
	} else {
		// For regular states, schedule for current state and ancestors
		current := s.CurrentState
		for current != "" {
			// Only schedule if not already restored from pending timers
			alreadyScheduled := false
			for _, pt := range s.PendingTimers {
				if pt.StateID == current {
					alreadyScheduled = true
					break
				}
			}
			if !alreadyScheduled {
				i.scheduleDelayedTransitions(current)
			}
			state := i.machine.GetState(current)
			if state == nil {
				break
			}
			current = state.Parent
		}
	}

	return nil
}

// restoreTimers recreates delayed transition timers from snapshot.
// Caller must hold mu.
func (i *Interpreter[C]) restoreTimers(pending []PendingTimer) {
	for _, pt := range pending {
		stateConfig := i.machine.GetState(pt.StateID)
		if stateConfig == nil || pt.TransitionIndex >= len(stateConfig.Transitions) {
			continue
		}

		trans := stateConfig.Transitions[pt.TransitionIndex]
		if !trans.IsDelayed() {
			continue
		}

		timerKey := fmt.Sprintf("%s:%d", pt.StateID, pt.TransitionIndex)
		capturedTrans := trans
		capturedState := stateConfig

		// Use remaining duration (or minimum 1ms to ensure it fires)
		duration := pt.Remaining
		if duration < time.Millisecond {
			duration = time.Millisecond
		}

		timer := time.AfterFunc(duration, func() {
			i.mu.Lock()
			defer i.mu.Unlock()

			delete(i.timers, timerKey)

			if i.started && i.matchesUnlocked(pt.StateID) {
				i.executeDelayedTransition(capturedState, capturedTrans)
			}
		})
		i.timers[timerKey] = timer
	}
}

// MarshalJSON serializes the snapshot to JSON.
func (s Snapshot[C]) MarshalJSON() ([]byte, error) {
	type Alias Snapshot[C]
	return json.Marshal(Alias(s))
}

// UnmarshalJSON deserializes the snapshot from JSON.
func (s *Snapshot[C]) UnmarshalJSON(data []byte) error {
	type Alias Snapshot[C]
	return json.Unmarshal(data, (*Alias)(s))
}

// RestoreError represents an error during snapshot restoration.
type RestoreError struct {
	Code    string
	Message string
}

func (e *RestoreError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error codes for snapshot operations
const (
	ErrCodeSnapshotMachineMismatch = "SNAPSHOT_MACHINE_MISMATCH"
	ErrCodeSnapshotInvalidState    = "SNAPSHOT_INVALID_STATE"
	ErrCodeSnapshotVersionMismatch = "SNAPSHOT_VERSION_MISMATCH"
)
