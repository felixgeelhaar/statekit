package export_test

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

func buildFeatureMachine(t *testing.T) *statekit.MachineConfig[struct{}] {
	t.Helper()
	m, err := statekit.NewMachine[struct{}]("features").
		WithInitial("gate").
		State("gate").
		Tags("transient", "auto").
		Always().Target("done").Raise("ARRIVED").End().
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return m
}

func TestNativeExport_IncludesAlwaysTagsRaise(t *testing.T) {
	m := buildFeatureMachine(t)
	vm := export.NewNativeExporter(m).Export()

	gate := vm.States["gate"]
	if gate == nil {
		t.Fatal("missing gate state")
	}
	if len(gate.Tags) != 2 || gate.Tags[0] != "transient" {
		t.Errorf("tags = %v, want [transient auto]", gate.Tags)
	}
	if len(gate.Always) != 1 {
		t.Fatalf("always count = %d, want 1", len(gate.Always))
	}
	al := gate.Always[0]
	if al.Target != "done" {
		t.Errorf("always target = %q, want done", al.Target)
	}
	if len(al.Raise) != 1 || al.Raise[0] != "ARRIVED" {
		t.Errorf("always raise = %v, want [ARRIVED]", al.Raise)
	}
}

func TestXStateExport_IncludesAlwaysTagsRaise(t *testing.T) {
	m := buildFeatureMachine(t)
	out := export.NewXStateExporter(m).Export()

	states := out["states"].(map[string]any)
	gate := states["gate"].(map[string]any)

	// tags
	tags, ok := gate["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "transient" {
		t.Errorf("tags = %v", gate["tags"])
	}

	// always (single entry collapses to an object)
	always, ok := gate["always"].(map[string]any)
	if !ok {
		t.Fatalf("always = %v (want object)", gate["always"])
	}
	if always["target"] != "done" {
		t.Errorf("always.target = %v, want done", always["target"])
	}

	// raise surfaces as an xstate.raise action descriptor
	actions, ok := always["actions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("always.actions = %v (want 1 raise descriptor)", always["actions"])
	}
	raise := actions[0].(map[string]any)
	if raise["type"] != "xstate.raise" {
		t.Errorf("action type = %v, want xstate.raise", raise["type"])
	}
	evt := raise["event"].(map[string]any)
	if evt["type"] != "ARRIVED" {
		t.Errorf("raised event = %v, want ARRIVED", evt["type"])
	}
}
