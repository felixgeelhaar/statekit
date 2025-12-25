package main

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestIncidentLifecycle_HappyPath(t *testing.T) {
	machine := buildMachine()
	interp := statekit.NewInterpreter(machine)

	interp.UpdateContext(func(c *IncidentContext) {
		c.IncidentID = "TEST-001"
		c.Title = "Test incident"
		c.Severity = "P2"
	})

	interp.Start()
	if interp.State().Value != "triggered" {
		t.Errorf("expected triggered, got %s", interp.State().Value)
	}

	// Acknowledge
	interp.Send(statekit.Event{Type: "ACK", Payload: "responder@test.com"})
	if interp.State().Value != "investigating" {
		t.Errorf("expected investigating, got %s", interp.State().Value)
	}
	if interp.State().Context.AssignedTo != "responder@test.com" {
		t.Errorf("expected AssignedTo set")
	}

	// Resolve
	interp.Send(statekit.Event{Type: "RESOLVE"})
	if interp.State().Value != "resolved" {
		t.Errorf("expected resolved, got %s", interp.State().Value)
	}

	// Close without postmortem (blocked)
	interp.Send(statekit.Event{Type: "CLOSE"})
	if interp.State().Value != "resolved" {
		t.Errorf("expected to stay in resolved without postmortem")
	}

	// Schedule postmortem
	interp.Send(statekit.Event{Type: "SCHEDULE_POSTMORTEM"})
	if interp.State().Context.PostmortemID == "" {
		t.Error("expected PostmortemID to be set")
	}

	// Close with postmortem
	interp.Send(statekit.Event{Type: "CLOSE"})
	if interp.State().Value != "closed" {
		t.Errorf("expected closed, got %s", interp.State().Value)
	}
	if !interp.Done() {
		t.Error("expected Done() to be true")
	}
}

func TestIncidentLifecycle_Cancellation(t *testing.T) {
	machine := buildMachine()
	interp := statekit.NewInterpreter(machine)

	interp.UpdateContext(func(c *IncidentContext) {
		c.IncidentID = "TEST-002"
	})

	interp.Start()

	// Cancel from triggered (bubbles to active)
	interp.Send(statekit.Event{Type: "CANCEL"})
	if interp.State().Value != "cancelled" {
		t.Errorf("expected cancelled, got %s", interp.State().Value)
	}
	if !interp.Done() {
		t.Error("expected Done() after cancellation")
	}
}

func TestIncidentLifecycle_Escalation(t *testing.T) {
	machine := buildMachine()
	interp := statekit.NewInterpreter(machine)

	interp.UpdateContext(func(c *IncidentContext) {
		c.IncidentID = "TEST-003"
	})

	interp.Start()

	// Escalate multiple times
	interp.Send(statekit.Event{Type: "ESCALATE"})
	interp.Send(statekit.Event{Type: "ESCALATE"})

	if interp.State().Value != "triggered" {
		t.Errorf("expected triggered, got %s", interp.State().Value)
	}
	if interp.State().Context.Escalations != 2 {
		t.Errorf("expected 2 escalations, got %d", interp.State().Context.Escalations)
	}
	if interp.State().Context.NotifyCount != 3 {
		t.Errorf("expected 3 notifications (initial + 2 escalations), got %d", interp.State().Context.NotifyCount)
	}
}

func TestIncidentLifecycle_Reopen(t *testing.T) {
	machine := buildMachine()
	interp := statekit.NewInterpreter(machine)

	interp.UpdateContext(func(c *IncidentContext) {
		c.IncidentID = "TEST-004"
	})

	interp.Start()
	interp.Send(statekit.Event{Type: "ACK"})
	interp.Send(statekit.Event{Type: "RESOLVE"})

	if interp.State().Value != "resolved" {
		t.Errorf("expected resolved, got %s", interp.State().Value)
	}

	// Reopen
	interp.Send(statekit.Event{Type: "REOPEN"})
	if interp.State().Value != "investigating" {
		t.Errorf("expected investigating after reopen, got %s", interp.State().Value)
	}
}

func TestIncidentLifecycle_MatchesActive(t *testing.T) {
	machine := buildMachine()
	interp := statekit.NewInterpreter(machine)

	interp.Start()

	// Should match active (ancestor) and triggered (current)
	if !interp.Matches("active") {
		t.Error("expected Matches('active') to be true")
	}
	if !interp.Matches("triggered") {
		t.Error("expected Matches('triggered') to be true")
	}

	// Cancel (exit active)
	interp.Send(statekit.Event{Type: "CANCEL"})

	// Should no longer match active
	if interp.Matches("active") {
		t.Error("expected Matches('active') to be false after cancel")
	}
}
