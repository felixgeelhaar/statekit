package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"go.klarlabs.de/statekit/viz"
)

func hierarchicalMachine() *viz.VizMachine {
	return &viz.VizMachine{
		ID:      "editor",
		Initial: "editing",
		States: map[string]*viz.VizState{
			"editing": {
				ID:       "editing",
				Type:     viz.VizStateCompound,
				Initial:  "idle",
				Children: []string{"idle", "dirty", "hist"},
				Depth:    0,
			},
			"idle": {
				ID:     "idle",
				Type:   viz.VizStateAtomic,
				Parent: "editing",
				Depth:  1,
				Transitions: []viz.VizTransition{
					{Event: "TYPE", Target: "dirty"},
				},
			},
			"dirty": {
				ID:     "dirty",
				Type:   viz.VizStateAtomic,
				Parent: "editing",
				Depth:  1,
			},
			"hist": {
				ID:          "hist",
				Type:        viz.VizStateHistory,
				Parent:      "editing",
				HistoryType: "deep",
				Depth:       1,
			},
			"done": {
				ID:    "done",
				Type:  viz.VizStateFinal,
				Depth: 0,
			},
			"split": {
				ID:       "split",
				Type:     viz.VizStateParallel,
				Children: []string{"left"},
				Depth:    0,
			},
			"left": {
				ID:     "left",
				Type:   viz.VizStateCompound,
				Parent: "split",
				Depth:  1,
			},
		},
	}
}

func TestTreeView_ExpandCollapseToggle(t *testing.T) {
	tv := NewTreeView(hierarchicalMachine(), DefaultStyles())
	if len(tv.Root) == 0 {
		t.Fatal("expected root nodes")
	}

	// Find editing compound in flat list
	idx := -1
	for i, n := range tv.FlatList {
		if n.State.ID == "editing" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("editing node missing")
	}
	tv.cursor = idx
	before := len(tv.FlatList)

	tv.Collapse()
	if tv.FlatList[tv.cursor].Expanded {
		t.Error("expected editing collapsed")
	}
	if len(tv.FlatList) >= before {
		t.Error("expected fewer visible nodes after collapse")
	}

	tv.Expand()
	if !tv.FlatList[tv.cursor].Expanded {
		t.Error("expected editing expanded")
	}

	tv.Toggle()
	if tv.FlatList[tv.cursor].Expanded {
		t.Error("expected toggle to collapse")
	}
	tv.Toggle()
	if !tv.FlatList[tv.cursor].Expanded {
		t.Error("expected toggle to expand")
	}

	tv.CollapseAll()
	for _, n := range tv.Root {
		if n.Expanded {
			t.Errorf("root %s still expanded after CollapseAll", n.State.ID)
		}
	}

	tv.ExpandAll()
	for _, n := range tv.FlatList {
		if len(n.Children) > 0 && !n.Expanded {
			t.Errorf("node %s not expanded after ExpandAll", n.State.ID)
		}
	}
}

func TestTreeView_SelectedAndIndicators(t *testing.T) {
	tv := NewTreeView(hierarchicalMachine(), DefaultStyles())
	tv.cursor = -1
	if tv.Selected() != nil {
		t.Error("expected nil selection for invalid cursor")
	}
	tv.cursor = 0
	if tv.Selected() == nil {
		t.Fatal("expected selection")
	}

	view := tv.View(80, 20)
	for _, want := range []string{"[+]", "H*", "◉", "[||]"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing indicator %q\n%s", want, view)
		}
	}
}

func TestModel_Update_TreeKeys(t *testing.T) {
	model := New(hierarchicalMachine())
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Move to compound "editing"
	for i, n := range model.tree.FlatList {
		if n.State.ID == "editing" {
			model.tree.cursor = i
			break
		}
	}

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}) // collapse
	if model.tree.Selected().Expanded {
		t.Error("expected h to collapse")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}) // expand
	if !model.tree.Selected().Expanded {
		t.Error("expected l to expand")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // toggle
	if model.tree.Selected().Expanded {
		t.Error("expected enter to toggle closed")
	}
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}) // expand all
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}) // collapse all

	view := model.View()
	if !strings.Contains(view, "editor") && !strings.Contains(view, "editing") {
		// header uses machine id; tree shows states
		if view == "" {
			t.Error("expected non-empty view")
		}
	}
}

func TestKeyMap_HelpAndMatches(t *testing.T) {
	km := DefaultKeyMap()
	if !km.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, km.Quit) {
		t.Error("expected q to match Quit")
	}
	if km.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, km.Quit) {
		t.Error("x should not match Quit")
	}
	items := km.HelpItems()
	if len(items) < 4 {
		t.Fatalf("HelpItems = %d, want >= 4", len(items))
	}
}
func TestModel_DetailPanelRichState(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "rich",
		Initial: "loading",
		States: map[string]*viz.VizState{
			"loading": {
				ID:     "loading",
				Type:   viz.VizStateAtomic,
				Parent: "root",
				Entry:  []string{"startFetch"},
				Exit:   []string{"cancelFetch"},
				Transitions: []viz.VizTransition{
					{Event: "OK", Target: "ready", Guard: "valid", Actions: []string{"apply"}},
					{Event: "", Target: "timeout", IsDelayed: true, DelayMs: 5000},
				},
				Invocations: []viz.VizInvoke{{ID: "fetch"}},
			},
			"root": {
				ID:       "root",
				Type:     viz.VizStateCompound,
				Initial:  "loading",
				Children: []string{"loading"},
			},
			"ready":   {ID: "ready", Type: viz.VizStateAtomic},
			"timeout": {ID: "timeout", Type: viz.VizStateAtomic},
			"shallowH": {
				ID:          "shallowH",
				Type:        viz.VizStateHistory,
				HistoryType: "shallow",
			},
		},
	}

	model := New(machine)
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	for i, n := range model.tree.FlatList {
		if n.State.ID == "loading" {
			model.tree.cursor = i
			break
		}
	}

	view := model.View()
	for _, want := range []string{"loading", "Type:", "Parent:", "Entry:", "Exit:", "Transitions:", "Invokes:", "after 5000ms"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q\n%s", want, view)
		}
	}

	tv := model.tree
	tv.cursor = len(tv.FlatList) - 1
	_ = tv.View(40, 2)

	tv.cursor = -1
	model2 := New(machine)
	model2.tree = tv
	model2.ready = true
	model2.width, model2.height = 80, 24
	if !strings.Contains(model2.renderDetailPanel(), "No state selected") {
		t.Error("expected empty selection message")
	}

	styles := DefaultStyles()
	tv2 := NewTreeView(machine, styles)
	for _, n := range tv2.FlatList {
		if n.State.ID == "shallowH" {
			line := tv2.renderNode(n, false)
			if strings.Contains(line, "H*") {
				t.Errorf("shallow history rendered as deep: %s", line)
			}
			break
		}
	}
}
