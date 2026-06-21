package statekit

import "testing"

// TestSendResult_DistinguishesBlockedFromApplied verifies that SendResult
// reports false when an event is unhandled (no matching transition, or a guard
// blocks it) and true when a transition actually fires — the signal callers
// need to tell a silently-rejected transition from an applied one.
func TestSendResult_DistinguishesBlockedFromApplied(t *testing.T) {
	t.Parallel()
	type ctx struct{ Count int }

	machine, err := NewMachine[ctx]("sendresult").
		WithInitial("idle").
		WithGuard("hasCount", func(c ctx, _ Event) bool { return c.Count > 0 }).
		State("idle").On("GO").Target("active").Guard("hasCount").Done().
		State("active").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Guard blocks (Count == 0): not handled, state unchanged.
	if interp.SendResult(Event{Type: "GO"}) {
		t.Error("SendResult = true for a guard-blocked event, want false")
	}
	if interp.State().Value != "idle" {
		t.Errorf("state = %v after blocked event, want idle", interp.State().Value)
	}

	// Unknown event: not handled.
	if interp.SendResult(Event{Type: "NOPE"}) {
		t.Error("SendResult = true for an unmatched event, want false")
	}

	// Guard passes: handled, transition fires.
	interp.UpdateContext(func(c *ctx) { c.Count = 1 })
	if !interp.SendResult(Event{Type: "GO"}) {
		t.Error("SendResult = false for a firing transition, want true")
	}
	if interp.State().Value != "active" {
		t.Errorf("state = %v after GO, want active", interp.State().Value)
	}
}
