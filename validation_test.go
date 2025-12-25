package statekit

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

func TestBuild_Validation_MissingInitial(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		State("idle").Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing initial state")
	}

	valErr, ok := err.(*ir.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !containsIssueCode(valErr, ir.ErrCodeMissingInitial) {
		t.Errorf("expected MISSING_INITIAL error, got: %v", err)
	}
}

func TestBuild_Validation_InitialNotFound(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		WithInitial("nonexistent").
		State("idle").Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error for initial state not found")
	}

	valErr, ok := err.(*ir.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !containsIssueCode(valErr, ir.ErrCodeInitialNotFound) {
		t.Errorf("expected INITIAL_NOT_FOUND error, got: %v", err)
	}
}

func TestBuild_Validation_InvalidTransitionTarget(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("nonexistent").
		Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error for invalid transition target")
	}

	valErr, ok := err.(*ir.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !containsIssueCode(valErr, ir.ErrCodeInvalidTarget) {
		t.Errorf("expected INVALID_TARGET error, got: %v", err)
	}
}

func TestBuild_Validation_MissingAction(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		OnEntry("nonexistentAction").
		Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing action")
	}

	valErr, ok := err.(*ir.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !containsIssueCode(valErr, ir.ErrCodeMissingAction) {
		t.Errorf("expected MISSING_ACTION error, got: %v", err)
	}
}

func TestBuild_Validation_MissingGuard(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("running").Guard("nonexistentGuard").
		Done().
		State("running").Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing guard")
	}

	valErr, ok := err.(*ir.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !containsIssueCode(valErr, ir.ErrCodeMissingGuard) {
		t.Errorf("expected MISSING_GUARD error, got: %v", err)
	}
}

func TestBuild_Validation_ErrorMessage(t *testing.T) {
	_, err := NewMachine[struct{}]("test").
		WithInitial("nonexistent").
		State("idle").Done().
		Build()

	if err == nil {
		t.Fatal("expected validation error")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "INITIAL_NOT_FOUND") {
		t.Errorf("expected error message to contain INITIAL_NOT_FOUND, got: %s", errStr)
	}
}

func containsIssueCode(err *ir.ValidationError, code string) bool {
	for _, issue := range err.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
