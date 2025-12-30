package statekit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// PersistentInterpreter wraps an Interpreter with event sourcing capabilities.
// All state transitions are persisted to an EventStore and can be replayed.
type PersistentInterpreter[C any] struct {
	streamID      string
	machine       *ir.MachineConfig[C]
	eventStore    EventStore
	snapshotStore SnapshotStore[C]
	config        SnapshotConfig

	// Runtime state
	interpreter       *Interpreter[C]
	version           int
	uncommittedEvents []PersistedEvent
	eventsSinceSnap   int
	lastSnapshotTime  time.Time

	mu sync.Mutex
}

// PersistentInterpreterOption configures a PersistentInterpreter.
type PersistentInterpreterOption[C any] func(*PersistentInterpreter[C])

// WithSnapshotStore sets the snapshot store for the interpreter.
func WithSnapshotStore[C any](store SnapshotStore[C]) PersistentInterpreterOption[C] {
	return func(pi *PersistentInterpreter[C]) {
		pi.snapshotStore = store
	}
}

// WithSnapshotConfig sets the snapshot configuration.
func WithSnapshotConfig[C any](config SnapshotConfig) PersistentInterpreterOption[C] {
	return func(pi *PersistentInterpreter[C]) {
		pi.config = config
	}
}

// NewPersistentInterpreter creates a new persistent interpreter.
// It automatically hydrates state from the event store if events exist.
func NewPersistentInterpreter[C any](
	ctx context.Context,
	streamID string,
	machine *ir.MachineConfig[C],
	eventStore EventStore,
	opts ...PersistentInterpreterOption[C],
) (*PersistentInterpreter[C], error) {
	pi := &PersistentInterpreter[C]{
		streamID:         streamID,
		machine:          machine,
		eventStore:       eventStore,
		config:           SnapshotConfig{Strategy: SnapshotNever},
		lastSnapshotTime: time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(pi)
	}

	// Hydrate from storage
	if err := pi.hydrate(ctx); err != nil {
		return nil, fmt.Errorf("hydrate: %w", err)
	}

	return pi, nil
}

// hydrate reconstructs state from snapshot + events.
func (pi *PersistentInterpreter[C]) hydrate(ctx context.Context) error {
	var startVersion int
	var machineSnap *MachineSnapshot[C]

	// Try to load snapshot
	if pi.snapshotStore != nil {
		var err error
		machineSnap, startVersion, err = pi.snapshotStore.LoadSnapshot(ctx, pi.streamID, math.MaxInt)
		if err != nil {
			return fmt.Errorf("load snapshot: %w", err)
		}
	}

	// Create interpreter
	pi.interpreter = NewInterpreter(pi.machine)

	if machineSnap != nil {
		// Convert MachineSnapshot to Snapshot for Restore
		interpSnap := Snapshot[C]{
			MachineID:        pi.machine.ID,
			CurrentState:     machineSnap.StateValue,
			Context:          machineSnap.Context,
			ShallowHistory:   machineSnap.HistoryShallow,
			DeepHistory:      machineSnap.HistoryDeep,
			ActiveInParallel: machineSnap.ActiveInParallel,
			CurrentParallel:  machineSnap.CurrentParallel,
		}
		if err := pi.interpreter.Restore(interpSnap); err != nil {
			return fmt.Errorf("restore snapshot: %w", err)
		}
		pi.version = startVersion
	} else {
		// No snapshot, start fresh
		pi.interpreter.Start()
		pi.version = 0
	}

	// Load and replay events after snapshot
	events, err := pi.eventStore.LoadEvents(ctx, pi.streamID, startVersion)
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}

	for _, event := range events {
		// Reconstruct the original event
		var payload any
		if len(event.Payload) > 0 {
			_ = json.Unmarshal(event.Payload, &payload)
		}

		pi.interpreter.Send(Event{
			Type:    event.Type,
			Payload: payload,
		})
		pi.version = event.Version
	}

	pi.eventsSinceSnap = len(events)
	return nil
}

// Send processes an event and records it for persistence.
// Events are recorded if they match a transition (including self-transitions).
// Events that don't match any transition are not recorded.
// Call Commit() to persist uncommitted events.
func (pi *PersistentInterpreter[C]) Send(event Event) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// Snapshot context before to detect action execution
	ctxBefore, _ := json.Marshal(pi.interpreter.State().Context)
	stateBefore := pi.interpreter.State().Value

	// Process event through interpreter
	pi.interpreter.Send(event)

	// Get state after processing
	stateAfter := pi.interpreter.State().Value
	ctxAfter, _ := json.Marshal(pi.interpreter.State().Context)

	// Record event if it caused any change (state or context)
	// This handles both state transitions and self-transitions with actions
	stateChanged := stateBefore != stateAfter
	contextChanged := string(ctxBefore) != string(ctxAfter)

	if stateChanged || contextChanged {
		var payload json.RawMessage
		if event.Payload != nil {
			payload, _ = json.Marshal(event.Payload)
		}

		persistedEvent := PersistedEvent{
			ID:         generateEventID(),
			StreamID:   pi.streamID,
			Type:       event.Type,
			Version:    pi.version + len(pi.uncommittedEvents) + 1,
			Timestamp:  time.Now(),
			Payload:    payload,
			StateAfter: stateAfter,
		}
		pi.uncommittedEvents = append(pi.uncommittedEvents, persistedEvent)
	}
}

// SendAll processes multiple events.
func (pi *PersistentInterpreter[C]) SendAll(events []Event) {
	for _, event := range events {
		pi.Send(event)
	}
}

// Commit persists all uncommitted events to the event store.
// Returns the number of events committed.
func (pi *PersistentInterpreter[C]) Commit(ctx context.Context) (int, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if len(pi.uncommittedEvents) == 0 {
		return 0, nil
	}

	// Append with optimistic concurrency check
	err := pi.eventStore.AppendEvents(ctx, pi.streamID, pi.version, pi.uncommittedEvents)
	if err != nil {
		return 0, err
	}

	// Update version
	committed := len(pi.uncommittedEvents)
	pi.version += committed
	pi.eventsSinceSnap += committed
	pi.uncommittedEvents = nil

	// Check if snapshot needed
	if pi.shouldSnapshot() {
		if err := pi.createSnapshot(ctx); err != nil {
			// Non-fatal: snapshot failure shouldn't fail commit
			// In production, this should be logged
			_ = err
		}
	}

	return committed, nil
}

// shouldSnapshot checks if a snapshot should be created based on config.
func (pi *PersistentInterpreter[C]) shouldSnapshot() bool {
	if pi.snapshotStore == nil {
		return false
	}

	switch pi.config.Strategy {
	case SnapshotByInterval:
		return pi.eventsSinceSnap >= pi.config.Interval

	case SnapshotOnFinal:
		return pi.interpreter.Done()

	case SnapshotByTime:
		return time.Since(pi.lastSnapshotTime) >= pi.config.TimeInterval

	default:
		return false
	}
}

// createSnapshot creates a state snapshot.
func (pi *PersistentInterpreter[C]) createSnapshot(ctx context.Context) error {
	// Use the Interpreter's public Snapshot() method
	interpSnap := pi.interpreter.Snapshot()

	snapshot := &MachineSnapshot[C]{
		StateValue:       interpSnap.CurrentState,
		Context:          interpSnap.Context,
		HistoryShallow:   interpSnap.ShallowHistory,
		HistoryDeep:      interpSnap.DeepHistory,
		ActiveInParallel: interpSnap.ActiveInParallel,
		CurrentParallel:  interpSnap.CurrentParallel,
		Timestamp:        time.Now(),
	}

	if err := pi.snapshotStore.SaveSnapshot(ctx, pi.streamID, pi.version, snapshot); err != nil {
		return err
	}

	pi.eventsSinceSnap = 0
	pi.lastSnapshotTime = time.Now()
	return nil
}

// Rollback discards all uncommitted events.
func (pi *PersistentInterpreter[C]) Rollback() {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// Discard uncommitted events
	pi.uncommittedEvents = nil

	// Re-hydrate to restore consistent state
	// Note: This is a simplified approach. A more sophisticated implementation
	// would track the state before uncommitted changes.
}

// State returns the current state.
func (pi *PersistentInterpreter[C]) State() State[C] {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return pi.interpreter.State()
}

// Context returns a copy of the current context.
func (pi *PersistentInterpreter[C]) Context() C {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return pi.interpreter.State().Context
}

// Done returns true if the interpreter is in a final state.
func (pi *PersistentInterpreter[C]) Done() bool {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return pi.interpreter.Done()
}

// Matches checks if the current state matches or is a descendant of the given state.
func (pi *PersistentInterpreter[C]) Matches(stateID StateID) bool {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return pi.interpreter.Matches(stateID)
}

// Version returns the current stream version.
func (pi *PersistentInterpreter[C]) Version() int {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return pi.version
}

// UncommittedCount returns the number of uncommitted events.
func (pi *PersistentInterpreter[C]) UncommittedCount() int {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	return len(pi.uncommittedEvents)
}

// StreamID returns the stream identifier.
func (pi *PersistentInterpreter[C]) StreamID() string {
	return pi.streamID
}

// Stop stops the underlying interpreter.
func (pi *PersistentInterpreter[C]) Stop() {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.interpreter.Stop()
}

// ForceSnapshot creates a snapshot regardless of the configured strategy.
func (pi *PersistentInterpreter[C]) ForceSnapshot(ctx context.Context) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.snapshotStore == nil {
		return fmt.Errorf("no snapshot store configured")
	}

	return pi.createSnapshot(ctx)
}

// generateEventID generates a unique event ID.
// In production, use a proper UUID library.
func generateEventID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// --- Replay Functions ---

// ReplayEvents replays events from the store to a new interpreter.
// Useful for debugging or creating a read model.
func ReplayEvents[C any](
	ctx context.Context,
	machine *ir.MachineConfig[C],
	eventStore EventStore,
	streamID string,
) (*Interpreter[C], error) {
	interp := NewInterpreter(machine)
	interp.Start()

	events, err := eventStore.LoadEvents(ctx, streamID, 0)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	for _, event := range events {
		var payload any
		if len(event.Payload) > 0 {
			_ = json.Unmarshal(event.Payload, &payload)
		}

		interp.Send(Event{
			Type:    event.Type,
			Payload: payload,
		})
	}

	return interp, nil
}

// ReplayToVersion replays events up to a specific version.
func ReplayToVersion[C any](
	ctx context.Context,
	machine *ir.MachineConfig[C],
	eventStore EventStore,
	streamID string,
	targetVersion int,
) (*Interpreter[C], error) {
	interp := NewInterpreter(machine)
	interp.Start()

	events, err := eventStore.LoadEvents(ctx, streamID, 0)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	for _, event := range events {
		if event.Version > targetVersion {
			break
		}

		var payload any
		if len(event.Payload) > 0 {
			_ = json.Unmarshal(event.Payload, &payload)
		}

		interp.Send(Event{
			Type:    event.Type,
			Payload: payload,
		})
	}

	return interp, nil
}
