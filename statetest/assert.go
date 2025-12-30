package statetest

import (
	"fmt"
	"testing"

	"github.com/felixgeelhaar/statekit"
)

// AssertState asserts that the interpreter is in the expected state.
func AssertState[C any](t testing.TB, interp *statekit.Interpreter[C], expected statekit.StateID) {
	t.Helper()
	actual := interp.State().Value
	if actual != expected {
		t.Errorf("expected state %q, got %q", expected, actual)
	}
}

// AssertMatches asserts that the interpreter matches the given state ID.
// This includes matching ancestor states in hierarchical machines.
func AssertMatches[C any](t testing.TB, interp *statekit.Interpreter[C], stateID statekit.StateID) {
	t.Helper()
	if !interp.Matches(stateID) {
		t.Errorf("expected to match state %q, current state is %q", stateID, interp.State().Value)
	}
}

// AssertNotMatches asserts that the interpreter does not match the given state ID.
func AssertNotMatches[C any](t testing.TB, interp *statekit.Interpreter[C], stateID statekit.StateID) {
	t.Helper()
	if interp.Matches(stateID) {
		t.Errorf("expected not to match state %q, but it matched", stateID)
	}
}

// AssertDone asserts that the interpreter is in a final state.
func AssertDone[C any](t testing.TB, interp *statekit.Interpreter[C]) {
	t.Helper()
	if !interp.Done() {
		t.Errorf("expected interpreter to be done, current state is %q", interp.State().Value)
	}
}

// AssertNotDone asserts that the interpreter is not in a final state.
func AssertNotDone[C any](t testing.TB, interp *statekit.Interpreter[C]) {
	t.Helper()
	if interp.Done() {
		t.Errorf("expected interpreter not to be done, but it is in final state %q", interp.State().Value)
	}
}

// AssertRecorderState asserts that the recorder's interpreter is in the expected state.
func AssertRecorderState[C any](t testing.TB, rec *Recorder[C], expected statekit.StateID) {
	t.Helper()
	actual := rec.State().Value
	if actual != expected {
		t.Errorf("expected state %q, got %q", expected, actual)
	}
}

// AssertTransitioned asserts that a transition occurred from one state to another.
func AssertTransitioned[C any](t testing.TB, rec *Recorder[C], from, to statekit.StateID) {
	t.Helper()
	transitions := rec.Transitions()
	for _, tr := range transitions {
		if tr.FromState == from && tr.ToState == to && tr.Transitioned {
			return
		}
	}
	t.Errorf("expected transition from %q to %q, but it was not recorded", from, to)
}

// AssertNoTransition asserts that sending an event did not cause a state transition.
func AssertNoTransition[C any](t testing.TB, rec *Recorder[C], eventType statekit.EventType) {
	t.Helper()
	for _, tr := range rec.Transitions() {
		if tr.Event.Type == eventType && tr.Transitioned {
			t.Errorf("expected no transition for event %q, but transitioned from %q to %q",
				eventType, tr.FromState, tr.ToState)
			return
		}
	}
}

// AssertEventSequence asserts that the recorder received events in the given order.
func AssertEventSequence[C any](t testing.TB, rec *Recorder[C], events ...statekit.EventType) {
	t.Helper()
	recorded := rec.EventTypes()

	if len(recorded) != len(events) {
		t.Errorf("expected %d events, got %d", len(events), len(recorded))
		t.Errorf("expected: %v", events)
		t.Errorf("got:      %v", recorded)
		return
	}

	for i, expected := range events {
		if recorded[i] != expected {
			t.Errorf("event at position %d: expected %q, got %q", i, expected, recorded[i])
		}
	}
}

// AssertStateSequence asserts that the recorder visited states in the given order.
func AssertStateSequence[C any](t testing.TB, rec *Recorder[C], states ...statekit.StateID) {
	t.Helper()
	recorded := rec.States()

	if len(recorded) != len(states) {
		t.Errorf("expected %d states, got %d", len(states), len(recorded))
		t.Errorf("expected: %v", states)
		t.Errorf("got:      %v", recorded)
		return
	}

	for i, expected := range states {
		if recorded[i] != expected {
			t.Errorf("state at position %d: expected %q, got %q", i, expected, recorded[i])
		}
	}
}

// AssertContext asserts that the interpreter's context satisfies the given predicate.
func AssertContext[C any](t testing.TB, interp *statekit.Interpreter[C], check func(C) bool, msg string) {
	t.Helper()
	ctx := interp.State().Context
	if !check(ctx) {
		t.Errorf("context assertion failed: %s", msg)
	}
}

// AssertRecorderContext asserts that the recorder's context satisfies the given predicate.
func AssertRecorderContext[C any](t testing.TB, rec *Recorder[C], check func(C) bool, msg string) {
	t.Helper()
	ctx := rec.State().Context
	if !check(ctx) {
		t.Errorf("context assertion failed: %s", msg)
	}
}

// AssertTransitionCount asserts the number of transitions recorded.
func AssertTransitionCount[C any](t testing.TB, rec *Recorder[C], expected int) {
	t.Helper()
	// Subtract start events to count only user-sent events
	actual := rec.TransitionCount()
	startEvents := 0
	for _, tr := range rec.Transitions() {
		if tr.Event.Type == syntheticStartEvent {
			startEvents++
		}
	}
	userTransitions := actual - startEvents

	if userTransitions != expected {
		t.Errorf("expected %d transitions, got %d", expected, userTransitions)
	}
}

// AssertActualTransitionCount asserts the number of transitions where state actually changed.
func AssertActualTransitionCount[C any](t testing.TB, rec *Recorder[C], expected int) {
	t.Helper()
	actual := len(rec.ActualTransitions())
	if actual != expected {
		t.Errorf("expected %d actual transitions, got %d", expected, actual)
	}
}

// AssertVisitedState asserts that a particular state was visited at least once.
func AssertVisitedState[C any](t testing.TB, rec *Recorder[C], state statekit.StateID) {
	t.Helper()
	for _, s := range rec.UniqueStates() {
		if s == state {
			return
		}
	}
	t.Errorf("expected state %q to have been visited, but it was not", state)
}

// AssertNotVisitedState asserts that a particular state was never visited.
func AssertNotVisitedState[C any](t testing.TB, rec *Recorder[C], state statekit.StateID) {
	t.Helper()
	for _, s := range rec.UniqueStates() {
		if s == state {
			t.Errorf("expected state %q not to have been visited, but it was", state)
			return
		}
	}
}

// AssertLastState asserts the final state after all transitions.
func AssertLastState[C any](t testing.TB, rec *Recorder[C], expected statekit.StateID) {
	t.Helper()
	last := rec.LastTransition()
	if last == nil {
		t.Errorf("expected last state to be %q, but no transitions recorded", expected)
		return
	}
	if last.ToState != expected {
		t.Errorf("expected last state to be %q, got %q", expected, last.ToState)
	}
}

// AssertEventCausedTransition asserts that a specific event caused a state change.
func AssertEventCausedTransition[C any](t testing.TB, rec *Recorder[C], eventType statekit.EventType) {
	t.Helper()
	tr := rec.FindTransition(eventType)
	if tr == nil {
		t.Errorf("event %q was not recorded", eventType)
		return
	}
	if !tr.Transitioned {
		t.Errorf("event %q did not cause a transition (stayed in %q)", eventType, tr.FromState)
	}
}

// AssertEventDidNotCauseTransition asserts that a specific event did not cause a state change.
func AssertEventDidNotCauseTransition[C any](t testing.TB, rec *Recorder[C], eventType statekit.EventType) {
	t.Helper()
	tr := rec.FindTransition(eventType)
	if tr == nil {
		// Event wasn't sent, so it definitely didn't cause a transition
		return
	}
	if tr.Transitioned {
		t.Errorf("event %q caused a transition from %q to %q", eventType, tr.FromState, tr.ToState)
	}
}

// RequireState is like AssertState but fails the test immediately.
func RequireState[C any](t testing.TB, interp *statekit.Interpreter[C], expected statekit.StateID) {
	t.Helper()
	actual := interp.State().Value
	if actual != expected {
		t.Fatalf("required state %q, got %q", expected, actual)
	}
}

// RequireDone is like AssertDone but fails the test immediately.
func RequireDone[C any](t testing.TB, interp *statekit.Interpreter[C]) {
	t.Helper()
	if !interp.Done() {
		t.Fatalf("required interpreter to be done, current state is %q", interp.State().Value)
	}
}

// RequireNotDone is like AssertNotDone but fails the test immediately.
func RequireNotDone[C any](t testing.TB, interp *statekit.Interpreter[C]) {
	t.Helper()
	if interp.Done() {
		t.Fatalf("required interpreter not to be done, but it is in final state %q", interp.State().Value)
	}
}

// StateAssertion provides a fluent interface for state assertions.
type StateAssertion[C any] struct {
	t      testing.TB
	interp *statekit.Interpreter[C]
}

// NewStateAssertion creates a new fluent assertion helper.
func NewStateAssertion[C any](t testing.TB, interp *statekit.Interpreter[C]) *StateAssertion[C] {
	return &StateAssertion[C]{t: t, interp: interp}
}

// IsIn asserts the current state.
func (a *StateAssertion[C]) IsIn(state statekit.StateID) *StateAssertion[C] {
	a.t.Helper()
	AssertState(a.t, a.interp, state)
	return a
}

// Matches asserts matching state (including ancestors).
func (a *StateAssertion[C]) Matches(state statekit.StateID) *StateAssertion[C] {
	a.t.Helper()
	AssertMatches(a.t, a.interp, state)
	return a
}

// IsDone asserts the machine is in a final state.
func (a *StateAssertion[C]) IsDone() *StateAssertion[C] {
	a.t.Helper()
	AssertDone(a.t, a.interp)
	return a
}

// IsNotDone asserts the machine is not in a final state.
func (a *StateAssertion[C]) IsNotDone() *StateAssertion[C] {
	a.t.Helper()
	AssertNotDone(a.t, a.interp)
	return a
}

// RecorderAssertion provides a fluent interface for recorder assertions.
type RecorderAssertion[C any] struct {
	t   testing.TB
	rec *Recorder[C]
}

// NewRecorderAssertion creates a new fluent assertion helper for recorders.
func NewRecorderAssertion[C any](t testing.TB, rec *Recorder[C]) *RecorderAssertion[C] {
	return &RecorderAssertion[C]{t: t, rec: rec}
}

// IsIn asserts the current state.
func (a *RecorderAssertion[C]) IsIn(state statekit.StateID) *RecorderAssertion[C] {
	a.t.Helper()
	AssertRecorderState(a.t, a.rec, state)
	return a
}

// Visited asserts that a state was visited.
func (a *RecorderAssertion[C]) Visited(state statekit.StateID) *RecorderAssertion[C] {
	a.t.Helper()
	AssertVisitedState(a.t, a.rec, state)
	return a
}

// NotVisited asserts that a state was not visited.
func (a *RecorderAssertion[C]) NotVisited(state statekit.StateID) *RecorderAssertion[C] {
	a.t.Helper()
	AssertNotVisitedState(a.t, a.rec, state)
	return a
}

// TransitionedFrom asserts a transition occurred from the given state.
func (a *RecorderAssertion[C]) TransitionedFrom(from, to statekit.StateID) *RecorderAssertion[C] {
	a.t.Helper()
	AssertTransitioned(a.t, a.rec, from, to)
	return a
}

// StateSequence asserts the sequence of states visited.
func (a *RecorderAssertion[C]) StateSequence(states ...statekit.StateID) *RecorderAssertion[C] {
	a.t.Helper()
	AssertStateSequence(a.t, a.rec, states...)
	return a
}

// EventSequence asserts the sequence of events sent.
func (a *RecorderAssertion[C]) EventSequence(events ...statekit.EventType) *RecorderAssertion[C] {
	a.t.Helper()
	AssertEventSequence(a.t, a.rec, events...)
	return a
}

// Dump prints the recorded transitions for debugging.
func (a *RecorderAssertion[C]) Dump() *RecorderAssertion[C] {
	transitions := a.rec.Transitions()
	fmt.Printf("\n=== Recorded Transitions (%d) ===\n", len(transitions))
	for i, tr := range transitions {
		transitioned := ""
		if tr.Transitioned {
			transitioned = " [TRANSITIONED]"
		}
		fmt.Printf("%d. %s: %s -> %s%s (took %v)\n",
			i, tr.Event.Type, tr.FromState, tr.ToState, transitioned, tr.Duration)
	}
	fmt.Println("================================")
	return a
}
