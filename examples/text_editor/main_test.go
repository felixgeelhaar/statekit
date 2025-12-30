package main

import (
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestTextEditor_InitialState(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	state := interp.State()

	// Should be in editing (parallel) state
	if state.Value != "editing" {
		t.Errorf("Expected 'editing', got %s", state.Value)
	}

	// All 5 regions should be active
	if len(state.ActiveInParallel) != 5 {
		t.Errorf("Expected 5 regions, got %d", len(state.ActiveInParallel))
	}

	// Check initial states
	expected := map[statekit.StateID]statekit.StateID{
		"bold":      "bold_off",
		"italic":    "italic_off",
		"underline": "underline_off",
		"alignment": "align_left",
		"fontSize":  "size_medium",
	}

	for region, expectedState := range expected {
		if state.ActiveInParallel[region] != expectedState {
			t.Errorf("Expected %s=%s, got %s",
				region, expectedState, state.ActiveInParallel[region])
		}
	}
}

func TestTextEditor_ToggleBold(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Initially bold is off
	if interp.State().Context.IsBold {
		t.Error("Expected IsBold=false initially")
	}

	// Toggle on
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	if !interp.State().Context.IsBold {
		t.Error("Expected IsBold=true after toggle")
	}
	if interp.State().ActiveInParallel["bold"] != "bold_on" {
		t.Errorf("Expected bold region in 'bold_on', got %s",
			interp.State().ActiveInParallel["bold"])
	}

	// Toggle off
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	if interp.State().Context.IsBold {
		t.Error("Expected IsBold=false after second toggle")
	}
}

func TestTextEditor_IndependentRegions(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Toggle multiple formats
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	interp.Send(statekit.Event{Type: "TOGGLE_ITALIC"})
	interp.Send(statekit.Event{Type: "TOGGLE_UNDERLINE"})

	ctx := interp.State().Context
	if !ctx.IsBold || !ctx.IsItalic || !ctx.IsUnderline {
		t.Error("Expected all three formats enabled")
	}

	// Change alignment (shouldn't affect other regions)
	interp.Send(statekit.Event{Type: "ALIGN_CENTER"})

	ctx = interp.State().Context
	if !ctx.IsBold || !ctx.IsItalic || !ctx.IsUnderline {
		t.Error("Alignment change should not affect formatting")
	}
	if ctx.Alignment != "center" {
		t.Errorf("Expected alignment 'center', got %s", ctx.Alignment)
	}
}

func TestTextEditor_AlignmentExclusive(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Start at left
	if interp.State().Context.Alignment != "left" {
		t.Error("Expected initial alignment 'left'")
	}

	// Change to center
	interp.Send(statekit.Event{Type: "ALIGN_CENTER"})
	if interp.State().Context.Alignment != "center" {
		t.Error("Expected alignment 'center'")
	}

	// Change to right
	interp.Send(statekit.Event{Type: "ALIGN_RIGHT"})
	if interp.State().Context.Alignment != "right" {
		t.Error("Expected alignment 'right'")
	}

	// Back to left
	interp.Send(statekit.Event{Type: "ALIGN_LEFT"})
	if interp.State().Context.Alignment != "left" {
		t.Error("Expected alignment 'left'")
	}
}

func TestTextEditor_FontSizeExclusive(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Start at medium
	if interp.State().Context.FontSize != "medium" {
		t.Error("Expected initial fontSize 'medium'")
	}

	// Change to large
	interp.Send(statekit.Event{Type: "SIZE_LARGE"})
	if interp.State().Context.FontSize != "large" {
		t.Error("Expected fontSize 'large'")
	}

	// Change to small
	interp.Send(statekit.Event{Type: "SIZE_SMALL"})
	if interp.State().Context.FontSize != "small" {
		t.Error("Expected fontSize 'small'")
	}
}

func TestTextEditor_Matches(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Should match parent parallel state
	if !interp.Matches("editing") {
		t.Error("Expected to match 'editing'")
	}

	// Should match states in all regions
	if !interp.Matches("bold_off") {
		t.Error("Expected to match 'bold_off'")
	}
	if !interp.Matches("align_left") {
		t.Error("Expected to match 'align_left'")
	}

	// Toggle bold
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})

	// Should now match bold_on
	if !interp.Matches("bold_on") {
		t.Error("Expected to match 'bold_on'")
	}
	if interp.Matches("bold_off") {
		t.Error("Should not match 'bold_off' anymore")
	}
}

func TestTextEditor_Save(t *testing.T) {
	machine := buildEditorMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	defer interp.Stop()

	// Apply some formatting
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	interp.Send(statekit.Event{Type: "ALIGN_CENTER"})

	// Save
	interp.Send(statekit.Event{Type: "SAVE"})

	if interp.State().Value != "saved" {
		t.Errorf("Expected 'saved', got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("Expected document to be done (saved)")
	}

	// Context should be preserved
	ctx := interp.State().Context
	if !ctx.IsBold {
		t.Error("Expected bold to be preserved after save")
	}
	if ctx.Alignment != "center" {
		t.Error("Expected center alignment to be preserved after save")
	}
}
