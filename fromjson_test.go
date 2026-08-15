package statekit

import (
	"testing"

	"go.klarlabs.de/statekit/export"
)

// jsonCtx is a concrete, typed context used to prove FromJSON produces a
// genuinely typed *MachineConfig[C] rather than a map[string]any machine.
type jsonCtx struct {
	Count   int
	Entered bool
}

// TestFromJSON_TypedRoundTrip exports a typed machine to Native JSON, loads it
// back through the typed core loader, and runs it — asserting that typed
// actions and guards execute against the typed context.
func TestFromJSON_TypedRoundTrip(t *testing.T) {
	t.Parallel()
	// Build a machine with the fluent builder, including a guard, transition
	// action, and an entry action so the round trip exercises wiring.
	original, err := NewMachine[jsonCtx]("counter").
		WithInitial("idle").
		WithGuard("canStart", func(ctx jsonCtx, _ Event) bool { return ctx.Count >= 0 }).
		WithAction("inc", func(ctx *jsonCtx, _ Event) { ctx.Count++ }).
		WithAction("markEntered", func(ctx *jsonCtx, _ Event) { ctx.Entered = true }).
		State("idle").
		On("START").Target("running").Guard("canStart").Do("inc").End().
		Done().
		State("running").
		OnEntry("markEntered").
		On("STOP").Target("done").End().
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build original: %v", err)
	}

	// Export to Native JSON.
	data, err := export.NewNativeExporter(original).ExportJSON()
	if err != nil {
		t.Fatalf("export JSON: %v", err)
	}

	// Re-bind named actions/guards into a typed registry and load.
	reg := NewActionRegistry[jsonCtx]().
		WithGuard("canStart", func(ctx jsonCtx, _ Event) bool { return ctx.Count >= 0 }).
		WithAction("inc", func(ctx *jsonCtx, _ Event) { ctx.Count++ }).
		WithAction("markEntered", func(ctx *jsonCtx, _ Event) { ctx.Entered = true })

	loaded, err := FromJSON[jsonCtx]([]byte(data), reg)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	// Structural assertions on the typed machine.
	if loaded.ID != "counter" {
		t.Errorf("ID = %q, want %q", loaded.ID, "counter")
	}
	if loaded.Initial != "idle" {
		t.Errorf("Initial = %q, want %q", loaded.Initial, "idle")
	}
	if len(loaded.States) != 3 {
		t.Errorf("len(States) = %d, want 3", len(loaded.States))
	}

	// Behavioural assertions: run the loaded machine and prove typed actions
	// and guards run against the typed context.
	interp := NewInterpreter(loaded)
	defer func() { _ = interp.Close() }()
	interp.Start()

	if got := interp.State().Value; got != "idle" {
		t.Fatalf("initial state = %q, want idle", got)
	}

	interp.Send(Event{Type: "START"})
	if got := interp.State().Value; got != "running" {
		t.Fatalf("after START state = %q, want running", got)
	}
	if got := interp.State().Context.Count; got != 1 {
		t.Errorf("Count = %d, want 1 (transition action did not run on typed context)", got)
	}
	if !interp.State().Context.Entered {
		t.Error("Entered = false, want true (entry action did not run on typed context)")
	}

	interp.Send(Event{Type: "STOP"})
	if got := interp.State().Value; got != "done" {
		t.Fatalf("after STOP state = %q, want done", got)
	}
	if !interp.Done() {
		t.Error("Done() = false, want true (final state not reached)")
	}
}

// TestFromJSON_UnknownActionRejected ensures the loader fails loudly when the
// JSON references a name absent from the registry, rather than silently
// installing a no-op (which would mask authoring errors in typed consumers).
func TestFromJSON_UnknownActionRejected(t *testing.T) {
	t.Parallel()
	data := []byte(`{"id":"m","initial":"a","states":{"a":{"on":{"GO":{"target":"b","actions":["missing"]}}},"b":{}}}`)

	_, err := FromJSON[jsonCtx](data, NewActionRegistry[jsonCtx]())
	if err == nil {
		t.Fatal("expected error for unregistered action, got nil")
	}
}

// TestFromJSON_NilRegistryNoReferences allows a registry-free load when the
// machine references no actions or guards (pure structural state charts).
func TestFromJSON_NilRegistryNoReferences(t *testing.T) {
	t.Parallel()
	data := []byte(`{"id":"m","initial":"a","states":{"a":{"on":{"GO":"b"}},"b":{"type":"final"}}}`)

	m, err := FromJSON[jsonCtx](data, nil)
	if err != nil {
		t.Fatalf("FromJSON with nil registry: %v", err)
	}
	if m.Initial != "a" {
		t.Errorf("Initial = %q, want a", m.Initial)
	}
}
