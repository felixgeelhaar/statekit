package html

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit/viz"
)

func TestRenderer_Render(t *testing.T) {
	machine := &viz.VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*viz.VizState{
			"idle": {
				ID: "idle",
				Transitions: []viz.VizTransition{
					{Event: "NEXT", Target: "active"},
				},
			},
			"active": {ID: "active"},
		},
	}

	renderer := NewRenderer()
	html, err := renderer.Render(machine)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, `"id":"test"`) {
		t.Error("expected machine ID in JSON")
	}
	if !strings.Contains(html, `"initial":"idle"`) {
		t.Error("expected initial state in JSON")
	}
}
