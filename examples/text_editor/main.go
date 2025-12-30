// Package main demonstrates parallel (orthogonal) states with a text editor.
//
// This example shows:
// - Multiple regions executing simultaneously
// - Independent state management per region
// - Event broadcasting to all regions
// - Region-specific state queries
// - Exiting parallel states
package main

import (
	"fmt"
	"strings"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/export"
)

// EditorContext holds the editor state
type EditorContext struct {
	Content string

	// Formatting state (derived from parallel regions)
	IsBold      bool
	IsItalic    bool
	IsUnderline bool
	Alignment   string
	FontSize    string
}

func main() {
	machine := buildEditorMachine()

	// Export to XState JSON for visualization
	fmt.Println("=== XState JSON (paste at stately.ai/viz) ===")
	exporter := export.NewXStateExporter(machine)
	json, _ := exporter.ExportJSONIndent("", "  ")
	fmt.Println(json)
	fmt.Println()

	// Demo the editor
	fmt.Println("=== Text Editor Demo ===")
	runDemo(machine)
}

func buildEditorMachine() *statekit.MachineConfig[EditorContext] {
	machine, err := statekit.NewMachine[EditorContext]("text_editor").
		WithInitial("editing").
		WithContext(EditorContext{
			Content:   "Hello World",
			Alignment: "left",
			FontSize:  "medium",
		}).
		// Actions for format toggles
		WithAction("enableBold", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsBold = true
		}).
		WithAction("disableBold", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsBold = false
		}).
		WithAction("enableItalic", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsItalic = true
		}).
		WithAction("disableItalic", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsItalic = false
		}).
		WithAction("enableUnderline", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsUnderline = true
		}).
		WithAction("disableUnderline", func(ctx *EditorContext, e statekit.Event) {
			ctx.IsUnderline = false
		}).
		// Actions for alignment
		WithAction("setLeft", func(ctx *EditorContext, e statekit.Event) {
			ctx.Alignment = "left"
		}).
		WithAction("setCenter", func(ctx *EditorContext, e statekit.Event) {
			ctx.Alignment = "center"
		}).
		WithAction("setRight", func(ctx *EditorContext, e statekit.Event) {
			ctx.Alignment = "right"
		}).
		// Actions for font size
		WithAction("setSmall", func(ctx *EditorContext, e statekit.Event) {
			ctx.FontSize = "small"
		}).
		WithAction("setMedium", func(ctx *EditorContext, e statekit.Event) {
			ctx.FontSize = "medium"
		}).
		WithAction("setLarge", func(ctx *EditorContext, e statekit.Event) {
			ctx.FontSize = "large"
		}).
		// Main editing state with parallel regions
		State("editing").Parallel().
		On("SAVE").Target("saved").End().
		// Region 1: Bold formatting (toggle)
		Region("bold").
		WithInitial("bold_off").
		State("bold_off").
		OnEntry("disableBold").
		On("TOGGLE_BOLD").Target("bold_on").
		EndState().
		State("bold_on").
		OnEntry("enableBold").
		On("TOGGLE_BOLD").Target("bold_off").
		EndState().
		EndRegion().
		// Region 2: Italic formatting (toggle)
		Region("italic").
		WithInitial("italic_off").
		State("italic_off").
		OnEntry("disableItalic").
		On("TOGGLE_ITALIC").Target("italic_on").
		EndState().
		State("italic_on").
		OnEntry("enableItalic").
		On("TOGGLE_ITALIC").Target("italic_off").
		EndState().
		EndRegion().
		// Region 3: Underline formatting (toggle)
		Region("underline").
		WithInitial("underline_off").
		State("underline_off").
		OnEntry("disableUnderline").
		On("TOGGLE_UNDERLINE").Target("underline_on").
		EndState().
		State("underline_on").
		OnEntry("enableUnderline").
		On("TOGGLE_UNDERLINE").Target("underline_off").
		EndState().
		EndRegion().
		// Region 4: Text alignment (exclusive)
		Region("alignment").
		WithInitial("align_left").
		State("align_left").
		OnEntry("setLeft").
		On("ALIGN_CENTER").Target("align_center").
		On("ALIGN_RIGHT").Target("align_right").
		EndState().
		State("align_center").
		OnEntry("setCenter").
		On("ALIGN_LEFT").Target("align_left").
		On("ALIGN_RIGHT").Target("align_right").
		EndState().
		State("align_right").
		OnEntry("setRight").
		On("ALIGN_LEFT").Target("align_left").
		On("ALIGN_CENTER").Target("align_center").
		EndState().
		EndRegion().
		// Region 5: Font size (exclusive)
		Region("fontSize").
		WithInitial("size_medium").
		State("size_small").
		OnEntry("setSmall").
		On("SIZE_MEDIUM").Target("size_medium").
		On("SIZE_LARGE").Target("size_large").
		EndState().
		State("size_medium").
		OnEntry("setMedium").
		On("SIZE_SMALL").Target("size_small").
		On("SIZE_LARGE").Target("size_large").
		EndState().
		State("size_large").
		OnEntry("setLarge").
		On("SIZE_SMALL").Target("size_small").
		On("SIZE_MEDIUM").Target("size_medium").
		EndState().
		EndRegion().
		Done().
		// Saved state (final)
		State("saved").
		Final().
		Done().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build machine: %v", err))
	}

	return machine
}

func runDemo(machine *statekit.MachineConfig[EditorContext]) {
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	printState(interp, "Initial state")

	// Toggle some formatting
	fmt.Println("\n--- Formatting changes ---")

	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	printState(interp, "After TOGGLE_BOLD")

	interp.Send(statekit.Event{Type: "TOGGLE_ITALIC"})
	printState(interp, "After TOGGLE_ITALIC")

	interp.Send(statekit.Event{Type: "ALIGN_CENTER"})
	printState(interp, "After ALIGN_CENTER")

	interp.Send(statekit.Event{Type: "SIZE_LARGE"})
	printState(interp, "After SIZE_LARGE")

	// Toggle bold off
	fmt.Println("\n--- Toggle bold off ---")
	interp.Send(statekit.Event{Type: "TOGGLE_BOLD"})
	printState(interp, "After TOGGLE_BOLD again")

	// Add underline
	interp.Send(statekit.Event{Type: "TOGGLE_UNDERLINE"})
	printState(interp, "After TOGGLE_UNDERLINE")

	// Show formatted text preview
	fmt.Println("\n--- Formatted Preview ---")
	printFormattedPreview(interp)

	// Save
	fmt.Println("\n--- Save document ---")
	interp.Send(statekit.Event{Type: "SAVE"})
	fmt.Printf("Document saved: %v\n", interp.Done())

	interp.Stop()
}

func printState(interp *statekit.Interpreter[EditorContext], label string) {
	state := interp.State()
	ctx := state.Context

	fmt.Printf("\n%s:\n", label)
	fmt.Printf("  Parent state: %s\n", state.Value)
	fmt.Printf("  Regions: %v\n", state.ActiveInParallel)
	fmt.Printf("  Format: Bold=%v, Italic=%v, Underline=%v\n",
		ctx.IsBold, ctx.IsItalic, ctx.IsUnderline)
	fmt.Printf("  Alignment: %s, Size: %s\n", ctx.Alignment, ctx.FontSize)
}

func printFormattedPreview(interp *statekit.Interpreter[EditorContext]) {
	ctx := interp.State().Context

	text := ctx.Content

	// Apply formatting markers
	var markers []string
	if ctx.IsBold {
		markers = append(markers, "**")
	}
	if ctx.IsItalic {
		markers = append(markers, "_")
	}
	if ctx.IsUnderline {
		markers = append(markers, "~")
	}

	prefix := strings.Join(markers, "")
	suffix := strings.Join(reverse(markers), "")

	formatted := prefix + text + suffix

	// Apply alignment
	switch ctx.Alignment {
	case "center":
		formatted = "    " + formatted + "    "
	case "right":
		formatted = "        " + formatted
	}

	// Apply size
	switch ctx.FontSize {
	case "small":
		formatted = "[small]" + formatted + "[/small]"
	case "large":
		formatted = "[LARGE]" + formatted + "[/LARGE]"
	}

	fmt.Printf("  Preview: %s\n", formatted)
}

func reverse(s []string) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[len(s)-1-i] = v
	}
	return result
}
