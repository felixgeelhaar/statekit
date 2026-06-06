// Package statekit provides event sourcing support for state machine persistence.
package statekit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
)

// PersistedEvent represents a stored event in the event store.
type PersistedEvent struct {
	// ID is a unique identifier for this event (e.g., UUID)
	ID string `json:"id"`

	// StreamID identifies the aggregate/stream this event belongs to
	StreamID string `json:"stream_id"`

	// Type is the event type that triggered the transition
	Type EventType `json:"type"`

	// Version is the stream version (monotonically increasing per stream)
	Version int `json:"version"`

	// Timestamp when the event was recorded
	Timestamp time.Time `json:"timestamp"`

	// Payload contains serialized event data
	Payload json.RawMessage `json:"payload,omitempty"`

	// Metadata holds additional context (correlation ID, user ID, etc.)
	Metadata map[string]any `json:"metadata,omitempty"`

	// StateAfter is the state after processing this event
	StateAfter ir.StateID `json:"state_after"`
}

// EventStore defines the interface for event persistence.
// Implementations handle storage of events for event sourcing.
type EventStore interface {
	// AppendEvents atomically appends events to a stream.
	// Returns ErrConcurrencyConflict if expectedVersion doesn't match.
	AppendEvents(ctx context.Context, streamID string, expectedVersion int, events []PersistedEvent) error

	// LoadEvents loads all events for a stream from a specific version.
	// Pass fromVersion=0 to load all events.
	LoadEvents(ctx context.Context, streamID string, fromVersion int) ([]PersistedEvent, error)

	// GetStreamVersion returns the current version of a stream.
	// Returns 0 if the stream doesn't exist.
	GetStreamVersion(ctx context.Context, streamID string) (int, error)
}

// SnapshotStore defines the interface for state snapshot persistence.
// Snapshots enable faster state reconstruction by avoiding full event replay.
type SnapshotStore[C any] interface {
	// SaveSnapshot saves a state snapshot at the given version.
	SaveSnapshot(ctx context.Context, streamID string, version int, snapshot *MachineSnapshot[C]) error

	// LoadSnapshot loads the latest snapshot at or before maxVersion.
	// Returns nil if no snapshot exists.
	LoadSnapshot(ctx context.Context, streamID string, maxVersion int) (*MachineSnapshot[C], int, error)
}

// MachineSnapshot captures the complete state of an interpreter for persistence.
type MachineSnapshot[C any] struct {
	// Current state value
	StateValue ir.StateID `json:"state_value"`

	// Full state path for hierarchical states
	StatePath []ir.StateID `json:"state_path,omitempty"`

	// Machine context
	Context C `json:"context"`

	// History memory for shallow history states
	HistoryShallow map[ir.StateID]ir.StateID `json:"history_shallow,omitempty"`

	// History memory for deep history states
	HistoryDeep map[ir.StateID]ir.StateID `json:"history_deep,omitempty"`

	// Parallel region states
	ActiveInParallel map[ir.StateID]ir.StateID `json:"active_in_parallel,omitempty"`

	// Current parallel state (if any)
	CurrentParallel ir.StateID `json:"current_parallel,omitempty"`

	// Timestamp when snapshot was taken
	Timestamp time.Time `json:"timestamp"`
}

// SnapshotStrategy determines when snapshots are created.
type SnapshotStrategy int

const (
	// SnapshotNever disables automatic snapshots
	SnapshotNever SnapshotStrategy = iota

	// SnapshotByInterval creates snapshots every N events
	SnapshotByInterval

	// SnapshotOnFinal creates a snapshot when reaching a final state
	SnapshotOnFinal

	// SnapshotByTime creates snapshots after a time interval
	SnapshotByTime
)

// SnapshotConfig configures snapshot behavior.
type SnapshotConfig struct {
	// Strategy determines when to create snapshots
	Strategy SnapshotStrategy

	// Interval is the number of events between snapshots (for SnapshotByInterval)
	Interval int

	// TimeInterval is the duration between snapshots (for SnapshotByTime)
	TimeInterval time.Duration
}

// ErrConcurrencyConflict is returned when optimistic concurrency check fails.
type ErrConcurrencyConflict struct {
	StreamID        string
	ExpectedVersion int
	ActualVersion   int
}

func (e *ErrConcurrencyConflict) Error() string {
	return "concurrency conflict: expected version " +
		string(rune(e.ExpectedVersion+'0')) + " but got " +
		string(rune(e.ActualVersion+'0'))
}

// --- In-Memory Event Store Implementation ---

// MemoryEventStore is an in-memory implementation of EventStore for testing.
type MemoryEventStore struct {
	streams map[string][]PersistedEvent
	mu      sync.RWMutex
}

// NewMemoryEventStore creates a new in-memory event store.
func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		streams: make(map[string][]PersistedEvent),
	}
}

// AppendEvents atomically appends events to a stream.
func (s *MemoryEventStore) AppendEvents(ctx context.Context, streamID string, expectedVersion int, events []PersistedEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stream := s.streams[streamID]
	currentVersion := len(stream)

	if currentVersion != expectedVersion {
		return &ErrConcurrencyConflict{
			StreamID:        streamID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   currentVersion,
		}
	}

	s.streams[streamID] = append(stream, events...)
	return nil
}

// LoadEvents loads all events for a stream from a specific version.
func (s *MemoryEventStore) LoadEvents(ctx context.Context, streamID string, fromVersion int) ([]PersistedEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream := s.streams[streamID]
	if fromVersion >= len(stream) {
		return nil, nil
	}

	// Return a copy to prevent mutation
	result := make([]PersistedEvent, len(stream)-fromVersion)
	copy(result, stream[fromVersion:])
	return result, nil
}

// GetStreamVersion returns the current version of a stream.
func (s *MemoryEventStore) GetStreamVersion(ctx context.Context, streamID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.streams[streamID]), nil
}

// --- In-Memory Snapshot Store Implementation ---

// MemorySnapshotStore is an in-memory implementation of SnapshotStore for testing.
type MemorySnapshotStore[C any] struct {
	snapshots map[string]map[int]*MachineSnapshot[C]
	mu        sync.RWMutex
}

// NewMemorySnapshotStore creates a new in-memory snapshot store.
func NewMemorySnapshotStore[C any]() *MemorySnapshotStore[C] {
	return &MemorySnapshotStore[C]{
		snapshots: make(map[string]map[int]*MachineSnapshot[C]),
	}
}

// SaveSnapshot saves a state snapshot at the given version.
func (s *MemorySnapshotStore[C]) SaveSnapshot(ctx context.Context, streamID string, version int, snapshot *MachineSnapshot[C]) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.snapshots[streamID] == nil {
		s.snapshots[streamID] = make(map[int]*MachineSnapshot[C])
	}
	s.snapshots[streamID][version] = snapshot
	return nil
}

// LoadSnapshot loads the latest snapshot at or before maxVersion.
func (s *MemorySnapshotStore[C]) LoadSnapshot(ctx context.Context, streamID string, maxVersion int) (*MachineSnapshot[C], int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.snapshots[streamID]
	if versions == nil {
		return nil, 0, nil
	}

	// Find the highest version <= maxVersion
	bestVersion := -1
	for v := range versions {
		if v <= maxVersion && v > bestVersion {
			bestVersion = v
		}
	}

	if bestVersion < 0 {
		return nil, 0, nil
	}

	return versions[bestVersion], bestVersion, nil
}
