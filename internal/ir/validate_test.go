package ir

import (
	"strings"
	"testing"
)

type testCtx struct{}

func TestValidate_ValidMachine(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)
	machine.States["running"] = NewStateConfig("running", StateTypeAtomic)

	trans := NewTransitionConfig("START", "running")
	machine.States["idle"].Transitions = []*TransitionConfig{trans}

	err := Validate(machine)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingInitial(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for missing initial state")
	}

	if !containsCode(err, ErrCodeMissingInitial) {
		t.Errorf("expected MISSING_INITIAL error, got: %v", err)
	}
}

func TestValidate_InitialNotFound(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "nonexistent", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for initial state not found")
	}

	if !containsCode(err, ErrCodeInitialNotFound) {
		t.Errorf("expected INITIAL_NOT_FOUND error, got: %v", err)
	}
}

func TestValidate_NoStates(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for no states")
	}

	if !containsCode(err, ErrCodeNoStates) {
		t.Errorf("expected NO_STATES error, got: %v", err)
	}
}

func TestValidate_InvalidTransitionTarget(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)

	trans := NewTransitionConfig("GO", "nonexistent")
	machine.States["idle"].Transitions = []*TransitionConfig{trans}

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for invalid transition target")
	}

	if !containsCode(err, ErrCodeInvalidTarget) {
		t.Errorf("expected INVALID_TARGET error, got: %v", err)
	}
}

func TestValidate_MissingGuard(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)
	machine.States["running"] = NewStateConfig("running", StateTypeAtomic)

	trans := NewTransitionConfig("GO", "running")
	trans.Guard = "nonexistentGuard"
	machine.States["idle"].Transitions = []*TransitionConfig{trans}

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for missing guard")
	}

	if !containsCode(err, ErrCodeMissingGuard) {
		t.Errorf("expected MISSING_GUARD error, got: %v", err)
	}
}

func TestValidate_MissingEntryAction(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	state := NewStateConfig("idle", StateTypeAtomic)
	state.Entry = []ActionType{"nonexistentAction"}
	machine.States["idle"] = state

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for missing entry action")
	}

	if !containsCode(err, ErrCodeMissingAction) {
		t.Errorf("expected MISSING_ACTION error, got: %v", err)
	}
}

func TestValidate_MissingExitAction(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	state := NewStateConfig("idle", StateTypeAtomic)
	state.Exit = []ActionType{"nonexistentAction"}
	machine.States["idle"] = state

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for missing exit action")
	}

	if !containsCode(err, ErrCodeMissingAction) {
		t.Errorf("expected MISSING_ACTION error, got: %v", err)
	}
}

func TestValidate_MissingTransitionAction(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})
	machine.States["idle"] = NewStateConfig("idle", StateTypeAtomic)
	machine.States["running"] = NewStateConfig("running", StateTypeAtomic)

	trans := NewTransitionConfig("GO", "running")
	trans.Actions = []ActionType{"nonexistentAction"}
	machine.States["idle"].Transitions = []*TransitionConfig{trans}

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected error for missing transition action")
	}

	if !containsCode(err, ErrCodeMissingAction) {
		t.Errorf("expected MISSING_ACTION error, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "nonexistent", testCtx{})
	// Add a state with multiple issues
	state := NewStateConfig("idle", StateTypeAtomic)
	state.Entry = []ActionType{"missingAction1"}
	state.Exit = []ActionType{"missingAction2"}
	machine.States["idle"] = state

	err := Validate(machine)
	if err == nil {
		t.Fatal("expected errors")
	}

	// Should have at least 3 errors: initial not found + 2 missing actions
	if len(err.Issues) < 3 {
		t.Errorf("expected at least 3 issues, got %d: %v", len(err.Issues), err)
	}
}

func TestValidate_WithDefinedActionsAndGuards(t *testing.T) {
	machine := NewMachineConfig[testCtx]("test", "idle", testCtx{})

	// Define action and guard
	machine.Actions["myAction"] = func(ctx *testCtx, e Event) {}
	machine.Guards["myGuard"] = func(ctx testCtx, e Event) bool { return true }

	// Use them in state
	state := NewStateConfig("idle", StateTypeAtomic)
	state.Entry = []ActionType{"myAction"}
	state.Exit = []ActionType{"myAction"}
	machine.States["idle"] = state

	machine.States["running"] = NewStateConfig("running", StateTypeAtomic)

	trans := NewTransitionConfig("GO", "running")
	trans.Guard = "myGuard"
	trans.Actions = []ActionType{"myAction"}
	machine.States["idle"].Transitions = []*TransitionConfig{trans}

	err := Validate(machine)
	if err != nil {
		t.Errorf("expected no error with defined actions and guards, got: %v", err)
	}
}

func TestValidationError_String(t *testing.T) {
	err := &ValidationError{}
	err.AddIssue("TEST_CODE", "test message", "path", "to", "issue")

	str := err.Error()
	if !strings.Contains(str, "TEST_CODE") {
		t.Errorf("expected error string to contain code, got: %s", str)
	}
	if !strings.Contains(str, "test message") {
		t.Errorf("expected error string to contain message, got: %s", str)
	}
	if !strings.Contains(str, "path.to.issue") {
		t.Errorf("expected error string to contain path, got: %s", str)
	}
}

func containsCode(err *ValidationError, code string) bool {
	for _, issue := range err.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
