package statekit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
)

// --- Distributed Locking Interfaces ---

// StreamLock provides distributed locking for state machine streams.
// Implementations can use Redis, etcd, PostgreSQL advisory locks, etc.
type StreamLock interface {
	// Acquire attempts to acquire a lock for the given stream.
	// Returns a Lock handle if successful, or an error if the lock is held by another node.
	// The lock should automatically expire after ttl if not renewed.
	Acquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error)

	// TryAcquire attempts to acquire a lock without blocking.
	// Returns ErrLockHeld if the lock is already held by another node.
	TryAcquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error)
}

// Lock represents a held distributed lock.
type Lock interface {
	// Renew extends the lock's TTL.
	Renew(ctx context.Context, ttl time.Duration) error

	// Release releases the lock.
	Release(ctx context.Context) error

	// Done returns a channel that's closed when the lock is lost.
	Done() <-chan struct{}
}

// ErrLockHeld is returned when attempting to acquire a lock held by another node.
var ErrLockHeld = errors.New("lock held by another node")

// ErrLockLost is returned when a lock is lost unexpectedly.
var ErrLockLost = errors.New("lock lost")

// --- Distributed Interpreter ---

// DistributedInterpreter wraps a PersistentInterpreter with distributed locking.
// It ensures only one node processes events for a stream at a time.
type DistributedInterpreter[C any] struct {
	streamID      string
	machine       *ir.MachineConfig[C]
	eventStore    EventStore
	snapshotStore SnapshotStore[C]
	snapshotCfg   SnapshotConfig
	streamLock    StreamLock
	lockTTL       time.Duration
	renewInterval time.Duration

	// Runtime state
	persistent *PersistentInterpreter[C]
	lock       Lock
	renewCtx   context.Context
	renewStop  context.CancelFunc
	started    bool

	mu sync.Mutex
}

// DistributedInterpreterOption configures a DistributedInterpreter.
type DistributedInterpreterOption[C any] func(*DistributedInterpreter[C])

// WithDistributedSnapshotStore sets the snapshot store.
func WithDistributedSnapshotStore[C any](store SnapshotStore[C]) DistributedInterpreterOption[C] {
	return func(di *DistributedInterpreter[C]) {
		di.snapshotStore = store
	}
}

// WithDistributedSnapshotConfig sets the snapshot configuration.
func WithDistributedSnapshotConfig[C any](config SnapshotConfig) DistributedInterpreterOption[C] {
	return func(di *DistributedInterpreter[C]) {
		di.snapshotCfg = config
	}
}

// WithLockTTL sets the lock TTL and renewal interval.
// Defaults: TTL=30s, renewInterval=10s (renew at 1/3 of TTL).
func WithLockTTL[C any](ttl time.Duration) DistributedInterpreterOption[C] {
	return func(di *DistributedInterpreter[C]) {
		di.lockTTL = ttl
		di.renewInterval = ttl / 3
	}
}

// NewDistributedInterpreter creates a new distributed interpreter.
// It acquires a lock for the stream before hydrating state.
func NewDistributedInterpreter[C any](
	ctx context.Context,
	streamID string,
	machine *ir.MachineConfig[C],
	eventStore EventStore,
	streamLock StreamLock,
	opts ...DistributedInterpreterOption[C],
) (*DistributedInterpreter[C], error) {
	di := &DistributedInterpreter[C]{
		streamID:      streamID,
		machine:       machine,
		eventStore:    eventStore,
		streamLock:    streamLock,
		snapshotCfg:   SnapshotConfig{Strategy: SnapshotNever},
		lockTTL:       30 * time.Second,
		renewInterval: 10 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(di)
	}

	// Acquire lock
	lock, err := streamLock.Acquire(ctx, streamID, di.lockTTL)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	di.lock = lock

	// Start lock renewal
	di.renewCtx, di.renewStop = context.WithCancel(context.Background())
	go di.renewLoop()

	// Create persistent interpreter (hydrates from store)
	piOpts := []PersistentInterpreterOption[C]{}
	if di.snapshotStore != nil {
		piOpts = append(piOpts, WithSnapshotStore[C](di.snapshotStore))
	}
	piOpts = append(piOpts, WithSnapshotConfig[C](di.snapshotCfg))

	di.persistent, err = NewPersistentInterpreter(ctx, streamID, machine, eventStore, piOpts...)
	if err != nil {
		_ = lock.Release(ctx)
		di.renewStop()
		return nil, fmt.Errorf("create persistent interpreter: %w", err)
	}

	di.started = true
	return di, nil
}

// renewLoop periodically renews the lock.
func (di *DistributedInterpreter[C]) renewLoop() {
	ticker := time.NewTicker(di.renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-di.renewCtx.Done():
			return
		case <-di.lock.Done():
			// Lock lost - stop renewal
			return
		case <-ticker.C:
			if err := di.lock.Renew(di.renewCtx, di.lockTTL); err != nil {
				// Lock renewal failed - in production, log this
				return
			}
		}
	}
}

// Send processes an event if the lock is still held.
func (di *DistributedInterpreter[C]) Send(event Event) error {
	di.mu.Lock()
	defer di.mu.Unlock()

	if !di.started {
		return errors.New("interpreter not started")
	}

	// Check if lock is still held
	select {
	case <-di.lock.Done():
		return ErrLockLost
	default:
	}

	di.persistent.Send(event)
	return nil
}

// SendAll processes multiple events.
func (di *DistributedInterpreter[C]) SendAll(events []Event) error {
	for _, event := range events {
		if err := di.Send(event); err != nil {
			return err
		}
	}
	return nil
}

// Commit persists uncommitted events.
func (di *DistributedInterpreter[C]) Commit(ctx context.Context) (int, error) {
	di.mu.Lock()
	defer di.mu.Unlock()

	if !di.started {
		return 0, errors.New("interpreter not started")
	}

	// Check if lock is still held
	select {
	case <-di.lock.Done():
		return 0, ErrLockLost
	default:
	}

	return di.persistent.Commit(ctx)
}

// State returns the current state.
func (di *DistributedInterpreter[C]) State() State[C] {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.State()
}

// Context returns the current context.
func (di *DistributedInterpreter[C]) Context() C {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.Context()
}

// Done returns true if in a final state.
func (di *DistributedInterpreter[C]) Done() bool {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.Done()
}

// Matches checks if current state matches or is descendant of given state.
func (di *DistributedInterpreter[C]) Matches(stateID StateID) bool {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.Matches(stateID)
}

// Version returns the current stream version.
func (di *DistributedInterpreter[C]) Version() int {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.Version()
}

// UncommittedCount returns the number of uncommitted events.
func (di *DistributedInterpreter[C]) UncommittedCount() int {
	di.mu.Lock()
	defer di.mu.Unlock()
	return di.persistent.UncommittedCount()
}

// StreamID returns the stream identifier.
func (di *DistributedInterpreter[C]) StreamID() string {
	return di.streamID
}

// LockHeld returns true if the lock is still held.
func (di *DistributedInterpreter[C]) LockHeld() bool {
	select {
	case <-di.lock.Done():
		return false
	default:
		return true
	}
}

// LockLost returns a channel that's closed when the lock is lost.
func (di *DistributedInterpreter[C]) LockLost() <-chan struct{} {
	return di.lock.Done()
}

// Stop releases the lock and stops the interpreter.
func (di *DistributedInterpreter[C]) Stop(ctx context.Context) error {
	di.mu.Lock()
	defer di.mu.Unlock()

	if !di.started {
		return nil
	}

	di.started = false
	di.renewStop()
	di.persistent.Stop()

	return di.lock.Release(ctx)
}

// ForceSnapshot creates a snapshot.
func (di *DistributedInterpreter[C]) ForceSnapshot(ctx context.Context) error {
	di.mu.Lock()
	defer di.mu.Unlock()

	select {
	case <-di.lock.Done():
		return ErrLockLost
	default:
	}

	return di.persistent.ForceSnapshot(ctx)
}

// --- In-Memory Stream Lock (for testing) ---

// MemoryStreamLock is an in-memory implementation of StreamLock for testing.
// In production, use Redis, etcd, or PostgreSQL advisory locks.
type MemoryStreamLock struct {
	locks map[string]*memoryLock
	mu    sync.Mutex
}

type memoryLock struct {
	streamID  string
	holder    string
	expiresAt time.Time
	done      chan struct{}
	parent    *MemoryStreamLock
}

// NewMemoryStreamLock creates a new in-memory stream lock.
func NewMemoryStreamLock() *MemoryStreamLock {
	return &MemoryStreamLock{
		locks: make(map[string]*memoryLock),
	}
}

// Acquire blocks until the lock is acquired or context is cancelled.
func (s *MemoryStreamLock) Acquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		lock, err := s.TryAcquire(ctx, streamID, ttl)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrLockHeld) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Retry
		}
	}
}

// TryAcquire attempts to acquire a lock without blocking.
func (s *MemoryStreamLock) TryAcquire(ctx context.Context, streamID string, ttl time.Duration) (Lock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing lock
	if existing, ok := s.locks[streamID]; ok {
		if time.Now().Before(existing.expiresAt) {
			return nil, ErrLockHeld
		}
		// Lock expired, close done channel
		close(existing.done)
		delete(s.locks, streamID)
	}

	// Create new lock
	lock := &memoryLock{
		streamID:  streamID,
		holder:    fmt.Sprintf("node-%d", time.Now().UnixNano()),
		expiresAt: time.Now().Add(ttl),
		done:      make(chan struct{}),
		parent:    s,
	}

	s.locks[streamID] = lock
	return lock, nil
}

// Renew extends the lock's TTL.
func (l *memoryLock) Renew(ctx context.Context, ttl time.Duration) error {
	l.parent.mu.Lock()
	defer l.parent.mu.Unlock()

	current, ok := l.parent.locks[l.streamID]
	if !ok || current != l {
		return ErrLockLost
	}

	l.expiresAt = time.Now().Add(ttl)
	return nil
}

// Release releases the lock.
func (l *memoryLock) Release(ctx context.Context) error {
	l.parent.mu.Lock()
	defer l.parent.mu.Unlock()

	current, ok := l.parent.locks[l.streamID]
	if !ok || current != l {
		return nil // Already released or stolen
	}

	delete(l.parent.locks, l.streamID)
	close(l.done)
	return nil
}

// Done returns a channel that's closed when the lock is lost.
func (l *memoryLock) Done() <-chan struct{} {
	return l.done
}

// --- Cluster Coordination (Optional) ---

// ClusterNode represents a node in the cluster.
type ClusterNode struct {
	ID       string
	Address  string
	JoinedAt time.Time
	LastSeen time.Time
	Metadata map[string]string
}

// ClusterMembership provides cluster membership management.
// Implementations can use etcd, consul, or a custom gossip protocol.
type ClusterMembership interface {
	// Join registers this node with the cluster.
	Join(ctx context.Context, node ClusterNode) error

	// Leave removes this node from the cluster.
	Leave(ctx context.Context) error

	// Members returns all known cluster members.
	Members(ctx context.Context) ([]ClusterNode, error)

	// Watch returns a channel that receives membership changes.
	Watch(ctx context.Context) (<-chan MembershipEvent, error)
}

// MembershipEventType indicates the type of membership change.
type MembershipEventType int

const (
	// MemberJoined indicates a new member joined.
	MemberJoined MembershipEventType = iota
	// MemberLeft indicates a member left gracefully.
	MemberLeft
	// MemberFailed indicates a member failed (no heartbeat).
	MemberFailed
)

// MembershipEvent represents a cluster membership change.
type MembershipEvent struct {
	Type MembershipEventType
	Node ClusterNode
}

// --- Stream Router (for load distribution) ---

// StreamRouter determines which node should handle a given stream.
// This enables consistent hashing for stream distribution.
type StreamRouter interface {
	// RouteStream returns the node ID that should handle the stream.
	RouteStream(streamID string, members []ClusterNode) string

	// IsLocal returns true if this node should handle the stream.
	IsLocal(streamID string, members []ClusterNode, localNodeID string) bool
}

// ConsistentHashRouter routes streams using consistent hashing.
type ConsistentHashRouter struct {
	// Replicas is the number of virtual nodes per physical node.
	Replicas int
}

// NewConsistentHashRouter creates a new consistent hash router.
func NewConsistentHashRouter(replicas int) *ConsistentHashRouter {
	if replicas <= 0 {
		replicas = 100
	}
	return &ConsistentHashRouter{Replicas: replicas}
}

// RouteStream returns the node that should handle the stream using consistent hashing.
func (r *ConsistentHashRouter) RouteStream(streamID string, members []ClusterNode) string {
	if len(members) == 0 {
		return ""
	}

	// Simple consistent hashing using FNV-1a
	hash := fnv1a(streamID)

	// Build ring
	type ringEntry struct {
		hash   uint32
		nodeID string
	}

	ring := make([]ringEntry, 0, len(members)*r.Replicas)
	for _, m := range members {
		for i := 0; i < r.Replicas; i++ {
			vnode := fmt.Sprintf("%s-%d", m.ID, i)
			ring = append(ring, ringEntry{
				hash:   fnv1a(vnode),
				nodeID: m.ID,
			})
		}
	}

	// Sort ring by hash
	for i := 0; i < len(ring)-1; i++ {
		for j := i + 1; j < len(ring); j++ {
			if ring[i].hash > ring[j].hash {
				ring[i], ring[j] = ring[j], ring[i]
			}
		}
	}

	// Find first node with hash >= stream hash
	for _, entry := range ring {
		if entry.hash >= hash {
			return entry.nodeID
		}
	}

	// Wrap around
	return ring[0].nodeID
}

// IsLocal returns true if this node should handle the stream.
func (r *ConsistentHashRouter) IsLocal(streamID string, members []ClusterNode, localNodeID string) bool {
	return r.RouteStream(streamID, members) == localNodeID
}

// fnv1a computes FNV-1a hash.
func fnv1a(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)

	hash := uint32(offset32)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= prime32
	}
	return hash
}
