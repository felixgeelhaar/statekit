package mermaid

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/statekit/viz"
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
