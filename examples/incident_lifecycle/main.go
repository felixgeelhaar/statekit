// Package main demonstrates an SRE incident lifecycle using hierarchical states.
//
// This example shows:
// - Hierarchical states for incident phases
// - Event bubbling for escalation
// - Guards for state-based decisions
// - Actions for notifications and logging
// - XState export for visualization
package main

import (
	"fmt"
	"time"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// IncidentContext holds incident state data.
type IncidentContext struct {
	IncidentID   string
	Title        string
	Severity     string
	AssignedTo   string
	CreatedAt    time.Time
	AckedAt      time.Time
	ResolvedAt   time.Time
	Escalations  int
	NotifyCount  int
	PostmortemID string
}

func main() {
	machine := buildMachine()

	// Export to XState JSON
	fmt.Println("=== XState JSON (paste at stately.ai/viz) ===")
	exporter := export.NewXStateExporter(machine)
	json, _ := exporter.ExportJSONIndent("", "  ")
	fmt.Println(json)
	fmt.Println()

	// Run a sample incident lifecycle
	fmt.Println("=== Incident Lifecycle Demo ===")
	runDemo(machine)
}

func buildMachine() *ir.MachineConfig[IncidentContext] {
	machine, err := statekit.NewMachine[IncidentContext]("incident_lifecycle").
		WithInitial("active").
		// Actions
		WithAction("createIncident", func(ctx *IncidentContext, e statekit.Event) {
			ctx.CreatedAt = time.Now()
			fmt.Printf("[%s] Incident created: %s (%s)\n", ctx.IncidentID, ctx.Title, ctx.Severity)
		}).
		WithAction("notifyOnCall", func(ctx *IncidentContext, e statekit.Event) {
			ctx.NotifyCount++
			fmt.Printf("[%s] Notifying on-call engineer (attempt %d)\n", ctx.IncidentID, ctx.NotifyCount)
		}).
		WithAction("acknowledge", func(ctx *IncidentContext, e statekit.Event) {
			ctx.AckedAt = time.Now()
			if responder, ok := e.Payload.(string); ok {
				ctx.AssignedTo = responder
			}
			fmt.Printf("[%s] Acknowledged by %s\n", ctx.IncidentID, ctx.AssignedTo)
		}).
		WithAction("escalate", func(ctx *IncidentContext, e statekit.Event) {
			ctx.Escalations++
			fmt.Printf("[%s] Escalated (level %d)\n", ctx.IncidentID, ctx.Escalations)
		}).
		WithAction("resolve", func(ctx *IncidentContext, e statekit.Event) {
			ctx.ResolvedAt = time.Now()
			fmt.Printf("[%s] Resolved\n", ctx.IncidentID)
		}).
		WithAction("schedulePostmortem", func(ctx *IncidentContext, e statekit.Event) {
			ctx.PostmortemID = fmt.Sprintf("PM-%s", ctx.IncidentID)
			fmt.Printf("[%s] Postmortem scheduled: %s\n", ctx.IncidentID, ctx.PostmortemID)
		}).
		WithAction("closeIncident", func(ctx *IncidentContext, e statekit.Event) {
			fmt.Printf("[%s] Incident closed\n", ctx.IncidentID)
		}).
		// Guards
		WithGuard("isSevere", func(ctx IncidentContext, e statekit.Event) bool {
			return ctx.Severity == "P1" || ctx.Severity == "P2"
		}).
		WithGuard("hasPostmortem", func(ctx IncidentContext, e statekit.Event) bool {
			return ctx.PostmortemID != ""
		}).
		// States
		State("active").
		WithInitial("triggered").
		OnEntry("createIncident").
		On("CANCEL").Target("cancelled").End().
		// Triggered state - waiting for acknowledgment
		State("triggered").
		OnEntry("notifyOnCall").
		On("ACK").Target("investigating").Do("acknowledge").
		On("ESCALATE").Target("triggered").Do("escalate").
		End(). // End transition, return to triggered StateBuilder
		End(). // End triggered, return to active StateBuilder
		// Investigating state
		State("investigating").
		On("RESOLVE").Target("resolved").Do("resolve").
		On("ESCALATE").Target("investigating").Do("escalate").
		End(). // End transition
		End(). // End investigating
		// Resolved state
		State("resolved").
		On("REOPEN").Target("investigating").
		On("CLOSE").Target("closed").Guard("hasPostmortem").
		On("SCHEDULE_POSTMORTEM").Target("resolved").Do("schedulePostmortem").
		End(). // End transition
		End(). // End resolved
		Done().
		// Final states
		State("closed").
		OnEntry("closeIncident").
		Final().
		Done().
		State("cancelled").
		Final().
		Done().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build machine: %v", err))
	}

	return machine
}

func runDemo(machine *ir.MachineConfig[IncidentContext]) {
	ctx := IncidentContext{
		IncidentID: "INC-001",
		Title:      "Database connection pool exhausted",
		Severity:   "P1",
	}

	// Create interpreter with context
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(c *IncidentContext) {
		*c = ctx
	})

	// Start incident
	interp.Start()
	printState(interp)

	// Escalate once before ack
	fmt.Println("\n→ ESCALATE (no response)")
	interp.Send(statekit.Event{Type: "ESCALATE"})
	printState(interp)

	// Acknowledge
	fmt.Println("\n→ ACK")
	interp.Send(statekit.Event{Type: "ACK", Payload: "alice@example.com"})
	printState(interp)

	// Resolve
	fmt.Println("\n→ RESOLVE")
	interp.Send(statekit.Event{Type: "RESOLVE"})
	printState(interp)

	// Try to close without postmortem
	fmt.Println("\n→ CLOSE (blocked - no postmortem)")
	interp.Send(statekit.Event{Type: "CLOSE"})
	printState(interp)

	// Schedule postmortem
	fmt.Println("\n→ SCHEDULE_POSTMORTEM")
	interp.Send(statekit.Event{Type: "SCHEDULE_POSTMORTEM"})
	printState(interp)

	// Now close
	fmt.Println("\n→ CLOSE")
	interp.Send(statekit.Event{Type: "CLOSE"})
	printState(interp)

	fmt.Printf("\nIncident completed: %v\n", interp.Done())
}

func printState(interp *statekit.Interpreter[IncidentContext]) {
	state := interp.State()
	fmt.Printf("State: %s | Assigned: %s | Escalations: %d\n",
		state.Value, state.Context.AssignedTo, state.Context.Escalations)
}
