package statekit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// Interpreter is the statechart runtime that processes events and manages state
type Interpreter[C any] struct {
	machine *ir.MachineConfig[C]
	state   State[C]
	started bool

	// Mutex to protect all interpreter state from concurrent access (e.g., timer goroutines)
	mu sync.Mutex

	// History tracking (v2.0)
	// Maps compound state ID to the last immediate child that was active (shallow)
	shallowHistory map[ir.StateID]ir.StateID
	// Maps compound state ID to the last leaf state that was active (deep)
	deepHistory map[ir.StateID]ir.StateID

	// Timer management for delayed transitions (v2.0)
	// Maps timer key (stateID:index) to active timer
	// Protected by mu (single mutex to prevent deadlocks)
	timers map[string]*time.Timer

	// Parallel state tracking (v2.0)
	// When inside a parallel state, this holds the parallel state ID
	// The actual region states are tracked in state.ActiveInParallel
	currentParallel ir.StateID

	// Invoked services tracking (v3.0)
	// Maps invocation key (stateID:invokeID) to cancel function
	activeServices map[string]context.CancelFunc
}

// transitionSource holds the state that owns the transition and the transition itself
type transitionSource[C any] struct {
	state      *ir.StateConfig
	transition *ir.TransitionConfig
}

// NewInterpreter creates a new interpreter for the given machine configuration
func NewInterpreter[C any](machine *ir.MachineConfig[C]) *Interpreter[C] {
	return &Interpreter[C]{
		machine: machine,
		state: State[C]{
			Context:          machine.Context,
			ActiveInParallel: make(map[ir.StateID]ir.StateID),
		},
		shallowHistory: make(map[ir.StateID]ir.StateID),
		deepHistory:    make(map[ir.StateID]ir.StateID),
		timers:         make(map[string]*time.Timer),
		activeServices: make(map[string]context.CancelFunc),
	}
}

// Start initializes the interpreter and enters the initial state
func (i *Interpreter[C]) Start() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.started {
		return
	}
	i.started = true

	// Enter initial state, resolving to deepest leaf
	i.enterStateHierarchy(i.machine.Initial)
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

// Send processes an event and potentially transitions to a new state
func (i *Interpreter[C]) Send(event Event) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.started {
		return
	}

	// Handle parallel states: broadcast event to all regions (v2.0)
	if i.currentParallel != "" {
		i.sendToParallelRegions(event)
		return
	}

	// Get current state config
	currentState := i.machine.GetState(i.state.Value)
	if currentState == nil {
		return
	}

	// Find matching transition, bubbling up through ancestors
	source := i.findMatchingTransitionHierarchical(currentState, event)
	if source == nil {
		return // No matching transition in hierarchy
	}

	// Execute the transition
	i.executeTransitionHierarchical(source, event)
}

// UpdateContext allows updating the context with a function
func (i *Interpreter[C]) UpdateContext(fn func(ctx *C)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	fn(&i.state.Context)
}

// findMatchingTransition finds the first transition that matches the event and passes guards
func (i *Interpreter[C]) findMatchingTransition(state *ir.StateConfig, event Event) *ir.TransitionConfig {
	for _, t := range state.Transitions {
		if t.Event != event.Type {
			continue
		}

		// Check guard if present
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

	// 1. Execute exit actions (leaf to root order), cancel timers/services, and record history
	for _, stateID := range statesToExit {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			// Cancel any active delayed transitions (v2.0)
			i.cancelDelayedTransitions(stateID)
			// Cancel any active invoked services (v3.0)
			i.cancelInvokedServices(stateID)

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

	// 2. Execute transition actions
	i.executeActions(transition.Actions, event)

	// 3. Check if target is a parallel state (v2.0)
	targetConfig := i.machine.GetState(resolvedTarget)
	if targetConfig != nil && targetConfig.IsParallel() {
		// Enter the parallel state (handles all regions)
		i.enterParallelState(resolvedTarget, event)
		return
	}

	// 4. Execute entry actions (root to leaf order), schedule delayed transitions, start services
	for _, stateID := range statesToEnter {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			// Check if this is a parallel state within the entry path
			if stateConfig.IsParallel() {
				i.enterParallelState(stateID, event)
				return
			}
			i.executeActions(stateConfig.Entry, event)
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
		}
	}

	// 5. Update current state to the leaf
	i.state.Value = resolvedTarget
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
				preConfig := i.machine.GetState(preID)
				if preConfig != nil {
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
		stateConfig := i.machine.GetState(id)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, Event{})
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(id)
			// Start invoked services (v3.0)
			i.startInvokedServices(id)
		}
	}

	// Set current state to the leaf
	i.state.Value = leaf
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

// executeActions executes a list of actions
func (i *Interpreter[C]) executeActions(actions []ir.ActionType, event Event) {
	for _, actionName := range actions {
		action := i.machine.GetAction(actionName)
		if action != nil {
			action(&i.state.Context, event)
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

// --- Timer management for delayed transitions (v2.0) ---

// Stop cancels all active timers and services, and stops the interpreter
func (i *Interpreter[C]) Stop() {
	i.mu.Lock()
	defer i.mu.Unlock()

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

	i.started = false
}

// scheduleDelayedTransitions schedules timers for all delayed transitions in the given state
// Caller must hold mu.
func (i *Interpreter[C]) scheduleDelayedTransitions(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	for idx, trans := range stateConfig.Transitions {
		if !trans.IsDelayed() {
			continue
		}

		// Create timer key: stateID:transitionIndex
		timerKey := fmt.Sprintf("%s:%d", stateID, idx)

		// Capture transition for closure
		capturedTrans := trans

		timer := time.AfterFunc(trans.Delay, func() {
			i.mu.Lock()
			defer i.mu.Unlock()

			// Remove timer from map before executing
			delete(i.timers, timerKey)

			// Execute the delayed transition if still in the originating state
			if i.started && i.matchesUnlocked(stateID) {
				i.executeDelayedTransition(stateConfig, capturedTrans)
			}
		})
		i.timers[timerKey] = timer
	}
}

// cancelDelayedTransitions cancels all timers for the given state
// Caller must hold mu.
func (i *Interpreter[C]) cancelDelayedTransitions(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	for idx := range stateConfig.Transitions {
		timerKey := fmt.Sprintf("%s:%d", stateID, idx)
		if timer, ok := i.timers[timerKey]; ok {
			timer.Stop()
			delete(i.timers, timerKey)
		}
	}
}

// executeDelayedTransition executes a delayed transition
func (i *Interpreter[C]) executeDelayedTransition(sourceState *ir.StateConfig, trans *ir.TransitionConfig) {
	// Check guard if present
	if trans.Guard != "" {
		guard := i.machine.GetGuard(trans.Guard)
		if guard != nil && !guard(i.state.Context, Event{}) {
			return // Guard failed, don't execute
		}
	}

	source := &transitionSource[C]{
		state:      sourceState,
		transition: trans,
	}
	i.executeTransitionHierarchical(source, Event{})
}

// --- Parallel state management (v2.0) ---

// sendToParallelRegions broadcasts an event to all active parallel regions
func (i *Interpreter[C]) sendToParallelRegions(event Event) {
	parallelState := i.machine.GetState(i.currentParallel)
	if parallelState == nil {
		return
	}

	// Try to find a transition on the parallel state itself first (exits parallel)
	source := i.findMatchingTransition(parallelState, event)
	if source != nil {
		// Transition exits the parallel state entirely
		i.exitParallelState(event)
		transSource := &transitionSource[C]{
			state:      parallelState,
			transition: source,
		}
		i.executeTransitionHierarchical(transSource, event)
		return
	}

	// Broadcast event to each region independently
	for regionID, leafID := range i.state.ActiveInParallel {
		regionState := i.machine.GetState(leafID)
		if regionState == nil {
			continue
		}

		// Find matching transition in this region's hierarchy
		transSource := i.findMatchingTransitionInRegion(regionState, regionID, event)
		if transSource != nil {
			// Execute transition within the region
			i.executeTransitionInRegion(regionID, transSource, event)
		}
	}
}

// findMatchingTransitionInRegion finds a transition bubbling up within a region
func (i *Interpreter[C]) findMatchingTransitionInRegion(state *ir.StateConfig, regionID ir.StateID, event Event) *transitionSource[C] {
	current := state
	for current != nil {
		transition := i.findMatchingTransition(current, event)
		if transition != nil {
			return &transitionSource[C]{
				state:      current,
				transition: transition,
			}
		}

		// Stop at region boundary (don't bubble to parallel state)
		if current.ID == regionID {
			break
		}
		if current.Parent == "" {
			break
		}
		current = i.machine.GetState(current.Parent)
	}
	return nil
}

// executeTransitionInRegion executes a transition within a parallel region
func (i *Interpreter[C]) executeTransitionInRegion(regionID ir.StateID, source *transitionSource[C], event Event) {
	transition := source.transition
	sourceStateID := source.state.ID
	targetStateID := transition.Target

	// Resolve target to leaf
	resolvedTarget := i.resolveTarget(targetStateID)

	// Get current leaf in this region
	currentLeaf := i.state.ActiveInParallel[regionID]

	// Find LCA within the region
	lca := i.machine.FindLCA(sourceStateID, resolvedTarget)

	// Ensure we don't exit beyond the region
	if !i.machine.IsDescendantOf(lca, regionID) && lca != regionID {
		lca = regionID
	}

	isSelfTransition := sourceStateID == targetStateID

	var statesToExit []ir.StateID
	var statesToEnter []ir.StateID

	if isSelfTransition {
		statesToExit = i.getStatesToExit(currentLeaf, source.state.Parent)
		statesToEnter = i.getStatesToEnter(resolvedTarget, source.state.Parent)
	} else {
		statesToExit = i.getStatesToExit(currentLeaf, lca)
		statesToEnter = i.getStatesToEnter(resolvedTarget, lca)
	}

	// Execute exit actions
	for _, stateID := range statesToExit {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.cancelDelayedTransitions(stateID)
			// Cancel invoked services (v3.0)
			i.cancelInvokedServices(stateID)
			i.executeActions(stateConfig.Exit, event)
		}
	}

	// Execute transition actions
	i.executeActions(transition.Actions, event)

	// Execute entry actions
	for _, stateID := range statesToEnter {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, event)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
		}
	}

	// Update the region's active state
	i.state.ActiveInParallel[regionID] = resolvedTarget
}

// enterParallelState enters a parallel state and all its regions
func (i *Interpreter[C]) enterParallelState(parallelID ir.StateID, event Event) {
	parallelState := i.machine.GetState(parallelID)
	if parallelState == nil || !parallelState.IsParallel() {
		return
	}

	// Set current parallel state
	i.currentParallel = parallelID
	i.state.Value = parallelID

	// Execute entry actions for parallel state
	i.executeActions(parallelState.Entry, event)
	i.scheduleDelayedTransitions(parallelID)
	// Start invoked services (v3.0)
	i.startInvokedServices(parallelID)

	// Enter each region (child of parallel state)
	for _, regionID := range parallelState.Children {
		i.enterRegion(regionID, event)
	}
}

// enterRegion enters a single parallel region
func (i *Interpreter[C]) enterRegion(regionID ir.StateID, event Event) {
	regionState := i.machine.GetState(regionID)
	if regionState == nil {
		return
	}

	// Get the initial leaf for this region
	var leafID ir.StateID
	if regionState.IsCompound() {
		leafID = i.machine.GetInitialLeaf(regionID)
	} else {
		leafID = regionID
	}

	// Get path from region to leaf
	path := i.getEntryPath(regionID, leafID)

	// Enter each state in the path
	for _, stateID := range path {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, event)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
		}
	}

	// Track the leaf state for this region
	i.state.ActiveInParallel[regionID] = leafID
}

// exitParallelState exits a parallel state and all its regions
func (i *Interpreter[C]) exitParallelState(event Event) {
	if i.currentParallel == "" {
		return
	}

	parallelState := i.machine.GetState(i.currentParallel)
	if parallelState == nil {
		return
	}

	// Exit each region
	for regionID, leafID := range i.state.ActiveInParallel {
		i.exitRegion(regionID, leafID, event)
	}

	// Execute exit actions for parallel state
	i.cancelDelayedTransitions(i.currentParallel)
	// Cancel invoked services (v3.0)
	i.cancelInvokedServices(i.currentParallel)
	i.executeActions(parallelState.Exit, event)

	// Clear parallel state tracking
	i.currentParallel = ""
	i.state.ActiveInParallel = make(map[ir.StateID]ir.StateID)
}

// exitRegion exits all states in a region from leaf up to region boundary
func (i *Interpreter[C]) exitRegion(regionID, leafID ir.StateID, event Event) {
	// Get states to exit (leaf up to and including region)
	statesToExit := i.getStatesToExit(leafID, "")

	// Filter to only include states within or equal to the region
	var filtered []ir.StateID
	for _, stateID := range statesToExit {
		if stateID == regionID || i.machine.IsDescendantOf(stateID, regionID) {
			filtered = append(filtered, stateID)
		}
	}

	// Execute exit actions
	for _, stateID := range filtered {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.cancelDelayedTransitions(stateID)
			// Cancel invoked services (v3.0)
			i.cancelInvokedServices(stateID)
			i.executeActions(stateConfig.Exit, event)
		}
	}
}

// --- Invoked services management (v3.0) ---

// startInvokedServices starts all services for the given state
// Caller must hold mu.
func (i *Interpreter[C]) startInvokedServices(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil || len(stateConfig.Invocations) == 0 {
		return
	}

	for _, invoke := range stateConfig.Invocations {
		service := i.machine.GetService(invoke.Src)
		if service == nil {
			continue
		}

		// Create cancellation context for this invocation
		ctx, cancel := context.WithCancel(context.Background())
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)
		i.activeServices[invokeKey] = cancel

		// Capture invoke config for closure
		capturedInvoke := invoke
		capturedStateID := stateID

		// Start service in goroutine
		go func() {
			// Create service context
			svcCtx := ir.ServiceContext[C]{
				Context:        ctx,
				MachineContext: i.state.Context,
				Send: func(event Event) {
					i.Send(event)
				},
			}

			// Run the service
			err := service(svcCtx)

			// Handle completion (back on the interpreter goroutine)
			i.mu.Lock()
			defer i.mu.Unlock()

			// Check if we're still in the same state and service wasn't cancelled
			if !i.started || !i.matchesUnlocked(capturedStateID) {
				return
			}

			// Clean up
			delete(i.activeServices, invokeKey)

			// Handle result
			if err != nil {
				// Service failed
				if capturedInvoke.OnError != nil {
					i.executeServiceTransition(capturedStateID, capturedInvoke.OnError, Event{
						Type:    "error",
						Payload: err,
					})
				}
			} else {
				// Service succeeded
				if capturedInvoke.OnDone != nil {
					i.executeServiceTransition(capturedStateID, capturedInvoke.OnDone, Event{
						Type: "done",
					})
				}
			}
		}()
	}
}

// cancelInvokedServices cancels all services for the given state
// Caller must hold mu.
func (i *Interpreter[C]) cancelInvokedServices(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	for _, invoke := range stateConfig.Invocations {
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)
		if cancel, ok := i.activeServices[invokeKey]; ok {
			cancel()
			delete(i.activeServices, invokeKey)
		}
	}
}

// executeServiceTransition executes an OnDone or OnError transition
// Caller must hold mu.
func (i *Interpreter[C]) executeServiceTransition(sourceStateID ir.StateID, trans *ir.TransitionConfig, event Event) {
	if trans.Target == "" {
		// No target, just execute actions
		i.executeActions(trans.Actions, event)
		return
	}

	sourceState := i.machine.GetState(sourceStateID)
	if sourceState == nil {
		return
	}

	source := &transitionSource[C]{
		state:      sourceState,
		transition: trans,
	}
	i.executeTransitionHierarchical(source, event)
}
