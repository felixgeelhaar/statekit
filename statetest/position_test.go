package statetest_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit"
	statetesting "go.klarlabs.de/statekit/statetest"
)

type lifecycleCtx struct {
	Entered []string
}

// lifecycleMachine is the shipping machine — the one whose final states the
// test wants to prove terminal. It is not rebuilt or weakened for the test.
func lifecycleMachine(t *testing.T) *statekit.MachineConfig[lifecycleCtx] {
	t.Helper()
	machine, err := statekit.NewMachine[lifecycleCtx]("lifecycle").
		WithInitial("draft").
		WithAction("enter", func(c *lifecycleCtx, _ statekit.Event) {
			c.Entered = append(c.Entered, "published")
		}).
		State("draft").
		On("SUBMIT").Target("review").
		Done().
		State("review").
		On("APPROVE").Target("published").
		On("REJECT").Target("rejected").
		Done().
		State("published").
		OnEntry("enter").
		On("ARCHIVE").Target("archived").
		Done().
		State("archived").Final().Done().
		State("rejected").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	return machine
}

func TestInterpreterAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		state    statekit.StateID
		wantDone bool
	}{
		{name: "final state", state: "archived", wantDone: true},
		{name: "other final state", state: "rejected", wantDone: true},
		{name: "non-final state", state: "review", wantDone: false},
		{name: "initial state", state: "draft", wantDone: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interp := statetesting.InterpreterAt(lifecycleMachine(t), tt.state)
			defer func() { _ = interp.Close() }()

			if got := interp.State().Value; got != tt.state {
				t.Errorf("state = %q, want %q", got, tt.state)
			}
			if got := interp.Done(); got != tt.wantDone {
				t.Errorf("Done() = %v, want %v", got, tt.wantDone)
			}
			if !interp.Matches(tt.state) {
				t.Errorf("Matches(%q) = false, want true", tt.state)
			}
		})
	}
}

// TestInterpreterAt_FinalStatesAreTerminal is the property from issue #100:
// every final state of the shipping machine accepts nothing.
func TestInterpreterAt_FinalStatesAreTerminal(t *testing.T) {
	t.Parallel()
	machine := lifecycleMachine(t)
	allEvents := []statekit.EventType{"SUBMIT", "APPROVE", "REJECT", "ARCHIVE", "UNKNOWN"}

	for _, final := range []statekit.StateID{"archived", "rejected"} {
		t.Run(string(final), func(t *testing.T) {
			interp := statetesting.InterpreterAt(machine, final)
			defer func() { _ = interp.Close() }()

			statetesting.AssertTerminal(t, interp, allEvents...)
		})
	}
}

// TestInterpreterAt_AcceptsEventsInNonFinalState confirms the positioned
// interpreter is live, not merely parked: a non-final state still transitions.
func TestInterpreterAt_AcceptsEventsInNonFinalState(t *testing.T) {
	t.Parallel()
	interp := statetesting.InterpreterAt(lifecycleMachine(t), "review")
	defer func() { _ = interp.Close() }()

	interp.Send(statekit.Event{Type: "APPROVE"})
	statetesting.AssertState(t, interp, "published")

	interp.Send(statekit.Event{Type: "ARCHIVE"})
	statetesting.AssertState(t, interp, "archived")
	statetesting.AssertDone(t, interp)
}

func TestInterpreterAt_CompoundResolvesToInitialLeaf(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[lifecycleCtx]("nested").
		WithInitial("idle").
		State("idle").On("RUN").Target("running").Done().
		State("running").
		WithInitial("step1").
		State("step1").On("NEXT").Target("step2").End().End().
		State("step2").On("DONE").Target("finished").End().End().
		Done().
		State("finished").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}

	interp := statetesting.InterpreterAt(machine, "running")
	defer func() { _ = interp.Close() }()

	if got := interp.State().Value; got != "step1" {
		t.Errorf("state = %q, want %q (initial leaf of running)", got, "step1")
	}
	if !interp.Matches("running") {
		t.Error("Matches(running) = false, want true")
	}

	interp.Send(statekit.Event{Type: "NEXT"})
	statetesting.AssertState(t, interp, "step2")
}

// TestInterpreterAt_DoesNotRunEntryActions pins the documented caveat: the
// interpreter is placed at the state, not moved into it.
func TestInterpreterAt_DoesNotRunEntryActions(t *testing.T) {
	t.Parallel()
	interp := statetesting.InterpreterAt(lifecycleMachine(t), "published")
	defer func() { _ = interp.Close() }()

	if got := len(interp.State().Context.Entered); got != 0 {
		t.Errorf("entry action ran %d time(s), want 0", got)
	}
}

func TestInterpreterAt_Panics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		run     func(*testing.T)
		wantMsg string
	}{
		{
			name: "unknown state",
			run: func(t *testing.T) {
				statetesting.InterpreterAt(lifecycleMachine(t), "nope")
			},
			wantMsg: `state "nope" not found`,
		},
		{
			name: "nil machine",
			run: func(_ *testing.T) {
				statetesting.InterpreterAt[lifecycleCtx](nil, "anything")
			},
			wantMsg: "machine is nil",
		},
		{
			name:    "parallel state",
			run:     func(t *testing.T) { statetesting.InterpreterAt(parallelTestMachine(t), "both") },
			wantMsg: "is a parallel state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected a panic, got none")
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value = %v (%T), want string", r, r)
				}
				if !strings.Contains(msg, tt.wantMsg) {
					t.Errorf("panic = %q, want it to contain %q", msg, tt.wantMsg)
				}
			}()
			tt.run(t)
		})
	}
}

func parallelTestMachine(t *testing.T) *statekit.MachineConfig[lifecycleCtx] {
	t.Helper()
	machine, err := statekit.NewMachine[lifecycleCtx]("par").
		WithInitial("both").
		State("both").
		Parallel().
		Region("left").WithInitial("l1").
		State("l1").On("X").Target("l2").EndState().
		State("l2").EndState().
		EndRegion().
		Region("right").WithInitial("r1").
		State("r1").On("Y").Target("r2").EndState().
		State("r2").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	return machine
}

func TestAssertTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		state      statekit.StateID
		events     []statekit.EventType
		wantFailed bool
	}{
		{
			name:   "final state accepts nothing",
			state:  "archived",
			events: []statekit.EventType{"SUBMIT", "APPROVE", "ARCHIVE"},
		},
		{
			name:   "final state with no events checks done only",
			state:  "rejected",
			events: nil,
		},
		{
			name:       "non-final state is not terminal",
			state:      "review",
			events:     []statekit.EventType{"APPROVE"},
			wantFailed: true,
		},
		{
			name:       "non-final state with no events still fails",
			state:      "draft",
			events:     nil,
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interp := statetesting.InterpreterAt(lifecycleMachine(t), tt.state)
			defer func() { _ = interp.Close() }()

			mt := newMockT()
			statetesting.AssertTerminal(mt, interp, tt.events...)

			if mt.failed != tt.wantFailed {
				t.Errorf("AssertTerminal failed = %v, want %v (messages: %v)",
					mt.failed, tt.wantFailed, mt.messages)
			}
		})
	}
}

// TestAssertTerminal_DetectsAcceptedEvent covers the case AssertTerminal
// exists for: a state that is final but wrongly reachable out of.
func TestAssertTerminal_DetectsAcceptedEvent(t *testing.T) {
	t.Parallel()
	// A "final" state that still declares an outgoing transition. Build-time
	// validation permits this; the machine is simply wrong.
	machine, err := statekit.NewMachine[lifecycleCtx]("leaky").
		WithInitial("open").
		State("open").On("CLOSE").Target("closed").Done().
		State("closed").Final().On("REOPEN").Target("open").Done().
		Build()
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}

	interp := statetesting.InterpreterAt(machine, "closed")
	defer func() { _ = interp.Close() }()

	mt := newMockT()
	statetesting.AssertTerminal(mt, interp, "REOPEN")

	if !mt.failed {
		t.Error("AssertTerminal passed for a final state with an outgoing transition, want failure")
	}
}
