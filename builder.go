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
	machine := ir.NewMachineConfig(b.id, ir.StateID(b.initial), b.context)

	// Copy actions and guards
	for name, action := range b.actions {
		machine.Actions[ir.ActionType(name)] = ir.Action[C](action)
	}
	for name, guard := range b.guards {
		machine.Guards[ir.GuardType(name)] = ir.Guard[C](guard)
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
	stateType := ir.StateType(sb.stateType)
	if len(sb.children) > 0 && sb.stateType == StateTypeAtomic {
		stateType = ir.StateTypeCompound
	}

	state := ir.NewStateConfig(ir.StateID(sb.id), stateType)
	state.Parent = parentID

	// Set initial for compound states
	if len(sb.children) > 0 {
		state.Initial = ir.StateID(sb.initial)
		for _, child := range sb.children {
			state.Children = append(state.Children, ir.StateID(child.id))
		}
	}

	// Convert entry actions
	for _, a := range sb.entry {
		state.Entry = append(state.Entry, ir.ActionType(a))
	}
	// Convert exit actions
	for _, a := range sb.exit {
		state.Exit = append(state.Exit, ir.ActionType(a))
	}

	// Build transitions
	for _, tb := range sb.transitions {
		trans := ir.NewTransitionConfig(ir.EventType(tb.event), ir.StateID(tb.target))
		trans.Guard = ir.GuardType(tb.guard)
		for _, a := range tb.actions {
			trans.Actions = append(trans.Actions, ir.ActionType(a))
		}
		state.Transitions = append(state.Transitions, trans)
	}

	machine.States[ir.StateID(sb.id)] = state

	// Recursively build children
	for _, child := range sb.children {
		buildStateRecursive(child, ir.StateID(sb.id), machine)
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
