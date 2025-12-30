package ir

import "time"

// MachineConfig is the immutable internal representation of a statechart
type MachineConfig[C any] struct {
	ID       string
	Initial  StateID
	Context  C
	States   map[StateID]*StateConfig
	Actions  map[ActionType]Action[C]
	Guards   map[GuardType]Guard[C]
	Services map[ServiceType]Service[C] // v3.0: Invoked services
}

// StateConfig represents a single state node
type StateConfig struct {
	ID          StateID
	Type        StateType
	Parent      StateID   // Parent state ID (empty for root-level states)
	Initial     StateID   // Initial child state (for compound states only)
	Children    []StateID // Child state IDs (for compound states only)
	Entry       []ActionType
	Exit        []ActionType
	Transitions []*TransitionConfig

	// History state fields (v2.0)
	HistoryType    HistoryType // Shallow or Deep (only for StateTypeHistory)
	HistoryDefault StateID     // Default target if no history recorded

	// Invoked services (v3.0)
	Invocations []*InvokeConfig
}

// InvokeConfig defines a service to be invoked when entering a state
type InvokeConfig struct {
	// ID uniquely identifies this invocation within the state
	ID string

	// Src references a registered service by name
	Src ServiceType

	// OnDone transition when service completes successfully
	OnDone *TransitionConfig

	// OnError transition when service fails
	OnError *TransitionConfig
}

// TransitionConfig represents a single transition
type TransitionConfig struct {
	Event   EventType
	Target  StateID
	Guard   GuardType // Optional, empty string means no guard
	Actions []ActionType

	// Delayed transition fields (v2.0)
	// When Delay > 0, this is a delayed (after) transition
	Delay time.Duration
}

// IsDelayed returns true if this is a delayed transition
func (t *TransitionConfig) IsDelayed() bool {
	return t.Delay > 0
}

// NewMachineConfig creates a new MachineConfig with initialized maps
func NewMachineConfig[C any](id string, initial StateID, ctx C) *MachineConfig[C] {
	return &MachineConfig[C]{
		ID:       id,
		Initial:  initial,
		Context:  ctx,
		States:   make(map[StateID]*StateConfig),
		Actions:  make(map[ActionType]Action[C]),
		Guards:   make(map[GuardType]Guard[C]),
		Services: make(map[ServiceType]Service[C]),
	}
}

// NewStateConfig creates a new StateConfig
func NewStateConfig(id StateID, stateType StateType) *StateConfig {
	return &StateConfig{
		ID:   id,
		Type: stateType,
	}
}

// NewTransitionConfig creates a new TransitionConfig
func NewTransitionConfig(event EventType, target StateID) *TransitionConfig {
	return &TransitionConfig{
		Event:  event,
		Target: target,
	}
}

// GetState returns the state config for the given ID, or nil if not found
func (m *MachineConfig[C]) GetState(id StateID) *StateConfig {
	return m.States[id]
}

// GetAction returns the action for the given type, or nil if not found
func (m *MachineConfig[C]) GetAction(t ActionType) Action[C] {
	return m.Actions[t]
}

// GetGuard returns the guard for the given type, or nil if not found
func (m *MachineConfig[C]) GetGuard(t GuardType) Guard[C] {
	return m.Guards[t]
}

// GetService returns the service for the given type, or nil if not found
func (m *MachineConfig[C]) GetService(t ServiceType) Service[C] {
	return m.Services[t]
}

// FindTransition finds the first matching transition for the given event
// Returns nil if no matching transition is found
func (s *StateConfig) FindTransition(event EventType) *TransitionConfig {
	for _, t := range s.Transitions {
		if t.Event == event {
			return t
		}
	}
	return nil
}

// IsCompound returns true if this is a compound state with children
func (s *StateConfig) IsCompound() bool {
	return s.Type == StateTypeCompound && len(s.Children) > 0
}

// IsAtomic returns true if this is an atomic (leaf) state
func (s *StateConfig) IsAtomic() bool {
	return s.Type == StateTypeAtomic
}

// IsFinal returns true if this is a final state
func (s *StateConfig) IsFinal() bool {
	return s.Type == StateTypeFinal
}

// IsHistory returns true if this is a history state
func (s *StateConfig) IsHistory() bool {
	return s.Type == StateTypeHistory
}

// IsParallel returns true if this is a parallel state
func (s *StateConfig) IsParallel() bool {
	return s.Type == StateTypeParallel
}

// GetDelayedTransitions returns all delayed transitions for this state
func (s *StateConfig) GetDelayedTransitions() []*TransitionConfig {
	var delayed []*TransitionConfig
	for _, t := range s.Transitions {
		if t.IsDelayed() {
			delayed = append(delayed, t)
		}
	}
	return delayed
}

// GetAncestors returns all ancestor state IDs from immediate parent to root
func (m *MachineConfig[C]) GetAncestors(stateID StateID) []StateID {
	var ancestors []StateID
	current := m.GetState(stateID)
	for current != nil && current.Parent != "" {
		ancestors = append(ancestors, current.Parent)
		current = m.GetState(current.Parent)
	}
	return ancestors
}

// GetPath returns the full path from root to the given state
func (m *MachineConfig[C]) GetPath(stateID StateID) []StateID {
	ancestors := m.GetAncestors(stateID)
	// Reverse to get root-to-leaf order
	path := make([]StateID, len(ancestors)+1)
	for i, id := range ancestors {
		path[len(ancestors)-1-i] = id
	}
	path[len(path)-1] = stateID
	return path
}

// GetInitialLeaf resolves the initial state to its deepest leaf
// For atomic states, returns the state itself
// For compound states, recursively follows initial children
func (m *MachineConfig[C]) GetInitialLeaf(stateID StateID) StateID {
	state := m.GetState(stateID)
	if state == nil {
		return stateID
	}
	if state.IsCompound() && state.Initial != "" {
		return m.GetInitialLeaf(state.Initial)
	}
	return stateID
}

// IsDescendantOf checks if stateID is a descendant of ancestorID
func (m *MachineConfig[C]) IsDescendantOf(stateID, ancestorID StateID) bool {
	ancestors := m.GetAncestors(stateID)
	for _, a := range ancestors {
		if a == ancestorID {
			return true
		}
	}
	return false
}

// FindLCA finds the Lowest Common Ancestor of two states
func (m *MachineConfig[C]) FindLCA(stateA, stateB StateID) StateID {
	// Get ancestors including the states themselves
	pathA := m.GetPath(stateA)
	pathB := m.GetPath(stateB)

	var lca StateID
	for i := 0; i < len(pathA) && i < len(pathB); i++ {
		if pathA[i] == pathB[i] {
			lca = pathA[i]
		} else {
			break
		}
	}
	return lca
}

// WithContext returns a copy of the machine config with the given context.
// This is useful for initializing child actors with a derived context.
func (m *MachineConfig[C]) WithContext(ctx C) *MachineConfig[C] {
	return &MachineConfig[C]{
		ID:       m.ID,
		Initial:  m.Initial,
		Context:  ctx,
		States:   m.States,   // Share state configs (immutable)
		Actions:  m.Actions,  // Share actions (immutable)
		Guards:   m.Guards,   // Share guards (immutable)
		Services: m.Services, // Share services (immutable)
	}
}
