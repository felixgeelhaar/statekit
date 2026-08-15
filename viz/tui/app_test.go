package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go.klarlabs.de/statekit/viz"
)

func createTestMachine() *viz.VizMachine {
	return &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:   "idle",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "START", Target: "running"},
				},
			},
			"running": {
				ID:   "running",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "STOP", Target: "idle"},
				},
			},
		},
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.Machine != machine {
		t.Error("expected machine to be set")
	}
	if model.tree == nil {
		t.Error("expected tree to be initialized")
	}
	if model.styles == nil {
		t.Error("expected styles to be initialized")
	}
	if model.keymap == nil {
		t.Error("expected keymap to be initialized")
	}
}

func TestModel_Init(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	cmd := model.Init()
	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	// Send window size message
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(msg)

	m := newModel.(*Model)
	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
	if !m.ready {
		t.Error("expected ready to be true")
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	// Initialize with window size
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Start at first node
	selected := model.tree.Selected()
	if selected == nil {
		t.Fatal("expected initial selection")
	}
	initialID := selected.State.ID

	// Navigate down
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newSelected := model.tree.Selected()
	if newSelected.State.ID == initialID && len(model.tree.FlatList) > 1 {
		t.Error("expected cursor to move down")
	}

	// Navigate back up
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	upSelected := model.tree.Selected()
	if upSelected.State.ID != initialID {
		t.Error("expected cursor to move back up")
	}
}

func TestModel_Update_Quit(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	// Send quit key
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Should return quit command
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModel_View_NotReady(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	view := model.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...', got %q", view)
	}
}

func TestModel_View_Ready(t *testing.T) {
	t.Parallel()
	machine := createTestMachine()
	model := New(machine)

	// Initialize with window size
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	view := model.View()
	if view == "Loading..." {
		t.Error("expected rendered view, got Loading...")
	}
	if view == "" {
		t.Error("expected non-empty view")
	}
}
