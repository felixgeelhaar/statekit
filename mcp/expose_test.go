package mcp

import (
	"testing"

	mcpgo "github.com/felixgeelhaar/mcp-go"
	"github.com/felixgeelhaar/mcp-go/server"

	"github.com/felixgeelhaar/statekit"
)

type exposeCtx struct {
	Hits int `json:"hits"`
}

func newTestInterp(t *testing.T) *statekit.Interpreter[exposeCtx] {
	t.Helper()
	machine, err := statekit.NewMachine[exposeCtx]("expose-test").
		WithInitial("idle").
		WithAction("incr", func(c *exposeCtx, e statekit.Event) { c.Hits++ }).
		State("idle").
		On("GO").Target("running").
		Done().
		State("running").
		OnEntry("incr").
		On("STOP").Target("idle").
		On("FINISH").Target("done").
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	t.Cleanup(func() { _ = interp.Close() })
	return interp
}

func newTestServer() *server.Server {
	return mcpgo.NewServer(server.Info{
		Name:    "expose-test",
		Version: "0.0.1",
		Capabilities: server.Capabilities{
			Tools:     true,
			Resources: false,
		},
	})
}

// TestExposeInterpreter_Registration verifies the helper does not
// panic and accepts a typed interpreter without explicit type args.
func TestExposeInterpreter_Registration(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	interp := newTestInterp(t)

	// Type inference must work: no explicit [exposeCtx] needed.
	ExposeInterpreter(srv, "tl", interp)
}

// TestExposeInterpreter_DriveMachine drives the interpreter through
// transitions to confirm the registered tools observe live state.
func TestExposeInterpreter_DriveMachine(t *testing.T) {
	t.Parallel()
	srv := newTestServer()
	interp := newTestInterp(t)
	ExposeInterpreter(srv, "tl", interp)

	if got := string(interp.State().Value); got != "idle" {
		t.Fatalf("initial state = %q, want idle", got)
	}

	interp.Send(statekit.Event{Type: "GO"})
	if got := string(interp.State().Value); got != "running" {
		t.Fatalf("after GO: state = %q, want running", got)
	}
	if interp.State().Context.Hits != 1 {
		t.Errorf("Hits after entry = %d, want 1", interp.State().Context.Hits)
	}

	interp.Send(statekit.Event{Type: "FINISH"})
	if !interp.Done() {
		t.Errorf("expected Done after FINISH")
	}
}

// TestExposeInterpreter_ContextTyped confirms the context is exposed
// with the correct concrete type, not erased to any.
func TestExposeInterpreter_ContextTyped(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	out := ExposeContextOutput[exposeCtx]{Context: interp.State().Context}
	if out.Context.Hits != 0 {
		t.Errorf("initial Hits = %d, want 0", out.Context.Hits)
	}

	interp.Send(statekit.Event{Type: "GO"})
	out = ExposeContextOutput[exposeCtx]{Context: interp.State().Context}
	if out.Context.Hits != 1 {
		t.Errorf("Hits after GO = %d, want 1", out.Context.Hits)
	}
}

// TestExposeInterpreter_Matches verifies the matches tool uses the
// hierarchical Matches semantics (matches current state or ancestor).
func TestExposeInterpreter_Matches(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	if !interp.Matches("idle") {
		t.Error("expected Matches(idle) at start")
	}
	if interp.Matches("running") {
		t.Error("did not expect Matches(running) at start")
	}
	if interp.Matches("nonexistent") {
		t.Error("did not expect Matches(nonexistent)")
	}
}

func TestExposeSendEvent(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	out := ExposeSendEvent(interp, ExposeInput{Event: "GO"})
	if out.PreviousState != "idle" {
		t.Errorf("PreviousState = %q, want idle", out.PreviousState)
	}
	if out.CurrentState != "running" {
		t.Errorf("CurrentState = %q, want running", out.CurrentState)
	}
	if !out.Transitioned {
		t.Error("expected Transitioned=true")
	}
	if out.Done {
		t.Error("did not expect Done after GO")
	}

	// Non-matching event — no transition, but reported truthfully.
	out = ExposeSendEvent(interp, ExposeInput{Event: "UNKNOWN"})
	if out.Transitioned {
		t.Error("expected Transitioned=false for unknown event")
	}

	// Payload is forwarded.
	out = ExposeSendEvent(interp, ExposeInput{
		Event:   "FINISH",
		Payload: map[string]any{"reason": "test"},
	})
	if !out.Done {
		t.Error("expected Done after FINISH")
	}
}

func TestExposeGetState(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	out := ExposeGetState(interp)
	if out.CurrentState != "idle" {
		t.Errorf("initial state = %q, want idle", out.CurrentState)
	}
	if out.Done {
		t.Error("did not expect Done at start")
	}

	interp.Send(statekit.Event{Type: "GO"})
	interp.Send(statekit.Event{Type: "FINISH"})
	out = ExposeGetState(interp)
	if !out.Done {
		t.Error("expected Done after FINISH")
	}
}

func TestExposeGetContext(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	out := ExposeGetContext(interp)
	if out.Context.Hits != 0 {
		t.Errorf("initial Hits = %d, want 0", out.Context.Hits)
	}

	interp.Send(statekit.Event{Type: "GO"})
	out = ExposeGetContext(interp)
	if out.Context.Hits != 1 {
		t.Errorf("Hits after GO = %d, want 1", out.Context.Hits)
	}
}

func TestExposeMatches(t *testing.T) {
	t.Parallel()
	interp := newTestInterp(t)

	out := ExposeMatches(interp, ExposeMatchInput{StateID: "idle"})
	if !out.Matches {
		t.Error("expected match for idle at start")
	}

	out = ExposeMatches(interp, ExposeMatchInput{StateID: "running"})
	if out.Matches {
		t.Error("did not expect match for running at start")
	}
}
