package mermaid

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit/viz"
)

func TestRenderer_SimpleTrafficLight(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "traffic_light",
		Initial: "green",
		States: map[string]*viz.VizState{
			"green": {
				ID:   "green",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "TIMER", Target: "yellow"},
				},
			},
			"yellow": {
				ID:   "yellow",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "TIMER", Target: "red"},
				},
			},
			"red": {
				ID:   "red",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "TIMER", Target: "green"},
				},
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	// Check header
	if !strings.Contains(output, "stateDiagram-v2") {
		t.Error("expected stateDiagram-v2 header")
	}

	// Check initial
	if !strings.Contains(output, "[*] --> green") {
		t.Error("expected initial state marker")
	}

	// Check transitions
	if !strings.Contains(output, "green --> yellow : TIMER") {
		t.Error("expected green -> yellow transition")
	}
	if !strings.Contains(output, "yellow --> red : TIMER") {
		t.Error("expected yellow -> red transition")
	}
	if !strings.Contains(output, "red --> green : TIMER") {
		t.Error("expected red -> green transition")
	}
}

func TestRenderer_FinalState(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "running",
		States: map[string]*viz.VizState{
			"running": {
				ID:   "running",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "DONE", Target: "finished"},
				},
			},
			"finished": {
				ID:   "finished",
				Type: viz.VizStateFinal,
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	if !strings.Contains(output, "finished --> [*]") {
		t.Error("expected final state marker")
	}
}

func TestRenderer_WithGuardAndActions(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:   "idle",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "GO", Target: "running", Guard: "canGo", Actions: []string{"doAction"}},
				},
			},
			"running": {
				ID:   "running",
				Type: viz.VizStateAtomic,
			},
		},
	}

	r := NewRenderer()
	r.ShowGuards = true
	r.ShowActions = true
	output := r.Render(machine)

	if !strings.Contains(output, "[canGo]") {
		t.Error("expected guard in output")
	}
	if !strings.Contains(output, "/ doAction") {
		t.Error("expected action in output")
	}
}

func TestRenderer_Direction(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "a",
		States: map[string]*viz.VizState{
			"a": {ID: "a", Type: viz.VizStateAtomic},
		},
	}

	r := NewRenderer()
	r.Direction = "LR"
	output := r.Render(machine)

	if !strings.Contains(output, "direction LR") {
		t.Error("expected direction LR")
	}

	r.Direction = "TB"
	output = r.Render(machine)

	// TB is default, shouldn't appear
	if strings.Contains(output, "direction TB") {
		t.Error("TB direction should not be explicit")
	}
}

func TestRenderer_CompoundState(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "active",
		States: map[string]*viz.VizState{
			"active": {
				ID:       "active",
				Type:     viz.VizStateCompound,
				Initial:  "idle",
				Children: []string{"idle", "running"},
			},
			"idle": {
				ID:     "idle",
				Type:   viz.VizStateAtomic,
				Parent: "active",
			},
			"running": {
				ID:     "running",
				Type:   viz.VizStateAtomic,
				Parent: "active",
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	if !strings.Contains(output, "state active {") {
		t.Error("expected compound state block")
	}
	if !strings.Contains(output, "[*] --> idle") {
		t.Error("expected nested initial state")
	}
}

func TestRenderer_ParallelHistoryAndEntry(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "rich",
		Initial: "editor",
		States: map[string]*viz.VizState{
			"editor": {
				ID:       "editor",
				Type:     viz.VizStateParallel,
				Children: []string{"bold", "italic"},
			},
			"bold": {
				ID:       "bold",
				Type:     viz.VizStateCompound,
				Parent:   "editor",
				Initial:  "off",
				Children: []string{"off", "on"},
			},
			"italic": {
				ID:       "italic",
				Type:     viz.VizStateCompound,
				Parent:   "editor",
				Initial:  "off2",
				Children: []string{"off2"},
			},
			"off": {
				ID:     "off",
				Type:   viz.VizStateAtomic,
				Parent: "bold",
				Entry:  []string{"logOff"},
				Exit:   []string{"cleanup"},
				Transitions: []viz.VizTransition{
					{Event: "TOGGLE", Target: "on"},
				},
			},
			"on": {
				ID:     "on",
				Type:   viz.VizStateAtomic,
				Parent: "bold",
			},
			"off2": {
				ID:     "off2",
				Type:   viz.VizStateAtomic,
				Parent: "italic",
			},
			"hist": {
				ID:             "hist",
				Type:           viz.VizStateHistory,
				HistoryType:    "deep",
				HistoryDefault: "off",
			},
			"shallow": {
				ID:          "shallow",
				Type:        viz.VizStateHistory,
				HistoryType: "shallow",
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	if !strings.Contains(output, "state editor {") {
		t.Error("expected parallel state block")
	}
	if !strings.Contains(output, "--") {
		t.Error("expected region separator for parallel children")
	}
	if !strings.Contains(output, `state "hist (H*)" as hist`) {
		t.Error("expected deep history notation")
	}
	if !strings.Contains(output, `state "shallow (H)" as shallow`) {
		t.Error("expected shallow history notation")
	}
	if !strings.Contains(output, "hist --> off : default") {
		t.Error("expected history default edge")
	}
	if !strings.Contains(output, "note right of off : entry: logOff") {
		t.Error("expected entry note on atomic state")
	}
}

func TestRenderer_HideGuardsAndActions(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:   "idle",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "GO", Target: "running", Guard: "canGo", Actions: []string{"act"}},
				},
			},
			"running": {ID: "running", Type: viz.VizStateAtomic},
		},
	}

	r := NewRenderer()
	r.ShowGuards = false
	r.ShowActions = false
	output := r.Render(machine)

	if strings.Contains(output, "[canGo]") {
		t.Error("guards should be hidden")
	}
	if strings.Contains(output, "/ act") {
		t.Error("actions should be hidden")
	}
	if !strings.Contains(output, "idle --> running : GO") {
		t.Error("expected bare event label")
	}
}
