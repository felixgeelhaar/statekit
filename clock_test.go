package statekit

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSystemClock_Fires(t *testing.T) {
	t.Parallel()
	var fired atomic.Bool
	done := make(chan struct{})
	timer := SystemClock().AfterFunc(5*time.Millisecond, func() {
		fired.Store(true)
		close(done)
	})
	t.Cleanup(func() { timer.Stop() })

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timer did not fire within 1s")
	}

	if !fired.Load() {
		t.Error("expected timer to fire")
	}
}

func TestSystemClock_StopBeforeFire(t *testing.T) {
	t.Parallel()
	var fired atomic.Bool
	timer := SystemClock().AfterFunc(time.Hour, func() { fired.Store(true) })

	if !timer.Stop() {
		t.Error("Stop should report true for not-yet-fired timer")
	}
	if fired.Load() {
		t.Error("did not expect callback to fire")
	}
}

func TestFakeClock_AdvanceFires(t *testing.T) {
	t.Parallel()
	clk := NewFakeClock(time.Unix(1_000_000, 0))
	var fired atomic.Int32
	clk.AfterFunc(100*time.Millisecond, func() { fired.Add(1) })

	if fired.Load() != 0 {
		t.Error("timer fired before Advance")
	}
	if got := clk.PendingTimers(); got != 1 {
		t.Errorf("PendingTimers = %d, want 1", got)
	}

	clk.Advance(99 * time.Millisecond)
	if fired.Load() != 0 {
		t.Error("timer fired before deadline")
	}

	clk.Advance(2 * time.Millisecond)
	if fired.Load() != 1 {
		t.Errorf("expected 1 fire after Advance, got %d", fired.Load())
	}
	if got := clk.PendingTimers(); got != 0 {
		t.Errorf("PendingTimers after fire = %d, want 0", got)
	}
}

func TestFakeClock_OrderOfFires(t *testing.T) {
	t.Parallel()
	clk := NewFakeClock(time.Unix(0, 0))
	var order []int
	clk.AfterFunc(50*time.Millisecond, func() { order = append(order, 50) })
	clk.AfterFunc(10*time.Millisecond, func() { order = append(order, 10) })
	clk.AfterFunc(30*time.Millisecond, func() { order = append(order, 30) })

	clk.Advance(100 * time.Millisecond)

	want := []int{10, 30, 50}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %d, want %d", i, order[i], want[i])
		}
	}
}

func TestFakeClock_Stop(t *testing.T) {
	t.Parallel()
	clk := NewFakeClock(time.Unix(0, 0))
	var fired atomic.Bool
	timer := clk.AfterFunc(10*time.Millisecond, func() { fired.Store(true) })

	if !timer.Stop() {
		t.Error("first Stop should report true")
	}
	if timer.Stop() {
		t.Error("second Stop should report false (already stopped)")
	}

	clk.Advance(time.Hour)
	if fired.Load() {
		t.Error("stopped timer should not fire")
	}
	if got := clk.PendingTimers(); got != 0 {
		t.Errorf("PendingTimers after stop = %d, want 0", got)
	}
}

// TestInterpreter_WithFakeClock_DelayedTransition verifies that an
// interpreter using FakeClock fires its delayed transition only when
// the clock is advanced past the deadline.
func TestInterpreter_WithFakeClock_DelayedTransition(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[struct{}]("delay-test").
		WithInitial("loading").
		State("loading").
		After(50 * time.Millisecond).Target("ready").
		Done().
		State("ready").Final().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	clk := NewFakeClock(time.Unix(0, 0))
	interp := NewInterpreter(machine, WithClock[struct{}](clk))
	defer func() { _ = interp.Close() }()

	interp.Start()

	if got := string(interp.State().Value); got != "loading" {
		t.Fatalf("after Start: state = %q, want loading", got)
	}

	clk.Advance(49 * time.Millisecond)
	if got := string(interp.State().Value); got != "loading" {
		t.Errorf("at 49ms: state = %q, want still loading", got)
	}

	clk.Advance(2 * time.Millisecond)
	if got := string(interp.State().Value); got != "ready" {
		t.Errorf("at 51ms: state = %q, want ready", got)
	}
	if !interp.Done() {
		t.Error("expected Done after timer fired")
	}
}
