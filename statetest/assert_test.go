package statetest_test

import (
	"bytes"
	"testing"

	"github.com/felixgeelhaar/statekit"
	statetesting "github.com/felixgeelhaar/statekit/statetest"
)

// mockT is a mock testing.TB for testing assertions
type mockT struct {
	testing.TB
	failed   bool
	fataled  bool
	messages []string
}

func newMockT() *mockT {
	return &mockT{messages: make([]string, 0)}
}

func (m *mockT) Helper() {}

func (m *mockT) Errorf(format string, args ...any) {
	m.failed = true
	m.messages = append(m.messages, format)
}

func (m *mockT) Fatalf(format string, args ...any) {
	m.fataled = true
	m.messages = append(m.messages, format)
}

func (m *mockT) Error(args ...any) {
	m.failed = true
}

func TestAssertState(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should pass
	mt := newMockT()
	statetesting.AssertState(mt, interp, "idle")
	if mt.failed {
		t.Error("AssertState should not fail for correct state")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertState(mt, interp, "running")
	if !mt.failed {
		t.Error("AssertState should fail for incorrect state")
	}
}

func TestAssertMatches(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should pass
	mt := newMockT()
	statetesting.AssertMatches(mt, interp, "idle")
	if mt.failed {
		t.Error("AssertMatches should not fail for matching state")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertMatches(mt, interp, "running")
	if !mt.failed {
		t.Error("AssertMatches should fail for non-matching state")
	}
}

func TestAssertDone(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should fail (not done)
	mt := newMockT()
	statetesting.AssertDone(mt, interp)
	if !mt.failed {
		t.Error("AssertDone should fail when not done")
	}

	// Go to final state
	interp.Send(statekit.Event{Type: "START"})
	interp.Send(statekit.Event{Type: "STOP"})

	// Should pass
	mt = newMockT()
	statetesting.AssertDone(mt, interp)
	if mt.failed {
		t.Error("AssertDone should not fail when done")
	}
}

func TestAssertNotDone(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should pass (not done)
	mt := newMockT()
	statetesting.AssertNotDone(mt, interp)
	if mt.failed {
		t.Error("AssertNotDone should not fail when not done")
	}

	// Go to final state
	interp.Send(statekit.Event{Type: "START"})
	interp.Send(statekit.Event{Type: "STOP"})

	// Should fail
	mt = newMockT()
	statetesting.AssertNotDone(mt, interp)
	if !mt.failed {
		t.Error("AssertNotDone should fail when done")
	}
}

func TestAssertTransitioned(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})

	// Should pass
	mt := newMockT()
	statetesting.AssertTransitioned(mt, rec, "idle", "running")
	if mt.failed {
		t.Error("AssertTransitioned should not fail for actual transition")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertTransitioned(mt, rec, "idle", "done")
	if !mt.failed {
		t.Error("AssertTransitioned should fail for non-existent transition")
	}
}

func TestAssertNoTransition(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "INVALID"})

	// Should pass (no transition happened)
	mt := newMockT()
	statetesting.AssertNoTransition(mt, rec, "INVALID")
	if mt.failed {
		t.Error("AssertNoTransition should not fail when no transition occurred")
	}

	rec.Send(statekit.Event{Type: "START"})

	// Should fail (transition happened)
	mt = newMockT()
	statetesting.AssertNoTransition(mt, rec, "START")
	if !mt.failed {
		t.Error("AssertNoTransition should fail when transition occurred")
	}
}

func TestAssertEventSequence(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "PAUSE"})

	// Should pass
	mt := newMockT()
	statetesting.AssertEventSequence(mt, rec, "START", "PAUSE")
	if mt.failed {
		t.Error("AssertEventSequence should not fail for correct sequence")
	}

	// Should fail (wrong order)
	mt = newMockT()
	statetesting.AssertEventSequence(mt, rec, "PAUSE", "START")
	if !mt.failed {
		t.Error("AssertEventSequence should fail for wrong order")
	}

	// Should fail (missing event)
	mt = newMockT()
	statetesting.AssertEventSequence(mt, rec, "START", "PAUSE", "RESUME")
	if !mt.failed {
		t.Error("AssertEventSequence should fail for missing events")
	}
}

func TestAssertStateSequence(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "PAUSE"})

	// Should pass
	mt := newMockT()
	statetesting.AssertStateSequence(mt, rec, "idle", "running", "paused")
	if mt.failed {
		t.Error("AssertStateSequence should not fail for correct sequence")
	}

	// Should fail (wrong sequence)
	mt = newMockT()
	statetesting.AssertStateSequence(mt, rec, "idle", "paused", "running")
	if !mt.failed {
		t.Error("AssertStateSequence should fail for wrong sequence")
	}
}

func TestAssertContext(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Entry action increments counter
	mt := newMockT()
	statetesting.AssertContext(mt, interp, func(ctx TestContext) bool {
		return ctx.Counter == 1
	}, "counter should be 1")
	if mt.failed {
		t.Error("AssertContext should not fail when predicate passes")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertContext(mt, interp, func(ctx TestContext) bool {
		return ctx.Counter == 100
	}, "counter should be 100")
	if !mt.failed {
		t.Error("AssertContext should fail when predicate fails")
	}
}

func TestAssertTransitionCount(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "PAUSE"})

	// Should pass (2 user events sent)
	mt := newMockT()
	statetesting.AssertTransitionCount(mt, rec, 2)
	if mt.failed {
		t.Error("AssertTransitionCount should not fail for correct count")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertTransitionCount(mt, rec, 5)
	if !mt.failed {
		t.Error("AssertTransitionCount should fail for incorrect count")
	}
}

func TestAssertVisitedState(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})

	// Should pass
	mt := newMockT()
	statetesting.AssertVisitedState(mt, rec, "running")
	if mt.failed {
		t.Error("AssertVisitedState should not fail for visited state")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertVisitedState(mt, rec, "done")
	if !mt.failed {
		t.Error("AssertVisitedState should fail for unvisited state")
	}
}

func TestAssertNotVisitedState(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})

	// Should pass
	mt := newMockT()
	statetesting.AssertNotVisitedState(mt, rec, "done")
	if mt.failed {
		t.Error("AssertNotVisitedState should not fail for unvisited state")
	}

	// Should fail
	mt = newMockT()
	statetesting.AssertNotVisitedState(mt, rec, "running")
	if !mt.failed {
		t.Error("AssertNotVisitedState should fail for visited state")
	}
}

func TestRequireState(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Should pass
	mt := newMockT()
	statetesting.RequireState(mt, interp, "idle")
	if mt.fataled {
		t.Error("RequireState should not fatal for correct state")
	}

	// Should fatal
	mt = newMockT()
	statetesting.RequireState(mt, interp, "running")
	if !mt.fataled {
		t.Error("RequireState should fatal for incorrect state")
	}
}

func TestStateAssertion_Fluent(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	mt := newMockT()
	statetesting.NewStateAssertion(mt, interp).
		IsIn("idle").
		Matches("idle").
		IsNotDone()

	if mt.failed {
		t.Error("Fluent assertions should not fail for valid state")
	}
}

func TestRecorderAssertion_Fluent(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "PAUSE"})

	mt := newMockT()
	statetesting.NewRecorderAssertion(mt, rec).
		IsIn("paused").
		Visited("running").
		NotVisited("done").
		TransitionedFrom("idle", "running").
		StateSequence("idle", "running", "paused").
		EventSequence("START", "PAUSE")

	if mt.failed {
		t.Error("Fluent assertions should not fail for valid recorder state")
	}
}

func TestRecorderAssertion_Dump(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})

	// Just ensure Dump doesn't panic and returns the assertion
	mt := newMockT()
	result := statetesting.NewRecorderAssertion(mt, rec).Dump()
	if result == nil {
		t.Error("Dump should return the assertion for chaining")
	}
}

func TestAssertEventCausedTransition(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "INVALID"})

	// START caused transition
	mt := newMockT()
	statetesting.AssertEventCausedTransition(mt, rec, "START")
	if mt.failed {
		t.Error("AssertEventCausedTransition should pass for event that caused transition")
	}

	// INVALID did not cause transition
	mt = newMockT()
	statetesting.AssertEventCausedTransition(mt, rec, "INVALID")
	if !mt.failed {
		t.Error("AssertEventCausedTransition should fail for event that didn't transition")
	}
}

func TestAssertEventDidNotCauseTransition(t *testing.T) {
	machine := buildTestMachine()
	rec := statetesting.RecordMachine(machine)
	rec.Send(statekit.Event{Type: "INVALID"})
	rec.Send(statekit.Event{Type: "START"})

	// INVALID did not cause transition
	mt := newMockT()
	statetesting.AssertEventDidNotCauseTransition(mt, rec, "INVALID")
	if mt.failed {
		t.Error("AssertEventDidNotCauseTransition should pass for event that didn't transition")
	}

	// START caused transition
	mt = newMockT()
	statetesting.AssertEventDidNotCauseTransition(mt, rec, "START")
	if !mt.failed {
		t.Error("AssertEventDidNotCauseTransition should fail for event that caused transition")
	}
}

// Suppress unused import warning
var _ = bytes.Buffer{}
