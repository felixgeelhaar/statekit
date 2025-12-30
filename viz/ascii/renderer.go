// Package ascii provides ASCII/Unicode box diagram rendering for state machines.
package ascii

import (
	"fmt"
	"sort"
	"strings"

	"github.com/felixgeelhaar/statekit/viz"
)

// BoxChars contains characters for drawing boxes.
type BoxChars struct {
	TopLeft, TopRight, BottomLeft, BottomRight rune
	Horizontal, Vertical                       rune
	TeeDown, TeeUp, TeeRight, TeeLeft          rune
	Cross                                      rune
	ArrowRight, ArrowDown, ArrowLeft, ArrowUp  rune
	Initial, Final                             rune
}

var (
	unicodeChars = BoxChars{
		TopLeft:     '┌',
		TopRight:    '┐',
		BottomLeft:  '└',
		BottomRight: '┘',
		Horizontal:  '─',
		Vertical:    '│',
		TeeDown:     '┬',
		TeeUp:       '┴',
		TeeRight:    '├',
		TeeLeft:     '┤',
		Cross:       '┼',
		ArrowRight:  '→',
		ArrowDown:   '↓',
		ArrowLeft:   '←',
		ArrowUp:     '↑',
		Initial:     '●',
		Final:       '◉',
	}

	asciiChars = BoxChars{
		TopLeft:     '+',
		TopRight:    '+',
		BottomLeft:  '+',
		BottomRight: '+',
		Horizontal:  '-',
		Vertical:    '|',
		TeeDown:     '+',
		TeeUp:       '+',
		TeeRight:    '+',
		TeeLeft:     '+',
		Cross:       '+',
		ArrowRight:  '>',
		ArrowDown:   'v',
		ArrowLeft:   '<',
		ArrowUp:     '^',
		Initial:     '*',
		Final:       '#',
	}
)

// Renderer produces ASCII/Unicode box diagrams.
type Renderer struct {
	UseUnicode  bool // Use Unicode box-drawing characters
	ShowActions bool // Include actions
	ShowGuards  bool // Include guards
	MaxWidth    int  // Maximum width (0 = unlimited)
}

// NewRenderer creates a new ASCII renderer with default settings.
func NewRenderer() *Renderer {
	return &Renderer{
		UseUnicode:  true,
		ShowActions: true,
		ShowGuards:  true,
		MaxWidth:    120,
	}
}

// Render produces a text-based state machine diagram.
func (r *Renderer) Render(m *viz.VizMachine) string {
	chars := unicodeChars
	if !r.UseUnicode {
		chars = asciiChars
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf(" %s ", m.ID)
	titleWidth := len(title) + 4

	// Calculate content
	content := r.renderContent(m, chars)
	contentLines := strings.Split(content, "\n")

	// Find max width
	maxWidth := titleWidth
	for _, line := range contentLines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	// Add padding
	maxWidth += 4

	if r.MaxWidth > 0 && maxWidth > r.MaxWidth {
		maxWidth = r.MaxWidth
	}

	// Top border with title
	b.WriteRune(chars.TopLeft)
	b.WriteString(strings.Repeat(string(chars.Horizontal), maxWidth-2))
	b.WriteRune(chars.TopRight)
	b.WriteString("\n")

	// Title line
	b.WriteRune(chars.Vertical)
	b.WriteString(" ")
	b.WriteString(m.ID)
	b.WriteString(strings.Repeat(" ", maxWidth-3-len(m.ID)))
	b.WriteRune(chars.Vertical)
	b.WriteString("\n")

	// Title separator
	b.WriteRune(chars.TeeRight)
	b.WriteString(strings.Repeat(string(chars.Horizontal), maxWidth-2))
	b.WriteRune(chars.TeeLeft)
	b.WriteString("\n")

	// Content
	for _, line := range contentLines {
		if line == "" {
			continue
		}
		b.WriteRune(chars.Vertical)
		b.WriteString("  ")
		b.WriteString(line)
		padding := maxWidth - 4 - len(line)
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(" ")
		b.WriteRune(chars.Vertical)
		b.WriteString("\n")
	}

	// Bottom border
	b.WriteRune(chars.BottomLeft)
	b.WriteString(strings.Repeat(string(chars.Horizontal), maxWidth-2))
	b.WriteRune(chars.BottomRight)
	b.WriteString("\n")

	return b.String()
}

func (r *Renderer) renderContent(m *viz.VizMachine, chars BoxChars) string {
	var b strings.Builder

	// Initial indicator
	if m.Initial != "" {
		b.WriteRune(chars.Initial)
		b.WriteString(" ")
		b.WriteRune(chars.ArrowRight)
		b.WriteString(" ")
		b.WriteString(m.Initial)
		b.WriteString("\n\n")
	}

	// Render states as a tree-like structure
	rootStates := m.GetRootStates()
	sort.Strings(rootStates)

	for i, stateID := range rootStates {
		state := m.States[stateID]
		r.renderStateTree(&b, m, state, "", i == len(rootStates)-1, chars)
	}

	// Render transitions
	b.WriteString("\nTransitions:\n")
	r.renderTransitions(&b, m, chars)

	return b.String()
}

func (r *Renderer) renderStateTree(b *strings.Builder, m *viz.VizMachine, s *viz.VizState, prefix string, isLast bool, chars BoxChars) {
	// Connector
	connector := string(chars.TeeRight) + string(chars.Horizontal) + string(chars.Horizontal) + " "
	if isLast {
		connector = string(chars.BottomLeft) + string(chars.Horizontal) + string(chars.Horizontal) + " "
	}

	// State indicator
	indicator := ""
	switch s.Type {
	case viz.VizStateFinal:
		indicator = fmt.Sprintf(" %c", chars.Final)
	case viz.VizStateCompound:
		indicator = " [+]"
	case viz.VizStateParallel:
		indicator = " [||]"
	case viz.VizStateHistory:
		if s.HistoryType == "deep" {
			indicator = " (H*)"
		} else {
			indicator = " (H)"
		}
	}

	// Initial marker
	initialMark := ""
	if m.IsInitial(s.ID) {
		initialMark = fmt.Sprintf("%c ", chars.Initial)
	}

	// State line
	b.WriteString(prefix)
	b.WriteString(connector)
	b.WriteString(initialMark)
	b.WriteString(s.ID)
	b.WriteString(indicator)

	// Entry/Exit actions
	if r.ShowActions {
		if len(s.Entry) > 0 {
			b.WriteString(fmt.Sprintf(" entry:%s", strings.Join(s.Entry, ",")))
		}
		if len(s.Exit) > 0 {
			b.WriteString(fmt.Sprintf(" exit:%s", strings.Join(s.Exit, ",")))
		}
	}

	b.WriteString("\n")

	// Children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += string(chars.Vertical) + "   "
	}

	for i, childID := range s.Children {
		child := m.States[childID]
		if child != nil {
			r.renderStateTree(b, m, child, childPrefix, i == len(s.Children)-1, chars)
		}
	}
}

func (r *Renderer) renderTransitions(b *strings.Builder, m *viz.VizMachine, chars BoxChars) {
	// Collect all transitions
	type transInfo struct {
		from  string
		trans viz.VizTransition
	}

	var transitions []transInfo
	for stateID, state := range m.States {
		for _, t := range state.Transitions {
			transitions = append(transitions, transInfo{from: stateID, trans: t})
		}
	}

	// Sort for deterministic output
	sort.Slice(transitions, func(i, j int) bool {
		if transitions[i].from != transitions[j].from {
			return transitions[i].from < transitions[j].from
		}
		return transitions[i].trans.Event < transitions[j].trans.Event
	})

	for _, t := range transitions {
		b.WriteString("  ")
		b.WriteString(t.from)
		b.WriteString(" ")
		b.WriteString(string(chars.Horizontal))
		b.WriteString(string(chars.Horizontal))
		b.WriteString("[")
		b.WriteString(t.trans.Event)
		if t.trans.Guard != "" && r.ShowGuards {
			b.WriteString(" if ")
			b.WriteString(t.trans.Guard)
		}
		b.WriteString("]")
		b.WriteString(string(chars.Horizontal))
		b.WriteRune(chars.ArrowRight)
		b.WriteString(" ")
		b.WriteString(t.trans.Target)
		if len(t.trans.Actions) > 0 && r.ShowActions {
			b.WriteString(" / ")
			b.WriteString(strings.Join(t.trans.Actions, ", "))
		}
		b.WriteString("\n")
	}
}
