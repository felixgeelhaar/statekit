package viz_test

import (
	"testing"
	"time"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/export"
	"go.klarlabs.de/statekit/viz"
	"go.klarlabs.de/statekit/viz/ascii"
	"go.klarlabs.de/statekit/viz/mermaid"
)

type vizCtx struct {
	Count int
}

func mustBuild(t *testing.T, b *statekit.MachineBuilder[vizCtx]) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	m, err := b.Build()
	if err != nil {
		t.Fatalf("build machine: %v", err)
	}
	return m
}

// flatMachine is the shape from issue #98: a lifecycle assembled at runtime,
// with no literal machine definition for the Go source parser to find.
func flatMachine(t *testing.T) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	states := []string{"draft", "review", "approved", "published", "archived"}
	builder := statekit.NewMachine[vizCtx]("lifecycle").WithInitial("draft")
	for i, s := range states {
		sb := builder.State(statekit.StateID(s))
		if i == len(states)-1 {
			sb = sb.Final()
		} else {
			sb = sb.On("NEXT").Target(statekit.StateID(states[i+1])).End()
		}
		builder = sb.Done()
	}
	return mustBuild(t, builder)
}

func compoundMachine(t *testing.T) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	return mustBuild(t, statekit.NewMachine[vizCtx]("compound").
		WithInitial("idle").
		WithAction("log", func(_ *vizCtx, _ statekit.Event) {}).
		WithGuard("ready", func(_ vizCtx, _ statekit.Event) bool { return true }).
		State("idle").
		OnEntry("log").
		On("RUN").Target("running").Guard("ready").Do("log").
		Done().
		State("running").
		WithInitial("step1").
		OnExit("log").
		State("step1").On("NEXT").Target("step2").End().End().
		State("step2").On("DONE").Target("finished").End().End().
		After(5*time.Second).Target("finished").
		Done().
		State("finished").Final().Done())
}

func parallelMachine(t *testing.T) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	return mustBuild(t, statekit.NewMachine[vizCtx]("editor").
		WithInitial("editing").
		State("editing").
		Parallel().
		Region("bold").WithInitial("off").
		State("off").On("TOGGLE_BOLD").Target("on").EndState().
		State("on").On("TOGGLE_BOLD").Target("off").EndState().
		EndRegion().
		Region("italic").WithInitial("off_i").
		State("off_i").On("TOGGLE_ITALIC").Target("on_i").EndState().
		State("on_i").On("TOGGLE_ITALIC").Target("off_i").EndState().
		EndRegion().
		Done())
}

func historyMachine(t *testing.T) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	return mustBuild(t, statekit.NewMachine[vizCtx]("player").
		WithInitial("active").
		State("active").
		WithInitial("playing").
		State("playing").On("PAUSE").Target("paused").End().End().
		State("paused").On("PLAY").Target("playing").End().End().
		History("hist").Deep().Default("playing").End().
		On("STOP").Target("stopped").
		Done().
		State("stopped").On("RESUME").Target("hist").Done())
}

func gateMachine(t *testing.T) *statekit.MachineConfig[vizCtx] {
	t.Helper()
	return mustBuild(t, statekit.NewMachine[vizCtx]("gate").
		WithInitial("checking").
		WithGuard("ok", func(_ vizCtx, _ statekit.Event) bool { return true }).
		State("checking").
		Tags("transient", "internal").
		Always().Target("allowed").Guard("ok").End().
		Always().Target("denied").End().
		Done().
		State("allowed").Final().Done().
		State("denied").Final().Done())
}

// TestFromMachine_MatchesJSONRoundTrip is the property issue #98 asks for:
// rendering a compiled machine directly must produce the same diagram as
// rendering it after a trip through statekit's native JSON.
func TestFromMachine_MatchesJSONRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		machine func(*testing.T) *statekit.MachineConfig[vizCtx]
	}{
		{"flat", flatMachine},
		{"compound", compoundMachine},
		{"parallel", parallelMachine},
		{"history", historyMachine},
		{"eventless and tags", gateMachine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := tt.machine(t)

			direct := viz.FromMachine(machine)
			if direct == nil {
				t.Fatal("FromMachine returned nil for a valid machine")
			}

			jsonStr, err := export.NewNativeExporter(machine).ExportJSON()
			if err != nil {
				t.Fatalf("ExportJSON: %v", err)
			}
			roundTripped, err := viz.ParseNativeJSON([]byte(jsonStr))
			if err != nil {
				t.Fatalf("ParseNativeJSON: %v", err)
			}

			if got, want := mermaid.NewRenderer().Render(direct), mermaid.NewRenderer().Render(roundTripped); got != want {
				t.Errorf("mermaid output differs.\ndirect:\n%s\nround-tripped:\n%s", got, want)
			}
			if got, want := ascii.NewRenderer().Render(direct), ascii.NewRenderer().Render(roundTripped); got != want {
				t.Errorf("ascii output differs.\ndirect:\n%s\nround-tripped:\n%s", got, want)
			}
		})
	}
}

// TestFromMachine_Structure checks the translation field by field, so a
// regression points at the field rather than at a diff of rendered text.
func TestFromMachine_Structure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		machine func(*testing.T) *statekit.MachineConfig[vizCtx]
		check   func(*testing.T, *viz.VizMachine)
	}{
		{
			name:    "id and initial",
			machine: flatMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				if vm.ID != "lifecycle" {
					t.Errorf("ID = %q, want %q", vm.ID, "lifecycle")
				}
				if vm.Initial != "draft" {
					t.Errorf("Initial = %q, want %q", vm.Initial, "draft")
				}
				if len(vm.States) != 5 {
					t.Errorf("len(States) = %d, want 5", len(vm.States))
				}
			},
		},
		{
			name:    "final state marked",
			machine: flatMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				s := vm.States["archived"]
				if s == nil {
					t.Fatal("state archived missing")
				}
				if s.Type != viz.VizStateFinal {
					t.Errorf("archived.Type = %q, want %q", s.Type, viz.VizStateFinal)
				}
			},
		},
		{
			name:    "transitions carry event and target",
			machine: flatMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				s := vm.States["draft"]
				if s == nil {
					t.Fatal("state draft missing")
				}
				if len(s.Transitions) != 1 {
					t.Fatalf("len(draft.Transitions) = %d, want 1", len(s.Transitions))
				}
				if got := s.Transitions[0]; got.Event != "NEXT" || got.Target != "review" {
					t.Errorf("draft transition = %+v, want NEXT -> review", got)
				}
			},
		},
		{
			name:    "guards actions and entry exit",
			machine: compoundMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				idle := vm.States["idle"]
				if idle == nil {
					t.Fatal("state idle missing")
				}
				if len(idle.Entry) != 1 || idle.Entry[0] != "log" {
					t.Errorf("idle.Entry = %v, want [log]", idle.Entry)
				}
				if len(idle.Transitions) != 1 {
					t.Fatalf("len(idle.Transitions) = %d, want 1", len(idle.Transitions))
				}
				tr := idle.Transitions[0]
				if tr.Guard != "ready" {
					t.Errorf("transition Guard = %q, want %q", tr.Guard, "ready")
				}
				if len(tr.Actions) != 1 || tr.Actions[0] != "log" {
					t.Errorf("transition Actions = %v, want [log]", tr.Actions)
				}
				running := vm.States["running"]
				if running == nil {
					t.Fatal("state running missing")
				}
				if len(running.Exit) != 1 || running.Exit[0] != "log" {
					t.Errorf("running.Exit = %v, want [log]", running.Exit)
				}
			},
		},
		{
			name:    "compound parent child and depth",
			machine: compoundMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				running := vm.States["running"]
				if running == nil {
					t.Fatal("state running missing")
				}
				if running.Type != viz.VizStateCompound {
					t.Errorf("running.Type = %q, want %q", running.Type, viz.VizStateCompound)
				}
				if running.Initial != "step1" {
					t.Errorf("running.Initial = %q, want %q", running.Initial, "step1")
				}
				if len(running.Children) != 2 {
					t.Errorf("running.Children = %v, want 2 entries", running.Children)
				}
				if running.Depth != 0 {
					t.Errorf("running.Depth = %d, want 0", running.Depth)
				}
				step1 := vm.States["step1"]
				if step1 == nil {
					t.Fatal("state step1 missing")
				}
				if step1.Parent != "running" {
					t.Errorf("step1.Parent = %q, want %q", step1.Parent, "running")
				}
				if step1.Depth != 1 {
					t.Errorf("step1.Depth = %d, want 1", step1.Depth)
				}
			},
		},
		{
			name:    "delayed transition",
			machine: compoundMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				running := vm.States["running"]
				if running == nil {
					t.Fatal("state running missing")
				}
				var delayed *viz.VizTransition
				for i := range running.Transitions {
					if running.Transitions[i].IsDelayed {
						delayed = &running.Transitions[i]
					}
				}
				if delayed == nil {
					t.Fatal("no delayed transition on running")
				}
				if delayed.DelayMs != 5000 {
					t.Errorf("DelayMs = %d, want 5000", delayed.DelayMs)
				}
				if delayed.Target != "finished" {
					t.Errorf("delayed Target = %q, want %q", delayed.Target, "finished")
				}
			},
		},
		{
			name:    "parallel regions",
			machine: parallelMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				editing := vm.States["editing"]
				if editing == nil {
					t.Fatal("state editing missing")
				}
				if editing.Type != viz.VizStateParallel {
					t.Errorf("editing.Type = %q, want %q", editing.Type, viz.VizStateParallel)
				}
				if len(editing.Children) != 2 {
					t.Errorf("editing.Children = %v, want 2 regions", editing.Children)
				}
				bold := vm.States["bold"]
				if bold == nil {
					t.Fatal("region bold missing")
				}
				if bold.Parent != "editing" {
					t.Errorf("bold.Parent = %q, want %q", bold.Parent, "editing")
				}
				if vm.States["on"].Depth != 2 {
					t.Errorf("on.Depth = %d, want 2", vm.States["on"].Depth)
				}
			},
		},
		{
			name:    "history state",
			machine: historyMachine,
			check: func(t *testing.T, vm *viz.VizMachine) {
				h := vm.States["hist"]
				if h == nil {
					t.Fatal("state hist missing")
				}
				if h.Type != viz.VizStateHistory {
					t.Errorf("hist.Type = %q, want %q", h.Type, viz.VizStateHistory)
				}
				if h.HistoryType != "deep" {
					t.Errorf("hist.HistoryType = %q, want %q", h.HistoryType, "deep")
				}
				if h.HistoryDefault != "playing" {
					t.Errorf("hist.HistoryDefault = %q, want %q", h.HistoryDefault, "playing")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, viz.FromMachine(tt.machine(t)))
		})
	}
}

// TestFromMachine_EventlessAndTags covers the v1.x additions: eventless
// ("always") transitions and state tags.
func TestFromMachine_EventlessAndTags(t *testing.T) {
	t.Parallel()
	machine := gateMachine(t)

	vm := viz.FromMachine(machine)
	checking := vm.States["checking"]
	if checking == nil {
		t.Fatal("state checking missing")
	}
	if len(checking.Always) != 2 {
		t.Fatalf("len(Always) = %d, want 2", len(checking.Always))
	}
	if checking.Always[0].Target != "allowed" || checking.Always[0].Guard != "ok" {
		t.Errorf("Always[0] = %+v, want allowed guarded by ok", checking.Always[0])
	}
	if len(checking.Tags) != 2 || checking.Tags[0] != "transient" {
		t.Errorf("Tags = %v, want [transient internal]", checking.Tags)
	}
}

// TestFromMachine_MatchesNativeExporter pins FromMachine and the existing
// exporter to the same output, since the exporter now delegates to it.
func TestFromMachine_MatchesNativeExporter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		machine func(*testing.T) *statekit.MachineConfig[vizCtx]
	}{
		{"flat", flatMachine},
		{"compound", compoundMachine},
		{"parallel", parallelMachine},
		{"history", historyMachine},
		{"eventless and tags", gateMachine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := tt.machine(t)
			direct := mermaid.NewRenderer().Render(viz.FromMachine(machine))
			viaExporter := mermaid.NewRenderer().Render(export.NewNativeExporter(machine).Export())
			if direct != viaExporter {
				t.Errorf("FromMachine and NativeExporter.Export disagree.\ndirect:\n%s\nexporter:\n%s", direct, viaExporter)
			}
		})
	}
}

func TestFromMachine_Nil(t *testing.T) {
	t.Parallel()
	if got := viz.FromMachine[vizCtx](nil); got != nil {
		t.Errorf("FromMachine(nil) = %+v, want nil", got)
	}
}

// TestFromMachine_Snapshot confirms the returned model does not alias the
// machine: mutating it must not disturb a later render.
func TestFromMachine_Snapshot(t *testing.T) {
	t.Parallel()
	machine := flatMachine(t)

	first := viz.FromMachine(machine)
	before := mermaid.NewRenderer().Render(first)

	delete(first.States, "review")
	first.Initial = "tampered"

	after := mermaid.NewRenderer().Render(viz.FromMachine(machine))
	if before != after {
		t.Errorf("mutating a returned model changed a later render.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
