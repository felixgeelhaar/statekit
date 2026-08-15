package statekit_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit"
)

type termCtx struct{}

// TestTerminatorShapes builds the three shapes documented in the package
// "Closing the builder" section, so the documented terminator sequences are
// verified rather than merely asserted in prose.
func TestTerminatorShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		build      func() (*statekit.MachineConfig[termCtx], error)
		wantStates []statekit.StateID
		wantLeaf   statekit.StateID
	}{
		{
			name: "flat closes every state with Done",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("order").
					WithInitial("cart").
					State("cart").On("CHECKOUT").Target("paid").Done().
					State("paid").Final().Done().
					Build()
			},
			wantStates: []statekit.StateID{"cart", "paid"},
			wantLeaf:   "cart",
		},
		{
			name: "nested closes children with End and the parent with Done",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("editor").
					WithInitial("editing").
					State("editing").
					WithInitial("idle").
					State("idle").On("TYPE").Target("dirty").End().End().
					State("dirty").On("CLEAR").Target("idle").End().End().
					Done().
					Build()
			},
			wantStates: []statekit.StateID{"editing", "idle", "dirty"},
			wantLeaf:   "idle",
		},
		{
			name: "nested prefers Up after a transition (≡ End.End)",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("editor_up").
					WithInitial("editing").
					State("editing").
					WithInitial("idle").
					State("idle").On("TYPE").Target("dirty").Up().
					State("dirty").On("CLEAR").Target("idle").Up().
					Done().
					Build()
			},
			wantStates: []statekit.StateID{"editing", "idle", "dirty"},
			wantLeaf:   "idle",
		},
		{
			name: "EndTo unwinds to a named ancestor",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("deep").
					WithInitial("app").
					State("app").
					WithInitial("editor").
					State("editor").
					WithInitial("idle").
					State("idle").On("TYPE").Target("dirty").Up().
					State("dirty").On("CLEAR").Target("idle").EndTo("app").
					Done().
					Build()
			},
			wantStates: []statekit.StateID{"app", "editor", "idle", "dirty"},
			wantLeaf:   "idle",
		},
		{
			name: "parallel closes region states with EndState",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("styles").
					WithInitial("editing").
					State("editing").
					Parallel().
					Region("bold").WithInitial("off").
					State("off").On("TOGGLE_BOLD").Target("on").EndState().
					State("on").On("TOGGLE_BOLD").Target("off").EndState().
					EndRegion().
					Done().
					Build()
			},
			wantStates: []statekit.StateID{"editing", "bold", "off", "on"},
			wantLeaf:   "editing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine, err := tt.build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			for _, id := range tt.wantStates {
				if machine.GetState(id) == nil {
					t.Errorf("state %q missing from machine", id)
				}
			}

			interp := statekit.NewInterpreter(machine)
			defer func() { _ = interp.Close() }()
			interp.Start()

			if got := interp.State().Value; got != tt.wantLeaf {
				t.Errorf("initial leaf = %q, want %q", got, tt.wantLeaf)
			}
		})
	}
}

// TestTerminatorLoopShape covers the case from issue #99: a flat machine built
// in a loop, where the terminator sequence is not visible in the source.
func TestTerminatorLoopShape(t *testing.T) {
	t.Parallel()
	type transition struct{ event, target string }
	states := []string{"draft", "review", "published", "archived"}
	transitions := map[string][]transition{
		"draft":     {{"SUBMIT", "review"}},
		"review":    {{"APPROVE", "published"}, {"REJECT", "draft"}},
		"published": {{"ARCHIVE", "archived"}},
	}

	builder := statekit.NewMachine[termCtx]("lifecycle").WithInitial("draft")
	for _, s := range states {
		sb := builder.State(statekit.StateID(s))
		if s == "archived" {
			sb = sb.Final()
		}
		for _, tr := range transitions[s] {
			sb = sb.On(statekit.EventType(tr.event)).Target(statekit.StateID(tr.target)).End()
		}
		builder = sb.Done()
	}

	machine, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	for _, step := range []struct {
		event statekit.EventType
		want  statekit.StateID
	}{
		{"SUBMIT", "review"},
		{"APPROVE", "published"},
		{"ARCHIVE", "archived"},
	} {
		interp.Send(statekit.Event{Type: step.event})
		if got := interp.State().Value; got != step.want {
			t.Fatalf("after %q: state = %q, want %q", step.event, got, step.want)
		}
	}
	if !interp.Done() {
		t.Error("expected the machine to be done in archived")
	}
}

// TestTerminatorEquivalence pins the pairs that must stay interchangeable.
// EndMachine is deprecated in favour of Done but keeps working.
func TestTerminatorEquivalence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() (*statekit.MachineConfig[termCtx], error)
	}{
		{
			name: "Done on a state",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").Done().
					State("b").Done().
					Build()
			},
		},
		{
			name: "EndMachine on a state",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").End().EndMachine().
					State("b").EndMachine().
					Build()
			},
		},
		{
			name: "EndMachine on a transition",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").EndMachine().
					State("b").Done().
					Build()
			},
		},
		{
			name: "End then Done on a transition",
			build: func() (*statekit.MachineConfig[termCtx], error) {
				return statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").End().Done().
					State("b").Done().
					Build()
			},
		},
	}

	var reference string
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine, err := tt.build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			interp := statekit.NewInterpreter(machine)
			defer func() { _ = interp.Close() }()
			interp.Start()
			interp.Send(statekit.Event{Type: "X"})

			got := describe(machine, interp.State().Value)
			if i == 0 {
				reference = got
				return
			}
			if got != reference {
				t.Errorf("machine differs from the Done-built reference.\ngot:  %s\nwant: %s", got, reference)
			}
		})
	}
}

func describe[C any](m *statekit.MachineConfig[C], leaf statekit.StateID) string {
	var b strings.Builder
	b.WriteString("initial=" + string(m.Initial) + " leaf=" + string(leaf))
	for _, id := range []statekit.StateID{"a", "b"} {
		s := m.GetState(id)
		if s == nil {
			b.WriteString(" " + string(id) + "=missing")
			continue
		}
		b.WriteString(" " + string(id) + "=[")
		for _, tr := range s.Transitions {
			b.WriteString(string(tr.Event) + "->" + string(tr.Target) + " ")
		}
		b.WriteString("]")
	}
	return b.String()
}

// TestTerminatorMisusePanics covers the guard rails: a terminator with no
// destination now fails at the mistake with a message naming the fix, instead
// of returning a nil builder that panics opaquely at some later call.
func TestTerminatorMisusePanics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		run     func()
		wantMsg string
	}{
		{
			name: "End on a top-level state",
			run: func() {
				statekit.NewMachine[termCtx]("m").WithInitial("a").State("a").End()
			},
			wantMsg: `End called on top-level state "a"`,
		},
		{
			name: "EndState outside a region",
			run: func() {
				statekit.NewMachine[termCtx]("m").WithInitial("a").State("a").EndState()
			},
			wantMsg: `EndState called on state "a", which is not inside a parallel region`,
		},
		{
			name: "EndState on a transition outside a region",
			run: func() {
				statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").EndState()
			},
			wantMsg: "not inside a parallel region",
		},
		{
			name: "Up on a top-level transition",
			run: func() {
				statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").On("X").Target("b").Up()
			},
			wantMsg: `End called on top-level state "a"`,
		},
		{
			name: "EndTo missing ancestor",
			run: func() {
				statekit.NewMachine[termCtx]("m").WithInitial("a").
					State("a").
					WithInitial("b").
					State("b").EndTo("ghost")
			},
			wantMsg: `EndTo("ghost")`,
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
				if !strings.Contains(msg, "use ") && !strings.Contains(msg, "EndTo") {
					t.Errorf("panic = %q, want it to name the terminator to use instead", msg)
				}
			}()
			tt.run()
		})
	}
}

// TestNestedEndUnwindsOneLevel distinguishes End from Done on a nested state:
// End steps up one level, Done goes all the way to the machine root.
func TestNestedEndUnwindsOneLevel(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[termCtx]("deep").
		WithInitial("l1").
		State("l1").
		WithInitial("l2").
		State("l2").
		WithInitial("l3").
		State("l3").On("X").Target("l3").End().End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	for id, wantParent := range map[statekit.StateID]statekit.StateID{
		"l1": "",
		"l2": "l1",
		"l3": "l2",
	} {
		s := machine.GetState(id)
		if s == nil {
			t.Fatalf("state %q missing", id)
		}
		if s.Parent != wantParent {
			t.Errorf("%s.Parent = %q, want %q", id, s.Parent, wantParent)
		}
	}
}
