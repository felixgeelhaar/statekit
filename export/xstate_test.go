package export_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

func TestXStateExporter_Atomic(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("traffic").
		WithInitial("green").
		State("green").On("TIMER").Target("yellow").Done().
		State("yellow").On("TIMER").Target("red").Done().
		State("red").On("TIMER").Target("green").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	exp := export.NewXStateExporter(machine)
	out := exp.Export()

	if out["id"] != "traffic" {
		t.Errorf("id = %v, want traffic", out["id"])
	}
	if out["initial"] != "green" {
		t.Errorf("initial = %v, want green", out["initial"])
	}

	states, ok := out["states"].(map[string]any)
	if !ok {
		t.Fatalf("states is not a map: %T", out["states"])
	}
	green, ok := states["green"].(map[string]any)
	if !ok {
		t.Fatalf("green missing")
	}
	on, ok := green["on"].(map[string]any)
	if !ok {
		t.Fatalf("green.on missing")
	}
	timer, ok := on["TIMER"].(map[string]any)
	if !ok {
		t.Fatalf("green.on.TIMER not an object: %T", on["TIMER"])
	}
	if timer["target"] != "yellow" {
		t.Errorf("green.on.TIMER.target = %v, want yellow", timer["target"])
	}
}

func TestXStateExporter_Final(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("flow").
		WithInitial("a").
		State("a").On("DONE").Target("end").Done().
		State("end").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	end := states["end"].(map[string]any)
	if end["type"] != "final" {
		t.Errorf("end.type = %v, want final", end["type"])
	}
}

func TestXStateExporter_Compound_Nests(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("hier").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").On("GO").Target("running").End().End().
		State("running").On("STOP").Target("idle").End().End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	active, ok := states["active"].(map[string]any)
	if !ok {
		t.Fatalf("active missing")
	}
	if active["initial"] != "idle" {
		t.Errorf("active.initial = %v, want idle", active["initial"])
	}
	nested, ok := active["states"].(map[string]any)
	if !ok {
		t.Fatalf("active.states missing")
	}
	if _, ok := nested["idle"]; !ok {
		t.Error("nested idle missing")
	}
	if _, ok := nested["running"]; !ok {
		t.Error("nested running missing")
	}
}

func TestXStateExporter_DelayedTransition(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("timeout").
		WithInitial("loading").
		State("loading").
		After(5 * time.Second).Target("done").
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	loading := states["loading"].(map[string]any)
	after, ok := loading["after"].(map[string]any)
	if !ok {
		t.Fatalf("loading.after missing")
	}
	entry, ok := after["5000"].(map[string]any)
	if !ok {
		t.Fatalf(`loading.after["5000"] missing or wrong type: %T`, after["5000"])
	}
	if entry["target"] != "done" {
		t.Errorf("after.5000.target = %v, want done", entry["target"])
	}
}

func TestXStateExporter_Guards_Actions(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("ga").
		WithInitial("a").
		WithAction("logIt", func(_ *struct{}, _ statekit.Event) {}).
		WithGuard("ok", func(_ struct{}, _ statekit.Event) bool { return true }).
		State("a").
		OnEntry("logIt").
		On("X").Target("b").Guard("ok").Do("logIt").
		Done().
		State("b").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	a := states["a"].(map[string]any)
	entry, ok := a["entry"].([]string)
	if !ok || len(entry) == 0 || entry[0] != "logIt" {
		t.Errorf("a.entry = %v", a["entry"])
	}
	on := a["on"].(map[string]any)
	x := on["X"].(map[string]any)
	if x["guard"] != "ok" {
		t.Errorf("guard = %v, want ok", x["guard"])
	}
	if actions, ok := x["actions"].([]string); !ok || len(actions) == 0 || actions[0] != "logIt" {
		t.Errorf("actions = %v", x["actions"])
	}
}

func TestXStateExporter_MultipleTransitionsCollapseToArray(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("multi").
		WithInitial("a").
		WithGuard("g1", func(_ struct{}, _ statekit.Event) bool { return true }).
		WithGuard("g2", func(_ struct{}, _ statekit.Event) bool { return false }).
		State("a").
		On("X").Target("b").Guard("g1").
		On("X").Target("c").Guard("g2").
		Done().
		State("b").Done().
		State("c").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	a := states["a"].(map[string]any)
	on := a["on"].(map[string]any)
	arr, ok := on["X"].([]any)
	if !ok {
		t.Fatalf("on.X = %T, want []any (multiple transitions)", on["X"])
	}
	if len(arr) != 2 {
		t.Errorf("on.X length = %d, want 2", len(arr))
	}
}

func TestXStateExporter_ExportJSON_Compact(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("c").
		WithInitial("a").
		State("a").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	got, err := export.NewXStateExporter(machine).ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty JSON")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestXStateExporter_HistoryState(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("h").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").On("GO").Target("done").End().End().
		State("done").Final().End().
		History("hist").Shallow().Default("idle").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	active := states["active"].(map[string]any)
	nested := active["states"].(map[string]any)
	hist, ok := nested["hist"].(map[string]any)
	if !ok {
		t.Fatalf("hist state missing")
	}
	if hist["type"] != "history" {
		t.Errorf("hist.type = %v, want history", hist["type"])
	}
	if hist["history"] != "shallow" {
		t.Errorf("hist.history = %v, want shallow", hist["history"])
	}
	if hist["target"] != "idle" {
		t.Errorf("hist.target = %v, want idle (default)", hist["target"])
	}
}

func TestXStateExporter_Invoke(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("inv").
		WithInitial("loading").
		WithService("fetch", func(_ statekit.ServiceContext[struct{}]) error { return nil }).
		State("loading").
		Invoke("fetch").ID("f1").OnDone("ready").OnError("failed").End().
		Done().
		State("ready").Final().Done().
		State("failed").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	out := export.NewXStateExporter(machine).Export()
	states := out["states"].(map[string]any)
	loading := states["loading"].(map[string]any)
	invokes, ok := loading["invoke"].([]map[string]any)
	if !ok || len(invokes) == 0 {
		t.Fatalf("loading.invoke missing or wrong type: %T", loading["invoke"])
	}
	if invokes[0]["src"] != "fetch" {
		t.Errorf("invoke.src = %v, want fetch", invokes[0]["src"])
	}
	if invokes[0]["onDone"] != "ready" {
		t.Errorf("invoke.onDone = %v, want ready", invokes[0]["onDone"])
	}
}

func TestXStateExporter_JSON_RoundTripShape(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("rt").
		WithInitial("idle").
		State("idle").On("GO").Target("done").Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	jsonStr, err := export.NewXStateExporter(machine).ExportJSONIndent("", "  ")
	if err != nil {
		t.Fatalf("ExportJSONIndent: %v", err)
	}

	// Re-parse to confirm valid JSON of the shape XState parsers expect.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, jsonStr)
	}
	if parsed["id"] != "rt" {
		t.Errorf("parsed id = %v", parsed["id"])
	}
}
