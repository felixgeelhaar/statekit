// Package main demonstrates the plugin system for extending interpreter behavior.
//
// This example shows:
// - Implementing various plugin hooks
// - Logging state transitions
// - Tracking metrics (transition counts, timing)
// - Event transformation
// - Error handling
package main

import (
	"fmt"
	"strings"
	"time"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/plugin"
)

// OrderContext holds order processing state
type OrderContext struct {
	OrderID   string
	Items     int
	Total     float64
	Status    string
	UpdatedAt time.Time
}

func main() {
	machine := buildOrderMachine()

	fmt.Println("=== Plugin System Demo ===")
	fmt.Println()

	// Create interpreter with multiple plugins
	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()

	// Add plugins
	logger := NewLoggingPlugin[OrderContext]()
	metrics := NewMetricsPlugin[OrderContext]()
	transformer := NewEventTransformer[OrderContext]()

	interp.Use(logger)
	interp.Use(metrics)
	interp.Use(transformer)

	// Set initial context
	interp.UpdateContext(func(ctx *OrderContext) {
		ctx.OrderID = "ORD-12345"
		ctx.Items = 3
		ctx.Total = 99.99
	})

	// Run the order workflow
	fmt.Println("--- Starting Order Workflow ---")
	interp.Start()

	fmt.Println("\n--- Processing Events ---")
	interp.Send(statekit.Event{Type: "SUBMIT"})
	interp.Send(statekit.Event{Type: "APPROVE"})
	interp.Send(statekit.Event{Type: "SHIP"})

	fmt.Println("\n--- Stopping Interpreter ---")

	// Print metrics summary
	fmt.Println("\n--- Metrics Summary ---")
	metrics.PrintSummary()
}

func buildOrderMachine() *statekit.MachineConfig[OrderContext] {
	machine, err := statekit.NewMachine[OrderContext]("order_processor").
		WithInitial("pending").
		WithAction("updateStatus", func(ctx *OrderContext, e statekit.Event) {
			ctx.UpdatedAt = time.Now()
		}).
		WithAction("markPending", func(ctx *OrderContext, e statekit.Event) {
			ctx.Status = "pending"
		}).
		WithAction("markProcessing", func(ctx *OrderContext, e statekit.Event) {
			ctx.Status = "processing"
		}).
		WithAction("markShipped", func(ctx *OrderContext, e statekit.Event) {
			ctx.Status = "shipped"
		}).
		WithAction("markCompleted", func(ctx *OrderContext, e statekit.Event) {
			ctx.Status = "completed"
		}).
		State("pending").
		OnEntry("markPending").
		OnEntry("updateStatus").
		On("SUBMIT").Target("processing").
		Done().
		State("processing").
		OnEntry("markProcessing").
		OnEntry("updateStatus").
		On("APPROVE").Target("approved").
		On("REJECT").Target("rejected").
		Done().
		State("approved").
		OnEntry("updateStatus").
		On("SHIP").Target("shipped").
		Done().
		State("shipped").
		OnEntry("markShipped").
		OnEntry("updateStatus").
		On("DELIVER").Target("completed").
		Done().
		State("completed").
		OnEntry("markCompleted").
		OnEntry("updateStatus").
		Final().
		Done().
		State("rejected").
		OnEntry("updateStatus").
		Final().
		Done().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build machine: %v", err))
	}

	return machine
}

// --- Logging Plugin ---

// LoggingPlugin logs all state machine events for debugging
type LoggingPlugin[C any] struct {
	prefix string
}

// NewLoggingPlugin creates a new logging plugin
func NewLoggingPlugin[C any]() *LoggingPlugin[C] {
	return &LoggingPlugin[C]{prefix: "[LOG]"}
}

func (p *LoggingPlugin[C]) Name() string {
	return "logging"
}

// OnStart implements OnStartStopHook
func (p *LoggingPlugin[C]) OnStart(ctx plugin.Context[C]) {
	fmt.Printf("%s Interpreter started for machine '%s'\n", p.prefix, ctx.MachineID)
}

// OnStop implements OnStartStopHook
func (p *LoggingPlugin[C]) OnStop(ctx plugin.Context[C]) {
	fmt.Printf("%s Interpreter stopped, final state: %s\n", p.prefix, ctx.CurrentState)
}

// OnEnter implements OnStateHook
func (p *LoggingPlugin[C]) OnEnter(_ plugin.Context[C], state plugin.StateID) {
	fmt.Printf("%s → Entered state: %s\n", p.prefix, state)
}

// OnExit implements OnStateHook
func (p *LoggingPlugin[C]) OnExit(_ plugin.Context[C], state plugin.StateID) {
	fmt.Printf("%s ← Exited state: %s\n", p.prefix, state)
}

// BeforeTransition implements OnTransitionHook
func (p *LoggingPlugin[C]) BeforeTransition(_ plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
	fmt.Printf("%s Transition: %s -[%s]→ %s\n", p.prefix, from, event.Type, to)
}

// AfterTransition implements OnTransitionHook
func (p *LoggingPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
	// Optional: log after transition completes
}

// BeforeAction implements OnActionHook
func (p *LoggingPlugin[C]) BeforeAction(_ plugin.Context[C], action plugin.ActionType, _ plugin.Event) {
	fmt.Printf("%s   Running action: %s\n", p.prefix, action)
}

// AfterAction implements OnActionHook
func (p *LoggingPlugin[C]) AfterAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event) {
	// Optional: log after action completes
}

// OnError implements OnErrorHook
func (p *LoggingPlugin[C]) OnError(_ plugin.Context[C], err error) {
	fmt.Printf("%s ❌ Error: %v\n", p.prefix, err)
}

// --- Metrics Plugin ---

// MetricsPlugin tracks execution metrics
type MetricsPlugin[C any] struct {
	transitionCount int
	actionCount     int
	eventCount      int
	errorCount      int
	startTime       time.Time
	stateTime       map[plugin.StateID]time.Duration
	currentState    plugin.StateID
	stateEnterTime  time.Time
}

// NewMetricsPlugin creates a new metrics plugin
func NewMetricsPlugin[C any]() *MetricsPlugin[C] {
	return &MetricsPlugin[C]{
		stateTime: make(map[plugin.StateID]time.Duration),
	}
}

func (p *MetricsPlugin[C]) Name() string {
	return "metrics"
}

func (p *MetricsPlugin[C]) OnStart(_ plugin.Context[C]) {
	p.startTime = time.Now()
}

func (p *MetricsPlugin[C]) OnStop(_ plugin.Context[C]) {
	// Record time in final state
	if p.currentState != "" {
		p.stateTime[p.currentState] += time.Since(p.stateEnterTime)
	}
}

func (p *MetricsPlugin[C]) OnEnter(_ plugin.Context[C], state plugin.StateID) {
	p.currentState = state
	p.stateEnterTime = time.Now()
}

func (p *MetricsPlugin[C]) OnExit(_ plugin.Context[C], state plugin.StateID) {
	if p.currentState == state {
		p.stateTime[state] += time.Since(p.stateEnterTime)
	}
}

func (p *MetricsPlugin[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	p.eventCount++
	return event
}

func (p *MetricsPlugin[C]) BeforeTransition(_ plugin.Context[C], _, _ plugin.StateID, _ plugin.Event) {
	p.transitionCount++
}

func (p *MetricsPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
}

func (p *MetricsPlugin[C]) BeforeAction(_ plugin.Context[C], _ plugin.ActionType, _ plugin.Event) {
	p.actionCount++
}

func (p *MetricsPlugin[C]) AfterAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event) {
}

func (p *MetricsPlugin[C]) OnError(_ plugin.Context[C], _ error) {
	p.errorCount++
}

// PrintSummary outputs collected metrics
func (p *MetricsPlugin[C]) PrintSummary() {
	totalTime := time.Since(p.startTime)
	fmt.Printf("Total duration: %v\n", totalTime)
	fmt.Printf("Events processed: %d\n", p.eventCount)
	fmt.Printf("Transitions: %d\n", p.transitionCount)
	fmt.Printf("Actions executed: %d\n", p.actionCount)
	fmt.Printf("Errors: %d\n", p.errorCount)
	fmt.Println("Time per state:")
	for state, duration := range p.stateTime {
		fmt.Printf("  %s: %v\n", state, duration)
	}
}

// --- Event Transformer Plugin ---

// EventTransformer normalizes and validates events
type EventTransformer[C any] struct{}

// NewEventTransformer creates a new event transformer
func NewEventTransformer[C any]() *EventTransformer[C] {
	return &EventTransformer[C]{}
}

func (p *EventTransformer[C]) Name() string {
	return "event-transformer"
}

// OnEvent normalizes event types to uppercase
func (p *EventTransformer[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	// Normalize event type to uppercase
	normalized := plugin.EventType(strings.ToUpper(string(event.Type)))
	if normalized != event.Type {
		fmt.Printf("[TRANSFORM] Normalized event: %s → %s\n", event.Type, normalized)
		event.Type = normalized
	}
	return event
}
