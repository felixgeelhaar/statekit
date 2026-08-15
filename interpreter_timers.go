package statekit

import (
	"fmt"

	"go.klarlabs.de/statekit/internal/ir"
)

// Delayed transition timer helpers (v2.0).

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

		timer := i.clock.AfterFunc(trans.Delay, func() {
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
