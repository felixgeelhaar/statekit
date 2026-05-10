// Package lint provides static analysis for statekit state machines.
// It detects potential issues like unreachable states, dead ends,
// non-deterministic transitions, and other structural problems.
package lint

import (
	"fmt"
	"sort"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// Severity indicates the importance of a diagnostic.
type Severity int

const (
	// SeverityError indicates a definite problem that will cause issues.
	SeverityError Severity = iota
	// SeverityWarning indicates a likely problem that should be reviewed.
	SeverityWarning
	// SeverityInfo indicates a suggestion or observation.
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single lint finding.
type Diagnostic struct {
	Severity Severity
	Rule     string
	State    statekit.StateID // Empty for machine-level issues
	Event    statekit.EventType
	Message  string
}

func (d Diagnostic) String() string {
	loc := string(d.State)
	if loc == "" {
		loc = "(machine)"
	}
	return fmt.Sprintf("[%s] %s: %s (%s)", d.Severity, loc, d.Message, d.Rule)
}

// Result contains all diagnostics from linting.
type Result struct {
	MachineID   string
	Diagnostics []Diagnostic
}

// HasErrors returns true if any diagnostics are errors.
func (r *Result) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any diagnostics are warnings or errors.
func (r *Result) HasWarnings() bool {
	for _, d := range r.Diagnostics {
		if d.Severity <= SeverityWarning {
			return true
		}
	}
	return false
}

// Errors returns only error-level diagnostics.
func (r *Result) Errors() []Diagnostic {
	var errors []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			errors = append(errors, d)
		}
	}
	return errors
}

// Warnings returns only warning-level diagnostics.
func (r *Result) Warnings() []Diagnostic {
	var warnings []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityWarning {
			warnings = append(warnings, d)
		}
	}
	return warnings
}

// String returns a formatted summary of all diagnostics.
func (r *Result) String() string {
	if len(r.Diagnostics) == 0 {
		return fmt.Sprintf("Machine %q: no issues found", r.MachineID)
	}

	var errors, warnings, infos int
	for _, d := range r.Diagnostics {
		switch d.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		case SeverityInfo:
			infos++
		}
	}

	result := fmt.Sprintf("Machine %q: %d error(s), %d warning(s), %d info(s)\n",
		r.MachineID, errors, warnings, infos)

	for _, d := range r.Diagnostics {
		result += "  " + d.String() + "\n"
	}

	return result
}

// Linter performs static analysis on state machines.
type Linter struct {
	// Configuration options
	IgnoreRules map[string]bool
}

// NewLinter creates a new linter with default configuration.
func NewLinter() *Linter {
	return &Linter{
		IgnoreRules: make(map[string]bool),
	}
}

// Ignore configures the linter to skip the given rule.
func (l *Linter) Ignore(rule string) *Linter {
	l.IgnoreRules[rule] = true
	return l
}

// Lint analyzes a machine and returns all findings.
func Lint[C any](machine *ir.MachineConfig[C]) *Result {
	return CheckTyped(NewLinter(), machine)
}

// CheckTyped analyzes a typed machine configuration.
func CheckTyped[C any](l *Linter, machine *ir.MachineConfig[C]) *Result {
	result := &Result{
		MachineID:   machine.ID,
		Diagnostics: []Diagnostic{},
	}

	// Run all checks
	checkUnreachable(l, machine, result)
	checkDeadEnds(l, machine, result)
	checkNonDeterminism(l, machine, result)
	checkCompoundInitials(l, machine, result)
	checkSelfTransitions(l, machine, result)
	checkUnusedActions(l, machine, result)
	checkUnusedGuards(l, machine, result)
	checkInvokeMissingOnError(l, machine, result)
	checkInvokeIDCollision(l, machine, result)
	checkAutoForwardRedundancy(l, machine, result)
	checkDeepNesting(l, machine, result)

	// Sort diagnostics by severity, then state
	sort.Slice(result.Diagnostics, func(i, j int) bool {
		if result.Diagnostics[i].Severity != result.Diagnostics[j].Severity {
			return result.Diagnostics[i].Severity < result.Diagnostics[j].Severity
		}
		return result.Diagnostics[i].State < result.Diagnostics[j].State
	})

	return result
}

// checkUnreachable finds states that cannot be reached from initial state.
func checkUnreachable[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["unreachable"] {
		return
	}

	reachable := findReachable(machine)

	for id := range machine.States {
		if !reachable[id] {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Rule:     "unreachable",
				State:    id,
				Message:  "state is unreachable from initial state",
			})
		}
	}
}

func findReachable[C any](machine *ir.MachineConfig[C]) map[statekit.StateID]bool {
	reachable := make(map[statekit.StateID]bool)

	var visit func(statekit.StateID)
	visit = func(id statekit.StateID) {
		if reachable[id] {
			return
		}
		reachable[id] = true

		state := machine.States[id]
		if state == nil {
			return
		}

		// Visit parent (ancestors are reachable if child is)
		if state.Parent != "" {
			reachable[state.Parent] = true
		}

		// Visit children via initial
		if state.Initial != "" {
			visit(state.Initial)
		}

		// Visit transition targets
		for _, t := range state.Transitions {
			visit(t.Target)
		}
	}

	visit(machine.Initial)
	return reachable
}

// checkDeadEnds finds non-final leaf states with no outgoing transitions.
func checkDeadEnds[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["dead-end"] {
		return
	}

	for id, state := range machine.States {
		// Skip final states - they're supposed to be dead ends
		if state.Type == ir.StateTypeFinal {
			continue
		}

		// Skip compound states - they delegate to children
		if len(state.Children) > 0 {
			continue
		}

		// Check if this leaf state has any transitions
		if len(state.Transitions) == 0 {
			// Also check if parent has transitions (event bubbling)
			hasParentTransitions := false
			parent := state.Parent
			for parent != "" {
				if parentState := machine.States[parent]; parentState != nil {
					if len(parentState.Transitions) > 0 {
						hasParentTransitions = true
						break
					}
					parent = parentState.Parent
				} else {
					break
				}
			}

			if !hasParentTransitions {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityWarning,
					Rule:     "dead-end",
					State:    id,
					Message:  "non-final state has no outgoing transitions",
				})
			}
		}
	}
}

// checkNonDeterminism finds states where the same event has multiple transitions without guards.
func checkNonDeterminism[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["non-determinism"] {
		return
	}

	for id, state := range machine.States {
		// Group transitions by event
		byEvent := make(map[statekit.EventType][]*ir.TransitionConfig)
		for _, t := range state.Transitions {
			byEvent[t.Event] = append(byEvent[t.Event], t)
		}

		// Check each event
		for event, transitions := range byEvent {
			if len(transitions) <= 1 {
				continue
			}

			// Multiple transitions for same event - check guards
			unguarded := 0
			for _, t := range transitions {
				if t.Guard == "" {
					unguarded++
				}
			}

			if unguarded > 1 {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Rule:     "non-determinism",
					State:    id,
					Event:    event,
					Message:  fmt.Sprintf("event %q has %d unguarded transitions", event, unguarded),
				})
			} else if unguarded == 1 && len(transitions) > 1 {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityInfo,
					Rule:     "non-determinism",
					State:    id,
					Event:    event,
					Message:  fmt.Sprintf("event %q has %d transitions with mixed guard usage", event, len(transitions)),
				})
			}
		}
	}
}

// checkCompoundInitials ensures compound states have initial children.
func checkCompoundInitials[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["compound-initial"] {
		return
	}

	for id, state := range machine.States {
		if state.Type == ir.StateTypeCompound && len(state.Children) > 0 {
			if state.Initial == "" {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Rule:     "compound-initial",
					State:    id,
					Message:  "compound state has children but no initial state",
				})
			}
		}
	}
}

// checkSelfTransitions warns about self-transitions without guards.
func checkSelfTransitions[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["self-transition"] {
		return
	}

	for id, state := range machine.States {
		for _, t := range state.Transitions {
			if t.Target == id && t.Guard == "" {
				// Check if there are entry actions that could cause issues
				if len(state.Entry) > 0 {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						Severity: SeverityInfo,
						Rule:     "self-transition",
						State:    id,
						Event:    t.Event,
						Message:  fmt.Sprintf("unguarded self-transition on %q will re-run entry actions", t.Event),
					})
				}
			}
		}
	}
}

// checkUnusedActions finds actions registered but never referenced.
func checkUnusedActions[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["unused-action"] {
		return
	}

	used := make(map[statekit.ActionType]bool)

	for _, state := range machine.States {
		for _, a := range state.Entry {
			used[a] = true
		}
		for _, a := range state.Exit {
			used[a] = true
		}
		for _, t := range state.Transitions {
			for _, a := range t.Actions {
				used[a] = true
			}
		}
	}

	for name := range machine.Actions {
		if !used[name] {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Rule:     "unused-action",
				Message:  fmt.Sprintf("action %q is registered but never used", name),
			})
		}
	}
}

// checkUnusedGuards finds guards registered but never referenced.
func checkUnusedGuards[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["unused-guard"] {
		return
	}

	used := make(map[statekit.GuardType]bool)

	for _, state := range machine.States {
		for _, t := range state.Transitions {
			if t.Guard != "" {
				used[t.Guard] = true
			}
		}
	}

	for name := range machine.Guards {
		if !used[name] {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Rule:     "unused-guard",
				Message:  fmt.Sprintf("guard %q is registered but never used", name),
			})
		}
	}
}

// checkInvokeMissingOnError warns when invoked services or child machines
// have no OnError handler — silent failure path in production.
func checkInvokeMissingOnError[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["invoke-missing-onerror"] {
		return
	}

	for id, state := range machine.States {
		for _, inv := range state.Invocations {
			if inv.OnError == nil {
				name := inv.ID
				if name == "" {
					name = string(inv.Src)
				}
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityWarning,
					Rule:     "invoke-missing-onerror",
					State:    id,
					Message:  fmt.Sprintf("invocation %q has no OnError handler — service errors will not transition", name),
				})
			}
		}
		for _, inv := range state.MachineInvocations {
			if inv.OnError == nil {
				name := inv.ID
				if name == "" {
					name = inv.MachineRef
				}
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityWarning,
					Rule:     "invoke-missing-onerror",
					State:    id,
					Message:  fmt.Sprintf("machine invocation %q has no OnError handler — child errors will not transition", name),
				})
			}
		}
	}
}

// checkInvokeIDCollision detects duplicate invocation IDs within the same state.
// Collisions cause unpredictable lookup of the active invocation at runtime.
func checkInvokeIDCollision[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["invoke-id-collision"] {
		return
	}

	for id, state := range machine.States {
		seen := make(map[string]int)
		for _, inv := range state.Invocations {
			if inv.ID != "" {
				seen[inv.ID]++
			}
		}
		for _, inv := range state.MachineInvocations {
			if inv.ID != "" {
				seen[inv.ID]++
			}
		}
		for invID, count := range seen {
			if count > 1 {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: SeverityError,
					Rule:     "invoke-id-collision",
					State:    id,
					Message:  fmt.Sprintf("invocation ID %q is used %d times — IDs must be unique within a state", invID, count),
				})
			}
		}
	}
}

// checkAutoForwardRedundancy warns when a state's invoked-machine
// AutoForward list includes an event the same state declares a
// transition for. The parent's transition fires first and consumes
// the event, so the AutoForward entry never reaches the child —
// almost certainly a footgun.
func checkAutoForwardRedundancy[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["auto-forward-redundancy"] {
		return
	}

	for id, state := range machine.States {
		if len(state.MachineInvocations) == 0 {
			continue
		}
		// Collect events handled by the state itself.
		handled := make(map[ir.EventType]struct{}, len(state.Transitions))
		for _, t := range state.Transitions {
			handled[t.Event] = struct{}{}
		}
		for _, inv := range state.MachineInvocations {
			for _, evt := range inv.AutoForward {
				if _, ok := handled[evt]; ok {
					name := inv.ID
					if name == "" {
						name = inv.MachineRef
					}
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						Severity: SeverityWarning,
						Rule:     "auto-forward-redundancy",
						State:    id,
						Event:    evt,
						Message:  fmt.Sprintf("event %q is forwarded to child %q but the parent already handles it — the parent transition fires first", evt, name),
					})
				}
			}
		}
	}
}

// checkDeepNesting warns when state hierarchy depth exceeds the
// cognitive-load threshold. Five-deep is a soft signal that the
// machine has grown into separate concerns that may be better split
// into invoked child machines.
func checkDeepNesting[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules["deep-nesting"] {
		return
	}
	const threshold = 5
	for id := range machine.States {
		depth := len(machine.GetAncestors(id))
		if depth >= threshold {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityInfo,
				Rule:     "deep-nesting",
				State:    id,
				Message:  fmt.Sprintf("state nests %d levels deep — consider splitting into a child machine via InvokeMachine", depth),
			})
		}
	}
}

// Rule names for reference
const (
	RuleUnreachable           = "unreachable"
	RuleDeadEnd               = "dead-end"
	RuleNonDeterminism        = "non-determinism"
	RuleCompoundInitial       = "compound-initial"
	RuleSelfTransition        = "self-transition"
	RuleUnusedAction          = "unused-action"
	RuleUnusedGuard           = "unused-guard"
	RuleInvokeMissingOnError  = "invoke-missing-onerror"
	RuleInvokeIDCollision     = "invoke-id-collision"
	RuleAutoForwardRedundancy = "auto-forward-redundancy"
	RuleDeepNesting           = "deep-nesting"
)

// AllRules returns all available rule names.
func AllRules() []string {
	return []string{
		RuleUnreachable,
		RuleDeadEnd,
		RuleNonDeterminism,
		RuleCompoundInitial,
		RuleSelfTransition,
		RuleUnusedAction,
		RuleUnusedGuard,
		RuleInvokeMissingOnError,
		RuleInvokeIDCollision,
		RuleAutoForwardRedundancy,
		RuleDeepNesting,
	}
}
