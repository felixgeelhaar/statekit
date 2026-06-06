// Package mermaid provides Mermaid stateDiagram-v2 rendering for state machines.
package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"go.klarlabs.de/statekit/viz"
)

// Renderer produces Mermaid stateDiagram-v2 syntax.
type Renderer struct {
	Direction   string // "LR" (left-right) or "TB" (top-bottom, default)
	ShowActions bool   // Include actions in labels
	ShowGuards  bool   // Include guards in labels
}

// NewRenderer creates a new Mermaid renderer with default settings.
func NewRenderer() *Renderer {
	return &Renderer{
		Direction:   "TB",
		ShowActions: true,
		ShowGuards:  true,
	}
}

// Render produces Mermaid diagram syntax.
func (r *Renderer) Render(m *viz.VizMachine) string {
	var b strings.Builder

	// Header
	b.WriteString("stateDiagram-v2\n")
	if r.Direction != "" && r.Direction != "TB" {
		fmt.Fprintf(&b, "    direction %s\n", r.Direction)
	}
	b.WriteString("\n")

	// Comment with machine ID
	fmt.Fprintf(&b, "    %%%% Machine: %s\n\n", m.ID)

	// Initial state marker
	if m.Initial != "" {
		fmt.Fprintf(&b, "    [*] --> %s\n", m.Initial)
	}

	// Render root-level states
	rootStates := m.GetRootStates()
	sort.Strings(rootStates)

	for _, stateID := range rootStates {
		state := m.States[stateID]
		r.renderState(&b, m, state, "    ")
	}

	return b.String()
}

func (r *Renderer) renderState(b *strings.Builder, m *viz.VizMachine, s *viz.VizState, indent string) {
	switch s.Type {
	case viz.VizStateFinal:
		// Final states transition to [*]
		fmt.Fprintf(b, "%s%s --> [*]\n", indent, s.ID)

	case viz.VizStateCompound:
		// Compound state with nested states
		fmt.Fprintf(b, "%sstate %s {\n", indent, s.ID)
		if s.Initial != "" {
			fmt.Fprintf(b, "%s    [*] --> %s\n", indent, s.Initial)
		}
		for _, childID := range s.Children {
			child := m.States[childID]
			if child != nil {
				r.renderState(b, m, child, indent+"    ")
			}
		}
		fmt.Fprintf(b, "%s}\n", indent)

	case viz.VizStateParallel:
		// Parallel state with regions separated by --
		fmt.Fprintf(b, "%sstate %s {\n", indent, s.ID)
		for i, childID := range s.Children {
			if i > 0 {
				fmt.Fprintf(b, "%s    --\n", indent)
			}
			child := m.States[childID]
			if child != nil {
				// Render region contents
				if child.Initial != "" {
					fmt.Fprintf(b, "%s    [*] --> %s\n", indent, child.Initial)
				}
				for _, grandchildID := range child.Children {
					grandchild := m.States[grandchildID]
					if grandchild != nil {
						r.renderState(b, m, grandchild, indent+"    ")
					}
				}
			}
		}
		fmt.Fprintf(b, "%s}\n", indent)

	case viz.VizStateHistory:
		// History state notation
		histType := "H"
		if s.HistoryType == "deep" {
			histType = "H*"
		}
		fmt.Fprintf(b, "%sstate \"%s (%s)\" as %s\n", indent, s.ID, histType, s.ID)
		if s.HistoryDefault != "" {
			fmt.Fprintf(b, "%s%s --> %s : default\n", indent, s.ID, s.HistoryDefault)
		}

	case viz.VizStateAtomic:
		// Atomic states - just render if they have entry/exit for note
		if len(s.Entry) > 0 || len(s.Exit) > 0 {
			fmt.Fprintf(b, "%sstate %s\n", indent, s.ID)
			if len(s.Entry) > 0 {
				fmt.Fprintf(b, "%snote right of %s : entry: %s\n", indent, s.ID, strings.Join(s.Entry, ", "))
			}
		}
	}

	// Render transitions
	for _, t := range s.Transitions {
		label := r.formatTransitionLabel(t)
		fmt.Fprintf(b, "%s%s --> %s : %s\n", indent, s.ID, t.Target, label)
	}
}

func (r *Renderer) formatTransitionLabel(t viz.VizTransition) string {
	parts := []string{t.Event}

	if t.Guard != "" && r.ShowGuards {
		parts = append(parts, fmt.Sprintf("[%s]", t.Guard))
	}

	if len(t.Actions) > 0 && r.ShowActions {
		parts = append(parts, fmt.Sprintf("/ %s", strings.Join(t.Actions, ", ")))
	}

	return strings.Join(parts, " ")
}
