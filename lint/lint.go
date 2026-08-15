// Package lint provides static analysis for statekit state machines.
// It detects potential issues like unreachable states, dead ends,
// non-deterministic transitions, and other structural problems.
package lint

import (
	"fmt"
	"sort"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/internal/ir"
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
	checkActorIDCollision(l, machine, result)
	checkAutoForwardRedundancy(l, machine, result)
	checkDeepNesting(l, machine, result)
	checkHistoryWithoutSiblings(l, machine, result)
	checkGuardedOnlyEntry(l, machine, result)
	checkGuardedEventWithoutFallback(l, machine, result)
	checkAutoForwardLoop(l, machine, result)

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

// checkActorIDCollision detects the same MachineInvocation ID declared in
// multiple states. Child done/error events are keyed by ID
// (`xstate.done.actor.<id>`); reusing an ID across states makes routing
// ambiguous when parallel regions or overlapping lifecycles are active.
func checkActorIDCollision[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules[RuleActorIDCollision] {
		return
	}

	type occurrence struct {
		state ir.StateID
	}
	byID := make(map[string][]occurrence)
	for id, state := range machine.States {
		for _, inv := range state.MachineInvocations {
			if inv.ID == "" {
				continue
			}
			byID[inv.ID] = append(byID[inv.ID], occurrence{state: id})
		}
	}
	for invID, occs := range byID {
		if len(occs) < 2 {
			continue
		}
		// Deduplicate state IDs (same state already covered by invoke-id-collision).
		states := make(map[ir.StateID]struct{}, len(occs))
		for _, o := range occs {
			states[o.state] = struct{}{}
		}
		if len(states) < 2 {
			continue
		}
		var stateList []string
		for s := range states {
			stateList = append(stateList, string(s))
		}
		sort.Strings(stateList)
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: SeverityWarning,
			Rule:     RuleActorIDCollision,
			Message:  fmt.Sprintf("machine invocation ID %q is reused across states %v — done/error events are keyed by ID and may collide", invID, stateList),
		})
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

// checkHistoryWithoutSiblings warns when a history state sits in a compound
// parent that has no other children. History remembers the last active sibling;
// with only itself present, resume always falls through to the default — a
// no-op that usually means the modeller forgot the real children.
func checkHistoryWithoutSiblings[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules[RuleHistoryWithoutSiblings] {
		return
	}

	for id, state := range machine.States {
		if state.Type != ir.StateTypeHistory {
			continue
		}
		if state.Parent == "" {
			continue // HISTORY_NOT_IN_COMPOUND is a build-time validation error
		}
		parent := machine.States[state.Parent]
		if parent == nil {
			continue
		}
		siblings := 0
		for _, childID := range parent.Children {
			if childID == id {
				continue
			}
			child := machine.States[childID]
			if child != nil && child.Type != ir.StateTypeHistory {
				siblings++
			}
		}
		if siblings == 0 {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Rule:     RuleHistoryWithoutSiblings,
				State:    id,
				Message:  fmt.Sprintf("history state %q has no non-history siblings under %q — resume will always use the default", id, state.Parent),
			})
		}
	}
}

// checkGuardedOnlyEntry warns when a non-initial state is only reachable
// through guarded transitions (and eventless transitions that also carry
// guards). If every inbound guard fails at runtime the state is unreachable —
// a common "always-false guard combo" production hazard.
func checkGuardedOnlyEntry[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules[RuleGuardedOnlyEntry] {
		return
	}

	inbound := make(map[ir.StateID][]*ir.TransitionConfig)
	for _, state := range machine.States {
		for _, t := range state.Transitions {
			if t.Target == "" || t.Internal {
				continue
			}
			inbound[t.Target] = append(inbound[t.Target], t)
		}
		for _, t := range state.Always {
			if t.Target == "" {
				continue
			}
			inbound[t.Target] = append(inbound[t.Target], t)
		}
		for _, inv := range state.Invocations {
			if inv.OnDone != nil && inv.OnDone.Target != "" {
				inbound[inv.OnDone.Target] = append(inbound[inv.OnDone.Target], inv.OnDone)
			}
			if inv.OnError != nil && inv.OnError.Target != "" {
				inbound[inv.OnError.Target] = append(inbound[inv.OnError.Target], inv.OnError)
			}
		}
		for _, inv := range state.MachineInvocations {
			if inv.OnDone != nil && inv.OnDone.Target != "" {
				inbound[inv.OnDone.Target] = append(inbound[inv.OnDone.Target], inv.OnDone)
			}
			if inv.OnError != nil && inv.OnError.Target != "" {
				inbound[inv.OnError.Target] = append(inbound[inv.OnError.Target], inv.OnError)
			}
		}
	}

	// Compound initials and the machine initial are reachable without a
	// guarded transition.
	unguardedEntry := map[ir.StateID]bool{machine.Initial: true}
	for _, state := range machine.States {
		if state.Initial != "" {
			unguardedEntry[state.Initial] = true
		}
	}

	for id, edges := range inbound {
		if unguardedEntry[id] {
			continue
		}
		if len(edges) == 0 {
			continue
		}
		allGuarded := true
		for _, t := range edges {
			if t.Guard == "" {
				allGuarded = false
				break
			}
		}
		if allGuarded {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Rule:     RuleGuardedOnlyEntry,
				State:    id,
				Message:  fmt.Sprintf("state %q is only entered via guarded transitions — if every guard fails it is unreachable", id),
			})
		}
	}
}

// checkGuardedEventWithoutFallback warns when every transition for an event
// in a state carries a guard and none is an unguarded fallback. If all guards
// fail at runtime the event is silently dropped — the outbound complement of
// guarded-only-entry (which covers inbound reachability).
func checkGuardedEventWithoutFallback[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules[RuleGuardedEventWithoutFallback] {
		return
	}

	for id, state := range machine.States {
		byEvent := make(map[statekit.EventType][]*ir.TransitionConfig)
		for _, t := range state.Transitions {
			if t.Event == "" {
				continue // always / eventless handled elsewhere
			}
			byEvent[t.Event] = append(byEvent[t.Event], t)
		}
		for event, transitions := range byEvent {
			if len(transitions) == 0 {
				continue
			}
			allGuarded := true
			for _, t := range transitions {
				if t.Guard == "" {
					allGuarded = false
					break
				}
			}
			if !allGuarded {
				continue
			}
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: SeverityWarning,
				Rule:     RuleGuardedEventWithoutFallback,
				State:    id,
				Event:    event,
				Message:  fmt.Sprintf("event %q in state %q has only guarded transitions — if every guard fails the event is dropped", event, id),
			})
		}
	}
}

// checkAutoForwardLoop warns when a state both auto-forwards an event to a
// child machine and raises that same event from a transition on the state.
// The child completing work by sending the event back to the parent (or the
// parent re-raising it) forms a parent↔child ping-pong.
func checkAutoForwardLoop[C any](l *Linter, machine *ir.MachineConfig[C], result *Result) {
	if l.IgnoreRules[RuleAutoForwardLoop] {
		return
	}

	for id, state := range machine.States {
		if len(state.MachineInvocations) == 0 {
			continue
		}
		forwarded := make(map[ir.EventType]string) // event → child id
		for _, inv := range state.MachineInvocations {
			name := inv.ID
			if name == "" {
				name = inv.MachineRef
			}
			for _, evt := range inv.AutoForward {
				forwarded[evt] = name
			}
		}
		if len(forwarded) == 0 {
			continue
		}

		checkRaise := func(t *ir.TransitionConfig, via string) {
			for _, raised := range t.Raise {
				if child, ok := forwarded[raised]; ok {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						Severity: SeverityWarning,
						Rule:     RuleAutoForwardLoop,
						State:    id,
						Event:    raised,
						Message:  fmt.Sprintf("state %q auto-forwards %q to child %q and also raises it%s — parent/child ping-pong risk", id, raised, child, via),
					})
				}
			}
		}
		for _, t := range state.Transitions {
			checkRaise(t, "")
		}
		for _, t := range state.Always {
			checkRaise(t, " on an always transition")
		}
	}
}

// Rule names for reference
const (
	RuleUnreachable                 = "unreachable"
	RuleDeadEnd                     = "dead-end"
	RuleNonDeterminism              = "non-determinism"
	RuleCompoundInitial             = "compound-initial"
	RuleSelfTransition              = "self-transition"
	RuleUnusedAction                = "unused-action"
	RuleUnusedGuard                 = "unused-guard"
	RuleInvokeMissingOnError        = "invoke-missing-onerror"
	RuleInvokeIDCollision           = "invoke-id-collision"
	RuleAutoForwardRedundancy       = "auto-forward-redundancy"
	RuleDeepNesting                 = "deep-nesting"
	RuleHistoryWithoutSiblings      = "history-without-siblings"
	RuleGuardedOnlyEntry            = "guarded-only-entry"
	RuleGuardedEventWithoutFallback = "guarded-event-without-fallback"
	RuleAutoForwardLoop             = "auto-forward-loop"
	RuleActorIDCollision            = "actor-id-collision"
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
		RuleHistoryWithoutSiblings,
		RuleGuardedOnlyEntry,
		RuleGuardedEventWithoutFallback,
		RuleAutoForwardLoop,
		RuleActorIDCollision,
	}
}
