package statekit

import (
	"go.klarlabs.de/statekit/internal/ir"
)

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
			// Stop invoked machines (v0.14)
			i.stopInvokedMachines(stateID)
			// Stop spawned actors (v4.0)
			i.stopActorsForState(stateID)
			i.executeActions(stateConfig.Exit, event)
		}
	}

	// Execute transition actions
	i.executeActions(transition.Actions, event)

	// Execute entry actions
	for _, stateID := range statesToEnter {
		// Update the region's active state BEFORE entry actions
		i.state.ActiveInParallel[regionID] = stateID
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, event)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
			// Start invoked machines (v0.14)
			i.startInvokedMachines(stateID)
		}
	}
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
	// Start invoked machines (v0.14)
	i.startInvokedMachines(parallelID)

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
		// Track region state in ActiveInParallel BEFORE entry actions
		i.state.ActiveInParallel[regionID] = stateID
		stateConfig := i.machine.GetState(stateID)
		if stateConfig != nil {
			i.executeActions(stateConfig.Entry, event)
			i.scheduleDelayedTransitions(stateID)
			// Start invoked services (v3.0)
			i.startInvokedServices(stateID)
			// Start invoked machines (v0.14)
			i.startInvokedMachines(stateID)
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
	// Stop invoked machines (v0.14)
	i.stopInvokedMachines(i.currentParallel)
	// Stop spawned actors (v4.0)
	i.stopActorsForState(i.currentParallel)
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
			// Stop invoked machines (v0.14)
			i.stopInvokedMachines(stateID)
			// Stop spawned actors (v4.0)
			i.stopActorsForState(stateID)
			i.executeActions(stateConfig.Exit, event)
		}
	}
}
