package ascii

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/statekit/viz"
)

func TestRenderer_SimpleOutput(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:   "idle",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "GO", Target: "running"},
				},
			},
			"running": {
				ID:   "running",
				Type: viz.VizStateAtomic,
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	// Check title
	if !strings.Contains(output, "test") {
		t.Error("expected machine ID in output")
	}

	// Check states
	if !strings.Contains(output, "idle") {
		t.Error("expected 'idle' state in output")
	}
	if !strings.Contains(output, "running") {
		t.Error("expected 'running' state in output")
	}

	// Check transition
	if !strings.Contains(output, "GO") {
		t.Error("expected 'GO' event in output")
	}
}

func TestRenderer_UnicodeVsASCII(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {ID: "idle", Type: viz.VizStateAtomic},
		},
	}

	// Unicode
	r := NewRenderer()
	r.UseUnicode = true
	unicode := r.Render(machine)

	if !strings.Contains(unicode, "┌") {
		t.Error("expected Unicode box characters")
	}
	if !strings.Contains(unicode, "●") {
		t.Error("expected Unicode initial marker")
	}

	// ASCII
	r.UseUnicode = false
	ascii := r.Render(machine)

	if !strings.Contains(ascii, "+") {
		t.Error("expected ASCII box characters")
	}
	if !strings.Contains(ascii, "*") {
		t.Error("expected ASCII initial marker")
	}
}

func TestRenderer_FinalState(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "running",
		States: map[string]*viz.VizState{
			"running": {ID: "running", Type: viz.VizStateAtomic},
			"done":    {ID: "done", Type: viz.VizStateFinal},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	if !strings.Contains(output, "◉") {
		t.Error("expected final state marker")
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
				Children: []string{"idle"},
			},
			"idle": {
				ID:     "idle",
				Type:   viz.VizStateAtomic,
				Parent: "active",
			},
		},
	}

	r := NewRenderer()
	output := r.Render(machine)

	if !strings.Contains(output, "[+]") {
		t.Error("expected compound state indicator")
	}
}

func TestRenderer_WithActions(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:    "idle",
				Type:  viz.VizStateAtomic,
				Entry: []string{"onEnter"},
				Exit:  []string{"onExit"},
			},
		},
	}

	r := NewRenderer()
	r.ShowActions = true
	output := r.Render(machine)

	if !strings.Contains(output, "entry:onEnter") {
		t.Error("expected entry action in output")
	}
	if !strings.Contains(output, "exit:onExit") {
		t.Error("expected exit action in output")
	}

	// Without actions
	r.ShowActions = false
	output = r.Render(machine)

	if strings.Contains(output, "entry:") {
		t.Error("expected no entry action when ShowActions=false")
	}
}

func TestRenderer_TransitionsWithGuard(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID:   "idle",
				Type: viz.VizStateAtomic,
				Transitions: []viz.VizTransition{
					{Event: "GO", Target: "running", Guard: "canGo"},
				},
			},
			"running": {ID: "running", Type: viz.VizStateAtomic},
		},
	}

	r := NewRenderer()
	r.ShowGuards = true
	output := r.Render(machine)

	if !strings.Contains(output, "if canGo") {
		t.Error("expected guard condition in output")
	}
}
