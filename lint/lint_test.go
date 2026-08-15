package lint_test

import (
	"strings"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/internal/ir"
	"go.klarlabs.de/statekit/lint"
)

func TestLint_CleanMachine(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("clean").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").
		On("STOP").Target("idle").
		On("FINISH").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	if result.HasErrors() {
		t.Errorf("expected no errors, got: %v", result.Errors())
	}
	if result.HasWarnings() {
		t.Errorf("expected no warnings, got: %v", result.Warnings())
	}
}

func TestLint_UnreachableState(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("unreachable").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").
		On("STOP").Target("idle").
		Done().
		State("orphan"). // No transitions lead here
		On("ESCAPE").Target("idle").
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleUnreachable && d.State == "orphan" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected unreachable warning for 'orphan', got: %v", result.Diagnostics)
	}
}

func TestLint_DeadEndState(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("deadend").
		WithInitial("idle").
		State("idle").
		On("START").Target("stuck").
		Done().
		State("stuck"). // No transitions out, not final
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleDeadEnd && d.State == "stuck" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected dead-end warning for 'stuck', got: %v", result.Diagnostics)
	}
}

func TestLint_DeadEnd_FinalStateIsOK(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("final-ok").
		WithInitial("idle").
		State("idle").
		On("FINISH").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleDeadEnd && d.State == "done" {
			t.Errorf("final state should not be flagged as dead-end")
		}
	}
}

func TestLint_NonDeterminism(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("nondeterministic").
		WithInitial("idle").
		State("idle").
		On("GO").Target("a"). // No guard
		On("GO").Target("b"). // No guard - conflict!
		Done().
		State("a").Final().Done().
		State("b").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleNonDeterminism && d.State == "idle" {
			found = true
			if d.Severity != lint.SeverityError {
				t.Errorf("expected error severity for non-determinism")
			}
			break
		}
	}

	if !found {
		t.Errorf("expected non-determinism error for 'idle', got: %v", result.Diagnostics)
	}
}

func TestLint_NonDeterminism_WithGuardsIsOK(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("guarded").
		WithInitial("idle").
		WithGuard("checkA", func(_ struct{}, _ statekit.Event) bool { return true }).
		WithGuard("checkB", func(_ struct{}, _ statekit.Event) bool { return false }).
		State("idle").
		On("GO").Target("a").Guard("checkA").
		On("GO").Target("b").Guard("checkB").
		Done().
		State("a").Final().Done().
		State("b").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleNonDeterminism && d.Severity == lint.SeverityError {
			t.Errorf("guarded transitions should not be flagged as error: %v", d)
		}
	}
}

func TestLint_CompoundWithoutInitial(t *testing.T) {
	// This should actually fail at build time, but let's test the linter anyway
	// We'll need to construct the machine differently or test via internal package
	// For now, just verify the rule exists
	rules := lint.AllRules()
	found := false
	for _, r := range rules {
		if r == lint.RuleCompoundInitial {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected compound-initial rule to exist")
	}
}

func TestLint_SelfTransitionWithEntry(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("self-transition").
		WithInitial("counting").
		WithAction("increment", func(_ *struct{}, _ statekit.Event) {}).
		State("counting").
		OnEntry("increment").
		On("COUNT").Target("counting"). // Self-transition, will re-run entry
		On("DONE").Target("finished").
		Done().
		State("finished").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleSelfTransition && d.State == "counting" {
			found = true
			if d.Severity != lint.SeverityInfo {
				t.Errorf("expected info severity for self-transition")
			}
			break
		}
	}

	if !found {
		t.Errorf("expected self-transition info for 'counting', got: %v", result.Diagnostics)
	}
}

func TestLint_UnusedAction(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("unused-action").
		WithInitial("idle").
		WithAction("used", func(_ *struct{}, _ statekit.Event) {}).
		WithAction("unused", func(_ *struct{}, _ statekit.Event) {}). // Never used
		State("idle").
		OnEntry("used").
		On("DONE").Target("done").
		Done().
		State("done").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleUnusedAction && strings.Contains(d.Message, "unused") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected unused-action info, got: %v", result.Diagnostics)
	}
}

func TestLint_UnusedGuard(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("unused-guard").
		WithInitial("idle").
		WithGuard("used", func(_ struct{}, _ statekit.Event) bool { return true }).
		WithGuard("unused", func(_ struct{}, _ statekit.Event) bool { return true }). // Never used
		State("idle").
		On("GO").Target("done").Guard("used").
		Done().
		State("done").Final().Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleUnusedGuard && strings.Contains(d.Message, "unused") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected unused-guard info, got: %v", result.Diagnostics)
	}
}

func TestLinter_Ignore(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("ignore-test").
		WithInitial("idle").
		State("idle").
		On("START").Target("stuck").
		Done().
		State("stuck"). // Dead end
		Done().
		State("orphan"). // Unreachable
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	linter := lint.NewLinter().
		Ignore(lint.RuleDeadEnd).
		Ignore(lint.RuleUnreachable)

	result := lint.CheckTyped(linter, machine)

	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleDeadEnd || d.Rule == lint.RuleUnreachable {
			t.Errorf("ignored rule should not appear: %v", d)
		}
	}
}

func TestResult_HasErrors(t *testing.T) {
	result := &lint.Result{
		Diagnostics: []lint.Diagnostic{
			{Severity: lint.SeverityInfo, Message: "info"},
			{Severity: lint.SeverityWarning, Message: "warning"},
		},
	}

	if result.HasErrors() {
		t.Error("expected no errors")
	}

	result.Diagnostics = append(result.Diagnostics, lint.Diagnostic{
		Severity: lint.SeverityError, Message: "error",
	})

	if !result.HasErrors() {
		t.Error("expected errors")
	}
}

func TestResult_String(t *testing.T) {
	result := &lint.Result{
		MachineID: "test",
		Diagnostics: []lint.Diagnostic{
			{Severity: lint.SeverityError, Rule: "test-rule", State: "state1", Message: "error msg"},
		},
	}

	str := result.String()
	if !strings.Contains(str, "test") {
		t.Error("expected machine ID in string")
	}
	if !strings.Contains(str, "error") {
		t.Error("expected error in string")
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		s    lint.Severity
		want string
	}{
		{lint.SeverityError, "error"},
		{lint.SeverityWarning, "warning"},
		{lint.SeverityInfo, "info"},
		{lint.Severity(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestAllRules(t *testing.T) {
	rules := lint.AllRules()
	if len(rules) < 5 {
		t.Errorf("expected at least 5 rules, got %d", len(rules))
	}

	expected := []string{
		lint.RuleUnreachable,
		lint.RuleDeadEnd,
		lint.RuleNonDeterminism,
	}

	for _, exp := range expected {
		found := false
		for _, r := range rules {
			if r == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected rule %q in AllRules()", exp)
		}
	}
}

func TestResult_Errors(t *testing.T) {
	result := &lint.Result{
		Diagnostics: []lint.Diagnostic{
			{Severity: lint.SeverityInfo, Message: "info"},
			{Severity: lint.SeverityError, Message: "error1"},
			{Severity: lint.SeverityWarning, Message: "warning"},
			{Severity: lint.SeverityError, Message: "error2"},
		},
	}

	errors := result.Errors()
	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestResult_Warnings(t *testing.T) {
	result := &lint.Result{
		Diagnostics: []lint.Diagnostic{
			{Severity: lint.SeverityInfo, Message: "info"},
			{Severity: lint.SeverityError, Message: "error"},
			{Severity: lint.SeverityWarning, Message: "warning1"},
			{Severity: lint.SeverityWarning, Message: "warning2"},
		},
	}

	warnings := result.Warnings()
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}
}

func TestResult_HasWarnings_NoWarnings(t *testing.T) {
	result := &lint.Result{
		Diagnostics: []lint.Diagnostic{
			{Severity: lint.SeverityInfo, Message: "info"},
		},
	}

	if result.HasWarnings() {
		t.Error("expected no warnings")
	}
}

func TestDiagnostic_String_NoState(t *testing.T) {
	d := lint.Diagnostic{
		Severity: lint.SeverityWarning,
		Rule:     "test-rule",
		State:    "", // Empty state
		Message:  "test message",
	}

	str := d.String()
	if !strings.Contains(str, "(machine)") {
		t.Error("expected (machine) for empty state")
	}
}

func TestLint_DeadEnd_ParentHasTransitions(t *testing.T) {
	// Child state has no transitions but parent does (event bubbling)
	machine, err := statekit.NewMachine[struct{}]("parent-escape").
		WithInitial("active").
		State("active").
		WithInitial("working").
		On("RESET").Target("done").End(). // Parent handles escape
		State("working").                 // No direct transitions
		End().                            // End working StateBuilder, return to active
		Done().                           // Return to MachineBuilder (not End for top-level)
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)

	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleDeadEnd && d.State == "working" {
			t.Errorf("working state should not be dead-end (parent has transitions): %v", d)
		}
	}
}

func TestLint_InvokeMissingOnError(t *testing.T) {
	noop := func(ctx statekit.ServiceContext[struct{}]) error { return nil }
	machine, err := statekit.NewMachine[struct{}]("svc").
		WithInitial("loading").
		WithService("fetchData", noop).
		State("loading").
		Invoke("fetchData").
		ID("fetch").
		OnDone("ready").
		End(). // No OnError configured → should warn
		Done().
		State("ready").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleInvokeMissingOnError && d.State == "loading" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invoke-missing-onerror for 'loading', got: %v", result.Diagnostics)
	}
}

func TestLint_InvokeIDCollision(t *testing.T) {
	noop := func(ctx statekit.ServiceContext[struct{}]) error { return nil }
	machine, err := statekit.NewMachine[struct{}]("collision").
		WithInitial("loading").
		WithService("a", noop).
		WithService("b", noop).
		State("loading").
		Invoke("a").ID("dup").OnDone("ready").OnError("ready").End().
		Invoke("b").ID("dup").OnDone("ready").OnError("ready").End().
		Done().
		State("ready").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleInvokeIDCollision && d.State == "loading" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invoke-id-collision for 'loading', got: %v", result.Diagnostics)
	}
}

func TestLint_ActorIDCollision(t *testing.T) {
	t.Parallel()
	child, err := statekit.NewMachine[struct{}]("child").
		WithInitial("a").
		State("a").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("child build: %v", err)
	}
	factory := func(ctx struct{}, send func(statekit.Event) error) ir.ChildInterpreter {
		return statekit.NewInterpreter(child)
	}
	machine, err := statekit.NewMachine[struct{}]("parent").
		WithInitial("one").
		WithChildMachine("worker", factory).
		State("one").
		InvokeMachine("worker").ID("shared").OnDone("two").End().
		On("NEXT").Target("two").End().
		Done().
		State("two").
		InvokeMachine("worker").ID("shared").OnDone("done").End().
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("parent build: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleActorIDCollision {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected actor-id-collision, got: %v", result.Diagnostics)
	}
}

func TestLint_InvokeWithOnError_NoWarning(t *testing.T) {
	noop := func(ctx statekit.ServiceContext[struct{}]) error { return nil }
	machine, err := statekit.NewMachine[struct{}]("svc-ok").
		WithInitial("loading").
		WithService("fetchData", noop).
		State("loading").
		Invoke("fetchData").
		ID("fetch").
		OnDone("ready").
		OnError("failed").
		End().
		Done().
		State("ready").Final().Done().
		State("failed").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}
	result := lint.Lint(machine)
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleInvokeMissingOnError {
			t.Errorf("did not expect invoke-missing-onerror, got: %v", d)
		}
	}
}

func TestLint_AllRulesIncludesNew(t *testing.T) {
	rules := lint.AllRules()
	expected := []string{
		lint.RuleInvokeMissingOnError,
		lint.RuleInvokeIDCollision,
		lint.RuleHistoryWithoutSiblings,
		lint.RuleGuardedOnlyEntry,
		lint.RuleAutoForwardLoop,
		lint.RuleActorIDCollision,
	}
	for _, want := range expected {
		found := false
		for _, r := range rules {
			if r == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllRules missing %q", want)
		}
	}
	_ = strings.Join // keep import alive
}

func TestLint_AutoForwardRedundancy(t *testing.T) {
	t.Parallel()
	// Parent state declares a transition on TICK *and* asks
	// InvokeMachine to AutoForward TICK to the child. The parent
	// transition fires first, so the AutoForward never reaches the
	// child — that's the footgun the rule catches.
	childMachine, err := statekit.NewMachine[struct{}]("child").
		WithInitial("a").
		State("a").Done().
		Build()
	if err != nil {
		t.Fatalf("child build: %v", err)
	}

	machine, err := statekit.NewMachine[struct{}]("parent").
		WithInitial("active").
		WithChildMachine("worker", func(_ struct{}, _ func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("active").
		On("TICK").Target("active").End().
		InvokeMachine("worker").ID("w").AutoForward("TICK").OnDone("active").OnError("active").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("parent build: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleAutoForwardRedundancy && d.State == "active" && d.Event == "TICK" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected auto-forward-redundancy diagnostic, got: %v", result.Diagnostics)
	}
}

func TestLint_AutoForwardOK(t *testing.T) {
	t.Parallel()
	childMachine, err := statekit.NewMachine[struct{}]("c").
		WithInitial("a").
		State("a").Done().
		Build()
	if err != nil {
		t.Fatalf("child build: %v", err)
	}

	machine, err := statekit.NewMachine[struct{}]("p").
		WithInitial("active").
		WithChildMachine("worker", func(_ struct{}, _ func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("active").
		InvokeMachine("worker").ID("w").AutoForward("TICK").OnDone("active").OnError("active").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("parent build: %v", err)
	}

	result := lint.Lint(machine)
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleAutoForwardRedundancy {
			t.Errorf("did not expect redundancy diagnostic, got: %v", d)
		}
	}
}

func TestLint_DeepNesting(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("deep").
		WithInitial("l1").
		State("l1").
		WithInitial("l2").
		State("l2").
		WithInitial("l3").
		State("l3").
		WithInitial("l4").
		State("l4").
		WithInitial("l5").
		State("l5").
		WithInitial("leaf").
		State("leaf").End().End().End().End().End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleDeepNesting {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deep-nesting diagnostic for leaf, got: %v", result.Diagnostics)
	}
}

func TestLint_HistoryWithoutSiblings(t *testing.T) {
	t.Parallel()

	// Hand-built IR: history is the only child of its compound parent.
	// Build() would reject this shape; lint still catches the modelling mistake
	// when machines are assembled from other paths.
	m := ir.NewMachineConfig[struct{}]("lonely-hist", "active", struct{}{})
	active := ir.NewStateConfig("active", ir.StateTypeCompound)
	active.Initial = "hist"
	active.Children = []ir.StateID{"hist"}
	m.States["active"] = active
	hist := ir.NewStateConfig("hist", ir.StateTypeHistory)
	hist.Parent = "active"
	hist.HistoryType = ir.HistoryTypeShallow
	hist.HistoryDefault = "hist"
	m.States["hist"] = hist

	result := lint.CheckTyped(lint.NewLinter(), m)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleHistoryWithoutSiblings && d.State == "hist" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected history-without-siblings for hist, got: %v", result.Diagnostics)
	}
}

func TestLint_GuardedOnlyEntry(t *testing.T) {
	t.Parallel()

	machine, err := statekit.NewMachine[struct{}]("gated").
		WithInitial("idle").
		WithGuard("ok", func(_ struct{}, _ statekit.Event) bool { return true }).
		State("idle").
		On("GO").Target("special").Guard("ok").End().
		Done().
		State("special").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleGuardedOnlyEntry && d.State == "special" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected guarded-only-entry for special, got: %v", result.Diagnostics)
	}
}

func TestLint_GuardedOnlyEntry_UnguardedInboundOK(t *testing.T) {
	t.Parallel()

	machine, err := statekit.NewMachine[struct{}]("mixed").
		WithInitial("idle").
		WithGuard("ok", func(_ struct{}, _ statekit.Event) bool { return true }).
		State("idle").
		On("GO").Target("special").Guard("ok").End().
		On("FORCE").Target("special").End().
		Done().
		State("special").Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result := lint.Lint(machine)
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleGuardedOnlyEntry {
			t.Fatalf("unexpected guarded-only-entry: %v", d)
		}
	}
}

func TestLint_AutoForwardLoop(t *testing.T) {
	t.Parallel()

	childMachine, err := statekit.NewMachine[struct{}]("child").
		WithInitial("a").
		State("a").Done().
		Build()
	if err != nil {
		t.Fatalf("child build: %v", err)
	}

	machine, err := statekit.NewMachine[struct{}]("parent").
		WithInitial("active").
		WithChildMachine("worker", func(_ struct{}, _ func(statekit.Event) error) ir.ChildInterpreter {
			return statekit.NewInterpreter(childMachine)
		}).
		State("active").
		On("TICK").Target("active").Raise("TICK").End().
		InvokeMachine("worker").ID("w").AutoForward("TICK").OnDone("active").OnError("active").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result := lint.Lint(machine)
	found := false
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleAutoForwardLoop && d.Event == "TICK" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected auto-forward-loop for TICK, got: %v", result.Diagnostics)
	}
}

func TestLint_HistoryWithSiblings_NoWarning(t *testing.T) {
	t.Parallel()

	machine, err := statekit.NewMachine[struct{}]("ok-hist").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		History("hist").Shallow().Default("idle").End().
		State("idle").On("GO").Target("work").End().End().
		State("work").On("BACK").Target("idle").End().End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	result := lint.Lint(machine)
	for _, d := range result.Diagnostics {
		if d.Rule == lint.RuleHistoryWithoutSiblings {
			t.Fatalf("unexpected history-without-siblings: %v", d)
		}
	}
}
