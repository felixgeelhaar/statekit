package goparser

import (
	"testing"

	"go.klarlabs.de/statekit/viz"
)

func TestParser_ParsePackage_StatekitReflectTest(t *testing.T) {
	// Parse the actual statekit package to find machine definitions in reflect_test.go
	parser := NewParser()
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	// Should find several machine definitions from reflect_test.go
	if len(machines) == 0 {
		t.Fatal("expected to find machine definitions")
	}

	// Look for known machines
	found := make(map[string]bool)
	for _, m := range machines {
		found[m.ID] = true
	}

	expectedMachines := []string{"simple", "actions", "guards", "final", "hierarchical", "context"}
	for _, expected := range expectedMachines {
		if !found[expected] {
			t.Errorf("expected to find machine %q", expected)
		}
	}
}

func TestParser_ParsePackage_WithTypeFilter(t *testing.T) {
	parser := NewParser().WithTypeFilter("SimpleReflectMachine")
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	if len(machines) != 1 {
		t.Fatalf("expected 1 machine with filter, got %d", len(machines))
	}

	if machines[0].ID != "simple" {
		t.Errorf("expected machine ID 'simple', got %q", machines[0].ID)
	}
}

func TestParser_VerifyMachineStructure(t *testing.T) {
	parser := NewParser().WithTypeFilter("SimpleReflectMachine")
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	if len(machines) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(machines))
	}

	m := machines[0]

	// Verify basic structure
	if m.Initial != "idle" {
		t.Errorf("expected initial 'idle', got %q", m.Initial)
	}

	if len(m.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(m.States))
	}

	// Verify idle state
	idle := m.States["idle"]
	if idle == nil {
		t.Fatal("idle state not found")
	}
	if idle.Type != viz.VizStateAtomic {
		t.Errorf("expected atomic type, got %s", idle.Type)
	}
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition on idle, got %d", len(idle.Transitions))
	}
	if idle.Transitions[0].Event != "START" {
		t.Errorf("expected event 'START', got %q", idle.Transitions[0].Event)
	}
	if idle.Transitions[0].Target != "running" {
		t.Errorf("expected target 'running', got %q", idle.Transitions[0].Target)
	}

	// Verify running state
	running := m.States["running"]
	if running == nil {
		t.Fatal("running state not found")
	}
	if len(running.Transitions) != 1 {
		t.Fatalf("expected 1 transition on running, got %d", len(running.Transitions))
	}
}

func TestParser_MachineWithActions(t *testing.T) {
	parser := NewParser().WithTypeFilter("ActionReflectMachine")
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	if len(machines) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(machines))
	}

	m := machines[0]

	idle := m.States["idle"]
	if idle == nil {
		t.Fatal("idle state not found")
	}
	if len(idle.Entry) != 1 || idle.Entry[0] != "onEnterIdle" {
		t.Errorf("expected entry action 'onEnterIdle', got %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || idle.Exit[0] != "onExitIdle" {
		t.Errorf("expected exit action 'onExitIdle', got %v", idle.Exit)
	}
}

func TestParser_MachineWithGuards(t *testing.T) {
	parser := NewParser().WithTypeFilter("GuardReflectMachine")
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	if len(machines) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(machines))
	}

	m := machines[0]

	idle := m.States["idle"]
	if idle == nil {
		t.Fatal("idle state not found")
	}
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(idle.Transitions))
	}
	if idle.Transitions[0].Guard != "canStart" {
		t.Errorf("expected guard 'canStart', got %q", idle.Transitions[0].Guard)
	}
}

func TestParser_FinalState(t *testing.T) {
	parser := NewParser().WithTypeFilter("FinalReflectMachine")
	machines, err := parser.ParsePackage("go.klarlabs.de/statekit")
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	if len(machines) != 1 {
		t.Fatalf("expected 1 machine, got %d", len(machines))
	}

	m := machines[0]

	done := m.States["done"]
	if done == nil {
		t.Fatal("done state not found")
	}
	if done.Type != viz.VizStateFinal {
		t.Errorf("expected final type, got %s", done.Type)
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"idle", "idle"},
		{"Idle", "idle"},
		{"IdleState", "idle_state"},
		{"HTTPServer", "http_server"},
		{"APIGateway", "api_gateway"},
		{"getHTTPResponse", "get_http_response"},
		{"XMLHTTPRequest", "xmlhttp_request"},
		{"ID", "id"},
		{"userID", "user_id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTransition(t *testing.T) {
	tests := []struct {
		input   string
		event   string
		target  string
		guard   string
		actions []string
		wantErr bool
	}{
		{"START->running", "START", "running", "", nil, false},
		{"START->running:canStart", "START", "running", "canStart", nil, false},
		{"START->running/doAction", "START", "running", "", []string{"doAction"}, false},
		{"START->running/action1;action2", "START", "running", "", []string{"action1", "action2"}, false},
		{"START->running/action:guard", "START", "running", "guard", []string{"action"}, false},
		{"invalid", "", "", "", nil, true},
		{"->target", "", "", "", nil, true},
		{"EVENT->", "", "", "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			trans, err := parseTransition(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if trans.Event != tt.event {
				t.Errorf("event = %q, want %q", trans.Event, tt.event)
			}
			if trans.Target != tt.target {
				t.Errorf("target = %q, want %q", trans.Target, tt.target)
			}
			if trans.Guard != tt.guard {
				t.Errorf("guard = %q, want %q", trans.Guard, tt.guard)
			}
			if len(trans.Actions) != len(tt.actions) {
				t.Errorf("actions = %v, want %v", trans.Actions, tt.actions)
			}
		})
	}
}

func TestParseTransitions(t *testing.T) {
	transitions, err := parseTransitions("START->running,STOP->idle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}

	if transitions[0].Event != "START" || transitions[0].Target != "running" {
		t.Errorf("first transition = %+v", transitions[0])
	}
	if transitions[1].Event != "STOP" || transitions[1].Target != "idle" {
		t.Errorf("second transition = %+v", transitions[1])
	}
}
