package statekit

import "testing"

func TestState_Matches(t *testing.T) {
	state := State[struct{}]{
		Value: "green",
	}

	if !state.Matches("green") {
		t.Error("expected state to match 'green'")
	}

	if state.Matches("red") {
		t.Error("expected state not to match 'red'")
	}
}

func TestStateType_ReExports(t *testing.T) {
	// Verify constants are properly re-exported
	if StateTypeAtomic.String() != "atomic" {
		t.Errorf("expected 'atomic', got %v", StateTypeAtomic.String())
	}
	if StateTypeCompound.String() != "compound" {
		t.Errorf("expected 'compound', got %v", StateTypeCompound.String())
	}
	if StateTypeFinal.String() != "final" {
		t.Errorf("expected 'final', got %v", StateTypeFinal.String())
	}
}

func TestEvent_Creation(t *testing.T) {
	event := Event{
		Type:    "TIMER",
		Payload: map[string]int{"count": 1},
	}

	if event.Type != "TIMER" {
		t.Errorf("expected event type 'TIMER', got %v", event.Type)
	}

	payload, ok := event.Payload.(map[string]int)
	if !ok {
		t.Fatal("expected payload to be map[string]int")
	}
	if payload["count"] != 1 {
		t.Errorf("expected count 1, got %v", payload["count"])
	}
}
