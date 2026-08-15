package statekit

import (
	"testing"
	"time"
)

// waitUntil polls cond until it returns true or 1s elapses.
// Prefer this over fixed time.Sleep for async invoke/actor tests.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	const timeout = time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
