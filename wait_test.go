package statekit

import (
	"testing"
	"time"
)

// waitUntil polls cond until it returns true or timeout elapses.
// Prefer this over fixed time.Sleep for async invoke/actor tests.
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
