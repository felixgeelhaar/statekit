package main

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestOrderWorkflow_HappyPath(t *testing.T) {
	registry := statekit.NewActionRegistry[OrderContext]().
		WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("processRefund", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {
			if paymentID, ok := e.Payload.(string); ok {
				ctx.PaymentID = paymentID
			}
		}).
		WithAction("recordShipping", func(ctx *OrderContext, e statekit.Event) {
			if shippingID, ok := e.Payload.(string); ok {
				ctx.ShippingID = shippingID
			}
		}).
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return len(ctx.Items) > 0
		})

	ctx := OrderContext{
		OrderID: "TEST-001",
		Items:   []OrderItem{{SKU: "TEST", Name: "Test Item", Quantity: 1, Price: 10.00}},
		Total:   10.00,
	}

	machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](registry, ctx)
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "pending" {
		t.Errorf("expected pending, got %s", interp.State().Value)
	}

	// Submit order
	interp.Send(statekit.Event{Type: "SUBMIT"})
	if interp.State().Value != "validating" {
		t.Errorf("expected validating, got %s", interp.State().Value)
	}

	// Validate
	interp.Send(statekit.Event{Type: "VALID"})
	if interp.State().Value != "payment" {
		t.Errorf("expected payment, got %s", interp.State().Value)
	}

	// Pay
	interp.Send(statekit.Event{Type: "PAID", Payload: "PAY-123"})
	if interp.State().Value != "fulfillment" {
		t.Errorf("expected fulfillment, got %s", interp.State().Value)
	}
	if interp.State().Context.PaymentID != "PAY-123" {
		t.Errorf("expected PaymentID PAY-123, got %s", interp.State().Context.PaymentID)
	}

	// Ship
	interp.Send(statekit.Event{Type: "SHIPPED", Payload: "SHIP-456"})
	if interp.State().Value != "completed" {
		t.Errorf("expected completed, got %s", interp.State().Value)
	}
	if !interp.Done() {
		t.Error("expected Done() to be true")
	}
}

func TestOrderWorkflow_EmptyCartBlocked(t *testing.T) {
	registry := statekit.NewActionRegistry[OrderContext]().
		WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("processRefund", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordShipping", func(ctx *OrderContext, e statekit.Event) {}).
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return len(ctx.Items) > 0
		})

	ctx := OrderContext{
		OrderID: "TEST-002",
		Items:   []OrderItem{}, // Empty cart
	}

	machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](registry, ctx)
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Try to submit empty cart
	interp.Send(statekit.Event{Type: "SUBMIT"})
	if interp.State().Value != "pending" {
		t.Errorf("expected to stay in pending with empty cart, got %s", interp.State().Value)
	}
}

func TestOrderWorkflow_Cancellation(t *testing.T) {
	registry := statekit.NewActionRegistry[OrderContext]().
		WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("processRefund", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordShipping", func(ctx *OrderContext, e statekit.Event) {}).
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return len(ctx.Items) > 0
		})

	ctx := OrderContext{
		OrderID: "TEST-003",
		Items:   []OrderItem{{SKU: "TEST", Name: "Test", Quantity: 1, Price: 5.00}},
	}

	machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](registry, ctx)
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Cancel from pending
	interp.Send(statekit.Event{Type: "CANCEL"})
	if interp.State().Value != "cancelled" {
		t.Errorf("expected cancelled, got %s", interp.State().Value)
	}
	if !interp.Done() {
		t.Error("expected Done() to be true after cancellation")
	}
}

func TestOrderWorkflow_PaymentRetry(t *testing.T) {
	registry := statekit.NewActionRegistry[OrderContext]().
		WithAction("logPending", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("validateOrder", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("processRefund", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordPayment", func(ctx *OrderContext, e statekit.Event) {}).
		WithAction("recordShipping", func(ctx *OrderContext, e statekit.Event) {}).
		WithGuard("hasItems", func(ctx OrderContext, e statekit.Event) bool {
			return len(ctx.Items) > 0
		})

	ctx := OrderContext{
		OrderID: "TEST-004",
		Items:   []OrderItem{{SKU: "TEST", Name: "Test", Quantity: 1, Price: 5.00}},
	}

	machine, err := statekit.FromStructWithContext[OrderMachine, OrderContext](registry, ctx)
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	interp.Send(statekit.Event{Type: "SUBMIT"})
	interp.Send(statekit.Event{Type: "VALID"})

	// Payment fails
	interp.Send(statekit.Event{Type: "PAYMENT_FAILED"})
	if interp.State().Value != "payment_err" {
		t.Errorf("expected payment_err, got %s", interp.State().Value)
	}

	// Retry payment
	interp.Send(statekit.Event{Type: "RETRY"})
	if interp.State().Value != "payment" {
		t.Errorf("expected payment after retry, got %s", interp.State().Value)
	}
}
