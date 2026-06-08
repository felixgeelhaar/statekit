// Package health provides liveness and readiness probes for statekit state machines.
package health

import (
	"encoding/json"
	"net/http"
	"sync"

	"go.klarlabs.de/statekit"
)

// Status represents the health status of a component.
type Status string

const (
	// StatusHealthy indicates the component is healthy.
	StatusHealthy Status = "healthy"
	// StatusUnhealthy indicates the component is unhealthy.
	StatusUnhealthy Status = "unhealthy"
	// StatusDegraded indicates the component is partially healthy.
	StatusDegraded Status = "degraded"
)

// CheckResult represents the result of a health check.
type CheckResult struct {
	Status  Status            `json:"status"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// Checker performs health checks on state machines.
type Checker[C any] struct {
	mu           sync.RWMutex
	interpreters map[string]*statekit.Interpreter[C]
	readyStates  map[string][]statekit.StateID // States considered "ready" per machine
}

// NewChecker creates a new health checker.
func NewChecker[C any]() *Checker[C] {
	return &Checker[C]{
		interpreters: make(map[string]*statekit.Interpreter[C]),
		readyStates:  make(map[string][]statekit.StateID),
	}
}

// Register adds an interpreter to the health checker.
func (c *Checker[C]) Register(id string, interp *statekit.Interpreter[C]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interpreters[id] = interp
}

// Unregister removes an interpreter from the health checker.
func (c *Checker[C]) Unregister(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.interpreters, id)
	delete(c.readyStates, id)
}

// SetReadyStates configures which states are considered "ready" for a machine.
// If not set, any non-final state is considered ready.
func (c *Checker[C]) SetReadyStates(id string, states ...statekit.StateID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readyStates[id] = states
}

// Liveness checks if all registered interpreters are alive (not nil, initialized).
// Returns healthy if at least one interpreter is registered and all are valid.
func (c *Checker[C]) Liveness() CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.interpreters) == 0 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "no interpreters registered",
		}
	}

	details := make(map[string]string)
	for id, interp := range c.interpreters {
		if interp == nil {
			return CheckResult{
				Status:  StatusUnhealthy,
				Message: "interpreter is nil",
				Details: map[string]string{"machine": id},
			}
		}
		details[id] = "alive"
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: "all interpreters alive",
		Details: details,
	}
}

// Readiness checks if all registered interpreters are ready to process events.
// A machine is ready if it's in a configured ready state, or if no ready states
// are configured, it's in any non-final state.
func (c *Checker[C]) Readiness() CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.interpreters) == 0 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "no interpreters registered",
		}
	}

	details := make(map[string]string)
	allReady := true
	someReady := false

	for id, interp := range c.interpreters {
		if interp == nil {
			details[id] = "nil"
			allReady = false
			continue
		}

		state := interp.State()
		ready := c.isReady(id, interp, state.Value)

		if ready {
			details[id] = string(state.Value)
			someReady = true
		} else {
			if interp.Done() {
				details[id] = string(state.Value) + " (final)"
			} else {
				details[id] = string(state.Value) + " (not ready)"
			}
			allReady = false
		}
	}

	if allReady {
		return CheckResult{
			Status:  StatusHealthy,
			Message: "all interpreters ready",
			Details: details,
		}
	}

	if someReady {
		return CheckResult{
			Status:  StatusDegraded,
			Message: "some interpreters not ready",
			Details: details,
		}
	}

	return CheckResult{
		Status:  StatusUnhealthy,
		Message: "no interpreters ready",
		Details: details,
	}
}

// isReady checks if a machine is in a ready state.
func (c *Checker[C]) isReady(id string, interp *statekit.Interpreter[C], currentState statekit.StateID) bool {
	// If machine is done (final state), it's not ready
	if interp.Done() {
		return false
	}

	// If ready states are configured, check against them
	if states, ok := c.readyStates[id]; ok && len(states) > 0 {
		for _, s := range states {
			if interp.Matches(s) {
				return true
			}
		}
		return false
	}

	// Default: any non-final state is ready
	return true
}

// CheckMachine performs a health check on a specific machine.
func (c *Checker[C]) CheckMachine(id string) CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	interp, ok := c.interpreters[id]
	if !ok {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "machine not found",
			Details: map[string]string{"machine": id},
		}
	}

	if interp == nil {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "interpreter is nil",
			Details: map[string]string{"machine": id},
		}
	}

	state := interp.State()
	ready := c.isReady(id, interp, state.Value)

	details := map[string]string{
		"machine": id,
		"state":   string(state.Value),
		"done":    boolToString(interp.Done()),
		"ready":   boolToString(ready),
	}

	if ready {
		return CheckResult{
			Status:  StatusHealthy,
			Message: "machine is healthy",
			Details: details,
		}
	}

	return CheckResult{
		Status:  StatusUnhealthy,
		Message: "machine is not ready",
		Details: details,
	}
}

// LivenessHandler returns an HTTP handler for liveness probes.
func (c *Checker[C]) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := c.Liveness()
		c.writeResponse(w, result)
	})
}

// ReadinessHandler returns an HTTP handler for readiness probes.
func (c *Checker[C]) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := c.Readiness()
		c.writeResponse(w, result)
	})
}

// HealthHandler returns an HTTP handler that checks both liveness and readiness.
func (c *Checker[C]) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		liveness := c.Liveness()
		readiness := c.Readiness()

		// Combine results
		result := CheckResult{
			Details: map[string]string{
				"liveness":  string(liveness.Status),
				"readiness": string(readiness.Status),
			},
		}

		switch {
		case liveness.Status == StatusHealthy && readiness.Status == StatusHealthy:
			result.Status = StatusHealthy
			result.Message = "all checks passed"
		case liveness.Status == StatusUnhealthy:
			result.Status = StatusUnhealthy
			result.Message = "liveness check failed: " + liveness.Message
		case readiness.Status == StatusUnhealthy:
			result.Status = StatusUnhealthy
			result.Message = "readiness check failed: " + readiness.Message
		default:
			result.Status = StatusDegraded
			result.Message = "some checks degraded"
		}

		c.writeResponse(w, result)
	})
}

// writeResponse writes the health check result as JSON.
func (c *Checker[C]) writeResponse(w http.ResponseWriter, result CheckResult) {
	w.Header().Set("Content-Type", "application/json")

	switch result.Status {
	case StatusHealthy:
		w.WriteHeader(http.StatusOK)
	case StatusDegraded:
		w.WriteHeader(http.StatusOK) // 200 for degraded, still serving
	case StatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(result)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// MachineCount returns the number of registered interpreters.
func (c *Checker[C]) MachineCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.interpreters)
}

// MachineIDs returns the IDs of all registered interpreters.
func (c *Checker[C]) MachineIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.interpreters))
	for id := range c.interpreters {
		ids = append(ids, id)
	}
	return ids
}
