package export_test

import (
	"encoding/json"
	"testing"

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
	if _, ok := targets["rejected"]; !ok {
		t.Errorf("the unguarded fallback to rejected was dropped; got %v", targets)
	}
}
