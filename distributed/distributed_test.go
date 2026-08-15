package distributed

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/statekit"
)

func TestDistributedInterpreter_Basic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	di, err := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = di.Stop(ctx) }()

	// Check initial state
	if di.State().Value != "idle" {
		t.Errorf("expected idle, got %s", di.State().Value)
	}

	// Send event
	if err := di.Send(statekit.Event{Type: "START"}); err != nil {
		t.Fatal(err)
	}

	if di.State().Value != "running" {
		t.Errorf("expected running, got %s", di.State().Value)
	}

	// Commit
	committed, err := di.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if committed != 1 {
		t.Errorf("expected 1 committed, got %d", committed)
	}
}

func TestDistributedInterpreter_LockContention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	// First node acquires lock
	di1, err := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = di1.Stop(ctx) }()

	// Second node should fail to acquire (with timeout)
	ctx2, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	_, err = NewDistributedInterpreter(ctx2, "stream-1", machine, eventStore, streamLock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDistributedInterpreter_LockRelease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	// First node acquires and releases lock
	di1, err := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	if err != nil {
		t.Fatal(err)
	}

	_ = di1.Send(statekit.Event{Type: "NOOP"}) // Won't match but exercises code
	_, _ = di1.Commit(ctx)
	_ = di1.Stop(ctx)

	// Second node should now be able to acquire
	di2, err := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	if err != nil {
		t.Fatalf("expected success after release, got %v", err)
	}
	defer func() { _ = di2.Stop(ctx) }()
}

func TestDistributedInterpreter_Hydration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type Context struct {
		Count int
	}

	machine, _ := statekit.NewMachine[Context]("counter").
		WithInitial("counting").
		WithAction("inc", func(ctx *Context, e statekit.Event) {
			ctx.Count++
		}).
		State("counting").
		On("INC").Target("counting").Do("inc").
		On("DONE").Target("finished").
		Done().
		State("finished").Final().Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	// First node processes some events
	di1, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	_ = di1.Send(statekit.Event{Type: "INC"})
	_ = di1.Send(statekit.Event{Type: "INC"})
	_ = di1.Send(statekit.Event{Type: "INC"})
	_, _ = di1.Commit(ctx)

	if di1.Context().Count != 3 {
		t.Errorf("expected count 3, got %d", di1.Context().Count)
	}

	_ = di1.Stop(ctx)

	// Second node should hydrate the state
	di2, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	defer func() { _ = di2.Stop(ctx) }()

	if di2.Context().Count != 3 {
		t.Errorf("expected count 3 after hydration, got %d", di2.Context().Count)
	}
	if di2.Version() != 3 {
		t.Errorf("expected version 3, got %d", di2.Version())
	}
}

func TestDistributedInterpreter_LockExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	// First node acquires with short TTL and no renewal
	di1, err := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock,
		WithLockTTL[struct{}](100*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Stop renewal early
	di1.renewStop()

	// Poll until the lock TTL expires and another node can acquire.
	deadline := time.Now().Add(time.Second)
	var di2 *DistributedInterpreter[struct{}]
	for time.Now().Before(deadline) {
		di2, err = NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("expected success after expiry, got %v", err)
	}
	defer func() { _ = di2.Stop(ctx) }()

	// First node should fail on send
	err = di1.Send(statekit.Event{Type: "TEST"})
	if err != ErrLockLost {
		t.Errorf("expected ErrLockLost, got %v", err)
	}
}

func TestDistributedInterpreter_ConcurrentNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type Context struct {
		Value int
	}

	machine, _ := statekit.NewMachine[Context]("test").
		WithInitial("a").
		WithAction("set", func(ctx *Context, e statekit.Event) {
			if v, ok := e.Payload.(int); ok {
				ctx.Value = v
			}
		}).
		State("a").On("NEXT").Target("b").Do("set").Done().
		State("b").On("NEXT").Target("a").Do("set").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	var wg sync.WaitGroup
	acquired := make(chan int, 10)
	failed := make(chan int, 10)

	// Use a barrier to start all goroutines at the same time
	start := make(chan struct{})

	// Multiple "nodes" competing for the lock simultaneously
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(nodeID int) {
			defer wg.Done()

			// Wait for start signal
			<-start

			// Try to acquire with short timeout
			nodeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			di, err := NewDistributedInterpreter(nodeCtx, "stream-1", machine, eventStore, streamLock)
			if err != nil {
				failed <- nodeID
				return // Couldn't acquire lock
			}

			// Process some events (hold the lock for a bit)
			for j := 0; j < 3; j++ {
				_ = di.Send(statekit.Event{Type: "NEXT", Payload: nodeID*10 + j})
			}
			_, _ = di.Commit(ctx)
			acquired <- nodeID

			// Hold lock for remainder of timeout period
			time.Sleep(200 * time.Millisecond)
			_ = di.Stop(ctx)
		}(i)
	}

	// Start all goroutines simultaneously
	close(start)
	wg.Wait()
	close(acquired)
	close(failed)

	// Only one node should have acquired the lock
	acquiredCount := 0
	for range acquired {
		acquiredCount++
	}
	failedCount := 0
	for range failed {
		failedCount++
	}

	if acquiredCount != 1 {
		t.Errorf("expected exactly 1 node to acquire lock, got %d", acquiredCount)
	}
	if failedCount != 4 {
		t.Errorf("expected 4 nodes to fail, got %d", failedCount)
	}
}

func TestMemoryStreamLock_TryAcquire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lock := NewMemoryStreamLock()

	// First acquire should succeed
	l1, err := lock.TryAcquire(ctx, "stream-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Second acquire should fail
	_, err = lock.TryAcquire(ctx, "stream-1", time.Second)
	if err != ErrLockHeld {
		t.Errorf("expected ErrLockHeld, got %v", err)
	}

	// Release and try again
	_ = l1.Release(ctx)

	l2, err := lock.TryAcquire(ctx, "stream-1", time.Second)
	if err != nil {
		t.Fatalf("expected success after release, got %v", err)
	}
	defer func() { _ = l2.Release(ctx) }()
}

func TestMemoryStreamLock_Renew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lock := NewMemoryStreamLock()

	l, err := lock.TryAcquire(ctx, "stream-1", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Renew before expiry
	time.Sleep(50 * time.Millisecond)
	err = l.Renew(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Should still be held after original TTL
	time.Sleep(60 * time.Millisecond)

	_, err = lock.TryAcquire(ctx, "stream-1", time.Second)
	if err != ErrLockHeld {
		t.Errorf("expected lock to still be held after renewal")
	}

	_ = l.Release(ctx)
}

func TestMemoryStreamLock_Done(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lock := NewMemoryStreamLock()

	l, _ := lock.TryAcquire(ctx, "stream-1", 100*time.Millisecond)

	// Done should not be closed yet
	select {
	case <-l.Done():
		t.Error("Done should not be closed before release")
	default:
	}

	// Release
	_ = l.Release(ctx)

	// Done should be closed
	select {
	case <-l.Done():
		// Expected
	case <-time.After(time.Second):
		t.Error("Done should be closed after release")
	}
}

func TestConsistentHashRouter_RouteStream(t *testing.T) {
	t.Parallel()
	router := NewConsistentHashRouter(100)

	members := []ClusterNode{
		{ID: "node-1", Address: "127.0.0.1:8001"},
		{ID: "node-2", Address: "127.0.0.1:8002"},
		{ID: "node-3", Address: "127.0.0.1:8003"},
	}

	// Same stream should always route to same node
	route1 := router.RouteStream("stream-123", members)
	route2 := router.RouteStream("stream-123", members)
	if route1 != route2 {
		t.Error("same stream should route to same node")
	}

	// Distribution across nodes - use more diverse stream IDs
	distribution := make(map[string]int)
	for i := 0; i < 10000; i++ {
		streamID := "stream-" + string(rune('a'+i%26)) + string(rune('0'+i/26%10)) + "-" + string(rune('A'+i/260%26))
		node := router.RouteStream(streamID, members)
		distribution[node]++
	}

	// Each node should get at least some streams (>5% each)
	for _, member := range members {
		if distribution[member.ID] < 500 {
			t.Errorf("node %s got only %d streams (expected >500)", member.ID, distribution[member.ID])
		}
	}
}

func TestConsistentHashRouter_IsLocal(t *testing.T) {
	t.Parallel()
	router := NewConsistentHashRouter(100)

	members := []ClusterNode{
		{ID: "node-1", Address: "127.0.0.1:8001"},
		{ID: "node-2", Address: "127.0.0.1:8002"},
	}

	// Find a stream that routes to node-1
	var localStream string
	for i := 0; i < 100; i++ {
		streamID := "test-stream-" + string(rune('a'+i))
		if router.RouteStream(streamID, members) == "node-1" {
			localStream = streamID
			break
		}
	}

	if localStream == "" {
		t.Fatal("could not find stream routing to node-1")
	}

	if !router.IsLocal(localStream, members, "node-1") {
		t.Error("expected IsLocal to return true for routed stream")
	}
	if router.IsLocal(localStream, members, "node-2") {
		t.Error("expected IsLocal to return false for non-local stream")
	}
}

func TestDistributedInterpreter_WithSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type Context struct {
		Count int
	}

	machine, _ := statekit.NewMachine[Context]("counter").
		WithInitial("counting").
		WithAction("inc", func(ctx *Context, e statekit.Event) {
			ctx.Count++
		}).
		State("counting").
		On("INC").Target("counting").Do("inc").
		Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	snapshotStore := statekit.NewMemorySnapshotStore[Context]()
	streamLock := NewMemoryStreamLock()

	// First node with snapshots
	di1, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock,
		WithDistributedSnapshotStore[Context](snapshotStore),
		WithDistributedSnapshotConfig[Context](statekit.SnapshotConfig{
			Strategy: statekit.SnapshotByInterval,
			Interval: 3,
		}),
	)

	// Send 3 events and commit - should trigger snapshot
	for i := 0; i < 3; i++ {
		_ = di1.Send(statekit.Event{Type: "INC"})
	}
	_, _ = di1.Commit(ctx)

	// Send 2 more events and commit
	for i := 0; i < 2; i++ {
		_ = di1.Send(statekit.Event{Type: "INC"})
	}
	_, _ = di1.Commit(ctx)
	_ = di1.Stop(ctx)

	// Verify snapshot was created at version 3
	snap, version, _ := snapshotStore.LoadSnapshot(ctx, "stream-1", 1000)
	if snap == nil {
		t.Fatal("expected snapshot to be created")
	}
	if version != 3 {
		t.Errorf("expected snapshot at version 3, got %d", version)
	}

	// Second node should use snapshot + replay remaining events
	di2, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock,
		WithDistributedSnapshotStore[Context](snapshotStore),
	)
	defer func() { _ = di2.Stop(ctx) }()

	if di2.Context().Count != 5 {
		t.Errorf("expected count 5, got %d", di2.Context().Count)
	}
}

func TestDistributedInterpreter_LockHeld(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	di, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)

	if !di.LockHeld() {
		t.Error("expected lock to be held")
	}

	_ = di.Stop(ctx)

	if di.LockHeld() {
		t.Error("expected lock to not be held after stop")
	}
}

func TestDistributedInterpreter_SendAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type Context struct {
		Count int
	}

	machine, _ := statekit.NewMachine[Context]("counter").
		WithInitial("a").
		WithAction("inc", func(ctx *Context, e statekit.Event) {
			ctx.Count++
		}).
		State("a").On("NEXT").Target("b").Do("inc").Done().
		State("b").On("NEXT").Target("a").Do("inc").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	streamLock := NewMemoryStreamLock()

	di, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock)
	defer func() { _ = di.Stop(ctx) }()

	events := []statekit.Event{
		{Type: "NEXT"},
		{Type: "NEXT"},
		{Type: "NEXT"},
	}

	if err := di.SendAll(events); err != nil {
		t.Fatal(err)
	}

	if di.Context().Count != 3 {
		t.Errorf("expected count 3, got %d", di.Context().Count)
	}
}

func TestDistributedInterpreter_ForceSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := statekit.NewMemoryEventStore()
	snapshotStore := statekit.NewMemorySnapshotStore[struct{}]()
	streamLock := NewMemoryStreamLock()

	di, _ := NewDistributedInterpreter(ctx, "stream-1", machine, eventStore, streamLock,
		WithDistributedSnapshotStore[struct{}](snapshotStore),
	)
	defer func() { _ = di.Stop(ctx) }()

	err := di.ForceSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	snap, _, _ := snapshotStore.LoadSnapshot(ctx, "stream-1", 1000)
	if snap == nil {
		t.Error("expected snapshot to be created")
	}
}
