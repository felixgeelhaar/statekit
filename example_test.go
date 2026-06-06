package statekit_test

import (
	"fmt"

	"go.klarlabs.de/statekit"
)

func Example() {
	// Create a simple traffic light state machine
	machine, _ := statekit.NewMachine[struct{}]("traffic").
		WithInitial("green").
		State("green").On("TIMER").Target("yellow").Done().
		State("yellow").On("TIMER").Target("red").Done().
		State("red").On("TIMER").Target("green").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	fmt.Println(interp.State().Value)
	interp.Send(statekit.Event{Type: "TIMER"})
	fmt.Println(interp.State().Value)

	// Output:
	// green
	// yellow
}

func ExampleNewMachine_withGuard() {
	type Context struct {
		Approved bool
	}

	machine, _ := statekit.NewMachine[Context]("approval").
		WithInitial("pending").
		WithContext(Context{Approved: true}).
		WithGuard("isApproved", func(ctx Context, e statekit.Event) bool {
			return ctx.Approved
		}).
		State("pending").
		On("SUBMIT").Target("approved").Guard("isApproved").
		Done().
		State("approved").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()
	interp.Send(statekit.Event{Type: "SUBMIT"})

	fmt.Println(interp.State().Value)
	// Output: approved
}

func ExampleNewMachine_withAction() {
	type Context struct {
		Count int
	}

	machine, _ := statekit.NewMachine[Context]("counter").
		WithInitial("idle").
		WithAction("increment", func(ctx *Context, e statekit.Event) {
			ctx.Count++
		}).
		State("idle").
		OnEntry("increment").
		On("COUNT").Target("idle").
		Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	fmt.Println(interp.State().Context.Count) // Entry action runs on Start
	interp.Send(statekit.Event{Type: "COUNT"})
	fmt.Println(interp.State().Context.Count) // Entry action runs again on self-transition

	// Output:
	// 1
	// 2
}

func ExampleNewMachine_hierarchical() {
	// Hierarchical state machine with nested states
	machine, _ := statekit.NewMachine[struct{}]("editor").
		WithInitial("editing").
		State("editing").
		WithInitial("idle").
		On("SAVE").Target("saved").End(). // Parent handles SAVE for all children
		State("idle").
		On("TYPE").Target("dirty").
		End().
		End().
		State("dirty").
		On("CLEAR").Target("idle").
		End().
		End().
		Done().
		State("saved").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	fmt.Println(interp.State().Value)      // Starts in leaf state
	fmt.Println(interp.Matches("editing")) // Matches parent
	interp.Send(statekit.Event{Type: "TYPE"})
	fmt.Println(interp.State().Value)
	interp.Send(statekit.Event{Type: "SAVE"}) // Bubbles up to parent
	fmt.Println(interp.State().Value)

	// Output:
	// idle
	// true
	// dirty
	// saved
}

func ExampleInterpreter_Matches() {
	machine, _ := statekit.NewMachine[struct{}]("nested").
		WithInitial("parent").
		State("parent").
		WithInitial("child").
		State("child").End().
		Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Current state is "child"
	fmt.Println(interp.State().Value)
	// Matches both the current state and its ancestors
	fmt.Println(interp.Matches("child"))
	fmt.Println(interp.Matches("parent"))

	// Output:
	// child
	// true
	// true
}
