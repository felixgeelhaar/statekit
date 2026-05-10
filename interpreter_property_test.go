package statekit

import (
	"math/rand"
	"testing"
	"testing/quick"
)

// PropertyContext tracks state machine behavior for property verification.
type PropertyContext struct {
	TransitionCount int
	EntryCount      int
	ExitCount       int
	ActionLog       []string
}

// TestProperty_StateAlwaysValid verifies that after any sequence of events,
// the interpreter is always in a valid, declared state.
func TestProperty_StateAlwaysValid(t *testing.T) {
	t.Parallel()
	machine := buildPropertyMachine(t)
	validStates := map[StateID]bool{
		"idle":       true,
		"loading":    true,
		"processing": true,
		"success":    true,
		"error":      true,
	}

	f := func(eventIndices []uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		events := []Event{
			{Type: "LOAD"},
			{Type: "PROCESS"},
			{Type: "SUCCESS"},
			{Type: "ERROR"},
			{Type: "RESET"},
			{Type: "RETRY"},
			{Type: "UNKNOWN"}, // Invalid event
		}

		for _, idx := range eventIndices {
			eventIdx := int(idx) % len(events)
			interp.Send(events[eventIdx])

			// Property: state must always be valid
			state := interp.State()
			if !validStates[state.Value] {
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

// TestProperty_ContextNeverNil verifies that context is never nil after any operation.
func TestProperty_ContextNeverNil(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[*PropertyContext]("context_test").
		WithInitial("idle").
		WithContext(&PropertyContext{}).
		WithAction("track", func(ctx **PropertyContext, e Event) {
			(*ctx).ActionLog = append((*ctx).ActionLog, string(e.Type))
		}).
		State("idle").
		OnEntry("track").
		On("GO").Target("active").
		Done().
		State("active").
		OnEntry("track").
		On("BACK").Target("idle").
		Done().
		Build()

	f := func(toggles []bool) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		for _, toggle := range toggles {
			if toggle {
				interp.Send(Event{Type: "GO"})
			} else {
				interp.Send(Event{Type: "BACK"})
			}

			// Property: context should never be nil
			state := interp.State()
			if state.Context == nil {
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// TestProperty_FinalStateIsTerminal verifies that once in a final state,
// no transitions occur.
func TestProperty_FinalStateIsTerminal(t *testing.T) {
	t.Parallel()
	type Ctx struct {
		TransitionCount int
	}

	machine, _ := NewMachine[Ctx]("final_test").
		WithInitial("running").
		WithAction("count", func(ctx *Ctx, e Event) {
			ctx.TransitionCount++
		}).
		State("running").
		On("FINISH").Target("done").Do("count").
		On("FAIL").Target("failed").Do("count").
		Done().
		State("done").Final().Done().
		State("failed").Final().Done().
		Build()

	f := func(finishFirst bool, extraEvents []uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		// First, reach a final state
		if finishFirst {
			interp.Send(Event{Type: "FINISH"})
		} else {
			interp.Send(Event{Type: "FAIL"})
		}

		if !interp.Done() {
			return false // Should be in final state
		}

		countBefore := interp.State().Context.TransitionCount
		stateBefore := interp.State().Value

		// Send many more events
		events := []EventType{"FINISH", "FAIL", "RESET", "UNKNOWN"}
		for _, idx := range extraEvents {
			eventType := events[int(idx)%len(events)]
			interp.Send(Event{Type: eventType})
		}

		// Properties after final state:
		// 1. State should not change
		// 2. Transition count should not increase
		// 3. Should still be done
		stateAfter := interp.State().Value
		countAfter := interp.State().Context.TransitionCount

		return stateAfter == stateBefore &&
			countAfter == countBefore &&
			interp.Done()
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// TestProperty_GuardsPreventTransitions verifies that guards properly block transitions.
func TestProperty_GuardsPreventTransitions(t *testing.T) {
	t.Parallel()
	type Ctx struct {
		Threshold int
		Value     int
	}

	machine, _ := NewMachine[Ctx]("guard_test").
		WithInitial("locked").
		WithContext(Ctx{Threshold: 5, Value: 0}).
		WithGuard("canUnlock", func(ctx Ctx, e Event) bool {
			return ctx.Value >= ctx.Threshold
		}).
		WithAction("increment", func(ctx *Ctx, e Event) {
			ctx.Value++
		}).
		State("locked").
		On("TRY_UNLOCK").Target("unlocked").Guard("canUnlock").
		On("INCREMENT").Target("locked").Do("increment").
		Done().
		State("unlocked").
		On("LOCK").Target("locked").
		Done().
		Build()

	f := func(incrementCount uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		// Increment some times
		for i := uint8(0); i < incrementCount%20; i++ {
			interp.Send(Event{Type: "INCREMENT"})
		}

		// Try to unlock
		ctx := interp.State().Context
		interp.Send(Event{Type: "TRY_UNLOCK"})

		// Property: should only unlock if value >= threshold
		if ctx.Value >= ctx.Threshold {
			return interp.State().Value == "unlocked"
		}
		return interp.State().Value == "locked"
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestProperty_MatchesReflectsCurrentState verifies Matches() consistency.
func TestProperty_MatchesReflectsCurrentState(t *testing.T) {
	t.Parallel()
	machine := buildPropertyMachine(t)
	states := []StateID{"idle", "loading", "processing", "success", "error"}

	f := func(eventIndices []uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		events := []Event{
			{Type: "LOAD"},
			{Type: "PROCESS"},
			{Type: "SUCCESS"},
			{Type: "ERROR"},
			{Type: "RESET"},
			{Type: "RETRY"},
		}

		for _, idx := range eventIndices {
			eventIdx := int(idx) % len(events)
			interp.Send(events[eventIdx])
		}

		currentState := interp.State().Value

		// Property: Matches(currentState) must be true
		if !interp.Matches(currentState) {
			return false
		}

		// Property: Matches(otherState) must be false for non-ancestors
		for _, state := range states {
			if state != currentState {
				// For flat machines, non-current states should not match
				if interp.Matches(state) {
					return false
				}
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// TestProperty_HierarchyMatchesAncestors verifies ancestor matching in nested states.
func TestProperty_HierarchyMatchesAncestors(t *testing.T) {
	t.Parallel()
	type Ctx struct{}

	// Build a simple nested machine using the exact same pattern as pedestrian_light example
	machine, err := NewMachine[Ctx]("hierarchy_test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		On("SHUTDOWN").Target("stopped").End().
		State("idle").
		On("START").Target("working").
		End().
		End().
		State("working").
		On("STOP").Target("idle").
		End().
		End().
		Done().
		State("stopped").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build hierarchy machine: %v", err)
	}

	f := func(eventSequence []uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		events := []Event{
			{Type: "START"},
			{Type: "STOP"},
			{Type: "SHUTDOWN"},
		}

		for _, idx := range eventSequence[:min(len(eventSequence), 10)] {
			interp.Send(events[int(idx)%len(events)])
		}

		currentState := interp.State().Value

		// Property: current state always matches itself
		if !interp.Matches(currentState) {
			return false
		}

		// Property: if in a nested state under "active", should match "active"
		if currentState == "idle" || currentState == "working" {
			if !interp.Matches("active") {
				return false
			}
		}

		// Property: if in "stopped", should not match "active"
		if currentState == "stopped" {
			if interp.Matches("active") {
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// TestProperty_SnapshotRestorePreservesState verifies snapshot/restore consistency.
func TestProperty_SnapshotRestorePreservesState(t *testing.T) {
	t.Parallel()
	type Ctx struct {
		Counter int
	}

	machine, _ := NewMachine[Ctx]("snapshot_test").
		WithInitial("a").
		WithAction("inc", func(ctx *Ctx, e Event) {
			ctx.Counter++
		}).
		State("a").On("NEXT").Target("b").Do("inc").Done().
		State("b").On("NEXT").Target("c").Do("inc").Done().
		State("c").On("NEXT").Target("a").Do("inc").Done().
		Build()

	f := func(transitionCount uint8) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		// Apply some transitions
		for i := uint8(0); i < transitionCount%30; i++ {
			interp.Send(Event{Type: "NEXT"})
		}

		// Take snapshot
		snapshot := interp.Snapshot()
		originalState := interp.State().Value
		originalCounter := interp.State().Context.Counter

		// Create new interpreter and restore
		interp2 := NewInterpreter(machine)
		if err := interp2.Restore(snapshot); err != nil {
			return false
		}

		restoredState := interp2.State().Value
		restoredCounter := interp2.State().Context.Counter

		// Property: restored state equals original
		return originalState == restoredState && originalCounter == restoredCounter
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestProperty_UpdateContextAppliesChanges verifies context updates are consistent.
func TestProperty_UpdateContextAppliesChanges(t *testing.T) {
	t.Parallel()
	type Ctx struct {
		Values []int
	}

	machine, _ := NewMachine[Ctx]("update_test").
		WithInitial("active").
		WithContext(Ctx{Values: make([]int, 0)}).
		State("active").Done().
		Build()

	f := func(updates []int) bool {
		interp := NewInterpreter(machine)
		interp.Start()

		// Apply updates
		for _, val := range updates[:min(len(updates), 50)] {
			interp.UpdateContext(func(ctx *Ctx) {
				ctx.Values = append(ctx.Values, val)
			})
		}

		// Property: all updates should be reflected
		actualValues := interp.State().Context.Values
		expectedLen := min(len(updates), 50)

		if len(actualValues) != expectedLen {
			return false
		}

		for i := 0; i < expectedLen; i++ {
			if actualValues[i] != updates[i] {
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// TestProperty_EventOrderMatters verifies that event order produces deterministic results.
func TestProperty_EventOrderMatters(t *testing.T) {
	t.Parallel()
	type Ctx struct {
		Path []string
	}

	machine, _ := NewMachine[Ctx]("order_test").
		WithInitial("start").
		WithContext(Ctx{Path: make([]string, 0)}).
		WithAction("recordA", func(ctx *Ctx, e Event) { ctx.Path = append(ctx.Path, "A") }).
		WithAction("recordB", func(ctx *Ctx, e Event) { ctx.Path = append(ctx.Path, "B") }).
		State("start").
		On("A").Target("stateA").Do("recordA").
		On("B").Target("stateB").Do("recordB").
		Done().
		State("stateA").
		On("B").Target("stateAB").Do("recordB").
		Done().
		State("stateB").
		On("A").Target("stateBA").Do("recordA").
		Done().
		State("stateAB").Done().
		State("stateBA").Done().
		Build()

	// Property: same events in same order produce same result
	f := func(seed int64) bool {
		rng1 := rand.New(rand.NewSource(seed))
		rng2 := rand.New(rand.NewSource(seed))

		events1 := generateEventSequence(rng1, 5)
		events2 := generateEventSequence(rng2, 5)

		interp1 := NewInterpreter(machine)
		interp1.Start()
		for _, e := range events1 {
			interp1.Send(e)
		}

		interp2 := NewInterpreter(machine)
		interp2.Start()
		for _, e := range events2 {
			interp2.Send(e)
		}

		// Property: identical sequences produce identical states
		return interp1.State().Value == interp2.State().Value
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// Helper functions

func buildPropertyMachine(t *testing.T) *MachineConfig[PropertyContext] {
	t.Helper()

	machine, err := NewMachine[PropertyContext]("property_test").
		WithInitial("idle").
		WithAction("trackTransition", func(ctx *PropertyContext, e Event) {
			ctx.TransitionCount++
		}).
		State("idle").
		On("LOAD").Target("loading").Do("trackTransition").
		Done().
		State("loading").
		On("PROCESS").Target("processing").Do("trackTransition").
		On("ERROR").Target("error").Do("trackTransition").
		Done().
		State("processing").
		On("SUCCESS").Target("success").Do("trackTransition").
		On("ERROR").Target("error").Do("trackTransition").
		Done().
		State("success").
		On("RESET").Target("idle").Do("trackTransition").
		Done().
		State("error").
		On("RETRY").Target("loading").Do("trackTransition").
		On("RESET").Target("idle").Do("trackTransition").
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build property machine: %v", err)
	}

	return machine
}

func generateEventSequence(rng *rand.Rand, count int) []Event {
	events := []EventType{"A", "B"}
	result := make([]Event, count)
	for i := 0; i < count; i++ {
		result[i] = Event{Type: events[rng.Intn(len(events))]}
	}
	return result
}
