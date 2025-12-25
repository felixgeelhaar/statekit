// Package main demonstrates an e-commerce order workflow using the reflection DSL.
//
// This example shows:
// - Reflection DSL for machine definition
// - Guards for conditional transitions
// - Actions for side effects
// - Context for order state
// - XState export for visualization
package main

import (
	"fmt"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

// OrderContext holds the state of an order.
type OrderContext struct {
	OrderID    string
	CustomerID string
	Items      []OrderItem
	Total      float64
	PaymentID  string
	ShippingID string
}

// OrderItem represents an item in the order.
type OrderItem struct {
	SKU      string
	Name     string
	Quantity int
	Price    float64
}

// --- State Definitions using Reflection DSL ---

// OrderMachine defines the complete order workflow.
type OrderMachine struct {
	statekit.MachineDef `id:"order_workflow" initial:"pending"`

	Pending     PendingState
	Validating  statekit.StateNode `on:"VALID->payment,INVALID->cancelled" entry:"validateOrder"`
	Payment     statekit.StateNode `on:"PAID->fulfillment/recordPayment,PAYMENT_FAILED->payment_err"`
	PaymentErr  statekit.StateNode `on:"RETRY->payment,CANCEL->cancelled"`
	Fulfillment statekit.StateNode `on:"SHIPPED->completed/recordShipping,OUT_OF_STOCK->refunding"`
	Refunding   statekit.StateNode `on:"REFUNDED->refunded" entry:"processRefund"`
	Completed   statekit.FinalNode
	Cancelled   statekit.FinalNode
	Refunded    statekit.FinalNode
}

// PendingState handles the initial order state.
type PendingState struct {
	statekit.StateNode `on:"SUBMIT->validating:hasItems,CANCEL->cancelled" entry:"logPending"`
}

func main() {
	// Create the action registry
	registry := statekit.NewActionRegistry[OrderContext]().
		// Entry actions
		WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {
			fmt.Printf("Order %s is pending\n", ctx.OrderID)
		}).
		WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {
			fmt.Printf("Validating order %s with %d items\n", ctx.OrderID, len(ctx.Items))
		}).
		WithAction("processRefund", func(ctx *OrderContext, e statekit.Event) {
			fmt.Printf("Processing refund for order %s: $%.2f\n", ctx.OrderID, ctx.Total)
		}).
		// Transition actions
		WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {
			if paymentID, ok := e.Payload.(string); ok {
				ctx.PaymentID = paymentID
			}
			fmt.Printf("Payment recorded for order %s: %s\n", ctx.OrderID, ctx.PaymentID)
		}).
		WithAction("recordShipping", func(ctx *OrderContext, e statekit.Event) {
			if shippingID, ok := e.Payload.(string); ok {
				ctx.ShippingID = shippingID
			}
			fmt.Printf("Shipping recorded for order %s: %s\n", ctx.OrderID, ctx.ShippingID)
		}).
		// Guards
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return len(ctx.Items) > 0
		})

	// Build the machine with initial context
	initialContext := OrderContext{
		OrderID:    "ORD-12345",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{SKU: "WIDGET-001", Name: "Premium Widget", Quantity: 2, Price: 29.99},
			{SKU: "GADGET-002", Name: "Deluxe Gadget", Quantity: 1, Price: 49.99},
		},
		Total: 109.97,
	}

	machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](registry, initialContext)
	if err != nil {
		fmt.Printf("Failed to build machine: %v\n", err)
		return
	}

	// Export to XState JSON for visualization
	fmt.Println("=== XState JSON (paste at stately.ai/viz) ===")
	exporter := export.NewXStateExporter(machine)
	json, _ := exporter.ExportJSONIndent("", "  ")
	fmt.Println(json)
	fmt.Println()

	// Run the workflow
	fmt.Println("=== Running Order Workflow ===")
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	printState(interp)

	// Submit the order
	fmt.Println("\n→ SUBMIT")
	interp.Send(statekit.Event{Type: "SUBMIT"})
	printState(interp)

	// Order is valid
	fmt.Println("\n→ VALID")
	interp.Send(statekit.Event{Type: "VALID"})
	printState(interp)

	// Payment succeeds
	fmt.Println("\n→ PAID")
	interp.Send(statekit.Event{Type: "PAID", Payload: "PAY-98765"})
	printState(interp)

	// Order shipped
	fmt.Println("\n→ SHIPPED")
	interp.Send(statekit.Event{Type: "SHIPPED", Payload: "SHIP-54321"})
	printState(interp)

	// Check final state
	fmt.Printf("\nOrder completed: %v\n", interp.Done())
	fmt.Printf("Final context: OrderID=%s, PaymentID=%s, ShippingID=%s\n",
		interp.State().Context.OrderID,
		interp.State().Context.PaymentID,
		interp.State().Context.ShippingID)
}

func printState(interp *statekit.Interpreter[OrderContext]) {
	fmt.Printf("State: %s\n", interp.State().Value)
}
