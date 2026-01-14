package statekit

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/felixgeelhaar/statekit/internal/ir"
)

// ErrActorNotFound is returned when an actor ID doesn't exist in the registry
var ErrActorNotFound = errors.New("actor not found")

// ErrActorAlreadyExists is returned when spawning an actor with a duplicate ID
var ErrActorAlreadyExists = errors.New("actor already exists")

// ErrNoParent is returned when SendParent is called on a root interpreter
var ErrNoParent = errors.New("no parent actor")

// ErrActorStopped is returned when sending to a stopped actor
var ErrActorStopped = errors.New("actor stopped")

// ActorRef provides a handle to communicate with a spawned child actor.
// It is safe to use concurrently from multiple goroutines.
type ActorRef struct {
	id        ActorID
	eventChan chan Event
	done      chan struct{}
	cancel    context.CancelFunc
	once      sync.Once // Ensures Stop is only executed once
}

// ID returns the actor's unique identifier
func (a *ActorRef) ID() ActorID {
	return a.id
}

// Send sends an event to the child actor.
// Returns ErrActorStopped if the actor has been stopped.
func (a *ActorRef) Send(e Event) error {
	// First check if already stopped (non-blocking)
	select {
	case <-a.done:
		return ErrActorStopped
	default:
	}
	// Try to send, but also watch for stop
	select {
	case <-a.done:
		return ErrActorStopped
	case a.eventChan <- e:
		return nil
	}
}

// Stop gracefully stops the child actor.
// Safe to call multiple times.
func (a *ActorRef) Stop() {
	a.once.Do(func() {
		a.cancel()
		close(a.done)
	})
}

// Done returns a channel that's closed when the actor stops.
// Use this to wait for actor completion.
func (a *ActorRef) Done() <-chan struct{} {
	return a.done
}

// actorEntry holds runtime information about a spawned actor
type actorEntry struct {
	ref         *ActorRef
	stateID     StateID // State that owns this actor (for state-scoped cleanup)
	supervision SupervisionStrategy
	autoForward map[EventType]bool
	onDone      *ir.TransitionConfig
	onError     *ir.TransitionConfig
}

// SpawnOption configures actor spawning behavior
type SpawnOption func(*spawnOptions)

type spawnOptions struct {
	supervision SupervisionStrategy
	autoForward []EventType
	onDone      StateID
	onError     StateID
}

// WithSupervision sets the supervision strategy for the spawned actor
func WithSupervision(s SupervisionStrategy) SpawnOption {
	return func(o *spawnOptions) {
		o.supervision = s
	}
}

// WithAutoForward configures events to automatically forward to the child
func WithAutoForward(events ...EventType) SpawnOption {
	return func(o *spawnOptions) {
		o.autoForward = events
	}
}

// WithOnDone sets the target state when the child reaches a final state
func WithOnDone(target StateID) SpawnOption {
	return func(o *spawnOptions) {
		o.onDone = target
	}
}

// WithOnError sets the target state when the child encounters an error
func WithOnError(target StateID) SpawnOption {
	return func(o *spawnOptions) {
		o.onError = target
	}
}

// runChildActor runs a child interpreter in a goroutine, processing events
// and communicating completion/errors back to the parent
func runChildActor[ParentC, ChildC any](
	parentInterp *Interpreter[ParentC],
	childInterp *Interpreter[ChildC],
	entry *actorEntry,
	ctx context.Context,
) {
	// Start the child interpreter
	childInterp.Start()

	// Process events until stopped or context cancelled
	for {
		select {
		case <-ctx.Done():
			// Parent stopped us
			childInterp.Stop()
			return

		case event, ok := <-entry.ref.eventChan:
			if !ok {
				// Channel closed
				childInterp.Stop()
				return
			}

			// Send event to child
			childInterp.Send(event)

			// Check if child is done (reached final state)
			if childInterp.Done() {
				// Notify parent of completion
				parentInterp.handleActorDone(entry)
				return
			}
		}
	}
}

// handleActorDone is called when a child actor reaches a final state
func (i *Interpreter[C]) handleActorDone(entry *actorEntry) {
	// Clean up actor registry under actorMu
	i.actorMu.Lock()
	delete(i.actorRegistry, entry.ref.id)

	// Remove from state tracking
	if actors, ok := i.actorsByState[entry.stateID]; ok {
		for idx, id := range actors {
			if id == entry.ref.id {
				i.actorsByState[entry.stateID] = append(actors[:idx], actors[idx+1:]...)
				break
			}
		}
	}
	i.actorMu.Unlock()

	// Check started and execute transition under main mutex
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.started {
		return
	}

	// Send done event to parent
	doneEvent := Event{
		Type: EventType(fmt.Sprintf("statekit.done.actor.%s", entry.ref.id)),
	}

	// Execute OnDone transition if configured
	if entry.onDone != nil && entry.onDone.Target != "" {
		currentState := i.machine.GetState(i.state.Value)
		if currentState != nil {
			source := &transitionSource[C]{
				state:      currentState,
				transition: entry.onDone,
			}
			i.executeTransitionHierarchical(source, doneEvent)
		}
	}
}

// stopActorsForState stops all actors spawned in the given state.
// This implements state-scoped actor lifecycle.
// This function acquires actorMu internally.
func (i *Interpreter[C]) stopActorsForState(stateID StateID) {
	i.actorMu.Lock()
	defer i.actorMu.Unlock()

	actorIDs, ok := i.actorsByState[stateID]
	if !ok {
		return
	}

	for _, id := range actorIDs {
		if entry, exists := i.actorRegistry[id]; exists {
			entry.ref.Stop()
			delete(i.actorRegistry, id)
		}
	}
	delete(i.actorsByState, stateID)
}

// GetActor returns the ActorRef for the given ID, or nil if not found
func (i *Interpreter[C]) GetActor(id ActorID) *ActorRef {
	i.actorMu.Lock()
	defer i.actorMu.Unlock()

	if entry, ok := i.actorRegistry[id]; ok {
		return entry.ref
	}
	return nil
}

// SendTo sends an event to a specific child actor by ID
func (i *Interpreter[C]) SendTo(id ActorID, event Event) error {
	i.actorMu.Lock()
	entry, ok := i.actorRegistry[id]
	i.actorMu.Unlock()

	if !ok {
		return ErrActorNotFound
	}
	return entry.ref.Send(event)
}

// SendParent sends an event to the parent interpreter.
// Returns ErrNoParent if this is a root interpreter (no parent).
func (i *Interpreter[C]) SendParent(event Event) error {
	i.actorMu.Lock()
	parentSend := i.parentSend
	i.actorMu.Unlock()

	if parentSend == nil {
		return ErrNoParent
	}
	return parentSend(event)
}

// broadcastToAutoForward sends an event to all actors configured to auto-forward this event type
func (i *Interpreter[C]) broadcastToAutoForward(event Event) {
	i.actorMu.Lock()
	defer i.actorMu.Unlock()

	for _, entry := range i.actorRegistry {
		if entry.autoForward[event.Type] {
			// Non-blocking send to avoid deadlock
			select {
			case entry.ref.eventChan <- event:
			default:
				// Buffer full, skip
			}
		}
	}
}
