package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/felixgeelhaar/statekit/viz"
)

// Model is the main Bubbletea model for the TUI.
type Model struct {
	Machine *viz.VizMachine
	tree    *TreeView
	styles  *Styles
	keymap  *KeyMap
	width   int
	height  int
	ready   bool
}

// New creates a new TUI model.
func New(machine *viz.VizMachine) *Model {
	styles := DefaultStyles()
	return &Model{
		Machine: machine,
		tree:    NewTreeView(machine, styles),
		styles:  styles,
		keymap:  DefaultKeyMap(),
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch {
		case m.keymap.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case m.keymap.Matches(msg, m.keymap.Up):
			m.tree.CursorUp()

		case m.keymap.Matches(msg, m.keymap.Down):
			m.tree.CursorDown()

		case m.keymap.Matches(msg, m.keymap.Left):
			m.tree.Collapse()

		case m.keymap.Matches(msg, m.keymap.Right):
			m.tree.Expand()

		case m.keymap.Matches(msg, m.keymap.Enter):
			m.tree.Toggle()

		case m.keymap.Matches(msg, m.keymap.Collapse):
			m.tree.CollapseAll()

		case m.keymap.Matches(msg, m.keymap.Expand):
			m.tree.ExpandAll()
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var sb strings.Builder

	// Header
	header := m.renderHeader()
	sb.WriteString(header)
	sb.WriteString("\n")

	// Calculate content area
	headerHeight := lipgloss.Height(header)
	footerHeight := 3
	contentHeight := m.height - headerHeight - footerHeight - 4 // padding

	// Main content: tree + detail panel
	treeWidth := m.width / 2
	detailWidth := m.width - treeWidth - 4

	// Tree view
	treeContent := m.tree.View(treeWidth, contentHeight)
	treePanel := lipgloss.NewStyle().
		Width(treeWidth).
		Height(contentHeight).
		Render(treeContent)

	// Detail panel
	detailContent := m.renderDetailPanel()
	detailPanel := m.styles.DetailPanel.
		Width(detailWidth).
		Height(contentHeight).
		Render(detailContent)

	// Join horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, detailPanel)
	sb.WriteString(content)
	sb.WriteString("\n")

	// Footer / help
	footer := m.renderFooter()
	sb.WriteString(footer)

	return m.styles.App.Render(sb.String())
}

// renderHeader renders the header section.
func (m *Model) renderHeader() string {
	title := fmt.Sprintf("State Machine: %s", m.Machine.ID)
	subtitle := fmt.Sprintf("Initial: %s  |  States: %d",
		m.Machine.Initial, len(m.Machine.States))

	return m.styles.Header.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.styles.Header.Render(title),
			m.styles.TreeBranch.Render(subtitle),
		),
	)
}

// renderDetailPanel renders the detail panel for the selected state.
func (m *Model) renderDetailPanel() string {
	selected := m.tree.Selected()
	if selected == nil {
		return "No state selected"
	}

	state := selected.State
	var sb strings.Builder

	// Title
	sb.WriteString(m.styles.DetailTitle.Render(state.ID))
	sb.WriteString("\n\n")

	// Type
	sb.WriteString(m.styles.DetailLabel.Render("Type:"))
	sb.WriteString(m.styles.DetailValue.Render(string(state.Type)))
	sb.WriteString("\n")

	// Parent
	if state.Parent != "" {
		sb.WriteString(m.styles.DetailLabel.Render("Parent:"))
		sb.WriteString(m.styles.DetailValue.Render(state.Parent))
		sb.WriteString("\n")
	}

	// Initial (for compound states)
	if state.Initial != "" {
		sb.WriteString(m.styles.DetailLabel.Render("Initial:"))
		sb.WriteString(m.styles.DetailValue.Render(state.Initial))
		sb.WriteString("\n")
	}

	// Entry actions
	if len(state.Entry) > 0 {
		sb.WriteString(m.styles.DetailLabel.Render("Entry:"))
		sb.WriteString(m.styles.DetailAction.Render(strings.Join(state.Entry, ", ")))
		sb.WriteString("\n")
	}

	// Exit actions
	if len(state.Exit) > 0 {
		sb.WriteString(m.styles.DetailLabel.Render("Exit:"))
		sb.WriteString(m.styles.DetailAction.Render(strings.Join(state.Exit, ", ")))
		sb.WriteString("\n")
	}

	// Transitions
	if len(state.Transitions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.styles.DetailLabel.Render("Transitions:"))
		sb.WriteString("\n")
		for _, t := range state.Transitions {
			line := m.renderTransition(t)
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	// Invocations
	if len(state.Invocations) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.styles.DetailLabel.Render("Invokes:"))
		sb.WriteString("\n")
		for _, inv := range state.Invocations {
			sb.WriteString("  ")
			sb.WriteString(m.styles.DetailAction.Render(inv.ID))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderTransition renders a single transition.
func (m *Model) renderTransition(t viz.VizTransition) string {
	var parts []string

	// Event
	event := t.Event
	if t.IsDelayed {
		event = fmt.Sprintf("after %dms", t.DelayMs)
	}
	parts = append(parts, m.styles.DetailTransition.Render(event))

	// Arrow and target
	parts = append(parts, m.styles.TreeBranch.Render(" → "))
	parts = append(parts, m.styles.DetailValue.Render(t.Target))

	// Guard
	if t.Guard != "" {
		parts = append(parts, m.styles.DetailGuard.Render(fmt.Sprintf(" [%s]", t.Guard)))
	}

	// Actions
	if len(t.Actions) > 0 {
		parts = append(parts, m.styles.DetailAction.Render(fmt.Sprintf(" / %s", strings.Join(t.Actions, ", "))))
	}

	return strings.Join(parts, "")
}

// renderFooter renders the help bar.
func (m *Model) renderFooter() string {
	items := m.keymap.HelpItems()
	var parts []string
	for _, item := range items {
		key := m.styles.HelpKey.Render(item.Key)
		text := m.styles.HelpText.Render(item.Desc)
		parts = append(parts, fmt.Sprintf("%s %s", key, text))
	}
	return m.styles.Footer.Render(strings.Join(parts, "  │  "))
}

// Run starts the TUI application.
func Run(machine *viz.VizMachine) error {
	model := New(machine)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
