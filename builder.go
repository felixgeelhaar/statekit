package statekit

import "github.com/felixgeelhaar/statekit/internal/ir"

// MachineBuilder provides a fluent API for constructing state machines
type MachineBuilder[C any] struct {
	id      string
	initial StateID
	context C
	states  []*StateBuilder[C]
	actions map[ActionType]Action[C]
	guards  map[GuardType]Guard[C]
}

// StateBuilder provides a fluent API for constructing states
type StateBuilder[C any] struct {
	machine     *MachineBuilder[C]
	parent      *StateBuilder[C] // Parent state for nested states
	id          StateID
	stateType   StateType
	initial     StateID // Initial child state (for compound states)
	children    []*StateBuilder[C]
	entry       []ActionType
	exit        []ActionType
	transitions []*TransitionBuilder[C]

	// History state fields (v2.0)
	historyType    HistoryType
	historyDefault StateID
}

// HistoryBuilder provides a fluent API for constructing history states
type HistoryBuilder[C any] struct {
	parent      *StateBuilder[C] // Parent compound state
	id          StateID
	historyType HistoryType
	defaultID   StateID
}

// TransitionBuilder provides a fluent API for constructing transitions
type TransitionBuilder[C any] struct {
	state   *StateBuilder[C]
	event   EventType
	target  StateID
	guard   GuardType
	actions []ActionType
}

// NewMachine creates a new MachineBuilder with the given ID
func NewMachine[C any](id string) *MachineBuilder[C] {
	return &MachineBuilder[C]{
		id:      id,
		actions: make(map[ActionType]Action[C]),
		guards:  make(map[GuardType]Guard[C]),
	}
}

// WithInitial sets the initial state ID
func (b *MachineBuilder[C]) WithInitial(initial StateID) *MachineBuilder[C] {
	b.initial = initial
	return b
}

// WithContext sets the initial context value
func (b *MachineBuilder[C]) WithContext(ctx C) *MachineBuilder[C] {
	b.context = ctx
	return b
}

// WithAction registers a named action
func (b *MachineBuilder[C]) WithAction(name ActionType, action Action[C]) *MachineBuilder[C] {
	b.actions[name] = action
	return b
}

// WithGuard registers a named guard
func (b *MachineBuilder[C]) WithGuard(name GuardType, guard Guard[C]) *MachineBuilder[C] {
	b.guards[name] = guard
	return b
}

// State starts building a new state with the given ID
func (b *MachineBuilder[C]) State(id StateID) *StateBuilder[C] {
	sb := &StateBuilder[C]{
		machine:   b,
		parent:    nil,
		id:        id,
		stateType: StateTypeAtomic,
	}
	b.states = append(b.states, sb)
	return sb
}

// Build constructs the final MachineConfig from the builder
func (b *MachineBuilder[C]) Build() (*ir.MachineConfig[C], error) {
	machine := ir.NewMachineConfig(b.id, b.initial, b.context)

	// Copy actions and guards (convert from statekit types to ir types)
	for name, action := range b.actions {
		machine.Actions[name] = ir.Action[C](action)
	}
	for name, guard := range b.guards {
		machine.Guards[name] = ir.Guard[C](guard)
	}

	// Build states recursively
	for _, sb := range b.states {
		buildStateRecursive(sb, "", machine)
	}

	// Validate the machine configuration
	if err := ir.Validate(machine); err != nil {
		return nil, err
	}

	return machine, nil
}

// buildStateRecursive adds a state and its children to the machine config
func buildStateRecursive[C any](sb *StateBuilder[C], parentID ir.StateID, machine *ir.MachineConfig[C]) {
	// Determine state type
	stateType := sb.stateType
	if len(sb.children) > 0 && sb.stateType == StateTypeAtomic {
		stateType = ir.StateTypeCompound
	}

	state := ir.NewStateConfig(sb.id, stateType)
	state.Parent = parentID

	// Set initial for compound states
	if len(sb.children) > 0 {
		state.Initial = sb.initial
		for _, child := range sb.children {
			state.Children = append(state.Children, child.id)
		}
	}

	// Set history state fields (v2.0)
	if stateType == ir.StateTypeHistory {
		state.HistoryType = sb.historyType
		state.HistoryDefault = sb.historyDefault
	}

	// Convert entry/exit actions
	state.Entry = append(state.Entry, sb.entry...)
	state.Exit = append(state.Exit, sb.exit...)

	// Build transitions
	for _, tb := range sb.transitions {
		trans := ir.NewTransitionConfig(tb.event, tb.target)
		trans.Guard = tb.guard
		trans.Actions = append(trans.Actions, tb.actions...)
		state.Transitions = append(state.Transitions, trans)
	}

	machine.States[sb.id] = state

	// Recursively build children
	for _, child := range sb.children {
		buildStateRecursive(child, sb.id, machine)
	}
}

// --- StateBuilder methods ---

// Final marks this state as a final state
func (b *StateBuilder[C]) Final() *StateBuilder[C] {
	b.stateType = StateTypeFinal
	return b
}

// OnEntry adds an entry action to the state
func (b *StateBuilder[C]) OnEntry(action ActionType) *StateBuilder[C] {
	b.entry = append(b.entry, action)
	return b
}

// OnExit adds an exit action to the state
func (b *StateBuilder[C]) OnExit(action ActionType) *StateBuilder[C] {
	b.exit = append(b.exit, action)
	return b
}

// WithInitial sets the initial child state for a compound state
func (b *StateBuilder[C]) WithInitial(initial StateID) *StateBuilder[C] {
	b.initial = initial
	return b
}

// State starts building a nested child state
func (b *StateBuilder[C]) State(id StateID) *StateBuilder[C] {
	child := &StateBuilder[C]{
		machine:   b.machine,
		parent:    b,
		id:        id,
		stateType: StateTypeAtomic,
	}
	b.children = append(b.children, child)
	return child
}

// On starts building a new transition triggered by the given event
func (b *StateBuilder[C]) On(event EventType) *TransitionBuilder[C] {
	tb := &TransitionBuilder[C]{
		state: b,
		event: event,
	}
	b.transitions = append(b.transitions, tb)
	return tb
}

// Done completes the state definition and returns to the parent builder
// For nested states, returns to the parent StateBuilder
// For root states, returns to the MachineBuilder
func (b *StateBuilder[C]) Done() *MachineBuilder[C] {
	// If this is a nested state, we need to return the machine builder
	// but the caller should use End() for better clarity
	return b.machine
}

// End completes a nested state and returns to the parent StateBuilder
// Use this instead of Done() when building nested states
func (b *StateBuilder[C]) End() *StateBuilder[C] {
	if b.parent != nil {
		return b.parent
	}
	// If no parent, this is a programming error, but we'll return nil
	return nil
}

// History starts building a history state within this compound state (v2.0)
// History states remember the last active child and transition back to it
func (b *StateBuilder[C]) History(id StateID) *HistoryBuilder[C] {
	return &HistoryBuilder[C]{
		parent:      b,
		id:          id,
		historyType: HistoryTypeShallow,
	}
}

// --- HistoryBuilder methods (v2.0) ---

// Shallow sets the history type to shallow (remembers immediate child)
func (b *HistoryBuilder[C]) Shallow() *HistoryBuilder[C] {
	b.historyType = HistoryTypeShallow
	return b
}

// Deep sets the history type to deep (remembers full leaf path)
func (b *HistoryBuilder[C]) Deep() *HistoryBuilder[C] {
	b.historyType = HistoryTypeDeep
	return b
}

// Default sets the default target state if no history is recorded
func (b *HistoryBuilder[C]) Default(target StateID) *HistoryBuilder[C] {
	b.defaultID = target
	return b
}

// End completes the history state definition and returns to the parent StateBuilder
func (b *HistoryBuilder[C]) End() *StateBuilder[C] {
	// Create a StateBuilder for the history state
	historyState := &StateBuilder[C]{
		machine:        b.parent.machine,
		parent:         b.parent,
		id:             b.id,
		stateType:      StateTypeHistory,
		historyType:    b.historyType,
		historyDefault: b.defaultID,
	}
	b.parent.children = append(b.parent.children, historyState)
	return b.parent
}

// --- TransitionBuilder methods ---

// Target sets the target state for the transition
func (b *TransitionBuilder[C]) Target(target StateID) *TransitionBuilder[C] {
	b.target = target
	return b
}

// Guard sets the guard condition for the transition
func (b *TransitionBuilder[C]) Guard(guard GuardType) *TransitionBuilder[C] {
	b.guard = guard
	return b
}

// Do adds an action to be executed during the transition
func (b *TransitionBuilder[C]) Do(action ActionType) *TransitionBuilder[C] {
	b.actions = append(b.actions, action)
	return b
}

// On starts a new transition on the same state (chainable)
func (b *TransitionBuilder[C]) On(event EventType) *TransitionBuilder[C] {
	return b.state.On(event)
}

// Done completes the state definition and returns to the machine builder
func (b *TransitionBuilder[C]) Done() *MachineBuilder[C] {
	return b.state.Done()
}

// End completes the transition and returns to the parent StateBuilder
// Use this instead of Done() when building transitions in nested states
func (b *TransitionBuilder[C]) End() *StateBuilder[C] {
	return b.state
}
