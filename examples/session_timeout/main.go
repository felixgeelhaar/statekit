// Package main demonstrates delayed (timed) transitions with a session timeout.
//
// This example shows:
// - Automatic transitions after a duration
// - Cancelling timers via manual transitions
// - Guards on delayed transitions
// - Actions on delayed transitions
// - Multiple delayed transitions (first one wins)
package main

import (
	"fmt"
	"time"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

// SessionContext holds session state
type SessionContext struct {
	UserID        string
	LastActivity  time.Time
	WarningCount  int
	IsVIP         bool
	SessionStart  time.Time
	TimeoutReason string
}

func main() {
	// Use short timeouts for demo purposes
	machine := buildSessionMachine(
		200*time.Millisecond, // warning timeout
		400*time.Millisecond, // expiry timeout
	)

	// Export to Statekit Native JSON for visualization
	fmt.Println("=== Statekit Native JSON ===")
	exporter := export.NewNativeExporter(machine)
	json, _ := exporter.ExportJSONIndent("", "  ")
	fmt.Println(json)
	fmt.Println()

	// Demo scenarios
	fmt.Println("=== Session Timeout Demo ===")
	runActiveUserDemo(machine)
	runTimeoutDemo(machine)
	runVIPDemo(machine)
}

func buildSessionMachine(warningTime, expiryTime time.Duration) *statekit.MachineConfig[SessionContext] {
	machine, err := statekit.NewMachine[SessionContext]("session").
		WithInitial("active").
		// Actions
		WithAction("recordStart", func(ctx *SessionContext, e statekit.Event) {
			ctx.SessionStart = time.Now()
			ctx.LastActivity = time.Now()
			fmt.Printf("[Session] Started for user %s\n", ctx.UserID)
		}).
		WithAction("recordActivity", func(ctx *SessionContext, e statekit.Event) {
			ctx.LastActivity = time.Now()
			fmt.Printf("[Session] Activity recorded at %s\n", ctx.LastActivity.Format("15:04:05.000"))
		}).
		WithAction("showWarning", func(ctx *SessionContext, e statekit.Event) {
			ctx.WarningCount++
			fmt.Printf("[Session] ⚠️  Warning #%d: Session about to expire!\n", ctx.WarningCount)
		}).
		WithAction("markExpired", func(ctx *SessionContext, e statekit.Event) {
			ctx.TimeoutReason = "inactivity"
			fmt.Println("[Session] ❌ Session expired due to inactivity")
		}).
		WithAction("markLoggedOut", func(ctx *SessionContext, e statekit.Event) {
			ctx.TimeoutReason = "logout"
			fmt.Println("[Session] 👋 User logged out")
		}).
		WithAction("extendVIP", func(ctx *SessionContext, e statekit.Event) {
			fmt.Println("[Session] ⭐ VIP session extended automatically")
		}).
		// Guards
		WithGuard("isVIP", func(ctx SessionContext, e statekit.Event) bool {
			return ctx.IsVIP
		}).
		WithGuard("notVIP", func(ctx SessionContext, e statekit.Event) bool {
			return !ctx.IsVIP
		}).
		// Active state with delayed transitions
		State("active").
		OnEntry("recordStart").
		On("ACTIVITY").Target("active").Do("recordActivity").
		On("LOGOUT").Target("ended").Do("markLoggedOut").
		After(warningTime).Target("warning").Guard("notVIP").               // Regular users get warning
		After(warningTime).Target("active").Guard("isVIP").Do("extendVIP"). // VIP users auto-extend
		Done().
		// Warning state - user has limited time to respond
		State("warning").
		OnEntry("showWarning").
		On("ACTIVITY").Target("active").Do("recordActivity"). // Activity resets to active
		On("STAY").Target("active").                          // Explicit stay request
		On("LOGOUT").Target("ended").Do("markLoggedOut").
		After(expiryTime - warningTime).Target("expired"). // Time remaining after warning
		Done().
		// Expired state (final)
		State("expired").
		OnEntry("markExpired").
		Final().
		Done().
		// Ended state (final - explicit logout)
		State("ended").
		Final().
		Done().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build machine: %v", err))
	}

	return machine
}

func runActiveUserDemo(machine *statekit.MachineConfig[SessionContext]) {
	fmt.Println("\n--- Scenario 1: Active User ---")

	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "alice"
	})
	interp.Start()

	fmt.Println("User active, sending activity before warning...")
	time.Sleep(100 * time.Millisecond)
	interp.Send(statekit.Event{Type: "ACTIVITY"})

	fmt.Println("Waiting to see if warning triggers...")
	time.Sleep(150 * time.Millisecond)

	state := interp.State()
	fmt.Printf("Result: state=%s (activity reset the timer)\n", state.Value)

	// Logout to clean up
	interp.Send(statekit.Event{Type: "LOGOUT"})
	interp.Stop()
}

func runTimeoutDemo(machine *statekit.MachineConfig[SessionContext]) {
	fmt.Println("\n--- Scenario 2: Timeout Flow ---")

	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "bob"
	})
	interp.Start()

	fmt.Println("User idle, waiting for warning...")
	time.Sleep(250 * time.Millisecond)

	state := interp.State()
	fmt.Printf("After warning time: state=%s\n", state.Value)

	fmt.Println("User still idle, waiting for expiry...")
	time.Sleep(250 * time.Millisecond)

	state = interp.State()
	fmt.Printf("After expiry time: state=%s, reason=%s\n", state.Value, state.Context.TimeoutReason)
	fmt.Printf("Session complete: %v\n", interp.Done())

	interp.Stop()
}

func runVIPDemo(machine *statekit.MachineConfig[SessionContext]) {
	fmt.Println("\n--- Scenario 3: VIP Auto-Extend ---")

	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *SessionContext) {
		ctx.UserID = "vip_charlie"
		ctx.IsVIP = true
	})
	interp.Start()

	fmt.Println("VIP user idle, waiting past warning time...")
	time.Sleep(250 * time.Millisecond)

	state := interp.State()
	fmt.Printf("After warning time: state=%s (VIP was auto-extended)\n", state.Value)

	// VIP continues without warning
	fmt.Println("VIP still active, logging out...")
	interp.Send(statekit.Event{Type: "LOGOUT"})

	state = interp.State()
	fmt.Printf("After logout: state=%s\n", state.Value)

	interp.Stop()
}
