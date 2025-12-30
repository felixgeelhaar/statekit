package viz

import (
	"testing"
)

func TestParseXStateJSON_SimpleTrafficLight(t *testing.T) {
	input := `{
		"id": "traffic_light",
		"initial": "green",
		"states": {
			"green": {"on": {"TIMER": {"target": "yellow"}}},
			"yellow": {"on": {"TIMER": {"target": "red"}}},
			"red": {"on": {"TIMER": {"target": "green"}}}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	if machine.ID != "traffic_light" {
		t.Errorf("expected ID 'traffic_light', got '%s'", machine.ID)
	}

	if machine.Initial != "green" {
		t.Errorf("expected initial 'green', got '%s'", machine.Initial)
	}

	if len(machine.States) != 3 {
		t.Errorf("expected 3 states, got %d", len(machine.States))
	}

	// Check green state
	green := machine.States["green"]
	if green == nil {
		t.Fatal("expected 'green' state")
	}
	if green.Type != VizStateAtomic {
		t.Errorf("expected atomic type, got %s", green.Type)
	}
	if len(green.Transitions) != 1 {
		t.Errorf("expected 1 transition, got %d", len(green.Transitions))
	}
	if green.Transitions[0].Event != "TIMER" {
		t.Errorf("expected event 'TIMER', got '%s'", green.Transitions[0].Event)
	}
	if green.Transitions[0].Target != "yellow" {
		t.Errorf("expected target 'yellow', got '%s'", green.Transitions[0].Target)
	}
}

func TestParseXStateJSON_Hierarchical(t *testing.T) {
	input := `{
		"id": "nested",
		"initial": "active",
		"states": {
			"active": {
				"initial": "idle",
				"states": {
					"idle": {"on": {"START": {"target": "running"}}},
					"running": {"on": {"STOP": {"target": "idle"}}}
				}
			},
			"done": {"type": "final"}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	if len(machine.States) != 4 {
		t.Errorf("expected 4 states (active, idle, running, done), got %d", len(machine.States))
	}

	// Check active is compound
	active := machine.States["active"]
	if active == nil {
		t.Fatal("expected 'active' state")
	}
	if active.Type != VizStateCompound {
		t.Errorf("expected compound type, got %s", active.Type)
	}
	if active.Initial != "idle" {
		t.Errorf("expected initial 'idle', got '%s'", active.Initial)
	}
	if len(active.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(active.Children))
	}

	// Check idle has correct parent
	idle := machine.States["idle"]
	if idle == nil {
		t.Fatal("expected 'idle' state")
	}
	if idle.Parent != "active" {
		t.Errorf("expected parent 'active', got '%s'", idle.Parent)
	}
	if idle.Depth != 1 {
		t.Errorf("expected depth 1, got %d", idle.Depth)
	}

	// Check done is final
	done := machine.States["done"]
	if done == nil {
		t.Fatal("expected 'done' state")
	}
	if done.Type != VizStateFinal {
		t.Errorf("expected final type, got %s", done.Type)
	}
}

func TestParseXStateJSON_WithGuardsAndActions(t *testing.T) {
	input := `{
		"id": "test",
		"initial": "idle",
		"states": {
			"idle": {
				"entry": ["logEntry"],
				"exit": ["logExit"],
				"on": {
					"GO": {
						"target": "running",
						"guard": "canGo",
						"actions": ["doAction"]
					}
				}
			},
			"running": {}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	idle := machine.States["idle"]
	if idle == nil {
		t.Fatal("expected 'idle' state")
	}

	if len(idle.Entry) != 1 || idle.Entry[0] != "logEntry" {
		t.Errorf("expected entry action 'logEntry', got %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || idle.Exit[0] != "logExit" {
		t.Errorf("expected exit action 'logExit', got %v", idle.Exit)
	}

	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(idle.Transitions))
	}

	trans := idle.Transitions[0]
	if trans.Guard != "canGo" {
		t.Errorf("expected guard 'canGo', got '%s'", trans.Guard)
	}
	if len(trans.Actions) != 1 || trans.Actions[0] != "doAction" {
		t.Errorf("expected action 'doAction', got %v", trans.Actions)
	}
}

func TestParseXStateJSON_DelayedTransitions(t *testing.T) {
	input := `{
		"id": "test",
		"initial": "loading",
		"states": {
			"loading": {
				"after": {
					"1000": {"target": "timeout"}
				}
			},
			"timeout": {}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	loading := machine.States["loading"]
	if loading == nil {
		t.Fatal("expected 'loading' state")
	}

	if len(loading.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(loading.Transitions))
	}

	trans := loading.Transitions[0]
	if !trans.IsDelayed {
		t.Error("expected delayed transition")
	}
	if trans.DelayMs != 1000 {
		t.Errorf("expected delay 1000ms, got %d", trans.DelayMs)
	}
	if trans.Target != "timeout" {
		t.Errorf("expected target 'timeout', got '%s'", trans.Target)
	}
}

func TestParseXStateJSON_History(t *testing.T) {
	input := `{
		"id": "test",
		"initial": "active",
		"states": {
			"active": {
				"initial": "idle",
				"states": {
					"idle": {},
					"hist": {
						"type": "history",
						"history": "shallow",
						"target": "idle"
					}
				}
			}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	hist := machine.States["hist"]
	if hist == nil {
		t.Fatal("expected 'hist' state")
	}

	if hist.Type != VizStateHistory {
		t.Errorf("expected history type, got %s", hist.Type)
	}
	if hist.HistoryType != "shallow" {
		t.Errorf("expected shallow history, got '%s'", hist.HistoryType)
	}
	if hist.HistoryDefault != "idle" {
		t.Errorf("expected default 'idle', got '%s'", hist.HistoryDefault)
	}
}

func TestParseXStateJSON_Parallel(t *testing.T) {
	input := `{
		"id": "test",
		"initial": "active",
		"states": {
			"active": {
				"type": "parallel",
				"states": {
					"upload": {
						"initial": "pending",
						"states": {
							"pending": {},
							"done": {"type": "final"}
						}
					},
					"download": {
						"initial": "waiting",
						"states": {
							"waiting": {},
							"done": {"type": "final"}
						}
					}
				}
			}
		}
	}`

	machine, err := ParseXStateJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseXStateJSON failed: %v", err)
	}

	active := machine.States["active"]
	if active == nil {
		t.Fatal("expected 'active' state")
	}

	if active.Type != VizStateParallel {
		t.Errorf("expected parallel type, got %s", active.Type)
	}

	if len(active.Children) != 2 {
		t.Errorf("expected 2 regions, got %d", len(active.Children))
	}
}

func TestVizMachine_GetRootStates(t *testing.T) {
	machine := &VizMachine{
		ID:      "test",
		Initial: "a",
		States: map[string]*VizState{
			"a":  {ID: "a", Parent: ""},
			"b":  {ID: "b", Parent: ""},
			"a1": {ID: "a1", Parent: "a"},
		},
	}

	roots := machine.GetRootStates()
	if len(roots) != 2 {
		t.Errorf("expected 2 root states, got %d", len(roots))
	}
}

func TestVizMachine_IsInitial(t *testing.T) {
	machine := &VizMachine{
		ID:      "test",
		Initial: "a",
		States: map[string]*VizState{
			"a":  {ID: "a", Parent: "", Initial: "a1", Children: []string{"a1", "a2"}},
			"a1": {ID: "a1", Parent: "a"},
			"a2": {ID: "a2", Parent: "a"},
		},
	}

	if !machine.IsInitial("a") {
		t.Error("expected 'a' to be initial")
	}
	if !machine.IsInitial("a1") {
		t.Error("expected 'a1' to be initial of 'a'")
	}
	if machine.IsInitial("a2") {
		t.Error("expected 'a2' to NOT be initial")
	}
}
