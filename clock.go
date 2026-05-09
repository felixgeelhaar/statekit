package statekit

import (
	"sort"
	"sync"
	"time"
)

// Clock abstracts wall-clock timing so timer-driven behavior
// (delayed transitions, supervision timers) can be deterministically
// controlled in tests via FakeClock.
//
// Production code uses the default systemClock; pass FakeClock via
// WithClock for tests.
type Clock interface {
	// AfterFunc schedules fn to run after d. The returned Timer.Stop
	// cancels the callback if it has not yet fired.
	AfterFunc(d time.Duration, fn func()) Timer
}

// Timer is the cancelable handle returned by Clock.AfterFunc. Matches
// the Stop semantics of *time.Timer.
type Timer interface {
	// Stop prevents the callback from firing. Returns true if the
	// call stops the timer, false if the timer has already expired
	// or has been stopped.
	Stop() bool
}

// systemClock is the default Clock — a thin wrapper over time.AfterFunc.
type systemClock struct{}

// AfterFunc delegates to time.AfterFunc.
func (systemClock) AfterFunc(d time.Duration, fn func()) Timer {
	return time.AfterFunc(d, fn)
}

// SystemClock returns the default wall-clock implementation.
func SystemClock() Clock { return systemClock{} }

// fakeTimer is the Timer returned by FakeClock.AfterFunc.
type fakeTimer struct {
	clock    *FakeClock
	id       int
	deadline time.Time
	fn       func()
	stopped  bool
}

// Stop removes the timer from its FakeClock if it has not yet fired.
func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	for i, candidate := range t.clock.timers {
		if candidate == t {
			t.clock.timers = append(t.clock.timers[:i], t.clock.timers[i+1:]...)
			t.stopped = true
			return true
		}
	}
	t.stopped = true
	return false
}

// FakeClock is a test-only Clock whose passage of time is controlled
// by Advance and Now. Safe for concurrent use; callbacks run on the
// goroutine that calls Advance.
type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
	nextID int
}

// NewFakeClock returns a FakeClock anchored at the given time. Use
// time.Now() (or any deterministic timestamp) as the anchor.
func NewFakeClock(anchor time.Time) *FakeClock {
	return &FakeClock{now: anchor}
}

// Now returns the FakeClock's current time. The clock does not advance
// on its own — call Advance to move it forward.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc schedules fn to fire when the clock advances past d.
func (c *FakeClock) AfterFunc(d time.Duration, fn func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	t := &fakeTimer{
		clock:    c,
		id:       c.nextID,
		deadline: c.now.Add(d),
		fn:       fn,
	}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves the clock forward by d, firing any callbacks whose
// deadline falls within the new interval. Callbacks fire in deadline
// order; ties resolve by registration order.
//
// Callbacks run synchronously on the caller's goroutine before
// Advance returns. This makes timer-driven tests deterministic.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	target := c.now

	// Pull out timers whose deadline has passed; sort them by deadline.
	due := make([]*fakeTimer, 0, len(c.timers))
	keep := c.timers[:0]
	for _, t := range c.timers {
		if !t.deadline.After(target) {
			due = append(due, t)
		} else {
			keep = append(keep, t)
		}
	}
	c.timers = keep
	c.mu.Unlock()

	sort.SliceStable(due, func(i, j int) bool {
		if due[i].deadline.Equal(due[j].deadline) {
			return due[i].id < due[j].id
		}
		return due[i].deadline.Before(due[j].deadline)
	})

	for _, t := range due {
		// Mark stopped so a late Stop() call is a no-op.
		c.mu.Lock()
		t.stopped = true
		c.mu.Unlock()
		t.fn()
	}
}

// PendingTimers reports how many timers are scheduled and not yet
// fired or stopped. Useful for test invariants.
func (c *FakeClock) PendingTimers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}
