package ir

import (
	"testing"
	"time"
)

type testContext struct {
	Count int
}

func TestNewMachineConfig(t *testing.T) {
	t.Parallel()
	ctx := testContext{Count: 0}
	machine := NewMachineConfig("test", "initial", ctx)

	if machine.ID != "test" {
		t.Errorf("expected ID 'test', got %v", machine.ID)
	}
	if machine.Initial != "initial" {
		t.Errorf("expected Initial 'initial', got %v", machine.Initial)
	}
	if machine.Context.Count != 0 {
		t.Errorf("expected Context.Count 0, got %v", machine.Context.Count)
	}
	if machine.States == nil {
		t.Error("expected States map to be initialized")
	}
	if machine.Actions == nil {
		t.Error("expected Actions map to be initialized")
	}
	if machine.Guards == nil {
		t.Error("expected Guards map to be initialized")
	}
}

func TestNewStateConfig(t *testing.T) {
	t.Parallel()
	state := NewStateConfig("green", StateTypeAtomic)

	if state.ID != "green" {
		t.Errorf("expected ID 'green', got %v", state.ID)
	}
	if state.Type != StateTypeAtomic {
		t.Errorf("expected Type Atomic, got %v", state.Type)
	}
}

func TestNewTransitionConfig(t *testing.T) {
	t.Parallel()
	trans := NewTransitionConfig("TIMER", "yellow")

	if trans.Event != "TIMER" {
		t.Errorf("expected Event 'TIMER', got %v", trans.Event)
	}
	if trans.Target != "yellow" {
		t.Errorf("expected Target 'yellow', got %v", trans.Target)
	}
	if trans.Guard != "" {
		t.Errorf("expected Guard to be empty, got %v", trans.Guard)
	}
}

func TestMachineConfig_GetState(t *testing.T) {
	t.Parallel()
	machine := NewMachineConfig[testContext]("test", "initial", testContext{})
	state := NewStateConfig("green", StateTypeAtomic)
	machine.States["green"] = state

	got := machine.GetState("green")
	if got != state {
		t.Error("expected to get the same state")
	}

	got = machine.GetState("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent state")
	}
}

func TestMachineConfig_GetAction(t *testing.T) {
	t.Parallel()
	machine := NewMachineConfig[testContext]("test", "initial", testContext{})
	action := func(ctx *testContext, e Event) {
		ctx.Count++
	}
	machine.Actions["increment"] = action

	got := machine.GetAction("increment")
	if got == nil {
		t.Fatal("expected to get action")
	}

	// Verify action works
	ctx := testContext{Count: 0}
	got(&ctx, Event{})
	if ctx.Count != 1 {
		t.Errorf("expected Count 1, got %v", ctx.Count)
	}

	got = machine.GetAction("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent action")
	}
}

func TestMachineConfig_GetGuard(t *testing.T) {
	t.Parallel()
	machine := NewMachineConfig[testContext]("test", "initial", testContext{})
	guard := func(ctx testContext, e Event) bool {
		return ctx.Count > 0
	}
	machine.Guards["hasCount"] = guard

	got := machine.GetGuard("hasCount")
	if got == nil {
		t.Fatal("expected to get guard")
	}

	// Verify guard works
	if got(testContext{Count: 0}, Event{}) {
		t.Error("expected guard to return false for Count 0")
	}
	if !got(testContext{Count: 1}, Event{}) {
		t.Error("expected guard to return true for Count 1")
	}

	got = machine.GetGuard("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent guard")
	}
}

func TestStateConfig_FindTransition(t *testing.T) {
	t.Parallel()
	state := NewStateConfig("green", StateTypeAtomic)
	trans1 := NewTransitionConfig("TIMER", "yellow")
	trans2 := NewTransitionConfig("RESET", "green")
	state.Transitions = []*TransitionConfig{trans1, trans2}

	got := state.FindTransition("TIMER")
	if got != trans1 {
		t.Error("expected to find TIMER transition")
	}

	got = state.FindTransition("RESET")
	if got != trans2 {
		t.Error("expected to find RESET transition")
	}

	got = state.FindTransition("NONEXISTENT")
	if got != nil {
		t.Error("expected nil for nonexistent event")
	}
}

func TestStateType_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		st   StateType
		want string
	}{
		{StateTypeAtomic, "atomic"},
		{StateTypeCompound, "compound"},
		{StateTypeFinal, "final"},
		{StateTypeHistory, "history"},
		{StateTypeParallel, "parallel"},
		{StateType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.st.String(); got != tt.want {
				t.Errorf("StateType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHistoryType_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ht   HistoryType
		want string
	}{
		{HistoryTypeShallow, "shallow"},
		{HistoryTypeDeep, "deep"},
		{HistoryType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ht.String(); got != tt.want {
				t.Errorf("HistoryType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransitionConfig_IsDelayed(t *testing.T) {
	t.Parallel()
	t.Run("not delayed", func(t *testing.T) {
		trans := NewTransitionConfig("GO", "target")
		if trans.IsDelayed() {
			t.Error("expected IsDelayed to return false for zero delay")
		}
	})

	t.Run("delayed", func(t *testing.T) {
		trans := NewTransitionConfig("", "target")
		trans.Delay = 100 * time.Millisecond
		if !trans.IsDelayed() {
			t.Error("expected IsDelayed to return true for non-zero delay")
		}
	})
}

func TestStateConfig_IsHistory(t *testing.T) {
	t.Parallel()
	t.Run("history state", func(t *testing.T) {
		state := NewStateConfig("hist", StateTypeHistory)
		if !state.IsHistory() {
			t.Error("expected IsHistory to return true")
		}
	})

	t.Run("non-history state", func(t *testing.T) {
		state := NewStateConfig("normal", StateTypeAtomic)
		if state.IsHistory() {
			t.Error("expected IsHistory to return false")
		}
	})
}

func TestStateConfig_IsParallel(t *testing.T) {
	t.Parallel()
	t.Run("parallel state", func(t *testing.T) {
		state := NewStateConfig("parallel", StateTypeParallel)
		if !state.IsParallel() {
			t.Error("expected IsParallel to return true")
		}
	})

	t.Run("non-parallel state", func(t *testing.T) {
		state := NewStateConfig("normal", StateTypeAtomic)
		if state.IsParallel() {
			t.Error("expected IsParallel to return false")
		}
	})
}

func TestStateConfig_GetDelayedTransitions(t *testing.T) {
	t.Parallel()
	state := NewStateConfig("waiting", StateTypeAtomic)

	// Add mix of normal and delayed transitions
	trans1 := NewTransitionConfig("GO", "target1")
	trans2 := NewTransitionConfig("", "target2")
	trans2.Delay = 100 * time.Millisecond
	trans3 := NewTransitionConfig("CANCEL", "target3")
	trans4 := NewTransitionConfig("", "target4")
	trans4.Delay = 200 * time.Millisecond

	state.Transitions = []*TransitionConfig{trans1, trans2, trans3, trans4}

	delayed := state.GetDelayedTransitions()
	if len(delayed) != 2 {
		t.Fatalf("expected 2 delayed transitions, got %d", len(delayed))
	}
	if delayed[0] != trans2 {
		t.Error("expected first delayed transition to be trans2")
	}
	if delayed[1] != trans4 {
		t.Error("expected second delayed transition to be trans4")
	}
}

func TestStateConfig_GetDelayedTransitions_None(t *testing.T) {
	t.Parallel()
	state := NewStateConfig("active", StateTypeAtomic)
	state.Transitions = []*TransitionConfig{
		NewTransitionConfig("GO", "target1"),
		NewTransitionConfig("STOP", "target2"),
	}

	delayed := state.GetDelayedTransitions()
	if len(delayed) != 0 {
		t.Errorf("expected 0 delayed transitions, got %d", len(delayed))
	}
}
