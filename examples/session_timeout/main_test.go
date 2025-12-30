package main

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/statekit"
)

func TestSessionTimeout_BasicFlow(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "test"
	})
	interp.Start()
	defer interp.Stop()

	// Should start in active
	if interp.State().Value != "active" {
		t.Errorf("Expected 'active', got %s", interp.State().Value)
	}

	// Wait for warning
	time.Sleep(60 * time.Millisecond)
	if interp.State().Value != "warning" {
		t.Errorf("Expected 'warning', got %s", interp.State().Value)
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)
	if interp.State().Value != "expired" {
		t.Errorf("Expected 'expired', got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("Expected session to be done")
	}
}

func TestSessionTimeout_ActivityResets(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "test"
	})
	interp.Start()
	defer interp.Stop()

	// Activity before warning
	time.Sleep(30 * time.Millisecond)
	interp.Send(statekit.Event{Type: "ACTIVITY"})

	// Should still be active
	if interp.State().Value != "active" {
		t.Errorf("Expected 'active', got %s", interp.State().Value)
	}

	// Wait, should now get warning (timer restarted)
	time.Sleep(60 * time.Millisecond)
	if interp.State().Value != "warning" {
		t.Errorf("Expected 'warning' after reset timer, got %s", interp.State().Value)
	}
}

func TestSessionTimeout_ActivityInWarning(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "test"
	})
	interp.Start()
	defer interp.Stop()

	// Wait for warning
	time.Sleep(60 * time.Millisecond)
	if interp.State().Value != "warning" {
		t.Fatalf("Expected 'warning', got %s", interp.State().Value)
	}

	// Activity in warning should return to active
	interp.Send(statekit.Event{Type: "ACTIVITY"})
	if interp.State().Value != "active" {
		t.Errorf("Expected 'active' after activity, got %s", interp.State().Value)
	}
}

func TestSessionTimeout_ExplicitLogout(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "test"
	})
	interp.Start()
	defer interp.Stop()

	// Immediate logout
	interp.Send(statekit.Event{Type: "LOGOUT"})
	if interp.State().Value != "ended" {
		t.Errorf("Expected 'ended', got %s", interp.State().Value)
	}

	ctx := interp.State().Context
	if ctx.TimeoutReason != "logout" {
		t.Errorf("Expected reason 'logout', got %s", ctx.TimeoutReason)
	}
}

func TestSessionTimeout_VIPNoWarning(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "vip"
		ctx.IsVIP = true
	})
	interp.Start()
	defer interp.Stop()

	// Wait past warning time
	time.Sleep(60 * time.Millisecond)

	// VIP should still be active (auto-extended)
	if interp.State().Value != "active" {
		t.Errorf("Expected 'active' for VIP, got %s", interp.State().Value)
	}
}

func TestSessionTimeout_StayInWarning(t *testing.T) {
	machine := buildSessionMachine(50*time.Millisecond, 100*time.Millisecond)
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "test"
	})
	interp.Start()
	defer interp.Stop()

	// Wait for warning
	time.Sleep(60 * time.Millisecond)
	if interp.State().Value != "warning" {
		t.Fatalf("Expected 'warning', got %s", interp.State().Value)
	}

	// Explicit stay request
	interp.Send(statekit.Event{Type: "STAY"})
	if interp.State().Value != "active" {
		t.Errorf("Expected 'active' after STAY, got %s", interp.State().Value)
	}
}
