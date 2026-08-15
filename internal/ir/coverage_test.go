package ir

import (
	"errors"
	"strings"
	"testing"
)

// Tests that cover gaps identified in coverage report:
// - GetService, GetChildMachine, WithContext (0%)
// - Validate: compound/parallel/history error branches
// - ValidationError.Error empty + single-issue paths
// - ValidationIssue.String without path

func TestMachineConfig_GetService(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("svc", "", struct{}{})
	called := false
	m.Services["s"] = func(ctx ServiceContext[struct{}]) error {
		called = true
		return nil
	}

	got := m.GetService("s")
	if got == nil {
		t.Fatal("expected service, got nil")
	}
	_ = got(ServiceContext[struct{}]{})
	if !called {
		t.Error("service not invoked")
	}

	if m.GetService("missing") != nil {
		t.Error("expected nil for missing service")
	}
}

func TestMachineConfig_GetChildMachine(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("parent", "", struct{}{})
	called := false
	m.ChildMachines["c"] = func(ctx struct{}, send func(Event) error) ChildInterpreter {
		called = true
		return nil
	}

	got := m.GetChildMachine("c")
	if got == nil {
		t.Fatal("expected factory, got nil")
	}
	_ = got(struct{}{}, nil)
	if !called {
		t.Error("factory not invoked")
	}

	if m.GetChildMachine("missing") != nil {
		t.Error("expected nil for missing factory")
	}
}

func TestMachineConfig_WithContext(t *testing.T) {
	t.Parallel()
	type ctx struct{ N int }
	m := NewMachineConfig[ctx]("withctx", "", ctx{})
	m.Initial = "s"
	m.States["s"] = NewStateConfig("s", StateTypeAtomic)
	m.Actions["a"] = func(c *ctx, e Event) {}
	m.Guards["g"] = func(c ctx, e Event) bool { return true }
	m.Services["sv"] = func(s ServiceContext[ctx]) error { return nil }

	clone := m.WithContext(ctx{N: 42})
	if clone.Context.N != 42 {
		t.Errorf("expected N=42, got %d", clone.Context.N)
	}
	if clone.ID != m.ID {
		t.Error("ID should be preserved")
	}
	if clone.Initial != m.Initial {
		t.Error("Initial should be preserved")
	}
	// Original unchanged
	if m.Context.N != 0 {
		t.Error("WithContext should not mutate the original")
	}
}

func TestValidationError_Error_EmptyAndSingle(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		e := &ValidationError{}
		got := e.Error()
		if got != "validation failed" {
			t.Errorf("expected default message, got: %q", got)
		}
	})

	t.Run("single issue", func(t *testing.T) {
		t.Parallel()
		e := &ValidationError{}
		e.AddIssue("CODE", "single issue", "p")
		got := e.Error()
		if got == "" {
			t.Fatal("expected non-empty message")
		}
		// Should not contain "validation failed with N issues" prefix
		if want := "[CODE] single issue (at p)"; got != want {
			t.Errorf("expected %q, got: %q", want, got)
		}
	})
}

func TestValidationIssue_String_WithoutPath(t *testing.T) {
	t.Parallel()
	v := ValidationIssue{Code: "C", Message: "m"}
	got := v.String()
	if want := "[C] m"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestValidate_CompoundInvalidInitial(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("ci", "", struct{}{})
	m.Initial = "root"
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "elsewhere" // not a child
	root.Children = []StateID{"c1"}
	m.States["root"] = root
	c1 := NewStateConfig("c1", StateTypeAtomic)
	c1.Parent = "root"
	m.States["c1"] = c1
	m.States["elsewhere"] = NewStateConfig("elsewhere", StateTypeAtomic)

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeCompoundInvalidInitial) {
		t.Errorf("expected COMPOUND_INVALID_INITIAL, got: %v", err)
	}
}

func TestValidate_CompoundChildNotFound(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("cnf", "", struct{}{})
	m.Initial = "root"
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "ghost"
	root.Children = []StateID{"ghost"}
	m.States["root"] = root

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidChild) {
		t.Errorf("expected INVALID_CHILD, got: %v", err)
	}
}

func TestValidate_CompoundChildWrongParent(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("cwp", "", struct{}{})
	m.Initial = "root"
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "c1"
	root.Children = []StateID{"c1"}
	m.States["root"] = root
	c1 := NewStateConfig("c1", StateTypeAtomic)
	c1.Parent = "other" // wrong parent
	m.States["c1"] = c1

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidChild) {
		t.Errorf("expected INVALID_CHILD for wrong parent, got: %v", err)
	}
}

func TestValidate_ParallelNoRegions(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("pnr", "", struct{}{})
	m.Initial = "p"
	p := NewStateConfig("p", StateTypeParallel)
	m.States["p"] = p

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeParallelNoRegions) {
		t.Errorf("expected PARALLEL_NO_REGIONS, got: %v", err)
	}
}

func TestValidate_ParallelRegionMissingInitial(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("prmi", "", struct{}{})
	m.Initial = "p"
	p := NewStateConfig("p", StateTypeParallel)
	p.Children = []StateID{"r1"}
	m.States["p"] = p
	r1 := NewStateConfig("r1", StateTypeCompound) // compound but no initial
	r1.Parent = "p"
	m.States["r1"] = r1

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeParallelRegionNoInitial) {
		t.Errorf("expected PARALLEL_REGION_NO_INITIAL, got: %v", err)
	}
}

func TestValidate_ParentNotFound(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("pnf", "", struct{}{})
	m.Initial = "child"
	c := NewStateConfig("child", StateTypeAtomic)
	c.Parent = "ghost-parent"
	m.States["child"] = c

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidParent) {
		t.Errorf("expected INVALID_PARENT for missing parent, got: %v", err)
	}
}

func TestValidate_ParentNotCompoundOrParallel(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("pnc", "", struct{}{})
	m.Initial = "p"
	p := NewStateConfig("p", StateTypeAtomic) // atomic parent — invalid
	m.States["p"] = p
	c := NewStateConfig("c", StateTypeAtomic)
	c.Parent = "p"
	m.States["c"] = c

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidParent) {
		t.Errorf("expected INVALID_PARENT for atomic parent, got: %v", err)
	}
}

func TestValidate_HistoryNotInCompound(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("hnic", "", struct{}{})
	m.Initial = "h"
	h := NewStateConfig("h", StateTypeHistory)
	h.HistoryDefault = "x"
	m.States["h"] = h
	m.States["x"] = NewStateConfig("x", StateTypeAtomic)

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeHistoryNotInCompound) {
		t.Errorf("expected HISTORY_NOT_IN_COMPOUND, got: %v", err)
	}
}

func TestValidate_HistoryInvalidDefault(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("hid", "", struct{}{})
	m.Initial = "root"
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "c1"
	root.Children = []StateID{"c1", "h"}
	m.States["root"] = root
	c1 := NewStateConfig("c1", StateTypeAtomic)
	c1.Parent = "root"
	m.States["c1"] = c1
	h := NewStateConfig("h", StateTypeHistory)
	h.Parent = "root"
	h.HistoryDefault = "ghost" // not a state
	m.States["h"] = h

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeHistoryInvalidDefault) {
		t.Errorf("expected HISTORY_INVALID_DEFAULT, got: %v", err)
	}
}

func TestValidate_HistoryDefaultNotSibling(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("hdns", "", struct{}{})
	m.Initial = "root"
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "c1"
	root.Children = []StateID{"c1", "h"}
	m.States["root"] = root
	c1 := NewStateConfig("c1", StateTypeAtomic)
	c1.Parent = "root"
	m.States["c1"] = c1
	other := NewStateConfig("other", StateTypeAtomic) // not a child of root
	m.States["other"] = other
	h := NewStateConfig("h", StateTypeHistory)
	h.Parent = "root"
	h.HistoryDefault = "other"
	m.States["h"] = h

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeHistoryDefaultNotSibling) {
		t.Errorf("expected HISTORY_DEFAULT_NOT_SIBLING, got: %v", err)
	}
}

func TestValidate_DelayNegative(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("dn", "", struct{}{})
	m.Initial = "s"
	s := NewStateConfig("s", StateTypeAtomic)
	s.Transitions = []*TransitionConfig{{
		Event:  "",
		Target: "s",
		Delay:  -1,
	}}
	m.States["s"] = s

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeDelayNegative) {
		t.Errorf("expected DELAY_NEGATIVE, got: %v", err)
	}
}

func TestValidate_NilWhenValid(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("ok", "", struct{}{})
	m.Initial = "s"
	m.States["s"] = NewStateConfig("s", StateTypeAtomic)

	if err := Validate(m); err != nil {
		t.Errorf("expected nil for valid machine, got: %v", err)
	}
}

func TestValidationError_Is_NotValidationError(t *testing.T) {
	t.Parallel()
	e := &ValidationError{}
	other := errors.New("other")
	if errors.Is(e, other) {
		t.Error("expected Is to return false for unrelated error type")
	}
}

func TestStateConfig_HasTag(t *testing.T) {
	t.Parallel()
	s := NewStateConfig("s", StateTypeAtomic)
	s.Tags = []string{"transient", "auto"}

	if !s.HasTag("transient") {
		t.Error("expected HasTag(transient) true")
	}
	if !s.HasTag("auto") {
		t.Error("expected HasTag(auto) true")
	}
	if s.HasTag("missing") {
		t.Error("expected HasTag(missing) false")
	}
	if NewStateConfig("empty", StateTypeAtomic).HasTag("x") {
		t.Error("expected HasTag on untagged state false")
	}
}

func TestMachineConfig_GetInitialLeaf_UnknownState(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("m", "s", struct{}{})
	m.States["s"] = NewStateConfig("s", StateTypeAtomic)

	if got := m.GetInitialLeaf("ghost"); got != "ghost" {
		t.Errorf("GetInitialLeaf(unknown) = %q, want ghost", got)
	}
}

func TestValidationError_Error_EmptyAndMultiple(t *testing.T) {
	t.Parallel()
	empty := &ValidationError{}
	if got := empty.Error(); got != "validation failed" {
		t.Errorf("empty Error() = %q, want validation failed", got)
	}

	multi := &ValidationError{}
	multi.AddIssue("A", "first")
	multi.AddIssue("B", "second", "states", "x")
	msg := multi.Error()
	if !containsAll(msg, "validation failed with 2 issues", "1. [A] first", "2. [B] second (at states.x)") {
		t.Errorf("multi Error() = %q, missing expected fragments", msg)
	}
}

func TestValidate_ParallelRegionNotFound(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("p", "p", struct{}{})
	p := NewStateConfig("p", StateTypeParallel)
	p.Children = []StateID{"ghost"}
	m.States["p"] = p

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidChild) {
		t.Errorf("expected INVALID_CHILD for missing region, got: %v", err)
	}
}

func TestValidate_ParallelRegionWrongParent(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("p", "p", struct{}{})
	p := NewStateConfig("p", StateTypeParallel)
	p.Children = []StateID{"r"}
	m.States["p"] = p
	r := NewStateConfig("r", StateTypeCompound)
	r.Parent = "other" // wrong parent
	r.Initial = "leaf"
	m.States["r"] = r
	leaf := NewStateConfig("leaf", StateTypeAtomic)
	leaf.Parent = "r"
	m.States["leaf"] = leaf

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeInvalidChild) {
		t.Errorf("expected INVALID_CHILD for wrong region parent, got: %v", err)
	}
}

func TestValidate_HistoryMissingDefault(t *testing.T) {
	t.Parallel()
	m := NewMachineConfig[struct{}]("h", "root", struct{}{})
	root := NewStateConfig("root", StateTypeCompound)
	root.Initial = "a"
	root.Children = []StateID{"a", "h"}
	m.States["root"] = root
	a := NewStateConfig("a", StateTypeAtomic)
	a.Parent = "root"
	m.States["a"] = a
	h := NewStateConfig("h", StateTypeHistory)
	h.Parent = "root"
	m.States["h"] = h

	err := Validate(m)
	if err == nil || !containsCode(err, ErrCodeHistoryMissingDefault) {
		t.Errorf("expected HISTORY_MISSING_DEFAULT, got: %v", err)
	}
}

func TestValidate_InternalTransitionTarget(t *testing.T) {
	t.Parallel()
	t.Run("empty target ok", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("i", "s", struct{}{})
		s := NewStateConfig("s", StateTypeAtomic)
		s.Transitions = []*TransitionConfig{{Event: "PING", Internal: true}}
		m.States["s"] = s
		if err := Validate(m); err != nil {
			t.Errorf("expected valid internal with empty target, got: %v", err)
		}
	})

	t.Run("self target ok", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("i", "s", struct{}{})
		s := NewStateConfig("s", StateTypeAtomic)
		s.Transitions = []*TransitionConfig{{Event: "PING", Target: "s", Internal: true}}
		m.States["s"] = s
		if err := Validate(m); err != nil {
			t.Errorf("expected valid internal with self target, got: %v", err)
		}
	})

	t.Run("foreign target rejected", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("i", "s", struct{}{})
		s := NewStateConfig("s", StateTypeAtomic)
		s.Transitions = []*TransitionConfig{{Event: "PING", Target: "other", Internal: true}}
		m.States["s"] = s
		m.States["other"] = NewStateConfig("other", StateTypeAtomic)
		err := Validate(m)
		if err == nil || !containsCode(err, ErrCodeInvalidTarget) {
			t.Errorf("expected INVALID_TARGET for foreign internal target, got: %v", err)
		}
	})
}

func TestValidate_AlwaysTransitions(t *testing.T) {
	t.Parallel()
	t.Run("missing target", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("a", "s", struct{}{})
		s := NewStateConfig("s", StateTypeAtomic)
		s.Always = []*TransitionConfig{{}}
		m.States["s"] = s
		err := Validate(m)
		if err == nil || !containsCode(err, ErrCodeAlwaysMissingTarget) {
			t.Errorf("expected ALWAYS_MISSING_TARGET, got: %v", err)
		}
	})

	t.Run("invalid target", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("a", "s", struct{}{})
		s := NewStateConfig("s", StateTypeAtomic)
		s.Always = []*TransitionConfig{{Target: "ghost"}}
		m.States["s"] = s
		err := Validate(m)
		if err == nil || !containsCode(err, ErrCodeInvalidTarget) {
			t.Errorf("expected INVALID_TARGET, got: %v", err)
		}
	})

	t.Run("missing guard", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("a", "s", struct{}{})
		m.States["done"] = NewStateConfig("done", StateTypeFinal)
		s := NewStateConfig("s", StateTypeAtomic)
		s.Always = []*TransitionConfig{{Target: "done", Guard: "nope"}}
		m.States["s"] = s
		err := Validate(m)
		if err == nil || !containsCode(err, ErrCodeMissingGuard) {
			t.Errorf("expected MISSING_GUARD, got: %v", err)
		}
	})

	t.Run("missing action", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("a", "s", struct{}{})
		m.States["done"] = NewStateConfig("done", StateTypeFinal)
		s := NewStateConfig("s", StateTypeAtomic)
		s.Always = []*TransitionConfig{{Target: "done", Actions: []ActionType{"nope"}}}
		m.States["s"] = s
		err := Validate(m)
		if err == nil || !containsCode(err, ErrCodeMissingAction) {
			t.Errorf("expected MISSING_ACTION, got: %v", err)
		}
	})

	t.Run("valid always", func(t *testing.T) {
		t.Parallel()
		m := NewMachineConfig[struct{}]("a", "s", struct{}{})
		m.Actions["log"] = func(_ *struct{}, _ Event) {}
		m.Guards["ok"] = func(_ struct{}, _ Event) bool { return true }
		m.States["done"] = NewStateConfig("done", StateTypeFinal)
		s := NewStateConfig("s", StateTypeAtomic)
		s.Always = []*TransitionConfig{{
			Target:  "done",
			Guard:   "ok",
			Actions: []ActionType{"log"},
		}}
		m.States["s"] = s
		if err := Validate(m); err != nil {
			t.Errorf("expected valid always, got: %v", err)
		}
	})
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
