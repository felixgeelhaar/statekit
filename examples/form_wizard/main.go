// Package main demonstrates history states with a multi-step form wizard.
//
// This example shows:
// - Shallow history (remembers immediate child)
// - Deep history (remembers full leaf path)
// - Using history to resume interrupted workflows
// - Default fallback when no history exists
package main

import (
	"fmt"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

// FormContext holds the form wizard state
type FormContext struct {
	// Form data
	PersonalInfo PersonalInfo
	AddressInfo  AddressInfo
	PaymentInfo  PaymentInfo

	// Progress tracking
	CurrentStep     string
	StepsCompleted  int
	PreviewCount    int
	ValidationError string
}

type PersonalInfo struct {
	FirstName string
	LastName  string
	Email     string
}

type AddressInfo struct {
	Street  string
	City    string
	Country string
}

type PaymentInfo struct {
	CardType string
	LastFour string
}

func main() {
	machine := buildWizardMachine()

	// Export to XState JSON for visualization
	fmt.Println("=== XState JSON (paste at stately.ai/viz) ===")
	exporter := export.NewXStateExporter(machine)
	json, _ := exporter.ExportJSONIndent("", "  ")
	fmt.Println(json)
	fmt.Println()

	// Demo the wizard flow
	fmt.Println("=== Form Wizard Demo ===")
	runDemo(machine)
}

func buildWizardMachine() *statekit.MachineConfig[FormContext] {
	machine, err := statekit.NewMachine[FormContext]("form_wizard").
		WithInitial("filling").
		WithContext(FormContext{}).
		// Actions
		WithAction("logStep", func(ctx *FormContext, e statekit.Event) {
			fmt.Printf("[Step] Entered: %s\n", ctx.CurrentStep)
		}).
		WithAction("setPersonal", func(ctx *FormContext, e statekit.Event) {
			ctx.CurrentStep = "personal"
			ctx.StepsCompleted = max(ctx.StepsCompleted, 1)
		}).
		WithAction("setAddress", func(ctx *FormContext, e statekit.Event) {
			ctx.CurrentStep = "address"
			ctx.StepsCompleted = max(ctx.StepsCompleted, 2)
		}).
		WithAction("setPayment", func(ctx *FormContext, e statekit.Event) {
			ctx.CurrentStep = "payment"
			ctx.StepsCompleted = max(ctx.StepsCompleted, 3)
		}).
		WithAction("setReview", func(ctx *FormContext, e statekit.Event) {
			ctx.CurrentStep = "review"
		}).
		WithAction("incrementPreview", func(ctx *FormContext, e statekit.Event) {
			ctx.PreviewCount++
			fmt.Printf("[Preview] Viewing form summary (count: %d)\n", ctx.PreviewCount)
		}).
		WithAction("submitForm", func(ctx *FormContext, e statekit.Event) {
			fmt.Println("[Submit] Form submitted successfully!")
		}).
		// Guards
		WithGuard("hasRequiredFields", func(ctx FormContext, e statekit.Event) bool {
			// Simplified validation
			return ctx.PersonalInfo.Email != "" &&
				ctx.AddressInfo.City != "" &&
				ctx.PaymentInfo.CardType != ""
		}).
		// Main filling state (compound with history)
		State("filling").
		WithInitial("personal").
		On("PREVIEW").Target("previewing").End().
		On("CANCEL").Target("cancelled").End().
		// Shallow history: remembers the last step (e.g., "address")
		History("hist").Shallow().Default("personal").End().
		// Deep history: remembers the exact sub-step within nested sections
		History("deepHist").Deep().Default("personal").End().
		// Step 1: Personal Info
		State("personal").
		OnEntry("setPersonal").
		OnEntry("logStep").
		On("NEXT").Target("address").
		End().
		End().
		// Step 2: Address
		State("address").
		OnEntry("setAddress").
		OnEntry("logStep").
		On("BACK").Target("personal").
		On("NEXT").Target("payment").
		End().
		End().
		// Step 3: Payment (compound state with sub-steps)
		State("payment").
		WithInitial("card_type").
		OnEntry("setPayment").
		OnEntry("logStep").
		On("BACK").Target("address").End().
		State("card_type").
		On("NEXT").Target("card_details").
		End().
		End().
		State("card_details").
		On("BACK").Target("card_type").
		On("NEXT").Target("review").
		End().
		End().
		End(). // End payment
		// Step 4: Review
		State("review").
		OnEntry("setReview").
		OnEntry("logStep").
		On("BACK").Target("payment").
		On("SUBMIT").Target("submitted").Guard("hasRequiredFields").
		End().
		Done(). // End filling
		// Preview state - uses history to return
		State("previewing").
		OnEntry("incrementPreview").
		On("BACK_SHALLOW").Target("hist").   // Returns to last major step
		On("BACK_DEEP").Target("deepHist").  // Returns to exact position
		On("BACK_START").Target("personal"). // Start over
		Done().
		// Final states
		State("submitted").
		OnEntry("submitForm").
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

func runDemo(machine *statekit.MachineConfig[FormContext]) {
	interp := statekit.NewInterpreter(machine)

	// Set some initial data
	interp.UpdateContext(func(ctx *FormContext) {
		ctx.PersonalInfo = PersonalInfo{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
		}
		ctx.AddressInfo = AddressInfo{
			Street:  "123 Main St",
			City:    "Seattle",
			Country: "USA",
		}
		ctx.PaymentInfo = PaymentInfo{
			CardType: "Visa",
			LastFour: "4242",
		}
	})

	interp.Start()
	printState(interp, "Initial")

	// Progress through the form
	fmt.Println("\n--- Filling out form ---")
	interp.Send(statekit.Event{Type: "NEXT"}) // personal -> address
	printState(interp, "After NEXT")

	interp.Send(statekit.Event{Type: "NEXT"}) // address -> payment
	printState(interp, "After NEXT")

	interp.Send(statekit.Event{Type: "NEXT"}) // card_type -> card_details
	printState(interp, "After NEXT (in payment)")

	// Now preview the form
	fmt.Println("\n--- Preview interruption ---")
	interp.Send(statekit.Event{Type: "PREVIEW"})
	printState(interp, "After PREVIEW")

	// Return using shallow history (goes to payment, then card_type initial)
	fmt.Println("\n--- Testing shallow history ---")
	interp.Send(statekit.Event{Type: "BACK_SHALLOW"})
	printState(interp, "After BACK_SHALLOW")
	fmt.Println("Note: Shallow history remembered 'payment' but entered its initial 'card_type'")

	// Go back to card_details
	interp.Send(statekit.Event{Type: "NEXT"}) // card_type -> card_details

	// Preview again
	fmt.Println("\n--- Preview again ---")
	interp.Send(statekit.Event{Type: "PREVIEW"})
	printState(interp, "After PREVIEW")

	// Return using deep history (goes directly to card_details)
	fmt.Println("\n--- Testing deep history ---")
	interp.Send(statekit.Event{Type: "BACK_DEEP"})
	printState(interp, "After BACK_DEEP")
	fmt.Println("Note: Deep history remembered the exact position 'card_details'")

	// Complete the form
	fmt.Println("\n--- Completing form ---")
	interp.Send(statekit.Event{Type: "NEXT"}) // card_details -> review
	printState(interp, "After NEXT")

	interp.Send(statekit.Event{Type: "SUBMIT"})
	printState(interp, "After SUBMIT")

	fmt.Printf("\nForm completed: %v\n", interp.Done())
	fmt.Printf("Steps completed: %d\n", interp.State().Context.StepsCompleted)
	fmt.Printf("Preview count: %d\n", interp.State().Context.PreviewCount)
}

func printState(interp *statekit.Interpreter[FormContext], label string) {
	state := interp.State()
	fmt.Printf("  %s: state=%s\n", label, state.Value)
}
