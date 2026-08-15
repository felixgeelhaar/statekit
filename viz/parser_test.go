package viz

import (
	"encoding/json"
	"testing"
)

func TestParseNativeJSON_Simple(t *testing.T) {
	t.Parallel()
	// Construct a VizMachine manually and marshal it to JSON
	vm := &VizMachine{
		ID:      "test",
		Initial: "idle",
		States: map[string]*VizState{
			"idle": {
				ID:   "idle",
				Type: VizStateAtomic,
				Transitions: []VizTransition{
					{Event: "GO", Target: "active"},
				},
			},
			"active": {
				ID:   "active",
				Type: VizStateAtomic,
			},
		},
	}

	data, err := json.Marshal(vm)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Parse it back
	parsed, err := ParseNativeJSON(data)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if parsed.ID != "test" {
		t.Errorf("expected ID 'test', got %s", parsed.ID)
	}
	if len(parsed.States) != 2 {
		t.Errorf("expected 2 states, got %d", len(parsed.States))
	}
	if parsed.States["idle"].Depth != 0 {
		t.Errorf("expected depth 0, got %d", parsed.States["idle"].Depth)
	}
}
