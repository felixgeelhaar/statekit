package export_test

import (
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/export"
	"go.klarlabs.de/statekit/viz"
)

// Both halves of the round-trip were tested, and the seam between them was
// not. export tested that the exporter emits what XState expects; viz tested
// that the parser reads hand-written fixtures. Neither asked whether the
// parser can read what the exporter writes — which is what `statekit viz` does
// on real output, and what a consumer does when it exports a machine and reads
// it back.
//
// It could not. A single eventless transition collapses to an object and the
// parser accepted only an array, failing the whole ParseNativeJSON call;
// guarded alternatives on one event collapse to an array and the parser
// accepted only an object, yielding zero transitions with no error at all.
func TestXStateExportParsesBack(t *testing.T) {
	m, err := statekit.NewMachine[struct{}]("orders").
		WithInitial("triage").
		WithGuard("isClean", func(_ struct{}, _ statekit.Event) bool { return true }).
		// One eventless transition: the exporter collapses this to an object.
		State("triage").
		Always().Target("review").Raise("TRIAGED").End().
		Done().
		// Two transitions on one event: the exporter emits an array.
		State("review").
		On("DECIDE").Target("approved").Guard("isClean").End().
		On("DECIDE").Target("rejected").End().
		Done().
		// A delayed transition: the exporter writes it under "after".
		State("stale").
		After(30 * time.Second).Target("rejected").End().
		Done().
		State("approved").Final().Done().
		State("rejected").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	raw, err := json.Marshal(export.NewXStateExporter(m).Export())
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}

	vm, err := viz.ParseNativeJSON(raw)
	if err != nil {
		t.Fatalf("ParseNativeJSON on this package's own output: %v\n%s", err, raw)
	}

	triage := vm.States["triage"]
	if triage == nil {
		t.Fatal("state triage missing after round-trip")
	}
	if len(triage.Always) != 1 {
		t.Fatalf("triage.Always = %d, want 1\n%s", len(triage.Always), raw)
	}
	if got := triage.Always[0].Target; got != "review" {
		t.Errorf("always target = %q, want review", got)
	}
	if got := triage.Always[0].Raise; len(got) != 1 || got[0] != "TRIAGED" {
		t.Errorf("always raise = %v, want [TRIAGED]", got)
	}

	review := vm.States["review"]
	if review == nil {
		t.Fatal("state review missing after round-trip")
	}
	if len(review.Transitions) != 2 {
		t.Fatalf("review.Transitions = %d, want 2 (both DECIDE alternatives)\n%s", len(review.Transitions), raw)
	}
	targets := map[string]string{}
	for _, tr := range review.Transitions {
		if tr.Event != "DECIDE" {
			t.Errorf("event = %q, want DECIDE", tr.Event)
		}
		targets[tr.Target] = tr.Guard
	}
	if guard, ok := targets["approved"]; !ok || guard != "isClean" {
		t.Errorf("approved guard = %q (present=%v), want isClean", guard, ok)
	}
	stale := vm.States["stale"]
	if stale == nil {
		t.Fatal("state stale missing after round-trip")
	}
	if len(stale.Transitions) != 1 {
		t.Fatalf("stale.Transitions = %d, want 1 (the delayed edge)\n%s", len(stale.Transitions), raw)
	}
	if d := stale.Transitions[0]; !d.IsDelayed || d.DelayMs != 30000 || d.Target != "rejected" {
		t.Errorf("delayed transition = %+v, want target=rejected isDelayed=true delayMs=30000", d)
	}

	if _, ok := targets["rejected"]; !ok {
		t.Errorf("the unguarded fallback to rejected was dropped; got %v", targets)
	}
}

// Native JSON is what `statekit viz` consumes from ExportJSON. Cover every
// transition shape the builder can emit so a future parser/exporter drift
// fails here instead of drawing a silently incomplete diagram.
func TestNativeExportParsesBack_AllTransitionShapes(t *testing.T) {
	m, err := statekit.NewMachine[struct{}]("shapes").
		WithInitial("gate").
		WithAction("bump", func(_ *struct{}, _ statekit.Event) {}).
		WithGuard("ok", func(_ struct{}, _ statekit.Event) bool { return true }).
		State("gate").
		Tags("transient", "entry").
		Always().Target("ready").Guard("ok").Raise("OPENED").End().
		Done().
		State("ready").
		On("KNOWN").Target("done").Do("bump").End().
		On("*").Target("done").End().
		On("TICK").Internal().Do("bump").End().
		After(5 * time.Second).Target("done").End().
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	raw, err := export.NewNativeExporter(m).ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	vm, err := viz.ParseNativeJSON([]byte(raw))
	if err != nil {
		t.Fatalf("ParseNativeJSON on native export: %v\n%s", err, raw)
	}

	gate := vm.States["gate"]
	if gate == nil {
		t.Fatal("gate missing")
	}
	if len(gate.Tags) != 2 || gate.Tags[0] != "transient" || gate.Tags[1] != "entry" {
		t.Errorf("gate.Tags = %v, want [transient entry]", gate.Tags)
	}
	if len(gate.Always) != 1 {
		t.Fatalf("gate.Always = %d, want 1", len(gate.Always))
	}
	if a := gate.Always[0]; a.Target != "ready" || a.Guard != "ok" ||
		len(a.Raise) != 1 || a.Raise[0] != "OPENED" {
		t.Errorf("always = %+v, want ready/ok/[OPENED]", a)
	}

	ready := vm.States["ready"]
	if ready == nil {
		t.Fatal("ready missing")
	}

	var (
		sawKnown, sawWild, sawInternal, sawDelayed bool
	)
	for _, tr := range ready.Transitions {
		switch {
		case tr.Event == "KNOWN" && tr.Target == "done":
			sawKnown = true
			if len(tr.Actions) != 1 || tr.Actions[0] != "bump" {
				t.Errorf("KNOWN actions = %v, want [bump]", tr.Actions)
			}
		case tr.Event == "*" && tr.Target == "done":
			sawWild = true
		case tr.Event == "TICK" && tr.Internal:
			sawInternal = true
			if tr.Target != "" {
				t.Errorf("internal TICK should have empty target, got %q", tr.Target)
			}
		case tr.IsDelayed && tr.DelayMs == 5000 && tr.Target == "done":
			sawDelayed = true
		}
	}
	if !sawKnown {
		t.Error("KNOWN transition missing after native round-trip")
	}
	if !sawWild {
		t.Error("wildcard * transition missing after native round-trip")
	}
	if !sawInternal {
		t.Error("internal TICK transition missing after native round-trip")
	}
	if !sawDelayed {
		t.Error("delayed after transition missing after native round-trip")
	}
}
