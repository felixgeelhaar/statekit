package statekit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.klarlabs.de/statekit/internal/ir"
	"go.klarlabs.de/statekit/plugin"
)

// Interpreter is the statechart runtime that processes events and manages state
type Interpreter[C any] struct {
	machine *ir.MachineConfig[C]
	state   State[C]
	started bool

	// Mutex to protect all interpreter state from concurrent access (e.g., timer goroutines)
	mu sync.Mutex

	// Plugin system (v0.14)
	plugins []plugin.Plugin[C]

	// History tracking (v2.0)
	// Maps compound state ID to the last immediate child that was active (shallow)
	shallowHistory map[ir.StateID]ir.StateID
	// Maps compound state ID to the last leaf state that was active (deep)
	deepHistory map[ir.StateID]ir.StateID

	// Timer management for delayed transitions (v2.0)
	// Maps timer key (stateID:index) to active timer
	// Protected by mu (single mutex to prevent deadlocks)
	timers map[string]Timer

	// clock supplies AfterFunc; defaults to systemClock (wall clock)
	// and can be overridden via WithClock for deterministic tests.
	clock Clock

	// Parallel state tracking (v2.0)
	// When inside a parallel state, this holds the parallel state ID
	// The actual region states are tracked in state.ActiveInParallel
	currentParallel ir.StateID

	// Invoked services tracking (v3.0)
	// Maps invocation key (stateID:invokeID) to cancel function
	activeServices map[string]context.CancelFunc

	// Actor management (v4.0)
	// Separate mutex for actor operations to allow spawning from within actions
	actorMu sync.Mutex
	// Maps ActorID to actor entry (contains ref and config)
	actorRegistry map[ActorID]*actorEntry
	// Maps StateID to list of ActorIDs spawned in that state (for state-scoped cleanup)
	actorsByState map[StateID][]ActorID
	// If this interpreter is a child actor, holds the function to send to parent
	parentSend func(Event) error

	// Invoked machine management (v0.14)
	// Maps invoke key (stateID:invokeID) to running child interpreter
	activeInvokedMachines map[string]ir.ChildInterpreter

	// Internal event queue for raised events (v1.x). Drained within the same
	// macrostep before control returns to the caller. Protected by mu.
	internalQueue []Event
}

// maxMicrosteps bounds a single macrostep (eventless + raised-event
// processing) to guard against guard-always-true cycles. Exceeding it stops
// the macrostep and reports via the OnError plugin hook.
const maxMicrosteps = 10_000

// transitionSource holds the state that owns the transition and the transition itself
type transitionSource[C any] struct {
	state      *ir.StateConfig
	transition *ir.TransitionConfig
}

// Option configures a new Interpreter at construction time.
type Option[C any] func(*Interpreter[C])

// WithClock overrides the interpreter's Clock. The default is the
// wall-clock; pass a FakeClock to make timer-driven behavior
// deterministic in tests.
func WithClock[C any](c Clock) Option[C] {
	return func(i *Interpreter[C]) { i.clock = c }
}

// NewInterpreter creates a new interpreter for the given machine configuration
func NewInterpreter[C any](machine *ir.MachineConfig[C], opts ...Option[C]) *Interpreter[C] {
	i := &Interpreter[C]{
		machine: machine,
		state: State[C]{
			Context:          machine.Context,
			ActiveInParallel: make(map[ir.StateID]ir.StateID),
		},
		shallowHistory:        make(map[ir.StateID]ir.StateID),
		deepHistory:           make(map[ir.StateID]ir.StateID),
		timers:                make(map[string]Timer),
		activeServices:        make(map[string]context.CancelFunc),
		actorRegistry:         make(map[ActorID]*actorEntry),
		actorsByState:         make(map[StateID][]ActorID),
		activeInvokedMachines: make(map[string]ir.ChildInterpreter),
		clock:                 systemClock{},
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// Start initializes the interpreter and enters the initial state
func (i *Interpreter[C]) Start() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.started {
		return
	}
	i.started = true

	// Call OnStart hooks
	i.callOnStart()

	// Enter initial state, resolving to deepest leaf
	i.enterStateHierarchy(i.machine.Initial)

	// Run eventless transitions / drain raised events from the initial state
	i.macrostep()
}

// State returns the current state of the interpreter
func (i *Interpreter[C]) State() State[C] {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.state
}

// Matches checks if the current state matches the given state ID
// For hierarchical states, returns true if current state equals id or is a descendant of id
// For parallel states, also checks all active region states
func (i *Interpreter[C]) Matches(id StateID) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.matchesUnlocked(id)
}

// matchesUnlocked is the internal version without locking (caller must hold mu)
func (i *Interpreter[C]) matchesUnlocked(id StateID) bool {
	if i.state.Value == id {
		return true
	}
	// Check if current state is a descendant of the given state
	if i.machine.IsDescendantOf(i.state.Value, id) {
		return true
	}
	// Check parallel regions (v2.0)
	for _, leafID := range i.state.ActiveInParallel {
		if leafID == id || i.machine.IsDescendantOf(leafID, id) {
			return true
		}
	}
	return false
}

// HasTag reports whether any currently active state carries the given tag.
// "Active" means the current leaf state, its ancestors, and — when inside a
// parallel state — every active region leaf and its ancestors (v1.x).
func (i *Interpreter[C]) HasTag(tag string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.started {
		return false
	}

	if i.stateChainHasTag(i.state.Value, tag) {
		return true
	}
	for _, leafID := range i.state.ActiveInParallel {
		if i.stateChainHasTag(leafID, tag) {
			return true
		}
	}
	return false
}

// stateChainHasTag checks the given state and all its ancestors for the tag.
func (i *Interpreter[C]) stateChainHasTag(leaf StateID, tag string) bool {
	for _, id := range i.machine.GetPath(leaf) {
		if sc := i.machine.GetState(id); sc != nil && sc.HasTag(tag) {
			return true
		}
	}
	return false
}

// findEventlessTransition returns the first enabled eventless ("always")
// transition for the current leaf, bubbling up through ancestors. Returns nil
// when none is enabled.
func (i *Interpreter[C]) findEventlessTransition() *transitionSource[C] {
	current := i.machine.GetState(i.state.Value)
	for current != nil {
		for _, t := range current.Always {
			if t.Guard != "" {
				guard := i.machine.GetGuard(t.Guard)
				if guard != nil && !guard(i.state.Context, Event{}) {
					continue
				}
			}
			return &transitionSource[C]{state: current, transition: t}
		}
		if current.Parent == "" {
			break
		}
		current = i.machine.GetState(current.Parent)
	}
	return nil
}

// enqueueRaised appends a transition's raised events to the internal queue.
func (i *Interpreter[C]) enqueueRaised(t *ir.TransitionConfig) {
	for _, evt := range t.Raise {
		i.internalQueue = append(i.internalQueue, Event{Type: evt})
	}
}

// macrostep runs to quiescence: it repeatedly takes enabled eventless
// transitions and processes raised internal events until neither remains.
// Eventless transitions have priority over queued events within an iteration.
// Bounded by maxMicrosteps to guard against always-true cycles.
//
// Scoped to non-parallel configurations for v1.x; inside a parallel state the
// macrostep is a no-op (eventless/raise within regions is a future addition).
func (i *Interpreter[C]) macrostep() {
	if i.currentParallel != "" {
		return
	}

	for step := 0; ; step++ {
		if step >= maxMicrosteps {
			i.callOnError(fmt.Errorf("statekit: macrostep exceeded %d microsteps (likely an always-true eventless cycle)", maxMicrosteps))
			i.internalQueue = nil
			return
		}

		// 1. Eventless transitions take priority.
		if src := i.findEventlessTransition(); src != nil {
			i.executeTransitionHierarchical(src, Event{})
			if i.currentParallel != "" {
				return // entered a parallel state; stop settling
			}
			continue
		}

		// 2. Drain one raised internal event.
		if len(i.internalQueue) > 0 {
			evt := i.internalQueue[0]
			i.internalQueue = i.internalQueue[1:]

			current := i.machine.GetState(i.state.Value)
			if current == nil {
				continue
			}
			if source := i.findMatchingTransitionHierarchical(current, evt); source != nil {
				i.executeTransitionHierarchical(source, evt)
				if i.currentParallel != "" {
					return
				}
			}
			continue
		}

		// Quiescent.
		return
	}
}

// Done returns true if the machine is in a final state
func (i *Interpreter[C]) Done() bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.started {
		return false
	}
	stateConfig := i.machine.GetState(i.state.Value)
	if stateConfig == nil {
		return false
	}
	return stateConfig.Type == ir.StateTypeFinal
}

// Send processes an event and potentially transitions to a new state.
func (i *Interpreter[C]) Send(event Event) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.sendLocked(event)
}

// SendResult processes an event like [Interpreter.Send] and reports whether it
// was handled — i.e. a matching, guard-passing transition fired. It returns
// false when no transition matched the event from the current state, or when
// every candidate transition was blocked by its guard. This lets callers
// distinguish an applied transition from one that was silently rejected (a
// blocked guard), instead of having to infer it from the resulting state.
func (i *Interpreter[C]) SendResult(event Event) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.sendLocked(event)
}

// sendLocked performs the event processing with i.mu already held and reports
// whether a transition (or parallel-region delivery) occurred.
func (i *Interpreter[C]) sendLocked(event Event) bool {
	if !i.started {
		return false
	}

	// Call OnEvent hooks (may modify event)
	event = i.callOnEvent(event)

	// Auto-forward to child actors (v4.0)
	i.broadcastToAutoForward(event)

	// Handle parallel states: broadcast event to all regions (v2.0)
	if i.currentParallel != "" {
		i.sendToParallelRegions(event)
		return true
	}

	// Get current state config
	currentState := i.machine.GetState(i.state.Value)
	if currentState == nil {
		return false
	}

	// Find matching transition, bubbling up through ancestors. A nil source
	// means either no transition matched the event or every candidate's guard
	// returned false — the event is not handled.
	source := i.findMatchingTransitionHierarchical(currentState, event)
	if source == nil {
		return false
	}

	// Execute the transition
	i.executeTransitionHierarchical(source, event)

	// Settle eventless transitions and drain any raised events
	i.macrostep()
	return true
}

// UpdateContext allows updating the context with a function
func (i *Interpreter[C]) UpdateContext(fn func(ctx *C)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	fn(&i.state.Context)
}

// findMatchingTransition finds the first transition that matches the event and passes guards
// wildcardEvent matches any event not handled by an exact transition (v1.x).
const wildcardEvent EventType = "*"

func (i *Interpreter[C]) findMatchingTransition(state *ir.StateConfig, event Event) *ir.TransitionConfig {
	// First pass: exact event matches take priority over the wildcard.
	if t := i.matchTransition(state, event, event.Type); t != nil {
		return t
	}
	// Second pass: a "*" wildcard transition catches any other event.
	// Eventless transitions (empty Type) must not be swallowed by "*".
	if event.Type != "" {
		return i.matchTransition(state, event, wildcardEvent)
	}
	return nil
}

// matchTransition returns the first transition for matchEvent whose guard
// passes (delayed transitions are excluded — those are timer-driven).
func (i *Interpreter[C]) matchTransition(state *ir.StateConfig, event Event, matchEvent EventType) *ir.TransitionConfig {
	for _, t := range state.Transitions {
		if t.Event != matchEvent || t.IsDelayed() {
			continue
		}
		if t.Guard != "" {
			guard := i.machine.GetGuard(t.Guard)
			if guard != nil && !guard(i.state.Context, event) {
				continue // Guard failed, try next transition
			}
		}
		return t
	}
	return nil
}

// findMatchingTransitionHierarchical finds a matching transition starting from the given state
// and bubbling up through ancestor states until a match is found
func (i *Interpreter[C]) findMatchingTransitionHierarchical(state *ir.StateConfig, event Event) *transitionSource[C] {
	// Start from the current (leaf) state
	current := state
	for current != nil {
		transition := i.findMatchingTransition(current, event)
		if transition != nil {
			return &transitionSource[C]{
				state:      current,
				transition: transition,
			}
		}

		// Bubble up to parent
		if current.Parent == "" {
			break
		}
		current = i.machine.GetState(current.Parent)
	}
	return nil
}

// executeTransitionHierarchical performs a hierarchical state transition
// Properly exits states up to LCA and enters states down to target
func (i *Interpreter[C]) executeTransitionHierarchical(source *transitionSource[C], event Event) {
	transition := source.transition

	// Internal transition (v1.x): run actions without exiting or re-entering
	// the source state — no exit/entry hooks, no state change, no history.
	if transition.Internal {
		currentLeaf := i.state.Value
		i.callBeforeTransition(currentLeaf, currentLeaf, event)
		i.executeActions(transition.Actions, event)
		i.enqueueRaised(transition)
		i.callAfterTransition(currentLeaf, currentLeaf, event)
		return
	}

	sourceStateID := source.state.ID
	targetStateID := transition.Target

	// Resolve target: handle history states or resolve to leaf state
	resolvedTarget := i.resolveTarget(targetStateID)

	// Get the current leaf state (what we're actually in)
	currentLeaf := i.state.Value

	// Find the Lowest Common Ancestor (LCA)
	// The LCA determines which states to exit and enter
	lca := i.machine.FindLCA(sourceStateID, resolvedTarget)

	// For self-transitions (or transitions within the same state hierarchy),
	// we need to exit and re-enter the source state. This is "external" transition behavior.
	// The LCA for external transitions should be the parent of the source state.
	isSelfTransition := sourceStateID == targetStateID

	var statesToExit []ir.StateID
	var statesToEnter []ir.StateID

	if isSelfTransition {
		// Self-transition: exit and re-enter the state (and any descendants)
		statesToExit = i.getStatesToExit(currentLeaf, source.state.Parent)
		statesToEnter = i.getStatesToEnter(resolvedTarget, source.state.Parent)
	} else {
		// Calculate states to exit: from current leaf up to (but not including) LCA
		statesToExit = i.getStatesToExit(currentLeaf, lca)

		// Calculate states to enter: from below LCA down to target
		statesToEnter = i.getStatesToEnter(resolvedTarget, lca)
	}

	// Call BeforeTransition hooks
	i.callBeforeTransition(currentLeaf, resolvedTarget, event)

	// 1. Execute exit actions (leaf to root order), cancel timers/services/actors, and record history
	for _, stateID := range statesToExit {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			// Cancel any active delayed transitions (v2.0)
			i.cancelDelayedTransitions(stateID)
			// Cancel any active invoked services (v3.0)
			i.cancelInvokedServices(stateID)
			// Stop invoked machines (v0.14)
			i.stopInvokedMachines(stateID)
			// Stop any spawned actors (v4.0)
			i.stopActorsForState(stateID)

			// Call OnExit hooks before exit actions
			i.callOnExit(stateID)

			i.executeActions(stateConfig.Exit, event)

			// Record history for parent compound states when exiting
			if stateConfig.Parent != "" {
				parent := i.machine.GetState(stateConfig.Parent)
				if parent != nil && parent.IsCompound() {
					// Record shallow history: immediate child that was active
					i.shallowHistory[parent.ID] = stateID
					// Record deep history: the current leaf state
					i.deepHistory[parent.ID] = currentLeaf
				}
			}
		}
	}

	// 2. Execute transition actions, then enqueue any raised internal events
	i.executeActions(transition.Actions, event)
	i.enqueueRaised(transition)

	// 3. Check if target is a parallel state (v2.0)
	targetConfig := i.machine.GetState(resolvedTarget)
	if targetConfig != nil && targetConfig.IsParallel() {
		// Enter the parallel state (handles all regions)
		i.enterParallelState(resolvedTarget, event)
		// Call AfterTransition hooks
		i.callAfterTransition(currentLeaf, resolvedTarget, event)
		return
	}

	// 4. Execute entry actions (root to leaf order), schedule delayed transitions, start services
	for _, stateID := range statesToEnter {
		// Set current state BEFORE entry actions so spawned actors are registered correctly
		i.state.Value = stateID
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			// Check if this is a parallel state within the entry path
			if stateConfig.IsParallel() {
				i.enterParallelState(stateID, event)
				// Call AfterTransition hooks
				i.callAfterTransition(currentLeaf, resolvedTarget, event)
				return
			}
			// Call OnEnter hooks before entry actions
			i.callOnEnter(stateID)

			i.executeActions(stateConfig.Entry, event)
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
			// Start invoked machines (v0.14)
			i.startInvokedMachines(stateID)
		}
	}

	// Call AfterTransition hooks
	i.callAfterTransition(currentLeaf, resolvedTarget, event)
}

// getStatesToExit returns states to exit in leaf-to-root order
// from currentLeaf up to (but not including) LCA
func (i *Interpreter[C]) getStatesToExit(currentLeaf, lca ir.StateID) []ir.StateID {
	var statesToExit []ir.StateID

	// Start from current leaf and go up
	current := currentLeaf
	for current != "" {
		// Stop when we reach the LCA (don't exit LCA)
		if current == lca {
			break
		}

		statesToExit = append(statesToExit, current)

		// Get parent
		state := i.machine.GetState(current)
		if state == nil {
			break
		}
		current = state.Parent
	}

	return statesToExit
}

// getStatesToEnter returns states to enter in root-to-leaf order
// from below LCA down to target (which should already be resolved to leaf)
func (i *Interpreter[C]) getStatesToEnter(target, lca ir.StateID) []ir.StateID {
	// Get the path from root to target
	path := i.machine.GetPath(target)

	// Find where LCA is in the path and start entering from after it
	var statesToEnter []ir.StateID
	foundLCA := lca == "" // If no LCA, enter all states

	for _, stateID := range path {
		if stateID == lca {
			foundLCA = true
			continue // Don't enter LCA itself
		}
		if foundLCA {
			statesToEnter = append(statesToEnter, stateID)
		}
	}

	return statesToEnter
}

// enterStateHierarchy enters a state and all its descendants to the initial leaf
func (i *Interpreter[C]) enterStateHierarchy(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	// Handle parallel states (v2.0)
	if stateConfig.IsParallel() {
		i.enterParallelState(stateID, Event{})
		return
	}

	// Get the path from this state to its initial leaf
	leaf := i.machine.GetInitialLeaf(stateID)
	path := i.getEntryPath(stateID, leaf)

	// Check if any state in the path is a parallel state
	for _, id := range path {
		sc := i.machine.GetState(id)
		if sc != nil && sc.IsParallel() {
			// Enter states up to the parallel state, then handle parallel
			prePath := i.getEntryPath(stateID, id)
			for _, preID := range prePath[:len(prePath)-1] {
				// Set current state BEFORE entry actions so spawned actors are registered correctly
				i.state.Value = preID
				preConfig := i.machine.GetState(preID)
				if preConfig != nil {
					// Call OnEnter hooks
					i.callOnEnter(preID)
					i.executeActions(preConfig.Entry, Event{})
					i.scheduleDelayedTransitions(preID)
				}
			}
			i.enterParallelState(id, Event{})
			return
		}
	}

	// Enter each state in root-to-leaf order
	for _, id := range path {
		// Set current state BEFORE entry actions so spawned actors are registered correctly
		i.state.Value = id
		stateConfig := i.machine.GetState(id)
		if stateConfig != nil {
			// Call OnEnter hooks
			i.callOnEnter(id)
			i.executeActions(stateConfig.Entry, Event{})
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(id)
			// Start invoked services (v3.0)
			i.startInvokedServices(id)
			// Start invoked machines (v0.14)
			i.startInvokedMachines(id)
		}
	}
}

// getEntryPath returns the states to enter from start to leaf (inclusive)
func (i *Interpreter[C]) getEntryPath(start, leaf ir.StateID) []ir.StateID {
	if start == leaf {
		return []ir.StateID{start}
	}

	// Get full path to leaf
	fullPath := i.machine.GetPath(leaf)

	// Find start in path and return from there
	var result []ir.StateID
	foundStart := false
	for _, id := range fullPath {
		if id == start {
			foundStart = true
		}
		if foundStart {
			result = append(result, id)
		}
	}

	return result
}

// executeActions executes a list of actions with plugin hooks
func (i *Interpreter[C]) executeActions(actions []ir.ActionType, event Event) {
	for _, actionName := range actions {
		action := i.machine.GetAction(actionName)
		if action != nil {
			// Call BeforeAction hooks
			i.callBeforeAction(actionName, event)

			// Execute action with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						err := fmt.Errorf("action %q panicked: %v", actionName, r)
						slog.Error("action panic recovered", "action", actionName, "panic", r)
						i.callOnError(err)
					}
				}()
				action(&i.state.Context, event)
			}()

			// Call AfterAction hooks
			i.callAfterAction(actionName, event)
		}
	}
}

// resolveTarget resolves the target state, handling history states, compound states, and parallel states
func (i *Interpreter[C]) resolveTarget(targetID ir.StateID) ir.StateID {
	targetState := i.machine.GetState(targetID)
	if targetState == nil {
		return targetID
	}

	// Handle history states
	if targetState.IsHistory() {
		return i.resolveHistoryTarget(targetState)
	}

	// For parallel states, return the parallel state itself (don't resolve to leaf)
	// The parallel state entry will handle entering all regions
	if targetState.IsParallel() {
		return targetID
	}

	// For compound states, resolve to initial leaf
	return i.machine.GetInitialLeaf(targetID)
}

// resolveHistoryTarget resolves a history state to the appropriate target
func (i *Interpreter[C]) resolveHistoryTarget(historyState *ir.StateConfig) ir.StateID {
	parentID := historyState.Parent
	if parentID == "" {
		// Fallback to default
		return i.machine.GetInitialLeaf(historyState.HistoryDefault)
	}

	// Check if we have recorded history for the parent
	var recordedHistory ir.StateID
	if historyState.HistoryType == ir.HistoryTypeDeep {
		recordedHistory = i.deepHistory[parentID]
	} else {
		recordedHistory = i.shallowHistory[parentID]
	}

	// If we have history, use it; otherwise use default
	if recordedHistory != "" {
		// For deep history, the recorded state is already the leaf
		if historyState.HistoryType == ir.HistoryTypeDeep {
			return recordedHistory
		}
		// For shallow history, we need to resolve to the initial leaf of the recorded child
		return i.machine.GetInitialLeaf(recordedHistory)
	}

	// No history recorded, use default
	return i.machine.GetInitialLeaf(historyState.HistoryDefault)
}

// Close implements io.Closer by calling Stop. Enables idiomatic
// `defer interp.Close()` cleanup.
func (i *Interpreter[C]) Close() error {
	i.Stop()
	return nil
}

// Stop cancels all active timers, services, invoked machines, and actors, then stops the interpreter
func (i *Interpreter[C]) Stop() {
	i.mu.Lock()

	// Call OnStop hooks before cleanup
	i.callOnStop()

	// Cancel all timers
	for key, timer := range i.timers {
		timer.Stop()
		delete(i.timers, key)
	}

	// Cancel all services (v3.0)
	for key, cancel := range i.activeServices {
		cancel()
		delete(i.activeServices, key)
	}

	// Stop all invoked machines (v0.14)
	for key, child := range i.activeInvokedMachines {
		child.Stop()
		delete(i.activeInvokedMachines, key)
	}

	i.started = false
	i.mu.Unlock()

	// Stop all actors (v4.0) under actorMu
	i.actorMu.Lock()
	for id, entry := range i.actorRegistry {
		entry.ref.Stop()
		delete(i.actorRegistry, id)
	}
	i.actorsByState = make(map[StateID][]ActorID)
	i.actorMu.Unlock()
}
