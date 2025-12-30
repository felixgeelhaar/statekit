package statekit

import (
	"context"
	"testing"
	"time"
)

func TestPersistentInterpreter_Basic(t *testing.T) {
	ctx := context.Background()

	machine, err := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	eventStore := NewMemoryEventStore()

	pi, err := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	if pi.State().Value != "idle" {
		t.Errorf("expected initial state 'idle', got %s", pi.State().Value)
	}

	// Send event
	pi.Send(Event{Type: "START"})
	if pi.State().Value != "running" {
		t.Errorf("expected state 'running', got %s", pi.State().Value)
	}

	// Uncommitted events
	if pi.UncommittedCount() != 1 {
		t.Errorf("expected 1 uncommitted event, got %d", pi.UncommittedCount())
	}

	// Commit
	committed, err := pi.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if committed != 1 {
		t.Errorf("expected 1 committed event, got %d", committed)
	}

	// Version should be updated
	if pi.Version() != 1 {
		t.Errorf("expected version 1, got %d", pi.Version())
	}

	// Uncommitted should be empty
	if pi.UncommittedCount() != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", pi.UncommittedCount())
	}

	pi.Stop()
}

func TestPersistentInterpreter_Hydration(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	eventStore := NewMemoryEventStore()

	// First interpreter - create some events
	pi1, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	pi1.Send(Event{Type: "START"})
	_, _ = pi1.Commit(ctx)
	pi1.Stop()

	// Second interpreter - should hydrate from events
	pi2, err := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	if err != nil {
		t.Fatal(err)
	}

	// Should be in "running" state from hydration
	if pi2.State().Value != "running" {
		t.Errorf("expected hydrated state 'running', got %s", pi2.State().Value)
	}

	// Version should match
	if pi2.Version() != 1 {
		t.Errorf("expected version 1, got %d", pi2.Version())
	}

	pi2.Stop()
}

func TestPersistentInterpreter_WithSnapshot(t *testing.T) {
	ctx := context.Background()

	type Context struct {
		Count int
	}

	machine, _ := NewMachine[Context]("counter").
		WithInitial("counting").
		WithAction("increment", func(ctx *Context, e Event) {
			ctx.Count++
		}).
		State("counting").
		On("INCREMENT").Target("counting").Do("increment").
		On("FINISH").Target("done").
		Done().
		State("done").Final().Done().
		Build()

	eventStore := NewMemoryEventStore()
	snapshotStore := NewMemorySnapshotStore[Context]()

	// Create interpreter with snapshot every 3 events
	pi1, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore,
		WithSnapshotStore[Context](snapshotStore),
		WithSnapshotConfig[Context](SnapshotConfig{
			Strategy: SnapshotByInterval,
			Interval: 3,
		}),
	)

	// Send 5 events
	for i := 0; i < 5; i++ {
		pi1.Send(Event{Type: "INCREMENT"})
	}
	_, _ = pi1.Commit(ctx)
	pi1.Stop()

	// Create new interpreter - should use snapshot
	pi2, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore,
		WithSnapshotStore[Context](snapshotStore),
	)

	// Context should be restored
	if pi2.Context().Count != 5 {
		t.Errorf("expected count 5, got %d", pi2.Context().Count)
	}

	pi2.Stop()
}

func TestPersistentInterpreter_ConcurrencyConflict(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	eventStore := NewMemoryEventStore()

	// Two interpreters on same stream
	pi1, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	pi2, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)

	// Both send events
	pi1.Send(Event{Type: "START"})
	pi2.Send(Event{Type: "START"})

	// First commit succeeds
	_, err := pi1.Commit(ctx)
	if err != nil {
		t.Errorf("first commit should succeed: %v", err)
	}

	// Second commit should fail with concurrency conflict
	_, err = pi2.Commit(ctx)
	if err == nil {
		t.Error("expected concurrency conflict error")
	}
	if _, ok := err.(*ErrConcurrencyConflict); !ok {
		t.Errorf("expected ErrConcurrencyConflict, got %T", err)
	}

	pi1.Stop()
	pi2.Stop()
}

func TestPersistentInterpreter_NoTransitionNoEvent(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	eventStore := NewMemoryEventStore()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)

	// Send event that doesn't cause transition (no handler for it)
	pi.Send(Event{Type: "UNKNOWN"})

	// No uncommitted events
	if pi.UncommittedCount() != 0 {
		t.Errorf("expected 0 uncommitted events for non-transition, got %d", pi.UncommittedCount())
	}

	pi.Stop()
}

func TestPersistentInterpreter_MultipleEvents(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("a").
		State("a").On("NEXT").Target("b").Done().
		State("b").On("NEXT").Target("c").Done().
		State("c").On("NEXT").Target("a").Done().
		Build()

	eventStore := NewMemoryEventStore()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)

	// Send multiple events
	pi.SendAll([]Event{
		{Type: "NEXT"},
		{Type: "NEXT"},
		{Type: "NEXT"},
	})

	// Should be back at 'a'
	if pi.State().Value != "a" {
		t.Errorf("expected state 'a', got %s", pi.State().Value)
	}

	// 3 uncommitted events
	if pi.UncommittedCount() != 3 {
		t.Errorf("expected 3 uncommitted events, got %d", pi.UncommittedCount())
	}

	// Commit all
	committed, _ := pi.Commit(ctx)
	if committed != 3 {
		t.Errorf("expected 3 committed events, got %d", committed)
	}

	// Verify version
	if pi.Version() != 3 {
		t.Errorf("expected version 3, got %d", pi.Version())
	}

	pi.Stop()
}

func TestPersistentInterpreter_SnapshotOnFinal(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").On("FINISH").Target("done").Done().
		State("done").Final().Done().
		Build()

	eventStore := NewMemoryEventStore()
	snapshotStore := NewMemorySnapshotStore[struct{}]()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore,
		WithSnapshotStore[struct{}](snapshotStore),
		WithSnapshotConfig[struct{}](SnapshotConfig{
			Strategy: SnapshotOnFinal,
		}),
	)

	// Send event to reach final state
	pi.Send(Event{Type: "FINISH"})
	_, _ = pi.Commit(ctx)

	// Check snapshot was created
	snapshot, version, _ := snapshotStore.LoadSnapshot(ctx, "stream-1", 100)
	if snapshot == nil {
		t.Error("expected snapshot on final state")
	}
	if version != 1 {
		t.Errorf("expected snapshot version 1, got %d", version)
	}

	pi.Stop()
}

func TestReplayEvents(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	eventStore := NewMemoryEventStore()

	// Create events
	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	pi.Send(Event{Type: "START"})
	pi.Send(Event{Type: "STOP"})
	pi.Send(Event{Type: "START"})
	_, _ = pi.Commit(ctx)
	pi.Stop()

	// Replay events
	replayed, err := ReplayEvents(ctx, machine, eventStore, "stream-1")
	if err != nil {
		t.Fatal(err)
	}

	// Should end up in 'running' state
	if replayed.State().Value != "running" {
		t.Errorf("expected replayed state 'running', got %s", replayed.State().Value)
	}
}

func TestReplayToVersion(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("a").
		State("a").On("NEXT").Target("b").Done().
		State("b").On("NEXT").Target("c").Done().
		State("c").Done().
		Build()

	eventStore := NewMemoryEventStore()

	// Create events: a -> b -> c
	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	pi.Send(Event{Type: "NEXT"}) // a -> b
	pi.Send(Event{Type: "NEXT"}) // b -> c
	_, _ = pi.Commit(ctx)
	pi.Stop()

	// Replay only first event
	replayed, err := ReplayToVersion(ctx, machine, eventStore, "stream-1", 1)
	if err != nil {
		t.Fatal(err)
	}

	// Should be in 'b' state
	if replayed.State().Value != "b" {
		t.Errorf("expected replayed state 'b', got %s", replayed.State().Value)
	}
}

func TestMemoryEventStore_LoadFromVersion(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryEventStore()

	// Append some events
	events := []PersistedEvent{
		{ID: "1", StreamID: "s1", Version: 1, Type: "A"},
		{ID: "2", StreamID: "s1", Version: 2, Type: "B"},
		{ID: "3", StreamID: "s1", Version: 3, Type: "C"},
	}
	store.AppendEvents(ctx, "s1", 0, events)

	// Load from version 2
	loaded, _ := store.LoadEvents(ctx, "s1", 2)
	if len(loaded) != 1 {
		t.Errorf("expected 1 event from version 2, got %d", len(loaded))
	}
	if loaded[0].Type != "C" {
		t.Errorf("expected event type 'C', got %s", loaded[0].Type)
	}
}

func TestMemorySnapshotStore_LoadLatest(t *testing.T) {
	ctx := context.Background()
	store := NewMemorySnapshotStore[struct{}]()

	// Save multiple snapshots
	store.SaveSnapshot(ctx, "s1", 5, &MachineSnapshot[struct{}]{StateValue: "state5"})
	store.SaveSnapshot(ctx, "s1", 10, &MachineSnapshot[struct{}]{StateValue: "state10"})
	store.SaveSnapshot(ctx, "s1", 15, &MachineSnapshot[struct{}]{StateValue: "state15"})

	// Load latest at version 12
	snap, version, _ := store.LoadSnapshot(ctx, "s1", 12)
	if version != 10 {
		t.Errorf("expected snapshot version 10, got %d", version)
	}
	if snap.StateValue != "state10" {
		t.Errorf("expected state 'state10', got %s", snap.StateValue)
	}

	// Load latest at version 100
	snap, version, _ = store.LoadSnapshot(ctx, "s1", 100)
	if version != 15 {
		t.Errorf("expected snapshot version 15, got %d", version)
	}
}

func TestPersistentInterpreter_ForceSnapshot(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := NewMemoryEventStore()
	snapshotStore := NewMemorySnapshotStore[struct{}]()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore,
		WithSnapshotStore[struct{}](snapshotStore),
	)

	// Force snapshot
	err := pi.ForceSnapshot(ctx)
	if err != nil {
		t.Errorf("ForceSnapshot failed: %v", err)
	}

	// Check snapshot exists
	snap, _, _ := snapshotStore.LoadSnapshot(ctx, "stream-1", 100)
	if snap == nil {
		t.Error("expected snapshot after ForceSnapshot")
	}

	pi.Stop()
}

func TestPersistentInterpreter_WithContext(t *testing.T) {
	ctx := context.Background()

	type OrderContext struct {
		OrderID string
		Total   float64
	}

	machine, _ := NewMachine[OrderContext]("order").
		WithInitial("pending").
		WithContext(OrderContext{OrderID: "ORD-001"}).
		WithAction("setTotal", func(ctx *OrderContext, e Event) {
			if total, ok := e.Payload.(float64); ok {
				ctx.Total = total
			}
		}).
		State("pending").On("PRICE").Target("priced").Do("setTotal").Done().
		State("priced").On("CONFIRM").Target("confirmed").Done().
		State("confirmed").Final().Done().
		Build()

	eventStore := NewMemoryEventStore()
	snapshotStore := NewMemorySnapshotStore[OrderContext]()

	pi, _ := NewPersistentInterpreter(ctx, "order-001", machine, eventStore,
		WithSnapshotStore[OrderContext](snapshotStore),
		WithSnapshotConfig[OrderContext](SnapshotConfig{
			Strategy: SnapshotByInterval,
			Interval: 1,
		}),
	)

	// Send events with payload
	pi.Send(Event{Type: "PRICE", Payload: 99.99})
	_, _ = pi.Commit(ctx)

	// Context should be updated
	if pi.Context().Total != 99.99 {
		t.Errorf("expected total 99.99, got %f", pi.Context().Total)
	}

	pi.Stop()

	// New interpreter should hydrate with context
	pi2, _ := NewPersistentInterpreter(ctx, "order-001", machine, eventStore,
		WithSnapshotStore[OrderContext](snapshotStore),
	)

	if pi2.Context().Total != 99.99 {
		t.Errorf("expected hydrated total 99.99, got %f", pi2.Context().Total)
	}
	if pi2.Context().OrderID != "ORD-001" {
		t.Errorf("expected OrderID 'ORD-001', got %s", pi2.Context().OrderID)
	}

	pi2.Stop()
}

func TestPersistentInterpreter_HierarchicalHydration(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("nested").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").On("START").Target("working").End().End().
		State("working").On("PAUSE").Target("paused").End().End().
		State("paused").End().
		Done().
		Build()

	eventStore := NewMemoryEventStore()

	pi1, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)
	pi1.Send(Event{Type: "START"}) // idle -> working
	pi1.Send(Event{Type: "PAUSE"}) // working -> paused
	_, _ = pi1.Commit(ctx)
	pi1.Stop()

	// Hydrate and check nested state
	pi2, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)

	if pi2.State().Value != "paused" {
		t.Errorf("expected state 'paused', got %s", pi2.State().Value)
	}
	if !pi2.Matches("active") {
		t.Error("expected to match ancestor 'active'")
	}

	pi2.Stop()
}

func TestPersistentInterpreter_EmptyStream(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	eventStore := NewMemoryEventStore()

	// New interpreter on empty stream
	pi, err := NewPersistentInterpreter(ctx, "new-stream", machine, eventStore)
	if err != nil {
		t.Fatal(err)
	}

	// Should be in initial state
	if pi.State().Value != "idle" {
		t.Errorf("expected state 'idle', got %s", pi.State().Value)
	}

	// Version should be 0
	if pi.Version() != 0 {
		t.Errorf("expected version 0, got %d", pi.Version())
	}

	pi.Stop()
}

func TestPersistentInterpreter_Done(t *testing.T) {
	ctx := context.Background()

	machine, _ := NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").On("FINISH").Target("done").Done().
		State("done").Final().Done().
		Build()

	eventStore := NewMemoryEventStore()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore)

	if pi.Done() {
		t.Error("should not be done initially")
	}

	pi.Send(Event{Type: "FINISH"})

	if !pi.Done() {
		t.Error("should be done after reaching final state")
	}

	pi.Stop()
}

func TestSnapshotByTime(t *testing.T) {
	ctx := context.Background()

	// Use alternating states so events are actually recorded
	machine, _ := NewMachine[struct{}]("test").
		WithInitial("a").
		State("a").On("TICK").Target("b").Done().
		State("b").On("TICK").Target("a").Done().
		Build()

	eventStore := NewMemoryEventStore()
	snapshotStore := NewMemorySnapshotStore[struct{}]()

	pi, _ := NewPersistentInterpreter(ctx, "stream-1", machine, eventStore,
		WithSnapshotStore[struct{}](snapshotStore),
		WithSnapshotConfig[struct{}](SnapshotConfig{
			Strategy:     SnapshotByTime,
			TimeInterval: 10 * time.Millisecond,
		}),
	)

	// First event shouldn't trigger snapshot (time not elapsed)
	pi.Send(Event{Type: "TICK"})
	_, _ = pi.Commit(ctx)

	// Wait for time to elapse
	time.Sleep(15 * time.Millisecond)

	// Second event should trigger snapshot
	pi.Send(Event{Type: "TICK"})
	_, _ = pi.Commit(ctx)

	// Check snapshot exists
	snap, _, _ := snapshotStore.LoadSnapshot(ctx, "stream-1", 100)
	if snap == nil {
		t.Error("expected snapshot after time interval")
	}

	pi.Stop()
}
