package main

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestFormWizard_BasicFlow(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)

	// Set required fields for submission
	interp.UpdateContext(func(ctx *FormContext) {
		ctx.PersonalInfo.Email = "test@example.com"
		ctx.AddressInfo.City = "Seattle"
		ctx.PaymentInfo.CardType = "Visa"
	})

	interp.Start()

	// Should start at personal
	if interp.State().Value != "personal" {
		t.Errorf("Expected 'personal', got %s", interp.State().Value)
	}

	// Navigate through all steps
	interp.Send(statekit.Event{Type: "NEXT"}) // -> address
	if interp.State().Value != "address" {
		t.Errorf("Expected 'address', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"}) // -> payment (card_type)
	if interp.State().Value != "card_type" {
		t.Errorf("Expected 'card_type', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"}) // -> card_details
	if interp.State().Value != "card_details" {
		t.Errorf("Expected 'card_details', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "NEXT"}) // -> review
	if interp.State().Value != "review" {
		t.Errorf("Expected 'review', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "SUBMIT"}) // -> submitted
	if interp.State().Value != "submitted" {
		t.Errorf("Expected 'submitted', got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("Expected form to be done")
	}
}

func TestFormWizard_ShallowHistory(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Navigate to payment
	interp.Send(statekit.Event{Type: "NEXT"}) // -> address
	interp.Send(statekit.Event{Type: "NEXT"}) // -> payment (card_type)
	interp.Send(statekit.Event{Type: "NEXT"}) // -> card_details

	if interp.State().Value != "card_details" {
		t.Fatalf("Expected 'card_details', got %s", interp.State().Value)
	}

	// Preview
	interp.Send(statekit.Event{Type: "PREVIEW"})
	if interp.State().Value != "previewing" {
		t.Errorf("Expected 'previewing', got %s", interp.State().Value)
	}

	// Return via shallow history - should go to payment's initial (card_type)
	interp.Send(statekit.Event{Type: "BACK_SHALLOW"})

	// Shallow history remembers the immediate child (payment)
	// but since payment is compound, we enter its initial (card_type)
	if interp.State().Value != "card_type" {
		t.Errorf("Expected 'card_type' (shallow history), got %s", interp.State().Value)
	}
}

func TestFormWizard_DeepHistory(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Navigate to payment
	interp.Send(statekit.Event{Type: "NEXT"}) // -> address
	interp.Send(statekit.Event{Type: "NEXT"}) // -> payment (card_type)
	interp.Send(statekit.Event{Type: "NEXT"}) // -> card_details

	if interp.State().Value != "card_details" {
		t.Fatalf("Expected 'card_details', got %s", interp.State().Value)
	}

	// Preview
	interp.Send(statekit.Event{Type: "PREVIEW"})
	if interp.State().Value != "previewing" {
		t.Errorf("Expected 'previewing', got %s", interp.State().Value)
	}

	// Return via deep history - should go directly to card_details
	interp.Send(statekit.Event{Type: "BACK_DEEP"})
	if interp.State().Value != "card_details" {
		t.Errorf("Expected 'card_details' (deep history), got %s", interp.State().Value)
	}
}

func TestFormWizard_HistoryDefault(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Go to preview without any navigation
	interp.Send(statekit.Event{Type: "PREVIEW"})
	if interp.State().Value != "previewing" {
		t.Errorf("Expected 'previewing', got %s", interp.State().Value)
	}

	// Return via shallow history - no history recorded, uses default
	interp.Send(statekit.Event{Type: "BACK_SHALLOW"})
	if interp.State().Value != "personal" {
		t.Errorf("Expected 'personal' (default), got %s", interp.State().Value)
	}
}

func TestFormWizard_Cancel(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Navigate partway through
	interp.Send(statekit.Event{Type: "NEXT"}) // -> address

	// Cancel
	interp.Send(statekit.Event{Type: "CANCEL"})
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected 'cancelled', got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("Expected form to be done (cancelled)")
	}
}

func TestFormWizard_SubmitGuard(t *testing.T) {
	machine := buildWizardMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Navigate to review without setting required fields
	interp.Send(statekit.Event{Type: "NEXT"}) // -> address
	interp.Send(statekit.Event{Type: "NEXT"}) // -> payment
	interp.Send(statekit.Event{Type: "NEXT"}) // -> card_details
	interp.Send(statekit.Event{Type: "NEXT"}) // -> review

	// Try to submit without required fields
	interp.Send(statekit.Event{Type: "SUBMIT"})

	// Should still be in review (guard blocked)
	if interp.State().Value != "review" {
		t.Errorf("Expected 'review' (blocked by guard), got %s", interp.State().Value)
	}

	// Set required fields
	interp.UpdateContext(func(ctx *FormContext) {
		ctx.PersonalInfo.Email = "test@example.com"
		ctx.AddressInfo.City = "Seattle"
		ctx.PaymentInfo.CardType = "Visa"
	})

	// Now submit should work
	interp.Send(statekit.Event{Type: "SUBMIT"})
	if interp.State().Value != "submitted" {
		t.Errorf("Expected 'submitted', got %s", interp.State().Value)
	}
}
