package statekit

import (
	"testing"
)

// TestHierarchical_BuildNestedStates tests building a machine with nested states
func TestHierarchical_BuildNestedStates(t *testing.T) {
	// Structure:
	// active (compound)
	// ├── idle (atomic)
	// └── working (compound)
	//     ├── loading (atomic)
	//     └── processing (atomic)
	// done (final)
	machine, err := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").
		On("START").Target("working").
		End(). // Returns to idle StateBuilder
		End(). // Returns to active StateBuilder (idle's parent)
		State("working").
		WithInitial("loading").
		State("loading").
		On("LOADED").Target("processing").
		End(). // Returns to loading StateBuilder
		End(). // Returns to working StateBuilder
		State("processing").
		On("DONE").Target("idle").
		End().  // Returns to processing StateBuilder
		End().  // Returns to working StateBuilder
		End().  // Returns to active StateBuilder
		Done(). // Returns to MachineBuilder
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	if machine.ID != "test" {
		t.Errorf("expected ID 'test', got %s", machine.ID)
	}

	// Verify state hierarchy
	if machine.States["active"].Type != StateTypeCompound {
		t.Error("expected 'active' to be compound")
	}
	if machine.States["working"].Type != StateTypeCompound {
		t.Error("expected 'working' to be compound")
	}
	if machine.States["idle"].Parent != "active" {
		t.Errorf("expected 'idle' parent to be 'active', got %s", machine.States["idle"].Parent)
	}
	if machine.States["loading"].Parent != "working" {
		t.Errorf("expected 'loading' parent to be 'working', got %s", machine.States["loading"].Parent)
	}
}

// TestHierarchical_StartEntersLeaf tests that Start() enters the initial leaf state
func TestHierarchical_StartEntersLeaf(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").End().
		State("working").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should be in the leaf state "idle", not the compound state "active"
	if interp.State().Value != "idle" {
		t.Errorf("expected to start in 'idle', got %v", interp.State().Value)
	}
}

// TestHierarchical_StartEntersDeepLeaf tests entering a deeply nested initial leaf
func TestHierarchical_StartEntersDeepLeaf(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("level1").
		State("level1").
		WithInitial("level2").
		State("level2").
		WithInitial("level3").
		State("level3").End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "level3" {
		t.Errorf("expected to start in 'level3', got %v", interp.State().Value)
	}
}

// TestHierarchical_MatchesAncestors tests that Matches() returns true for ancestors
func TestHierarchical_MatchesAncestors(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("working").
		State("working").
		WithInitial("loading").
		State("loading").End().
		State("processing").End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should be in "loading"
	if !interp.Matches("loading") {
		t.Error("expected to match 'loading'")
	}
	// Should also match ancestor "working"
	if !interp.Matches("working") {
		t.Error("expected to match ancestor 'working'")
	}
	// Should also match ancestor "active"
	if !interp.Matches("active") {
		t.Error("expected to match ancestor 'active'")
	}
	// Should not match sibling "processing"
	if interp.Matches("processing") {
		t.Error("should not match sibling 'processing'")
	}
}

// TestHierarchical_TransitionWithinCompound tests transitions between siblings
func TestHierarchical_TransitionWithinCompound(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").
		On("START").Target("working").
		End(). // TransitionBuilder.End() → StateBuilder[idle]
		End(). // StateBuilder[idle].End() → StateBuilder[active]
		State("working").
		On("STOP").Target("idle").
		End(). // TransitionBuilder.End() → StateBuilder[working]
		End(). // StateBuilder[working].End() → StateBuilder[active]
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if !interp.Matches("idle") {
		t.Error("expected to start in 'idle'")
	}

	interp.Send(Event{Type: "START"})
	if !interp.Matches("working") {
		t.Errorf("expected 'working' after START, got %v", interp.State().Value)
	}

	interp.Send(Event{Type: "STOP"})
	if !interp.Matches("idle") {
		t.Errorf("expected 'idle' after STOP, got %v", interp.State().Value)
	}
}

// TestHierarchical_TransitionToCompoundEntersLeaf tests that transitioning to a compound state enters its initial leaf
func TestHierarchical_TransitionToCompoundEntersLeaf(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("START").Target("active").
		Done().
		State("active").
		WithInitial("working").
		State("working").
		WithInitial("loading").
		State("loading").End().
		State("processing").End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "idle" {
		t.Errorf("expected to start in 'idle', got %v", interp.State().Value)
	}

	interp.Send(Event{Type: "START"})

	// Should be in "loading" (the initial leaf of "active")
	if interp.State().Value != "loading" {
		t.Errorf("expected 'loading' after START, got %v", interp.State().Value)
	}
	if !interp.Matches("active") {
		t.Error("expected to match 'active'")
	}
	if !interp.Matches("working") {
		t.Error("expected to match 'working'")
	}
}

// Context for tracking entry/exit order
type orderContext struct {
	Actions []string
}

// TestHierarchical_EntryExitOrder tests that entry/exit actions are called in correct order
func TestHierarchical_EntryExitOrder(t *testing.T) {
	machine, err := NewMachine[orderContext]("test").
		WithInitial("idle").
		WithContext(orderContext{}).
		WithAction("enterIdle", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:idle")
		}).
		WithAction("exitIdle", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:idle")
		}).
		WithAction("enterActive", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:active")
		}).
		WithAction("exitActive", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:active")
		}).
		WithAction("enterWorking", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:working")
		}).
		WithAction("exitWorking", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:working")
		}).
		WithAction("enterLoading", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:loading")
		}).
		WithAction("exitLoading", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:loading")
		}).
		State("idle").
		OnEntry("enterIdle").
		OnExit("exitIdle").
		On("START").Target("active").
		Done().
		State("active").
		WithInitial("working").
		OnEntry("enterActive").
		OnExit("exitActive").
		State("working").
		WithInitial("loading").
		OnEntry("enterWorking").
		OnExit("exitWorking").
		State("loading").
		OnEntry("enterLoading").
		OnExit("exitLoading").
		On("STOP").Target("idle").
		End().
		End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Should have entered "idle"
	ctx := interp.State().Context
	if len(ctx.Actions) != 1 || ctx.Actions[0] != "enter:idle" {
		t.Errorf("expected [enter:idle], got %v", ctx.Actions)
	}

	// Clear actions and transition to active
	interp.UpdateContext(func(c *orderContext) {
		c.Actions = nil
	})

	interp.Send(Event{Type: "START"})

	// Should have: exit:idle, enter:active, enter:working, enter:loading
	ctx = interp.State().Context
	expected := []string{"exit:idle", "enter:active", "enter:working", "enter:loading"}
	if len(ctx.Actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d: %v", len(expected), len(ctx.Actions), ctx.Actions)
	}
	for i, exp := range expected {
		if ctx.Actions[i] != exp {
			t.Errorf("expected action[%d] = %s, got %s", i, exp, ctx.Actions[i])
		}
	}

	// Clear and transition back to idle
	interp.UpdateContext(func(c *orderContext) {
		c.Actions = nil
	})

	interp.Send(Event{Type: "STOP"})

	// Should have: exit:loading, exit:working, exit:active, enter:idle
	ctx = interp.State().Context
	expected = []string{"exit:loading", "exit:working", "exit:active", "enter:idle"}
	if len(ctx.Actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d: %v", len(expected), len(ctx.Actions), ctx.Actions)
	}
	for i, exp := range expected {
		if ctx.Actions[i] != exp {
			t.Errorf("expected action[%d] = %s, got %s", i, exp, ctx.Actions[i])
		}
	}
}

// TestHierarchical_EventBubblesUp tests that events bubble up to parent states
func TestHierarchical_EventBubblesUp(t *testing.T) {
	machine, err := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		On("GLOBAL_RESET").Target("done").End(). // TransitionBuilder.End() → StateBuilder[active]
		State("idle").
		On("START").Target("working").
		End(). // TransitionBuilder.End() → StateBuilder[idle]
		End(). // StateBuilder[idle].End() → StateBuilder[active]
		State("working").End().
		Done().
		State("done").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	if !interp.Matches("idle") {
		t.Error("expected to start in 'idle'")
	}

	interp.Send(Event{Type: "START"})
	if !interp.Matches("working") {
		t.Error("expected 'working' after START")
	}

	// Send event that's only handled by parent "active"
	interp.Send(Event{Type: "GLOBAL_RESET"})
	if !interp.Matches("done") {
		t.Errorf("expected 'done' after GLOBAL_RESET, got %v", interp.State().Value)
	}
	if !interp.Done() {
		t.Error("expected to be done in final state")
	}
}

// TestHierarchical_ChildTransitionTakesPriority tests that child state transitions take priority over parent
func TestHierarchical_ChildTransitionTakesPriority(t *testing.T) {
	handled := ""

	machine, err := NewMachine[struct{}]("test").
		WithInitial("parent").
		WithAction("parentHandled", func(ctx *struct{}, e Event) {
			handled = "parent"
		}).
		WithAction("childHandled", func(ctx *struct{}, e Event) {
			handled = "child"
		}).
		State("parent").
		WithInitial("child").
		On("EVENT").Target("parent").Do("parentHandled").End(). // Parent handles EVENT
		State("child").
		On("EVENT").Target("child").Do("childHandled"). // Child also handles EVENT
		End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	interp.Send(Event{Type: "EVENT"})

	// Child's transition should take priority
	if handled != "child" {
		t.Errorf("expected child to handle event, got %s", handled)
	}
}

// TestHierarchical_TransitionToSibling tests transitions between siblings in a compound state
func TestHierarchical_TransitionToSibling(t *testing.T) {
	machine, err := NewMachine[orderContext]("test").
		WithInitial("active").
		WithContext(orderContext{}).
		WithAction("enterActive", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:active")
		}).
		WithAction("exitActive", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:active")
		}).
		WithAction("enterIdle", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:idle")
		}).
		WithAction("exitIdle", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:idle")
		}).
		WithAction("enterWorking", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "enter:working")
		}).
		WithAction("exitWorking", func(ctx *orderContext, e Event) {
			ctx.Actions = append(ctx.Actions, "exit:working")
		}).
		State("active").
		WithInitial("idle").
		OnEntry("enterActive").
		OnExit("exitActive").
		State("idle").
		OnEntry("enterIdle").
		OnExit("exitIdle").
		On("START").Target("working").
		End(). // TransitionBuilder.End() → StateBuilder[idle]
		End(). // StateBuilder[idle].End() → StateBuilder[active]
		State("working").
		OnEntry("enterWorking").
		OnExit("exitWorking").
		On("STOP").Target("idle").
		End(). // TransitionBuilder.End() → StateBuilder[working]
		End(). // StateBuilder[working].End() → StateBuilder[active]
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	// Start actions: enter:active, enter:idle
	ctx := interp.State().Context
	if len(ctx.Actions) != 2 {
		t.Fatalf("expected 2 start actions, got %d: %v", len(ctx.Actions), ctx.Actions)
	}

	// Clear and transition to sibling
	interp.UpdateContext(func(c *orderContext) {
		c.Actions = nil
	})

	interp.Send(Event{Type: "START"})

	// Transition to sibling should: exit:idle, enter:working
	// (NOT exit/enter the parent "active")
	ctx = interp.State().Context
	expected := []string{"exit:idle", "enter:working"}
	if len(ctx.Actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d: %v", len(expected), len(ctx.Actions), ctx.Actions)
	}
	for i, exp := range expected {
		if ctx.Actions[i] != exp {
			t.Errorf("expected action[%d] = %s, got %s", i, exp, ctx.Actions[i])
		}
	}
}
