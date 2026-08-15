package generate

import (
	"strings"
	"testing"
)

func TestGenerator_SimpleStateMachine(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "traffic-light",
		"initial": "green",
		"states": {
			"green": {
				"id": "green",
				"type": "atomic",
				"transitions": [ { "event": "TIMER", "target": "yellow" } ]
			},
			"yellow": {
				"id": "yellow",
				"type": "atomic",
				"transitions": [ { "event": "TIMER", "target": "red" } ]
			},
			"red": {
				"id": "red",
				"type": "atomic",
				"transitions": [ { "event": "TIMER", "target": "green" } ]
			}
		}
	}`

	gen := NewGenerator("main", "TrafficLight", "struct{}")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check package
	if !strings.Contains(codeStr, "package main") {
		t.Error("expected package main")
	}

	// Check function name
	if !strings.Contains(codeStr, "func BuildTrafficLight()") {
		t.Error("expected BuildTrafficLight function")
	}

	// Check machine ID
	if !strings.Contains(codeStr, `NewMachine[TrafficLightContext]("traffic-light")`) {
		t.Error("expected machine ID")
	}

	// Check initial state
	if !strings.Contains(codeStr, `WithInitial("green")`) {
		t.Error("expected initial state")
	}

	// Check states
	if !strings.Contains(codeStr, `State("green")`) {
		t.Error("expected green state")
	}
	if !strings.Contains(codeStr, `State("yellow")`) {
		t.Error("expected yellow state")
	}
	if !strings.Contains(codeStr, `State("red")`) {
		t.Error("expected red state")
	}

	// Check transitions
	if !strings.Contains(codeStr, `On("TIMER").Target("yellow")`) {
		t.Error("expected TIMER transition to yellow")
	}
}

func TestGenerator_WithActions(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "counter",
		"initial": "idle",
		"states": {
			"idle": {
				"id": "idle",
				"type": "atomic",
				"entry": ["logEntry"],
				"exit": ["logExit"],
				"transitions": [ { "event": "COUNT", "target": "counting", "actions": ["increment"] } ]
			},
			"counting": {
				"id": "counting",
				"type": "atomic",
				"transitions": [ { "event": "STOP", "target": "idle" } ]
			}
		}
	}`

	gen := NewGenerator("counter", "Counter", "CounterContext")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check action stubs
	if !strings.Contains(codeStr, "func actionLogEntry(") {
		t.Error("expected logEntry action stub")
	}
	if !strings.Contains(codeStr, "func actionLogExit(") {
		t.Error("expected logExit action stub")
	}
	if !strings.Contains(codeStr, "func actionIncrement(") {
		t.Error("expected increment action stub")
	}

	// Check WithAction calls
	if !strings.Contains(codeStr, `WithAction("logEntry", actionLogEntry)`) {
		t.Error("expected WithAction for logEntry")
	}

	// Check OnEntry/OnExit
	if !strings.Contains(codeStr, `OnEntry("logEntry")`) {
		t.Error("expected OnEntry")
	}
	if !strings.Contains(codeStr, `OnExit("logExit")`) {
		t.Error("expected OnExit")
	}

	// Check transition action
	if !strings.Contains(codeStr, `.Do("increment")`) {
		t.Error("expected Do action on transition")
	}
}

func TestGenerator_WithGuards(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "door",
		"initial": "closed",
		"states": {
			"closed": {
				"id": "closed",
				"type": "atomic",
				"transitions": [ { "event": "OPEN", "target": "open", "guard": "canOpen" } ]
			},
			"open": {
				"id": "open",
				"type": "atomic",
				"transitions": [ { "event": "CLOSE", "target": "closed", "guard": "canClose" } ]
			}
		}
	}`

	gen := NewGenerator("main", "Door", "struct{}")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check guard stubs
	if !strings.Contains(codeStr, "func guardCanOpen(") {
		t.Error("expected canOpen guard stub")
	}
	if !strings.Contains(codeStr, "func guardCanClose(") {
		t.Error("expected canClose guard stub")
	}

	// Check WithGuard calls
	if !strings.Contains(codeStr, `WithGuard("canOpen", guardCanOpen)`) {
		t.Error("expected WithGuard for canOpen")
	}

	// Check Guard on transition
	if !strings.Contains(codeStr, `.Guard("canOpen")`) {
		t.Error("expected Guard on transition")
	}
}

func TestGenerator_FinalState(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "workflow",
		"initial": "active",
		"states": {
			"active": {
				"id": "active",
				"type": "atomic",
				"transitions": [ { "event": "COMPLETE", "target": "done" } ]
			},
			"done": {
				"id": "done",
				"type": "final"
			}
		}
	}`

	gen := NewGenerator("main", "Workflow", "struct{}")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check final state
	if !strings.Contains(codeStr, `State("done").`) && !strings.Contains(codeStr, "Final()") {
		t.Error("expected Final() for done state")
	}
}

func TestGenerator_HierarchicalStates(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "nested",
		"initial": "parent",
		"states": {
			"parent": {
				"id": "parent",
				"type": "compound",
				"initial": "child1",
				"children": ["child1", "child2"]
			},
			"child1": {
				"id": "child1",
				"type": "atomic",
				"parent": "parent",
				"transitions": [ { "event": "NEXT", "target": "child2" } ]
			},
			"child2": {
				"id": "child2",
				"type": "atomic",
				"parent": "parent"
			}
		}
	}`

	gen := NewGenerator("main", "Nested", "struct{}")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check parent state with initial
	if !strings.Contains(codeStr, `State("parent")`) {
		t.Error("expected parent state")
	}
	if !strings.Contains(codeStr, `WithInitial("child1")`) {
		t.Error("expected initial child1")
	}

	// Check child states
	if !strings.Contains(codeStr, `State("child1")`) {
		t.Error("expected child1 state")
	}
	if !strings.Contains(codeStr, `State("child2")`) {
		t.Error("expected child2 state")
	}
}

func TestGenerator_MultipleTransitionActions(t *testing.T) {
	t.Parallel()
	json := `{
		"id": "multi",
		"initial": "a",
		"states": {
			"a": {
				"id": "a",
				"type": "atomic",
				"transitions": [ { "event": "GO", "target": "b", "actions": ["action1", "action2"] } ]
			},
			"b": {
				"id": "b",
				"type": "atomic"
			}
		}
	}`

	gen := NewGenerator("main", "Multi", "struct{}")
	code, err := gen.Generate(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}

	codeStr := string(code)

	// Check both actions are included
	if !strings.Contains(codeStr, `.Do("action1")`) {
		t.Error("expected action1")
	}
	if !strings.Contains(codeStr, `.Do("action2")`) {
		t.Error("expected action2")
	}
}

func TestToGoIdentifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"logEntry", "LogEntry"},
		{"log_entry", "LogEntry"},
		{"log-entry", "LogEntry"},
		{"LOG_ENTRY", "LOGENTRY"},
		{"canOpen", "CanOpen"},
		{"action1", "Action1"},
	}

	for _, tc := range tests {
		result := toGoIdentifier(tc.input)
		if result != tc.expected {
			t.Errorf("toGoIdentifier(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGenerator_DefaultValues(t *testing.T) {
	t.Parallel()
	gen := NewGenerator("", "", "")

	if gen.PackageName != "main" {
		t.Errorf("expected default package 'main', got %q", gen.PackageName)
	}
	if gen.TypeName != "Machine" {
		t.Errorf("expected default type 'Machine', got %q", gen.TypeName)
	}
	if gen.ContextType != "struct{}" {
		t.Errorf("expected default context 'struct{}', got %q", gen.ContextType)
	}
}
