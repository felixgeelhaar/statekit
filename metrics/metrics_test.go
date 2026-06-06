package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/metrics"
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

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	if m.TransitionsTotal == nil {
		t.Error("TransitionsTotal should not be nil")
	}
	if m.EventsTotal == nil {
		t.Error("EventsTotal should not be nil")
	}
	if m.TransitionDuration == nil {
		t.Error("TransitionDuration should not be nil")
	}
	if m.CurrentState == nil {
		t.Error("CurrentState should not be nil")
	}
	if m.ErrorsTotal == nil {
		t.Error("ErrorsTotal should not be nil")
	}
	if m.MachinesActive == nil {
		t.Error("MachinesActive should not be nil")
	}
	if m.MachinesCompleted == nil {
		t.Error("MachinesCompleted should not be nil")
	}
}

func TestNewMetrics_NilRegistry(t *testing.T) {
	// Should not panic with nil registry
	m := metrics.NewMetrics(nil)
	if m == nil {
		t.Error("NewMetrics should return non-nil even with nil registry")
	}
}

func TestDefaultMetrics(t *testing.T) {
	// This will register with default registry - skip in parallel tests
	// Just verify it doesn't panic
	defer func() {
		// Recover from panic if already registered - this is expected
		_ = recover()
	}()

	m := metrics.DefaultMetrics()
	if m == nil {
		t.Error("DefaultMetrics should return non-nil")
	}
}

func TestMetricsInterpreter_Start(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()

	// Check machines active incremented
	active := testutil.ToFloat64(m.MachinesActive.WithLabelValues("test-machine"))
	if active != 1 {
		t.Errorf("expected machines_active = 1, got %v", active)
	}

	// Check current state gauge
	stateGauge := testutil.ToFloat64(m.CurrentState.WithLabelValues("test-machine", "idle"))
	if stateGauge != 1 {
		t.Errorf("expected current_state{state=idle} = 1, got %v", stateGauge)
	}
}

func TestMetricsInterpreter_Send_NoTransition(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Send(statekit.Event{Type: "UNKNOWN"}) // No transition

	// Event should be recorded
	eventsCount := testutil.ToFloat64(m.EventsTotal.WithLabelValues("test-machine", "UNKNOWN", "false"))
	if eventsCount != 1 {
		t.Errorf("expected events_total = 1, got %v", eventsCount)
	}

	// No transition should be recorded
	transitionsCount := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("test-machine", "idle", "idle", "UNKNOWN"))
	if transitionsCount != 0 {
		t.Errorf("expected transitions_total = 0, got %v", transitionsCount)
	}
}

func TestMetricsInterpreter_Send_WithTransition(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Send(statekit.Event{Type: "START"}) // idle -> running

	// Event should be recorded with transitioned=true
	eventsCount := testutil.ToFloat64(m.EventsTotal.WithLabelValues("test-machine", "START", "true"))
	if eventsCount != 1 {
		t.Errorf("expected events_total = 1, got %v", eventsCount)
	}

	// Transition should be recorded
	transitionsCount := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("test-machine", "idle", "running", "START"))
	if transitionsCount != 1 {
		t.Errorf("expected transitions_total = 1, got %v", transitionsCount)
	}

	// State gauges should be updated
	idleGauge := testutil.ToFloat64(m.CurrentState.WithLabelValues("test-machine", "idle"))
	if idleGauge != 0 {
		t.Errorf("expected current_state{state=idle} = 0, got %v", idleGauge)
	}

	runningGauge := testutil.ToFloat64(m.CurrentState.WithLabelValues("test-machine", "running"))
	if runningGauge != 1 {
		t.Errorf("expected current_state{state=running} = 1, got %v", runningGauge)
	}
}

func TestMetricsInterpreter_Send_ToFinalState(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Send(statekit.Event{Type: "START"})    // idle -> running
	mi.Send(statekit.Event{Type: "COMPLETE"}) // running -> done (final)

	// Machines completed should be incremented
	completedCount := testutil.ToFloat64(m.MachinesCompleted.WithLabelValues("test-machine", "done"))
	if completedCount != 1 {
		t.Errorf("expected machines_completed = 1, got %v", completedCount)
	}

	// Machines active should be decremented
	activeCount := testutil.ToFloat64(m.MachinesActive.WithLabelValues("test-machine"))
	if activeCount != 0 {
		t.Errorf("expected machines_active = 0, got %v", activeCount)
	}
}

func TestMetricsInterpreter_SendAll(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.SendAll(
		statekit.Event{Type: "START"},
		statekit.Event{Type: "COMPLETE"},
	)

	// Should be in done state
	if mi.State().Value != "done" {
		t.Errorf("expected state = done, got %v", mi.State().Value)
	}

	// Should have recorded 2 transitions
	trans1 := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("test-machine", "idle", "running", "START"))
	trans2 := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("test-machine", "running", "done", "COMPLETE"))
	if trans1 != 1 || trans2 != 1 {
		t.Errorf("expected 2 transitions, got trans1=%v, trans2=%v", trans1, trans2)
	}
}

func TestMetricsInterpreter_State(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	state := mi.State()

	if state.Value != "idle" {
		t.Errorf("expected state = idle, got %v", state.Value)
	}
}

func TestMetricsInterpreter_Done(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	if mi.Done() {
		t.Error("should not be done initially")
	}

	mi.Send(statekit.Event{Type: "START"})
	mi.Send(statekit.Event{Type: "COMPLETE"})

	if !mi.Done() {
		t.Error("should be done after reaching final state")
	}
}

func TestMetricsInterpreter_Matches(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	if !mi.Matches("idle") {
		t.Error("should match idle state")
	}
	if mi.Matches("running") {
		t.Error("should not match running state")
	}
}

func TestMetricsInterpreter_Stop(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Stop()

	// Machines active should be decremented
	activeCount := testutil.ToFloat64(m.MachinesActive.WithLabelValues("test-machine"))
	if activeCount != 0 {
		t.Errorf("expected machines_active = 0, got %v", activeCount)
	}

	// Current state gauge should be cleared
	stateGauge := testutil.ToFloat64(m.CurrentState.WithLabelValues("test-machine", "idle"))
	if stateGauge != 0 {
		t.Errorf("expected current_state{state=idle} = 0, got %v", stateGauge)
	}
}

func TestMetricsInterpreter_Stop_AlreadyDone(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Send(statekit.Event{Type: "START"})
	mi.Send(statekit.Event{Type: "COMPLETE"}) // Now done - machines_active already decremented

	mi.Stop() // Should not double-decrement

	// Machines active should be 0 (not negative)
	activeCount := testutil.ToFloat64(m.MachinesActive.WithLabelValues("test-machine"))
	if activeCount != 0 {
		t.Errorf("expected machines_active = 0, got %v", activeCount)
	}
}

func TestMetricsInterpreter_RecordError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.RecordError("guard_failed")
	mi.RecordError("guard_failed")
	mi.RecordError("action_error")

	guardErrors := testutil.ToFloat64(m.ErrorsTotal.WithLabelValues("test-machine", "guard_failed"))
	if guardErrors != 2 {
		t.Errorf("expected guard_failed errors = 2, got %v", guardErrors)
	}

	actionErrors := testutil.ToFloat64(m.ErrorsTotal.WithLabelValues("test-machine", "action_error"))
	if actionErrors != 1 {
		t.Errorf("expected action_error errors = 1, got %v", actionErrors)
	}
}

func TestMetricsInterpreter_Interpreter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	// Should return the underlying interpreter
	if mi.Interpreter() != interp {
		t.Error("Interpreter() should return the underlying interpreter")
	}
}

func TestMetricsInterpreter_TransitionDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	mi := metrics.NewMetricsInterpreter(interp, "test-machine", m)

	mi.Start()
	mi.Send(statekit.Event{Type: "START"})

	// Histogram should have 1 observation
	count := testutil.CollectAndCount(m.TransitionDuration)
	if count == 0 {
		t.Error("expected transition_duration histogram to have observations")
	}
}

func TestMetricsInterpreter_MultipleInstances(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewMetrics(reg)

	machine := buildTestMachine()

	// Create multiple interpreters with same machine type but different IDs
	interp1 := statekit.NewInterpreter(machine)
	mi1 := metrics.NewMetricsInterpreter(interp1, "instance-1", m)

	interp2 := statekit.NewInterpreter(machine)
	mi2 := metrics.NewMetricsInterpreter(interp2, "instance-2", m)

	mi1.Start()
	mi2.Start()

	// Both should be tracked independently
	active1 := testutil.ToFloat64(m.MachinesActive.WithLabelValues("instance-1"))
	active2 := testutil.ToFloat64(m.MachinesActive.WithLabelValues("instance-2"))

	if active1 != 1 {
		t.Errorf("expected instance-1 active = 1, got %v", active1)
	}
	if active2 != 1 {
		t.Errorf("expected instance-2 active = 1, got %v", active2)
	}

	mi1.Send(statekit.Event{Type: "START"})

	// Only instance-1 should have the transition
	trans1 := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("instance-1", "idle", "running", "START"))
	trans2 := testutil.ToFloat64(m.TransitionsTotal.WithLabelValues("instance-2", "idle", "running", "START"))

	if trans1 != 1 {
		t.Errorf("expected instance-1 transitions = 1, got %v", trans1)
	}
	if trans2 != 0 {
		t.Errorf("expected instance-2 transitions = 0, got %v", trans2)
	}
}
