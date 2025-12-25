package parser

import (
	"reflect"
	"testing"
)

// Mock marker types for testing (must match the constant names in parser.go)
type MachineDef struct{}
type StateNode struct{}
type CompoundNode struct{}
type FinalNode struct{}

func TestParseMachineStruct_Simple(t *testing.T) {
	type SimpleMachine struct {
		MachineDef `id:"simple" initial:"idle"`
		Idle       StateNode `on:"START->running"`
		Running    StateNode `on:"STOP->idle"`
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(SimpleMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.ID != "simple" {
		t.Errorf("expected ID 'simple', got %q", schema.ID)
	}
	if schema.Initial != "idle" {
		t.Errorf("expected Initial 'idle', got %q", schema.Initial)
	}
	if len(schema.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(schema.States))
	}

	// Check idle state
	idle := schema.States[0]
	if idle.Name != "idle" {
		t.Errorf("expected state name 'idle', got %q", idle.Name)
	}
	if idle.Type != StateSchemaAtomic {
		t.Errorf("expected StateSchemaAtomic, got %v", idle.Type)
	}
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(idle.Transitions))
	}
	if idle.Transitions[0].Event != "START" {
		t.Errorf("expected event 'START', got %q", idle.Transitions[0].Event)
	}
	if idle.Transitions[0].Target != "running" {
		t.Errorf("expected target 'running', got %q", idle.Transitions[0].Target)
	}
}

func TestParseMachineStruct_WithActions(t *testing.T) {
	type ActionMachine struct {
		MachineDef `id:"actions" initial:"idle"`
		Idle       StateNode `on:"START->running" entry:"logStart" exit:"logExit"`
		Running    StateNode `on:"STOP->idle" entry:"logRunning,trackMetrics"`
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(ActionMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idle := schema.States[0]
	if len(idle.Entry) != 1 || idle.Entry[0] != "logStart" {
		t.Errorf("expected entry ['logStart'], got %v", idle.Entry)
	}
	if len(idle.Exit) != 1 || idle.Exit[0] != "logExit" {
		t.Errorf("expected exit ['logExit'], got %v", idle.Exit)
	}

	running := schema.States[1]
	if len(running.Entry) != 2 {
		t.Fatalf("expected 2 entry actions, got %d", len(running.Entry))
	}
	if running.Entry[0] != "logRunning" || running.Entry[1] != "trackMetrics" {
		t.Errorf("expected entry ['logRunning', 'trackMetrics'], got %v", running.Entry)
	}
}

func TestParseMachineStruct_WithGuards(t *testing.T) {
	type GuardMachine struct {
		MachineDef `id:"guards" initial:"idle"`
		Idle       StateNode `on:"START->running:canStart"`
		Running    StateNode `on:"STOP->idle"`
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(GuardMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idle := schema.States[0]
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(idle.Transitions))
	}
	if idle.Transitions[0].Guard != "canStart" {
		t.Errorf("expected guard 'canStart', got %q", idle.Transitions[0].Guard)
	}
}

func TestParseMachineStruct_WithTransitionActions(t *testing.T) {
	type TransActionMachine struct {
		MachineDef `id:"transactions" initial:"idle"`
		Idle       StateNode `on:"START->running/logTransition;notify"`
		Running    StateNode `on:"STOP->idle"`
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(TransActionMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idle := schema.States[0]
	if len(idle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(idle.Transitions))
	}
	trans := idle.Transitions[0]
	if len(trans.Actions) != 2 {
		t.Fatalf("expected 2 transition actions, got %d", len(trans.Actions))
	}
	if trans.Actions[0] != "logTransition" || trans.Actions[1] != "notify" {
		t.Errorf("expected actions ['logTransition', 'notify'], got %v", trans.Actions)
	}
}

func TestParseMachineStruct_MultipleTransitions(t *testing.T) {
	type MultiTransMachine struct {
		MachineDef `id:"multi" initial:"idle"`
		Idle       StateNode `on:"START->running,SKIP->done"`
		Running    StateNode `on:"STOP->idle,PAUSE->paused"`
		Paused     StateNode `on:"RESUME->running"`
		Done       FinalNode
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(MultiTransMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	idle := schema.States[0]
	if len(idle.Transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(idle.Transitions))
	}
	if idle.Transitions[0].Event != "START" || idle.Transitions[0].Target != "running" {
		t.Errorf("unexpected first transition: %+v", idle.Transitions[0])
	}
	if idle.Transitions[1].Event != "SKIP" || idle.Transitions[1].Target != "done" {
		t.Errorf("unexpected second transition: %+v", idle.Transitions[1])
	}
}

func TestParseMachineStruct_FinalState(t *testing.T) {
	type FinalMachine struct {
		MachineDef `id:"final" initial:"active"`
		Active     StateNode `on:"COMPLETE->done"`
		Done       FinalNode
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(FinalMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(schema.States))
	}

	done := schema.States[1]
	if done.Name != "done" {
		t.Errorf("expected state name 'done', got %q", done.Name)
	}
	if done.Type != StateSchemaFinal {
		t.Errorf("expected StateSchemaFinal, got %v", done.Type)
	}
}

func TestParseMachineStruct_Hierarchical(t *testing.T) {
	type ChildState struct {
		StateNode `on:"NEXT->sibling"`
	}
	type SiblingState struct {
		StateNode `on:"BACK->child"`
	}
	type ParentState struct {
		CompoundNode `initial:"child" on:"RESET->done"`
		Child        ChildState
		Sibling      SiblingState
	}
	type HierarchicalMachine struct {
		MachineDef `id:"hierarchical" initial:"parent"`
		Parent     ParentState
		Done       FinalNode
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(HierarchicalMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.States) != 2 {
		t.Fatalf("expected 2 root states, got %d", len(schema.States))
	}

	parent := schema.States[0]
	if parent.Name != "parent" {
		t.Errorf("expected state name 'parent', got %q", parent.Name)
	}
	if parent.Type != StateSchemaCompound {
		t.Errorf("expected StateSchemaCompound, got %v", parent.Type)
	}
	if parent.Initial != "child" {
		t.Errorf("expected initial 'child', got %q", parent.Initial)
	}
	if len(parent.Transitions) != 1 || parent.Transitions[0].Event != "RESET" {
		t.Errorf("expected RESET transition on parent, got %+v", parent.Transitions)
	}

	if len(parent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(parent.Children))
	}

	child := parent.Children[0]
	if child.Name != "child" {
		t.Errorf("expected child name 'child', got %q", child.Name)
	}
	if len(child.Transitions) != 1 || child.Transitions[0].Target != "sibling" {
		t.Errorf("unexpected child transitions: %+v", child.Transitions)
	}
}

func TestParseMachineStruct_MissingMachineDef(t *testing.T) {
	type InvalidMachine struct {
		Idle    StateNode `on:"START->running"`
		Running StateNode `on:"STOP->idle"`
	}

	_, err := ParseMachineStruct(reflect.TypeOf(InvalidMachine{}))
	if err == nil {
		t.Fatal("expected error for missing MachineDef")
	}
}

func TestParseMachineStruct_MissingID(t *testing.T) {
	type InvalidMachine struct {
		MachineDef `initial:"idle"`
		Idle       StateNode
	}

	_, err := ParseMachineStruct(reflect.TypeOf(InvalidMachine{}))
	if err == nil {
		t.Fatal("expected error for missing id tag")
	}
}

func TestParseMachineStruct_MissingInitial(t *testing.T) {
	type InvalidMachine struct {
		MachineDef `id:"test"`
		Idle       StateNode
	}

	_, err := ParseMachineStruct(reflect.TypeOf(InvalidMachine{}))
	if err == nil {
		t.Fatal("expected error for missing initial tag")
	}
}

func TestParseTransition_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing arrow", "EVENT target"},
		{"empty event", "->target"},
		{"empty target", "EVENT->"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTransition(tt.input)
			if err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Idle", "idle"},
		{"Running", "running"},
		{"DontWalk", "dont_walk"},
		{"HTTPServer", "http_server"},
		{"APIGateway", "api_gateway"},
		{"myState", "my_state"},
		{"ABC", "abc"},
		{"XMLParser", "xml_parser"},
		{"getHTTPResponse", "get_http_response"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTransition_ComplexFormat(t *testing.T) {
	// Test: EVENT->target/action1;action2:guard
	trans, err := parseTransition("SUBMIT->processing/validate;log:isValid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trans.Event != "SUBMIT" {
		t.Errorf("expected event 'SUBMIT', got %q", trans.Event)
	}
	if trans.Target != "processing" {
		t.Errorf("expected target 'processing', got %q", trans.Target)
	}
	if trans.Guard != "isValid" {
		t.Errorf("expected guard 'isValid', got %q", trans.Guard)
	}
	if len(trans.Actions) != 2 || trans.Actions[0] != "validate" || trans.Actions[1] != "log" {
		t.Errorf("expected actions ['validate', 'log'], got %v", trans.Actions)
	}
}

func TestParseMachineStruct_PointerType(t *testing.T) {
	type SimpleMachine struct {
		MachineDef `id:"ptr" initial:"idle"`
		Idle       StateNode `on:"START->running"`
		Running    StateNode `on:"STOP->idle"`
	}

	// Test with pointer to struct
	schema, err := ParseMachineStruct(reflect.TypeOf(&SimpleMachine{}))
	if err != nil {
		t.Fatalf("unexpected error for pointer type: %v", err)
	}

	if schema.ID != "ptr" {
		t.Errorf("expected ID 'ptr', got %q", schema.ID)
	}
}

func TestParseMachineStruct_NonStruct(t *testing.T) {
	_, err := ParseMachineStruct(reflect.TypeOf(42))
	if err == nil {
		t.Fatal("expected error for non-struct type")
	}
}

func TestParseTransition_Whitespace(t *testing.T) {
	// Test with extra whitespace
	trans, err := parseTransition("  EVENT  ->  target  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trans.Event != "EVENT" {
		t.Errorf("expected event 'EVENT', got %q", trans.Event)
	}
	if trans.Target != "target" {
		t.Errorf("expected target 'target', got %q", trans.Target)
	}
}

func TestParseMachineStruct_StateWithNoTags(t *testing.T) {
	type MinimalMachine struct {
		MachineDef `id:"minimal" initial:"idle"`
		Idle       StateNode
		Done       FinalNode
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(MinimalMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(schema.States))
	}

	idle := schema.States[0]
	if len(idle.Transitions) != 0 {
		t.Errorf("expected no transitions, got %d", len(idle.Transitions))
	}
	if len(idle.Entry) != 0 {
		t.Errorf("expected no entry actions, got %d", len(idle.Entry))
	}
}

func TestParseMachineStruct_DeeplyNested(t *testing.T) {
	// Level 3: Leaf states
	type LeafA struct {
		StateNode `on:"TO_B->leaf_b"`
	}
	type LeafB struct {
		StateNode `on:"TO_A->leaf_a"`
	}

	// Level 2: Parent of leaves
	type Level2 struct {
		CompoundNode `initial:"leaf_a"`
		LeafA        LeafA
		LeafB        LeafB
	}

	// Level 1: Root compound
	type Level1 struct {
		CompoundNode `initial:"level2" on:"EXIT->done"`
		Level2       Level2
	}

	type DeepMachine struct {
		MachineDef `id:"deep" initial:"level1"`
		Level1     Level1
		Done       FinalNode
	}

	schema, err := ParseMachineStruct(reflect.TypeOf(DeepMachine{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.States) != 2 {
		t.Fatalf("expected 2 root states, got %d", len(schema.States))
	}

	level1 := schema.States[0]
	if level1.Type != StateSchemaCompound {
		t.Errorf("expected compound state for level1")
	}
	if len(level1.Children) != 1 {
		t.Fatalf("expected 1 child in level1, got %d", len(level1.Children))
	}

	level2 := level1.Children[0]
	if level2.Type != StateSchemaCompound {
		t.Errorf("expected compound state for level2")
	}
	if len(level2.Children) != 2 {
		t.Fatalf("expected 2 children in level2, got %d", len(level2.Children))
	}

	leafA := level2.Children[0]
	if leafA.Name != "leaf_a" {
		t.Errorf("expected name 'leaf_a', got %q", leafA.Name)
	}
}

func TestSplitTrim_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		sep      string
		expected []string
	}{
		{"", ",", nil},
		{"  ", ",", nil},
		{"a", ",", []string{"a"}},
		{"  a  ", ",", []string{"a"}},
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{"  a , b , c  ", ",", []string{"a", "b", "c"}},
		{"a,,b", ",", []string{"a", "b"}}, // Empty parts are skipped
	}

	for _, tt := range tests {
		result := splitTrim(tt.input, tt.sep)
		if len(result) != len(tt.expected) {
			t.Errorf("splitTrim(%q, %q) len = %d, expected %d", tt.input, tt.sep, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitTrim(%q, %q)[%d] = %q, expected %q", tt.input, tt.sep, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestParseTransitions_ErrorContext(t *testing.T) {
	// Test that error messages include transition index
	_, err := parseTransitions("VALID->target, INVALID, ANOTHER->target")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
	// Error should mention "transition 2" since INVALID is the second transition
	errStr := err.Error()
	if !contains(errStr, "transition 2") {
		t.Errorf("error should mention 'transition 2', got: %s", errStr)
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
