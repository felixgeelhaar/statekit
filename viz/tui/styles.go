// Package tui provides an interactive terminal UI for visualizing state machines.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme colors.
var (
	ColorPrimary    = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary  = lipgloss.Color("#06B6D4") // Cyan
	ColorAccent     = lipgloss.Color("#F59E0B") // Amber
	ColorSuccess    = lipgloss.Color("#10B981") // Green
	ColorMuted      = lipgloss.Color("#6B7280") // Gray
	ColorBackground = lipgloss.Color("#1F2937") // Dark gray
	ColorForeground = lipgloss.Color("#F9FAFB") // Light gray
)

// Styles holds all the lipgloss styles for the TUI.
type Styles struct {
	// App-level styles
	App    lipgloss.Style
	Header lipgloss.Style
	Footer lipgloss.Style

	// Tree view styles
	TreeNode         lipgloss.Style
	TreeNodeSelected lipgloss.Style
	TreeBranch       lipgloss.Style
	TreeLeaf         lipgloss.Style
	TreeInitial      lipgloss.Style
	TreeFinal        lipgloss.Style
	TreeCompound     lipgloss.Style
	TreeParallel     lipgloss.Style
	TreeHistory      lipgloss.Style

	// Detail panel styles
	DetailPanel      lipgloss.Style
	DetailTitle      lipgloss.Style
	DetailLabel      lipgloss.Style
	DetailValue      lipgloss.Style
	DetailTransition lipgloss.Style
	DetailAction     lipgloss.Style
	DetailGuard      lipgloss.Style

	// Help bar
	HelpKey  lipgloss.Style
	HelpText lipgloss.Style
}

// DefaultStyles returns the default style set.
func DefaultStyles() *Styles {
	return &Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorMuted).
			PaddingBottom(1).
			MarginBottom(1),

		Footer: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorMuted).
			PaddingTop(1).
			MarginTop(1).
			Foreground(ColorMuted),

		TreeNode: lipgloss.NewStyle().
			Foreground(ColorForeground),

		TreeNodeSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1),

		TreeBranch: lipgloss.NewStyle().
			Foreground(ColorMuted),

		TreeLeaf: lipgloss.NewStyle().
			Foreground(ColorSecondary),

		TreeInitial: lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true),

		TreeFinal: lipgloss.NewStyle().
			Foreground(ColorAccent),

		TreeCompound: lipgloss.NewStyle().
			Foreground(ColorPrimary),

		TreeParallel: lipgloss.NewStyle().
			Foreground(ColorSecondary),

		TreeHistory: lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true),

		DetailPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(1, 2).
			MarginLeft(2),

		DetailTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1),

		DetailLabel: lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(12),

		DetailValue: lipgloss.NewStyle().
			Foreground(ColorForeground),

		DetailTransition: lipgloss.NewStyle().
			Foreground(ColorSecondary),

		DetailAction: lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Italic(true),

		DetailGuard: lipgloss.NewStyle().
			Foreground(ColorAccent),

		HelpKey: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		HelpText: lipgloss.NewStyle().
			Foreground(ColorMuted),
	}
}
