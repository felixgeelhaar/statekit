package statekit

import (
	"fmt"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
)

// MachineBuilder provides a fluent API for constructing state machines
type MachineBuilder[C any] struct {
	id            string
	initial       StateID
	context       C
	states        []*StateBuilder[C]
	actions       map[ActionType]Action[C]
	guards        map[GuardType]Guard[C]
	services      map[ServiceType]Service[C]           // v3.0: Invoked services
	childMachines map[string]ir.ChildMachineFactory[C] // v0.14: Child machine factories
}

// StateBuilder provides a fluent API for constructing states
type StateBuilder[C any] struct {
	machine     *MachineBuilder[C]
	parent      *StateBuilder[C]  // Parent state for nested states
	region      *RegionBuilder[C] // Parent region for states in parallel regions (v2.0)
	id          StateID
	stateType   StateType
	initial     StateID // Initial child state (for compound states)
	children    []*StateBuilder[C]
	entry       []ActionType
	exit        []ActionType
	transitions []*TransitionBuilder[C]
	always      []*TransitionBuilder[C] // eventless transitions (v1.x)
	tags        []string                // state tags (v1.x)

	// History state fields (v2.0)
	historyType    HistoryType
	historyDefault StateID

	// Invoked services (v3.0)
	invocations []*InvokeBuilder[C]

	// Invoked child machines (v0.14)
	machineInvocations []*MachineInvokeBuilder[C]
}

// InvokeBuilder provides a fluent API for constructing service invocations (v3.0)
//
// The name is kept for backward compatibility; use the
// InvokeServiceBuilder alias for clearer intent next to
// InvokeMachineBuilder.
type InvokeBuilder[C any] struct {
	state        *StateBuilder[C]
	id           string
	src          ServiceType
	onDoneTarget StateID
	onDoneAction ActionType
	onErrTarget  StateID
	onErrAction  ActionType
}

// InvokeServiceBuilder is a clearer alias for InvokeBuilder. Prefer
// this name in new code — it parallels InvokeMachineBuilder so the
// two service/machine invocation paths are self-documenting.
type InvokeServiceBuilder[C any] = InvokeBuilder[C]

// MachineInvokeBuilder provides a fluent API for constructing child machine invocations (v0.14)
//
// The name is kept for backward compatibility; use the
// InvokeMachineBuilder alias for clearer intent next to
// InvokeServiceBuilder.
type MachineInvokeBuilder[C any] struct {
	state        *StateBuilder[C]
	id           string
	machineRef   string
	onDoneTarget StateID
	onDoneAction ActionType
	onErrTarget  StateID
	onErrAction  ActionType
	autoForward  []EventType
}

// InvokeMachineBuilder is a clearer alias for MachineInvokeBuilder.
// Prefer this name in new code — it parallels InvokeServiceBuilder so
// the two service/machine invocation paths are self-documenting.
type InvokeMachineBuilder[C any] = MachineInvokeBuilder[C]

// HistoryBuilder provides a fluent API for constructing history states
type HistoryBuilder[C any] struct {
	parent      *StateBuilder[C] // Parent compound state
	id          StateID
	historyType HistoryType
	defaultID   StateID
}

// RegionBuilder provides a fluent API for constructing parallel regions (v2.0)
type RegionBuilder[C any] struct {
	parallel *StateBuilder[C] // Parent parallel state
	id       StateID
	initial  StateID
	children []*StateBuilder[C]
}

// TransitionBuilder provides a fluent API for constructing transitions
type TransitionBuilder[C any] struct {
	state   *StateBuilder[C]
	event   EventType
	target  StateID
	guard   GuardType
	actions []ActionType

	// Delayed transition fields (v2.0)
	delay time.Duration

	// Eventless ("always") transition flag (v1.x)
	eventless bool

	// Raised internal events (v1.x)
	raise []EventType

	// Internal transition flag — run actions without exit/entry (v1.x)
	internal bool
}

// NewMachine creates a new MachineBuilder with the given ID
func NewMachine[C any](id string) *MachineBuilder[C] {
	return &MachineBuilder[C]{
		id:            id,
		actions:       make(map[ActionType]Action[C]),
		guards:        make(map[GuardType]Guard[C]),
		services:      make(map[ServiceType]Service[C]),
		childMachines: make(map[string]ir.ChildMachineFactory[C]),
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

// WithService registers a named service (v3.0)
func (b *MachineBuilder[C]) WithService(name ServiceType, service Service[C]) *MachineBuilder[C] {
	b.services[name] = service
	return b
}

// WithChildMachine registers a child machine factory for machine composition (v0.14).
// The factory creates a child interpreter when the machine is invoked.
func (b *MachineBuilder[C]) WithChildMachine(name string, factory ir.ChildMachineFactory[C]) *MachineBuilder[C] {
	b.childMachines[name] = factory
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
	// Copy services (v3.0)
	for name, service := range b.services {
		machine.Services[name] = service
	}
	// Copy child machine factories (v0.14)
	for name, factory := range b.childMachines {
		machine.ChildMachines[name] = factory
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

	// Copy state tags (v1.x)
	state.Tags = append(state.Tags, sb.tags...)

	// Build transitions
	for _, tb := range sb.transitions {
		trans := ir.NewTransitionConfig(tb.event, tb.target)
		trans.Guard = tb.guard
		trans.Actions = append(trans.Actions, tb.actions...)
		trans.Delay = tb.delay // Delayed transitions (v2.0)
		trans.Raise = append(trans.Raise, tb.raise...)
		trans.Internal = tb.internal // Internal transitions (v1.x)
		state.Transitions = append(state.Transitions, trans)
	}

	// Build eventless ("always") transitions (v1.x)
	for _, tb := range sb.always {
		trans := ir.NewTransitionConfig("", tb.target)
		trans.Guard = tb.guard
		trans.Actions = append(trans.Actions, tb.actions...)
		trans.Raise = append(trans.Raise, tb.raise...)
		state.Always = append(state.Always, trans)
	}

	// Build invocations (v3.0)
	for _, ib := range sb.invocations {
		invoke := &ir.InvokeConfig{
			ID:  ib.id,
			Src: ib.src,
		}
		if ib.onDoneTarget != "" {
			invoke.OnDone = ir.NewTransitionConfig("", ib.onDoneTarget)
			if ib.onDoneAction != "" {
				invoke.OnDone.Actions = append(invoke.OnDone.Actions, ib.onDoneAction)
			}
		}
		if ib.onErrTarget != "" {
			invoke.OnError = ir.NewTransitionConfig("", ib.onErrTarget)
			if ib.onErrAction != "" {
				invoke.OnError.Actions = append(invoke.OnError.Actions, ib.onErrAction)
			}
		}
		state.Invocations = append(state.Invocations, invoke)
	}

	// Build machine invocations (v0.14)
	for _, mib := range sb.machineInvocations {
		machineInvoke := &ir.MachineInvokeConfig{
			ID:          mib.id,
			MachineRef:  mib.machineRef,
			AutoForward: mib.autoForward,
		}
		if mib.onDoneTarget != "" {
			machineInvoke.OnDone = ir.NewTransitionConfig("", mib.onDoneTarget)
			if mib.onDoneAction != "" {
				machineInvoke.OnDone.Actions = append(machineInvoke.OnDone.Actions, mib.onDoneAction)
			}
		}
		if mib.onErrTarget != "" {
			machineInvoke.OnError = ir.NewTransitionConfig("", mib.onErrTarget)
			if mib.onErrAction != "" {
				machineInvoke.OnError.Actions = append(machineInvoke.OnError.Actions, mib.onErrAction)
			}
		}
		state.MachineInvocations = append(state.MachineInvocations, machineInvoke)
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

// Always starts building an eventless ("always") transition (v1.x). It is
// evaluated when the state is entered and after every transition, in
// declaration order; the first whose guard passes is taken. A target is
// required. Use multiple Always() calls to express guarded routing with a
// final guardless fallback.
func (b *StateBuilder[C]) Always() *TransitionBuilder[C] {
	tb := &TransitionBuilder[C]{
		state:     b,
		eventless: true,
	}
	b.always = append(b.always, tb)
	return tb
}

// Tags attaches one or more tags to the state for lightweight querying via
// Interpreter.HasTag (v1.x).
func (b *StateBuilder[C]) Tags(tags ...string) *StateBuilder[C] {
	b.tags = append(b.tags, tags...)
	return b
}

// Done closes this state and returns to the MachineBuilder — the canonical
// terminator for a top-level state.
//
// It is the right choice whenever the next thing you write is another
// top-level State() or Build(). Every example in the README and the docs uses
// it.
//
//	machine, _ := statekit.NewMachine[Ctx]("order").
//		WithInitial("cart").
//		State("cart").On("CHECKOUT").Target("paid").Done().
//		State("paid").Final().Done().
//		Build()
//
// Called from a nested state it still returns the machine root, skipping every
// intermediate parent. That is occasionally what you want and usually not — see
// the terminator table in the package documentation, and prefer End to step up
// one level.
func (b *StateBuilder[C]) Done() *MachineBuilder[C] {
	return b.machine
}

// EndMachine closes this state and returns to the MachineBuilder.
//
// Deprecated: use Done, which is identical in signature, behaviour, and
// return value. Two spellings of one terminator is the confusion this
// deprecation removes; nothing at a call site distinguished them. EndMachine
// keeps working and is not scheduled for removal — replacing it is a
// find-and-replace whenever convenient.
func (b *StateBuilder[C]) EndMachine() *MachineBuilder[C] {
	return b.machine
}

// End closes this nested state and returns to the enclosing StateBuilder —
// one level up, not to the machine root.
//
// This is the terminator for a child of a compound state. Chain it once per
// level to unwind:
//
//	State("editing").
//		WithInitial("idle").
//		State("idle").On("TYPE").Target("dirty").End().End().
//		//                                       ^      ^ back to "editing"
//		//                                       back to "idle"
//		Done()
//
// It panics when called on a top-level state, which has no enclosing state to
// return to. Use Done there instead.
func (b *StateBuilder[C]) End() *StateBuilder[C] {
	if b.parent == nil {
		panic(fmt.Sprintf("statekit: End called on top-level state %q, "+
			"which has no enclosing state; use Done to return to the machine builder", b.id))
	}
	return b.parent
}

// EndState closes this state and returns to the enclosing RegionBuilder (v2.0)
// — the terminator for a state inside a parallel region.
//
// Close the region itself with EndRegion, and the parallel state with Done:
//
//	State("editor").
//		Parallel().
//		Region("bold").WithInitial("off").
//			State("off").On("TOGGLE").Target("on").EndState().
//			State("on").On("TOGGLE").Target("off").EndState().
//		EndRegion().
//		Done()
//
// It panics when called on a state that is not inside a region. Use End for a
// child of a compound state, or Done for a top-level state.
func (b *StateBuilder[C]) EndState() *RegionBuilder[C] {
	if b.region == nil {
		panic(fmt.Sprintf("statekit: EndState called on state %q, which is not inside a parallel region; "+
			"use End for a nested state or Done for a top-level state", b.id))
	}
	return b.region
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

// Parallel marks this state as a parallel state (v2.0)
// Use Region() to add orthogonal regions that execute simultaneously
func (b *StateBuilder[C]) Parallel() *StateBuilder[C] {
	b.stateType = StateTypeParallel
	return b
}

// Region starts building a new region within this parallel state (v2.0)
func (b *StateBuilder[C]) Region(id StateID) *RegionBuilder[C] {
	return &RegionBuilder[C]{
		parallel: b,
		id:       id,
	}
}

// After starts building a delayed transition that triggers automatically
// after the specified duration (v2.0)
func (b *StateBuilder[C]) After(d time.Duration) *TransitionBuilder[C] {
	tb := &TransitionBuilder[C]{
		state: b,
		delay: d,
	}
	b.transitions = append(b.transitions, tb)
	return tb
}

// Invoke starts building a service invocation for this state (v3.0)
// The service is started when entering the state and cancelled when exiting
func (b *StateBuilder[C]) Invoke(src ServiceType) *InvokeBuilder[C] {
	ib := &InvokeBuilder[C]{
		state: b,
		src:   src,
		id:    string(src), // Default ID is the service name
	}
	b.invocations = append(b.invocations, ib)
	return ib
}

// InvokeMachine starts building a child machine invocation for this state (v0.14).
// The child machine is spawned when entering the state and stopped when exiting.
// The machineRef must match a name registered with WithChildMachine.
//
// Tier 2 — experimental: the composition API may change in a future v1.x minor.
// See docs/reference/stability.md.
func (b *StateBuilder[C]) InvokeMachine(machineRef string) *MachineInvokeBuilder[C] {
	mib := &MachineInvokeBuilder[C]{
		state:      b,
		machineRef: machineRef,
		id:         machineRef, // Default ID is the machine reference
	}
	b.machineInvocations = append(b.machineInvocations, mib)
	return mib
}

// --- InvokeBuilder methods (v3.0) ---

// ID sets a custom ID for this invocation
func (b *InvokeBuilder[C]) ID(id string) *InvokeBuilder[C] {
	b.id = id
	return b
}

// OnDone sets the transition target when the service completes successfully
func (b *InvokeBuilder[C]) OnDone(target StateID) *InvokeBuilder[C] {
	b.onDoneTarget = target
	return b
}

// OnDoneAction sets an action to execute when the service completes successfully
func (b *InvokeBuilder[C]) OnDoneAction(action ActionType) *InvokeBuilder[C] {
	b.onDoneAction = action
	return b
}

// OnError sets the transition target when the service fails
func (b *InvokeBuilder[C]) OnError(target StateID) *InvokeBuilder[C] {
	b.onErrTarget = target
	return b
}

// OnErrorAction sets an action to execute when the service fails
func (b *InvokeBuilder[C]) OnErrorAction(action ActionType) *InvokeBuilder[C] {
	b.onErrAction = action
	return b
}

// End completes the invocation definition and returns to the StateBuilder
func (b *InvokeBuilder[C]) End() *StateBuilder[C] {
	return b.state
}

// Done completes the state definition and returns to the machine builder
func (b *InvokeBuilder[C]) Done() *MachineBuilder[C] {
	return b.state.Done()
}

// --- MachineInvokeBuilder methods (v0.14) ---

// ID sets a custom ID for this machine invocation
func (b *MachineInvokeBuilder[C]) ID(id string) *MachineInvokeBuilder[C] {
	b.id = id
	return b
}

// OnDone sets the transition target when the child machine reaches a final state
func (b *MachineInvokeBuilder[C]) OnDone(target StateID) *MachineInvokeBuilder[C] {
	b.onDoneTarget = target
	return b
}

// OnDoneAction sets an action to execute when the child machine completes
func (b *MachineInvokeBuilder[C]) OnDoneAction(action ActionType) *MachineInvokeBuilder[C] {
	b.onDoneAction = action
	return b
}

// OnError sets the transition target when the child machine encounters an error
func (b *MachineInvokeBuilder[C]) OnError(target StateID) *MachineInvokeBuilder[C] {
	b.onErrTarget = target
	return b
}

// OnErrorAction sets an action to execute when the child machine fails
func (b *MachineInvokeBuilder[C]) OnErrorAction(action ActionType) *MachineInvokeBuilder[C] {
	b.onErrAction = action
	return b
}

// AutoForward adds event types that should be auto-forwarded to the child machine
func (b *MachineInvokeBuilder[C]) AutoForward(events ...EventType) *MachineInvokeBuilder[C] {
	b.autoForward = append(b.autoForward, events...)
	return b
}

// End completes the machine invocation definition and returns to the StateBuilder
func (b *MachineInvokeBuilder[C]) End() *StateBuilder[C] {
	return b.state
}

// Done completes the state definition and returns to the machine builder
func (b *MachineInvokeBuilder[C]) Done() *MachineBuilder[C] {
	return b.state.Done()
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

// --- RegionBuilder methods (v2.0) ---

// WithInitial sets the initial state for this region
func (b *RegionBuilder[C]) WithInitial(initial StateID) *RegionBuilder[C] {
	b.initial = initial
	return b
}

// State starts building a state within this region
func (b *RegionBuilder[C]) State(id StateID) *StateBuilder[C] {
	child := &StateBuilder[C]{
		machine:   b.parallel.machine,
		parent:    nil,
		region:    b, // Set region reference so End() returns to region
		id:        id,
		stateType: StateTypeAtomic,
	}
	b.children = append(b.children, child)
	return child
}

// EndRegion completes the region and returns to the parent parallel state
func (b *RegionBuilder[C]) EndRegion() *StateBuilder[C] {
	// Create a StateBuilder for the region (as a compound state)
	regionState := &StateBuilder[C]{
		machine:   b.parallel.machine,
		parent:    b.parallel,
		id:        b.id,
		stateType: StateTypeCompound,
		initial:   b.initial,
		children:  b.children,
	}

	// Fix the parent references for all children
	for _, child := range b.children {
		child.parent = regionState
	}

	b.parallel.children = append(b.parallel.children, regionState)
	return b.parallel
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

// Raise enqueues internal events emitted when this transition is taken (v1.x).
// Raised events are processed in the same macrostep — before control returns
// to the caller and before any externally sent event.
func (b *TransitionBuilder[C]) Raise(events ...EventType) *TransitionBuilder[C] {
	b.raise = append(b.raise, events...)
	return b
}

// Internal marks the transition as internal (v1.x): its actions run without
// exiting or re-entering the source state — entry/exit hooks do not fire and
// the active state does not change. Target is optional; when set it must be
// the owning state. Contrast with an external self-transition (a plain
// Target back to the same state), which does exit and re-enter.
func (b *TransitionBuilder[C]) Internal() *TransitionBuilder[C] {
	b.internal = true
	return b
}

// On starts a new transition on the same state (chainable)
func (b *TransitionBuilder[C]) On(event EventType) *TransitionBuilder[C] {
	return b.state.On(event)
}

// After starts a new delayed transition on the same state (chainable) (v2.0)
func (b *TransitionBuilder[C]) After(d time.Duration) *TransitionBuilder[C] {
	return b.state.After(d)
}

// Done closes this transition and its owning state, returning to the
// MachineBuilder — the canonical terminator for a transition on a top-level
// state.
//
//	State("cart").On("CHECKOUT").Target("paid").Done().
//
// Consecutive On calls chain without a terminator between them, so one Done
// closes the whole group:
//
//	State("review").
//		On("APPROVE").Target("published").
//		On("REJECT").Target("rejected").
//		Done().
func (b *TransitionBuilder[C]) Done() *MachineBuilder[C] {
	return b.state.Done()
}

// EndMachine closes this transition and its owning state, returning to the
// MachineBuilder.
//
// Deprecated: use Done, which is identical in signature, behaviour, and
// return value. See StateBuilder.EndMachine.
func (b *TransitionBuilder[C]) EndMachine() *MachineBuilder[C] {
	return b.state.Done()
}

// End closes this transition and returns to the StateBuilder that owns it —
// the terminator to use when the state needs more definition afterwards, or
// when it is nested and you intend to unwind one level at a time.
//
//	State("idle").On("TYPE").Target("dirty").End().End()
//	//                                       ^ back to "idle"
func (b *TransitionBuilder[C]) End() *StateBuilder[C] {
	return b.state
}

// EndState closes this transition and its owning state, returning to the
// enclosing RegionBuilder (v2.0) — the terminator for a transition on a state
// inside a parallel region.
//
//	State("off").On("TOGGLE").Target("on").EndState().
//
// It panics when the owning state is not inside a region. Use End to return
// to the state, or Done for a top-level state.
func (b *TransitionBuilder[C]) EndState() *RegionBuilder[C] {
	return b.state.EndState()
}
