package ir

import (
	"fmt"
	"strings"
)

// ValidationIssue represents a single validation problem
type ValidationIssue struct {
	Code    string   // e.g., "MISSING_INITIAL", "INVALID_TARGET"
	Message string   // Human-readable description
	Path    []string // e.g., ["states", "green", "transitions", "0"]
}

// String returns a human-readable representation of the issue
func (v ValidationIssue) String() string {
	if len(v.Path) > 0 {
		return fmt.Sprintf("[%s] %s (at %s)", v.Code, v.Message, strings.Join(v.Path, "."))
	}
	return fmt.Sprintf("[%s] %s", v.Code, v.Message)
}

// ValidationError contains all validation issues found during validation
type ValidationError struct {
	Issues []ValidationIssue
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "validation failed"
	}
	if len(e.Issues) == 1 {
		return e.Issues[0].String()
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("validation failed with %d issues:\n", len(e.Issues)))
	for i, issue := range e.Issues {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, issue.String()))
	}
	return b.String()
}

// AddIssue adds a validation issue to the error
func (e *ValidationError) AddIssue(code, message string, path ...string) {
	e.Issues = append(e.Issues, ValidationIssue{
		Code:    code,
		Message: message,
		Path:    path,
	})
}

// HasIssues returns true if there are any validation issues
func (e *ValidationError) HasIssues() bool {
	return len(e.Issues) > 0
}

// Validation error codes
const (
	ErrCodeMissingInitial         = "MISSING_INITIAL"
	ErrCodeInitialNotFound        = "INITIAL_NOT_FOUND"
	ErrCodeInvalidTarget          = "INVALID_TARGET"
	ErrCodeMissingAction          = "MISSING_ACTION"
	ErrCodeMissingGuard           = "MISSING_GUARD"
	ErrCodeNoStates               = "NO_STATES"
	ErrCodeDuplicateState         = "DUPLICATE_STATE"
	ErrCodeCompoundMissingInitial = "COMPOUND_MISSING_INITIAL"
	ErrCodeCompoundInvalidInitial = "COMPOUND_INVALID_INITIAL"
	ErrCodeInvalidParent          = "INVALID_PARENT"
	ErrCodeInvalidChild           = "INVALID_CHILD"
)

// Validate checks the machine configuration for errors
func Validate[C any](m *MachineConfig[C]) *ValidationError {
	errs := &ValidationError{}

	// Check if initial state is set
	if m.Initial == "" {
		errs.AddIssue(ErrCodeMissingInitial, "initial state is required")
	}

	// Check if there are any states
	if len(m.States) == 0 {
		errs.AddIssue(ErrCodeNoStates, "at least one state is required")
	}

	// Check if initial state exists
	if m.Initial != "" && len(m.States) > 0 {
		if _, ok := m.States[m.Initial]; !ok {
			errs.AddIssue(ErrCodeInitialNotFound,
				fmt.Sprintf("initial state '%s' not found in states", m.Initial))
		}
	}

	// Validate each state
	for stateID, state := range m.States {
		statePath := []string{"states", string(stateID)}

		// Validate compound state requirements
		if state.Type == StateTypeCompound {
			// Compound states must have an initial child
			if state.Initial == "" {
				errs.AddIssue(ErrCodeCompoundMissingInitial,
					fmt.Sprintf("compound state '%s' must have an initial child state", stateID),
					statePath...)
			} else {
				// Initial must be one of its children
				isChild := false
				for _, childID := range state.Children {
					if childID == state.Initial {
						isChild = true
						break
					}
				}
				if !isChild {
					errs.AddIssue(ErrCodeCompoundInvalidInitial,
						fmt.Sprintf("initial state '%s' must be a child of compound state '%s'", state.Initial, stateID),
						statePath...)
				}
			}

			// Validate all children exist
			for i, childID := range state.Children {
				child, ok := m.States[childID]
				if !ok {
					errs.AddIssue(ErrCodeInvalidChild,
						fmt.Sprintf("child state '%s' not found", childID),
						append(statePath, "children", fmt.Sprintf("%d", i))...)
				} else if child.Parent != stateID {
					errs.AddIssue(ErrCodeInvalidChild,
						fmt.Sprintf("child state '%s' has incorrect parent '%s', expected '%s'", childID, child.Parent, stateID),
						append(statePath, "children", fmt.Sprintf("%d", i))...)
				}
			}
		}

		// Validate parent exists if set
		if state.Parent != "" {
			parent, ok := m.States[state.Parent]
			if !ok {
				errs.AddIssue(ErrCodeInvalidParent,
					fmt.Sprintf("parent state '%s' not found", state.Parent),
					statePath...)
			} else if parent.Type != StateTypeCompound {
				errs.AddIssue(ErrCodeInvalidParent,
					fmt.Sprintf("parent state '%s' is not a compound state", state.Parent),
					statePath...)
			}
		}

		// Validate entry actions exist
		for i, actionName := range state.Entry {
			if _, ok := m.Actions[actionName]; !ok {
				errs.AddIssue(ErrCodeMissingAction,
					fmt.Sprintf("entry action '%s' is not defined", actionName),
					append(statePath, "entry", fmt.Sprintf("%d", i))...)
			}
		}

		// Validate exit actions exist
		for i, actionName := range state.Exit {
			if _, ok := m.Actions[actionName]; !ok {
				errs.AddIssue(ErrCodeMissingAction,
					fmt.Sprintf("exit action '%s' is not defined", actionName),
					append(statePath, "exit", fmt.Sprintf("%d", i))...)
			}
		}

		// Validate transitions
		for i, trans := range state.Transitions {
			transPath := append(statePath, "transitions", fmt.Sprintf("%d", i))

			// Check target state exists
			if _, ok := m.States[trans.Target]; !ok {
				errs.AddIssue(ErrCodeInvalidTarget,
					fmt.Sprintf("transition target '%s' not found", trans.Target),
					transPath...)
			}

			// Check guard exists if specified
			if trans.Guard != "" {
				if _, ok := m.Guards[trans.Guard]; !ok {
					errs.AddIssue(ErrCodeMissingGuard,
						fmt.Sprintf("guard '%s' is not defined", trans.Guard),
						transPath...)
				}
			}

			// Check transition actions exist
			for j, actionName := range trans.Actions {
				if _, ok := m.Actions[actionName]; !ok {
					errs.AddIssue(ErrCodeMissingAction,
						fmt.Sprintf("transition action '%s' is not defined", actionName),
						append(transPath, "actions", fmt.Sprintf("%d", j))...)
				}
			}
		}
	}

	if errs.HasIssues() {
		return errs
	}
	return nil
}
