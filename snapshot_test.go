package statekit

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type snapshotContext struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

func TestSnapshot_Simple(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[snapshotContext]("test").
		WithInitial("idle").
		WithContext(snapshotContext{Count: 0, Name: "initial"}).
		State("idle").On("GO").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp := NewInterpreter(machine)
	interp.Start()
	interp.Send(Event{Type: "GO"})
	interp.UpdateContext(func(ctx *snapshotContext) {
		ctx.Count = 42
		ctx.Name = "updated"
	})

	// Take snapshot
	snap := interp.Snapshot()

	// Verify snapshot contents
	if snap.MachineID != "test" {
		t.Errorf("expected MachineID 'test', got %q", snap.MachineID)
	}
	if snap.CurrentState != "running" {
		t.Errorf("expected CurrentState 'running', got %q", snap.CurrentState)
	}
	if snap.Context.Count != 42 {
		t.Errorf("expected Context.Count 42, got %d", snap.Context.Count)
	}
	if snap.Context.Name != "updated" {
		t.Errorf("expected Context.Name 'updated', got %q", snap.Context.Name)
	}
	if snap.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestSnapshot_Restore(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[snapshotContext]("test").
		WithInitial("idle").
		WithContext(snapshotContext{Count: 0}).
		State("idle").On("GO").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	// Create first interpreter, advance state
	interp1 := NewInterpreter(machine)
	interp1.Start()
	interp1.Send(Event{Type: "GO"})
	interp1.UpdateContext(func(ctx *snapshotContext) {
		ctx.Count = 100
	})

	// Take snapshot
	snap := interp1.Snapshot()

	// Create new interpreter and restore
	interp2 := NewInterpreter(machine)
	err := interp2.Restore(snap)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify restored state
	state := interp2.State()
	if state.Value != "running" {
		t.Errorf("expected state 'running', got %q", state.Value)
	}
	if state.Context.Count != 100 {
		t.Errorf("expected Context.Count 100, got %d", state.Context.Count)
	}

	// Verify interpreter is functional after restore
	interp2.Send(Event{Type: "STOP"})
	if interp2.State().Value != "idle" {
		t.Errorf("expected state 'idle' after STOP, got %q", interp2.State().Value)
	}
}

func TestSnapshot_RestoreWithHistory(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[struct{}]("player").
		WithInitial("playing").
		State("playing").
		WithInitial("track1").
		On("PAUSE").Target("paused").End().
		History("hist").Shallow().Default("track1").End().
		State("track1").On("NEXT").Target("track2").End().End().
		State("track2").On("NEXT").Target("track3").End().End().
		State("track3").End().
		Done().
		State("paused").On("PLAY").Target("hist").Done().
		Build()

	// Advance to track2, pause, then snapshot
	interp1 := NewInterpreter(machine)
	interp1.Start()
	interp1.Send(Event{Type: "NEXT"}) // track1 -> track2
	interp1.Send(Event{Type: "PAUSE"})

	snap := interp1.Snapshot()

	// Restore and verify history works
	interp2 := NewInterpreter(machine)
	if err := interp2.Restore(snap); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	if interp2.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", interp2.State().Value)
	}

	// Resume should go to track2 (from history)
	interp2.Send(Event{Type: "PLAY"})
	if interp2.State().Value != "track2" {
		t.Errorf("expected 'track2' from history, got %q", interp2.State().Value)
	}
}

func TestSnapshot_RestoreWithParallel(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[struct{}]("editor").
		WithInitial("active").
		State("active").Parallel().
		Region("bold").WithInitial("bold_off").
		State("bold_off").On("TOGGLE_BOLD").Target("bold_on").EndState().
		State("bold_on").On("TOGGLE_BOLD").Target("bold_off").EndState().
		EndRegion().
		Region("italic").WithInitial("italic_off").
		State("italic_off").On("TOGGLE_ITALIC").Target("italic_on").EndState().
		State("italic_on").On("TOGGLE_ITALIC").Target("italic_off").EndState().
		EndRegion().
		Done().
		Build()
	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	interp1 := NewInterpreter(machine)
	interp1.Start()
	interp1.Send(Event{Type: "TOGGLE_BOLD"})

	snap := interp1.Snapshot()

	// Verify snapshot has parallel state info
	if snap.CurrentParallel != "active" {
		t.Errorf("expected CurrentParallel 'active', got %q", snap.CurrentParallel)
	}
	if len(snap.ActiveInParallel) != 2 {
		t.Errorf("expected 2 active regions, got %d", len(snap.ActiveInParallel))
	}

	// Restore to new interpreter
	interp2 := NewInterpreter(machine)
	if err := interp2.Restore(snap); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify parallel state is restored
	state := interp2.State()
	if state.Value != "active" {
		t.Errorf("expected 'active', got %q", state.Value)
	}
	if state.ActiveInParallel["bold"] != "bold_on" {
		t.Errorf("expected bold='bold_on', got %q", state.ActiveInParallel["bold"])
	}
	if state.ActiveInParallel["italic"] != "italic_off" {
		t.Errorf("expected italic='italic_off', got %q", state.ActiveInParallel["italic"])
	}

	// Verify transitions still work
	interp2.Send(Event{Type: "TOGGLE_ITALIC"})
	if interp2.State().ActiveInParallel["italic"] != "italic_on" {
		t.Errorf("expected italic='italic_on' after toggle")
	}
}

func TestSnapshot_RestoreMachineMismatch(t *testing.T) {
	t.Parallel()
	machine1, _ := NewMachine[struct{}]("machine1").
		WithInitial("idle").
		State("idle").Done().
		Build()

	machine2, _ := NewMachine[struct{}]("machine2").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp1 := NewInterpreter(machine1)
	interp1.Start()
	snap := interp1.Snapshot()

	interp2 := NewInterpreter(machine2)
	err := interp2.Restore(snap)

	if err == nil {
		t.Fatal("expected error for machine mismatch")
	}

	var restoreErr *RestoreError
	if !errors.As(err, &restoreErr) {
		t.Fatalf("expected RestoreError, got %T", err)
	}
	if restoreErr.Code != ErrCodeSnapshotMachineMismatch {
		t.Errorf("expected code %q, got %q", ErrCodeSnapshotMachineMismatch, restoreErr.Code)
	}
}

func TestSnapshot_RestoreInvalidState(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		State("running").Done().
		Build()

	interp := NewInterpreter(machine)

	snap := Snapshot[struct{}]{
		MachineID:    "test",
		CurrentState: "nonexistent",
		CreatedAt:    time.Now(),
	}

	err := interp.Restore(snap)
	if err == nil {
		t.Fatal("expected error for invalid state")
	}

	var restoreErr *RestoreError
	if !errors.As(err, &restoreErr) {
		t.Fatalf("expected RestoreError, got %T", err)
	}
	if restoreErr.Code != ErrCodeSnapshotInvalidState {
		t.Errorf("expected code %q, got %q", ErrCodeSnapshotInvalidState, restoreErr.Code)
	}
}

func TestSnapshot_JSONSerialization(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[snapshotContext]("test").
		WithInitial("idle").
		WithContext(snapshotContext{Count: 42, Name: "test"}).
		State("idle").Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	snap := interp.Snapshot()

	// Serialize to JSON
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var restored Snapshot[snapshotContext]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify
	if restored.MachineID != snap.MachineID {
		t.Errorf("MachineID mismatch")
	}
	if restored.CurrentState != snap.CurrentState {
		t.Errorf("CurrentState mismatch")
	}
	if restored.Context.Count != snap.Context.Count {
		t.Errorf("Context.Count mismatch")
	}
	if restored.Context.Name != snap.Context.Name {
		t.Errorf("Context.Name mismatch")
	}
}

func TestSnapshot_RestoreWithDelayedTransition(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[struct{}]("test").
		WithInitial("waiting").
		State("waiting").
		After(100 * time.Millisecond).Target("timeout").
		On("CANCEL").Target("cancelled").
		Done().
		State("timeout").Done().
		State("cancelled").Done().
		Build()

	clk1 := NewFakeClock(time.Unix(0, 0))
	interp1 := NewInterpreter(machine, WithClock[struct{}](clk1))
	interp1.Start()

	// Take snapshot while timer is pending
	snap := interp1.Snapshot()
	interp1.Stop()

	// Restore to new interpreter sharing a FakeClock so the restored timer is deterministic
	clk2 := NewFakeClock(time.Unix(0, 0))
	interp2 := NewInterpreter(machine, WithClock[struct{}](clk2))
	if err := interp2.Restore(snap); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	clk2.Advance(100 * time.Millisecond)

	if interp2.State().Value != "timeout" {
		t.Errorf("expected 'timeout', got %q", interp2.State().Value)
	}

	interp2.Stop()
}

func TestSnapshot_RestoreCancelsExistingTimers(t *testing.T) {
	t.Parallel()
	machine, _ := NewMachine[struct{}]("test").
		WithInitial("state1").
		State("state1").
		After(50 * time.Millisecond).Target("timeout1").
		On("GO").Target("state2").
		Done().
		State("state2").
		After(50 * time.Millisecond).Target("timeout2").
		Done().
		State("timeout1").Done().
		State("timeout2").Done().
		Build()

	clk := NewFakeClock(time.Unix(0, 0))
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	interp.Start()

	// Create snapshot at state2
	snap := Snapshot[struct{}]{
		MachineID:    "test",
		CurrentState: "state2",
		CreatedAt:    time.Now(),
	}

	// Restore should cancel state1's timer and start state2's timer
	if err := interp.Restore(snap); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	clk.Advance(50 * time.Millisecond)

	// Should be at timeout2, not timeout1
	if interp.State().Value != "timeout2" {
		t.Errorf("expected 'timeout2', got %q", interp.State().Value)
	}

	interp.Stop()
}

func TestRestoreError_Error(t *testing.T) {
	t.Parallel()
	err := &RestoreError{
		Code:    "TEST_CODE",
		Message: "test message",
	}

	expected := "TEST_CODE: test message"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
