package tui

import (
	"github.com/charmbracelet/bubbletea"
)

// KeyMap defines keyboard bindings for the TUI.
type KeyMap struct {
	Up       []string
	Down     []string
	Left     []string
	Right    []string
	Enter    []string
	Quit     []string
	Help     []string
	Collapse []string
	Expand   []string
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() *KeyMap {
	return &KeyMap{
		Up:       []string{"k", "up"},
		Down:     []string{"j", "down"},
		Left:     []string{"h", "left"},
		Right:    []string{"l", "right"},
		Enter:    []string{"enter"},
		Quit:     []string{"q", "ctrl+c", "esc"},
		Help:     []string{"?"},
		Collapse: []string{"-"},
		Expand:   []string{"+", "="},
	}
}

// Matches checks if a key matches any of the given bindings.
func (km *KeyMap) Matches(msg tea.KeyMsg, bindings []string) bool {
	for _, b := range bindings {
		if msg.String() == b {
			return true
		}
	}
	return false
}

// HelpItems returns help text for display.
func (km *KeyMap) HelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Desc: "navigate"},
		{Key: "h/l", Desc: "collapse/expand"},
		{Key: "+/-", Desc: "expand/collapse all"},
		{Key: "enter", Desc: "select"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
}

// HelpItem represents a single help entry.
type HelpItem struct {
	Key  string
	Desc string
}
