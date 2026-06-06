package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/health"
)

func buildTestMachine() *statekit.MachineConfig[struct{}] {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").
		On("STOP").Target("idle").
		On("COMPLETE").Target("done").
		Done().
		State("done").Final().
		Done().
		Build()
	return machine
}

func TestNewChecker(t *testing.T) {
	c := health.NewChecker[struct{}]()
	if c == nil {
		t.Error("NewChecker should return non-nil")
	}
	if c.MachineCount() != 0 {
		t.Errorf("expected 0 machines, got %d", c.MachineCount())
	}
}

func TestChecker_Register(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)

	if c.MachineCount() != 1 {
		t.Errorf("expected 1 machine, got %d", c.MachineCount())
	}

	ids := c.MachineIDs()
	if len(ids) != 1 || ids[0] != "test-1" {
		t.Errorf("expected [test-1], got %v", ids)
	}
}

func TestChecker_Unregister(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	c.Unregister("test-1")

	if c.MachineCount() != 0 {
		t.Errorf("expected 0 machines, got %d", c.MachineCount())
	}
}

func TestChecker_Liveness_NoInterpreters(t *testing.T) {
	c := health.NewChecker[struct{}]()
	result := c.Liveness()

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Message != "no interpreters registered" {
		t.Errorf("unexpected message: %v", result.Message)
	}
}

func TestChecker_Liveness_Healthy(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	result := c.Liveness()

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
	if result.Details["test-1"] != "alive" {
		t.Errorf("expected alive, got %v", result.Details["test-1"])
	}
}

func TestChecker_Liveness_NilInterpreter(t *testing.T) {
	c := health.NewChecker[struct{}]()
	c.Register("test-1", nil)

	result := c.Liveness()

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Message != "interpreter is nil" {
		t.Errorf("unexpected message: %v", result.Message)
	}
}

func TestChecker_Readiness_NoInterpreters(t *testing.T) {
	c := health.NewChecker[struct{}]()
	result := c.Readiness()

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
}

func TestChecker_Readiness_Healthy(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	result := c.Readiness()

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
}

func TestChecker_Readiness_FinalState(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	interp.Send(statekit.Event{Type: "START"})
	interp.Send(statekit.Event{Type: "COMPLETE"}) // Final state

	c.Register("test-1", interp)
	result := c.Readiness()

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy for final state, got %v", result.Status)
	}
}

func TestChecker_Readiness_Degraded(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()

	// One ready interpreter
	interp1 := statekit.NewInterpreter(machine)
	interp1.Start()
	c.Register("test-1", interp1)

	// One in final state (not ready)
	interp2 := statekit.NewInterpreter(machine)
	interp2.Start()
	interp2.Send(statekit.Event{Type: "START"})
	interp2.Send(statekit.Event{Type: "COMPLETE"})
	c.Register("test-2", interp2)

	result := c.Readiness()

	if result.Status != health.StatusDegraded {
		t.Errorf("expected degraded, got %v", result.Status)
	}
}

func TestChecker_Readiness_WithReadyStates(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	c.SetReadyStates("test-1", "running") // Only "running" is ready

	// In idle state - not ready
	result := c.Readiness()
	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy when not in ready state, got %v", result.Status)
	}

	// Transition to running - now ready
	interp.Send(statekit.Event{Type: "START"})
	result = c.Readiness()
	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy when in ready state, got %v", result.Status)
	}
}

func TestChecker_CheckMachine_NotFound(t *testing.T) {
	c := health.NewChecker[struct{}]()
	result := c.CheckMachine("nonexistent")

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Message != "machine not found" {
		t.Errorf("unexpected message: %v", result.Message)
	}
}

func TestChecker_CheckMachine_NilInterpreter(t *testing.T) {
	c := health.NewChecker[struct{}]()
	c.Register("test-1", nil)

	result := c.CheckMachine("test-1")

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Message != "interpreter is nil" {
		t.Errorf("unexpected message: %v", result.Message)
	}
}

func TestChecker_CheckMachine_Healthy(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	result := c.CheckMachine("test-1")

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
	if result.Details["state"] != "idle" {
		t.Errorf("expected state=idle, got %v", result.Details["state"])
	}
	if result.Details["ready"] != "true" {
		t.Errorf("expected ready=true, got %v", result.Details["ready"])
	}
}

func TestChecker_CheckMachine_NotReady(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	interp.Send(statekit.Event{Type: "START"})
	interp.Send(statekit.Event{Type: "COMPLETE"}) // Final state

	c.Register("test-1", interp)
	result := c.CheckMachine("test-1")

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Details["done"] != "true" {
		t.Errorf("expected done=true, got %v", result.Details["done"])
	}
}

func TestChecker_LivenessHandler(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	c.Register("test-1", interp)

	handler := c.LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var result health.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
}

func TestChecker_ReadinessHandler(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	c.Register("test-1", interp)

	handler := c.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var result health.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
}

func TestChecker_ReadinessHandler_Unhealthy(t *testing.T) {
	c := health.NewChecker[struct{}]()

	handler := c.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestChecker_HealthHandler_AllHealthy(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	c.Register("test-1", interp)

	handler := c.HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var result health.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Status)
	}
	if result.Details["liveness"] != "healthy" {
		t.Errorf("expected liveness=healthy, got %v", result.Details["liveness"])
	}
	if result.Details["readiness"] != "healthy" {
		t.Errorf("expected readiness=healthy, got %v", result.Details["readiness"])
	}
}

func TestChecker_HealthHandler_LivenessUnhealthy(t *testing.T) {
	c := health.NewChecker[struct{}]()
	// No interpreters = liveness fails

	handler := c.HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}

	var result health.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
}

func TestChecker_HealthHandler_Degraded(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()

	// One ready
	interp1 := statekit.NewInterpreter(machine)
	interp1.Start()
	c.Register("test-1", interp1)

	// One in final state
	interp2 := statekit.NewInterpreter(machine)
	interp2.Start()
	interp2.Send(statekit.Event{Type: "START"})
	interp2.Send(statekit.Event{Type: "COMPLETE"})
	c.Register("test-2", interp2)

	handler := c.HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for degraded, got %d", rec.Code)
	}

	var result health.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result.Status != health.StatusDegraded {
		t.Errorf("expected degraded, got %v", result.Status)
	}
}

func TestChecker_MachineIDs_Order(t *testing.T) {
	c := health.NewChecker[struct{}]()
	machine := buildTestMachine()

	interp1 := statekit.NewInterpreter(machine)
	interp1.Start()
	c.Register("alpha", interp1)

	interp2 := statekit.NewInterpreter(machine)
	interp2.Start()
	c.Register("beta", interp2)

	ids := c.MachineIDs()
	sort.Strings(ids)

	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] != "alpha" || ids[1] != "beta" {
		t.Errorf("unexpected ids: %v", ids)
	}
}

func TestChecker_Readiness_NilInterpreter(t *testing.T) {
	c := health.NewChecker[struct{}]()
	c.Register("test-1", nil)

	result := c.Readiness()

	if result.Status != health.StatusUnhealthy {
		t.Errorf("expected unhealthy, got %v", result.Status)
	}
	if result.Details["test-1"] != "nil" {
		t.Errorf("expected nil, got %v", result.Details["test-1"])
	}
}

func TestChecker_SetReadyStates_MatchesAncestor(t *testing.T) {
	// Build a hierarchical machine
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").
		On("START").Target("working").
		End(). // Returns to idle StateBuilder
		End(). // Returns to active StateBuilder
		State("working").
		On("STOP").Target("idle").
		End().  // Returns to working StateBuilder
		End().  // Returns to active StateBuilder
		Done(). // Returns to MachineBuilder
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("failed to build machine: %v", err)
	}

	c := health.NewChecker[struct{}]()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	c.Register("test-1", interp)
	c.SetReadyStates("test-1", "active") // Parent state

	// In idle (child of active) - should match via Matches
	result := c.Readiness()
	if result.Status != health.StatusHealthy {
		t.Errorf("expected healthy when matching ancestor, got %v", result.Status)
	}
}
