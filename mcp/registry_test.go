package mcp

import (
	"testing"

	"github.com/felixgeelhaar/statekit/viz"
)

func trafficLightViz() *viz.VizMachine {
	return &viz.VizMachine{
		ID:      "traffic-light",
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
}

func TestRegistry_Create(t *testing.T) {
	reg := NewRegistry()

	err := reg.Create(trafficLightViz())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Duplicate should fail
	err = reg.Create(trafficLightViz())
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()
	reg.Create(trafficLightViz())

	inst, ok := reg.Get("traffic-light")
	if !ok {
		t.Fatal("expected to find machine")
	}
	if string(inst.interp.State().Value) != "green" {
		t.Errorf("state = %q, want green", inst.interp.State().Value)
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Create(trafficLightViz())

	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(list))
	}
	if list[0].ID != "traffic-light" {
		t.Errorf("ID = %q, want traffic-light", list[0].ID)
	}
	if list[0].CurrentState != "green" {
		t.Errorf("state = %q, want green", list[0].CurrentState)
	}
}

func TestRegistry_Delete(t *testing.T) {
	reg := NewRegistry()
	reg.Create(trafficLightViz())

	if !reg.Delete("traffic-light") {
		t.Fatal("expected delete to succeed")
	}
	if reg.Delete("traffic-light") {
		t.Fatal("expected second delete to return false")
	}

	_, ok := reg.Get("traffic-light")
	if ok {
		t.Fatal("expected machine to be gone after delete")
	}
}

func TestBuildFromViz_Hierarchical(t *testing.T) {
	vm := &viz.VizMachine{
		ID:      "nested",
		Initial: "active",
		States: map[string]*viz.VizState{
			"active": {
				ID:       "active",
				Type:     viz.VizStateCompound,
				Initial:  "idle",
				Children: []string{"idle", "working"},
				Transitions: []viz.VizTransition{
					{Event: "RESET", Target: "done"},
				},
			},
			"idle": {
				ID:     "idle",
				Type:   viz.VizStateAtomic,
				Parent: "active",
				Transitions: []viz.VizTransition{
					{Event: "START", Target: "working"},
				},
			},
			"working": {
				ID:     "working",
				Type:   viz.VizStateAtomic,
				Parent: "active",
				Transitions: []viz.VizTransition{
					{Event: "STOP", Target: "idle"},
				},
			},
			"done": {
				ID:   "done",
				Type: viz.VizStateFinal,
			},
		},
	}

	_, err := buildFromViz(vm)
	if err != nil {
		t.Fatalf("buildFromViz: %v", err)
	}
}
