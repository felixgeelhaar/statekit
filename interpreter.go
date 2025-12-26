package statekit

import (
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

	// History tracking (v2.0)
	// Maps compound state ID to the last immediate child that was active (shallow)
	shallowHistory map[ir.StateID]ir.StateID
	// Maps compound state ID to the last leaf state that was active (deep)
	deepHistory map[ir.StateID]ir.StateID

	// Timer management for delayed transitions (v2.0)
	// Maps timer key (stateID:index) to active timer
	timers   map[string]*time.Timer
	timersMu sync.Mutex
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
			Value:   "",
			Context: machine.Context,
		},
		started:        false,
		shallowHistory: make(map[ir.StateID]ir.StateID),
		deepHistory:    make(map[ir.StateID]ir.StateID),
		timers:         make(map[string]*time.Timer),
	}
}

// Start initializes the interpreter and enters the initial state
func (i *Interpreter[C]) Start() {
	if i.started {
		return
	}
	i.started = true

	// Enter initial state, resolving to deepest leaf
	i.enterStateHierarchy(i.machine.Initial)
}

// State returns the current state of the interpreter
func (i *Interpreter[C]) State() State[C] {
	return i.state
}

// Matches checks if the current state matches the given state ID
// For hierarchical states, returns true if current state equals id or is a descendant of id
func (i *Interpreter[C]) Matches(id StateID) bool {
	if i.state.Value == id {
		return true
	}
	// Check if current state is a descendant of the given state
	return i.machine.IsDescendantOf(i.state.Value, id)
}

// Done returns true if the machine is in a final state
func (i *Interpreter[C]) Done() bool {
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
	if !i.started {
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

	// 1. Execute exit actions (leaf to root order), cancel timers, and record history
	for _, stateID := range statesToExit {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			// Cancel any active delayed transitions (v2.0)
			i.cancelDelayedTransitions(stateID)

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

	// 3. Execute entry actions (root to leaf order) and schedule delayed transitions
	for _, stateID := range statesToEnter {
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, event)
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(stateID)
		}
	}

	// 4. Update current state to the leaf
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
	// Get the path from this state to its initial leaf
	leaf := i.machine.GetInitialLeaf(stateID)
	path := i.getEntryPath(stateID, leaf)

	// Enter each state in root-to-leaf order
	for _, id := range path {
		stateConfig := i.machine.GetState(id)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, Event{})
			// Schedule delayed transitions (v2.0)
			i.scheduleDelayedTransitions(id)
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

// resolveTarget resolves the target state, handling history states and compound states
func (i *Interpreter[C]) resolveTarget(targetID ir.StateID) ir.StateID {
	targetState := i.machine.GetState(targetID)
	if targetState == nil {
		return targetID
	}

	// Handle history states
	if targetState.IsHistory() {
		return i.resolveHistoryTarget(targetState)
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

// Stop cancels all active timers and stops the interpreter
func (i *Interpreter[C]) Stop() {
	i.timersMu.Lock()
	defer i.timersMu.Unlock()

	for key, timer := range i.timers {
		timer.Stop()
		delete(i.timers, key)
	}
	i.started = false
}

// scheduleDelayedTransitions schedules timers for all delayed transitions in the given state
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

		i.timersMu.Lock()
		timer := time.AfterFunc(trans.Delay, func() {
			i.timersMu.Lock()
			// Remove timer from map before executing
			delete(i.timers, timerKey)
			i.timersMu.Unlock()

			// Execute the delayed transition if still in the originating state
			if i.started && i.Matches(stateID) {
				i.executeDelayedTransition(stateConfig, capturedTrans)
			}
		})
		i.timers[timerKey] = timer
		i.timersMu.Unlock()
	}
}

// cancelDelayedTransitions cancels all timers for the given state
func (i *Interpreter[C]) cancelDelayedTransitions(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	i.timersMu.Lock()
	defer i.timersMu.Unlock()

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
