//go:build ignore

// This is an example export tool demonstrating how to use statekit's CLI export
// helper. It creates machines and exports them to XState JSON format for
// visualization with tools like stately.ai/viz.
//
// Usage:
//
//	go run main.go -list                    # List available machines
//	go run main.go -pretty                  # Export all machines (pretty-printed)
//	go run main.go -machine=traffic -pretty # Export specific machine
//	go run main.go -o machines.json         # Export to file
package main

import (
	"log"
	"os"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// TrafficLightContext holds state for the traffic light machine.
type TrafficLightContext struct {
	CycleCount int
}

// OrderContext holds state for the order processing machine.
type OrderContext struct {
	OrderID string
	Total   float64
}

func main() {
	// Build machines
	trafficMachine := buildTrafficLightMachine()
	orderMachine := buildOrderMachine()

	// Create exporters
	machines := map[string]export.MachineExporter{
		"traffic": export.NewXStateExporter(trafficMachine),
		"order":   export.NewXStateExporter(orderMachine),
	}

	// Run CLI
	if err := export.RunCLI(machines, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func buildTrafficLightMachine() *ir.MachineConfig[TrafficLightContext] {
	machine, err := statekit.NewMachine[TrafficLightContext]("traffic_light").
		WithInitial("green").
		WithAction("incrementCycle", func(ctx *TrafficLightContext, e statekit.Event) {
			ctx.CycleCount++
		}).
		State("green").
		OnEntry("incrementCycle").
		On("TIMER").Target("yellow").
		Done().
		State("yellow").
		On("TIMER").Target("red").
		Done().
		State("red").
		On("TIMER").Target("green").
		Done().
		Build()
	if err != nil {
		log.Fatalf("failed to build traffic light machine: %v", err)
	}

	return machine
}

func buildOrderMachine() *ir.MachineConfig[OrderContext] {
	machine, err := statekit.NewMachine[OrderContext]("order_workflow").
		WithInitial("pending").
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return ctx.Total > 0
		}).
		WithAction("notifyCustomer", func(ctx *OrderContext, e statekit.Event) {
			// Notification logic here
		}).
		State("pending").
		On("SUBMIT").Target("payment").Guard("hasItems").
		On("CANCEL").Target("cancelled").
		Done().
		State("payment").
		On("PAYMENT_SUCCESS").Target("fulfillment").
		On("PAYMENT_FAILED").Target("payment_error").
		Done().
		State("payment_error").
		On("RETRY").Target("payment").
		On("CANCEL").Target("cancelled").
		Done().
		State("fulfillment").
		On("SHIPPED").Target("completed").
		On("REFUND").Target("refunded").
		Done().
		State("completed").
		OnEntry("notifyCustomer").
		Final().
		Done().
		State("cancelled").Final().Done().
		State("refunded").Final().Done().
		Build()
	if err != nil {
		log.Fatalf("failed to build order machine: %v", err)
	}

	return machine
}
