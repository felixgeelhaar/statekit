// Package http provides HTTP middleware and handlers for statekit state machines.
// It offers framework-agnostic utilities as well as adapters for popular Go web frameworks.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/felixgeelhaar/statekit"
)

// --- Framework-agnostic HTTP Handlers ---

// MachineHandler provides HTTP handlers for interacting with a state machine.
type MachineHandler[C any] struct {
	interpreter *statekit.Interpreter[C]
	mu          sync.RWMutex
}

// NewMachineHandler creates a new HTTP handler for a state machine.
func NewMachineHandler[C any](interp *statekit.Interpreter[C]) *MachineHandler[C] {
	return &MachineHandler[C]{
		interpreter: interp,
	}
}

// StateResponse represents the state machine state in JSON.
type StateResponse struct {
	CurrentState string `json:"currentState"`
	Done         bool   `json:"done"`
	MachineID    string `json:"machineId,omitempty"`
}

// EventRequest represents an incoming event.
type EventRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// EventResponse represents the response after sending an event.
type EventResponse struct {
	PreviousState string `json:"previousState"`
	CurrentState  string `json:"currentState"`
	Transitioned  bool   `json:"transitioned"`
	Done          bool   `json:"done"`
}

// HandleGetState returns the current state.
func (h *MachineHandler[C]) HandleGetState(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	state := h.interpreter.State()
	done := h.interpreter.Done()
	h.mu.RUnlock()

	resp := StateResponse{
		CurrentState: string(state.Value),
		Done:         done,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSendEvent processes an event and returns the new state.
func (h *MachineHandler[C]) HandleSendEvent(w http.ResponseWriter, r *http.Request) {
	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, `{"error": "event type is required"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	prevState := h.interpreter.State().Value

	event := statekit.Event{
		Type:    statekit.EventType(req.Type),
		Payload: req.Payload,
	}
	h.interpreter.Send(event)

	newState := h.interpreter.State().Value
	done := h.interpreter.Done()
	h.mu.Unlock()

	resp := EventResponse{
		PreviousState: string(prevState),
		CurrentState:  string(newState),
		Transitioned:  prevState != newState,
		Done:          done,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleGetContext returns the current context (as JSON).
func (h *MachineHandler[C]) HandleGetContext(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	ctx := h.interpreter.State().Context
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}

// ServeHTTP implements http.Handler for basic routing.
func (h *MachineHandler[C]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/state":
		h.HandleGetState(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/event":
		h.HandleSendEvent(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/context":
		h.HandleGetContext(w, r)
	default:
		http.NotFound(w, r)
	}
}

// --- Machine Registry for Multiple Machines ---

// MachineRegistry manages multiple state machine instances.
type MachineRegistry[C any] struct {
	machines map[string]*statekit.Interpreter[C]
	factory  func(id string) (*statekit.Interpreter[C], error)
	mu       sync.RWMutex
}

// NewMachineRegistry creates a new registry with a factory function.
func NewMachineRegistry[C any](factory func(id string) (*statekit.Interpreter[C], error)) *MachineRegistry[C] {
	return &MachineRegistry[C]{
		machines: make(map[string]*statekit.Interpreter[C]),
		factory:  factory,
	}
}

// Get retrieves or creates an interpreter for the given ID.
func (r *MachineRegistry[C]) Get(id string) (*statekit.Interpreter[C], error) {
	r.mu.RLock()
	if interp, ok := r.machines[id]; ok {
		r.mu.RUnlock()
		return interp, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if interp, ok := r.machines[id]; ok {
		return interp, nil
	}

	interp, err := r.factory(id)
	if err != nil {
		return nil, err
	}
	r.machines[id] = interp
	return interp, nil
}

// Remove stops and removes an interpreter.
func (r *MachineRegistry[C]) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if interp, ok := r.machines[id]; ok {
		interp.Stop()
		delete(r.machines, id)
	}
}

// List returns all registered machine IDs.
func (r *MachineRegistry[C]) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.machines))
	for id := range r.machines {
		ids = append(ids, id)
	}
	return ids
}

// --- Context Key for Request Context ---

type contextKey string

const (
	// MachineContextKey is the key for storing the interpreter in request context.
	MachineContextKey contextKey = "statekit.machine"
)

// WithMachine adds an interpreter to the request context.
func WithMachine[C any](ctx context.Context, interp *statekit.Interpreter[C]) context.Context {
	return context.WithValue(ctx, MachineContextKey, interp)
}

// MachineFromContext retrieves an interpreter from the request context.
func MachineFromContext[C any](ctx context.Context) (*statekit.Interpreter[C], bool) {
	interp, ok := ctx.Value(MachineContextKey).(*statekit.Interpreter[C])
	return interp, ok
}

// --- Standard Library Middleware ---

// Middleware is a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// MachineMiddleware returns middleware that injects a machine into the request context.
func MachineMiddleware[C any](interp *statekit.Interpreter[C]) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithMachine(r.Context(), interp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RegistryMiddleware returns middleware that looks up a machine by ID from the request.
func RegistryMiddleware[C any](registry *MachineRegistry[C], idExtractor func(*http.Request) string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := idExtractor(r)
			if id == "" {
				http.Error(w, `{"error": "machine ID not found"}`, http.StatusBadRequest)
				return
			}

			interp, err := registry.Get(id)
			if err != nil {
				http.Error(w, `{"error": "failed to get machine"}`, http.StatusInternalServerError)
				return
			}

			ctx := WithMachine(r.Context(), interp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// --- Helper: Create Standard Mux ---

// NewServeMux creates a standard library mux with state machine routes.
func NewServeMux[C any](interp *statekit.Interpreter[C], prefix string) *http.ServeMux {
	handler := NewMachineHandler(interp)
	mux := http.NewServeMux()

	mux.HandleFunc(prefix+"/state", handler.HandleGetState)
	mux.HandleFunc(prefix+"/event", handler.HandleSendEvent)
	mux.HandleFunc(prefix+"/context", handler.HandleGetContext)

	return mux
}
