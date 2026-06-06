package export_test

import (
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/export"
)

func TestExport_WildcardAndInternal(t *testing.T) {
	m, err := statekit.NewMachine[struct{}]("w").
		WithInitial("a").
		WithAction("noop", func(_ *struct{}, _ statekit.Event) {}).
		State("a").
		On("KNOWN").Target("b").End().
		On("*").Target("b").End().
		On("PING").Internal().Do("noop").End().
		Done().
		State("b").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Native export: wildcard event preserved; internal flag set.
	vm := export.NewNativeExporter(m).Export()
	a := vm.States["a"]
	var sawWildcard, sawInternal bool
	for _, tr := range a.Transitions {
		if tr.Event == "*" {
			sawWildcard = true
		}
		if tr.Event == "PING" && tr.Internal {
			sawInternal = true
		}
	}
	if !sawWildcard {
		t.Errorf("native: wildcard transition not exported")
	}
	if !sawInternal {
		t.Errorf("native: internal flag not exported")
	}

	// XState export: "*" is a valid event key; internal transition carries
	// the internal flag and omits target.
	out := export.NewXStateExporter(m).Export()
	on := out["states"].(map[string]any)["a"].(map[string]any)["on"].(map[string]any)
	if _, ok := on["*"]; !ok {
		t.Errorf("xstate: missing wildcard '*' handler")
	}
	ping, ok := on["PING"].(map[string]any)
	if !ok {
		t.Fatalf("xstate: PING entry = %v", on["PING"])
	}
	if ping["internal"] != true {
		t.Errorf("xstate: PING.internal = %v, want true", ping["internal"])
	}
	if _, hasTarget := ping["target"]; hasTarget {
		t.Errorf("xstate: internal PING should omit target, got %v", ping["target"])
	}
}
