// Package metrics provides Prometheus metrics integration for statekit state machines.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/felixgeelhaar/statekit"
)

const (
	namespace = "statekit"
)

// Metrics holds all Prometheus metrics for state machine monitoring.
type Metrics struct {
	// TransitionsTotal counts total state transitions
	TransitionsTotal *prometheus.CounterVec

	// EventsTotal counts total events processed
	EventsTotal *prometheus.CounterVec

	// TransitionDuration measures transition processing time
	TransitionDuration *prometheus.HistogramVec

	// CurrentState tracks the current state (gauge)
	CurrentState *prometheus.GaugeVec

	// ErrorsTotal counts errors during event processing
	ErrorsTotal *prometheus.CounterVec

	// MachinesActive tracks number of active machines
	MachinesActive *prometheus.GaugeVec

	// MachinesCompleted counts machines that reached final state
	MachinesCompleted *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance with all collectors.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		TransitionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "transitions_total",
				Help:      "Total number of state transitions",
			},
			[]string{"machine", "from_state", "to_state", "event"},
		),
		EventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "events_total",
				Help:      "Total number of events processed",
			},
			[]string{"machine", "event", "transitioned"},
		),
		TransitionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "transition_duration_seconds",
				Help:      "Duration of state transition processing",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"machine", "from_state", "to_state"},
		),
		CurrentState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "current_state",
				Help:      "Current state of the machine (1 = active in this state)",
			},
			[]string{"machine", "state"},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors during event processing",
			},
			[]string{"machine", "error_type"},
		),
		MachinesActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "machines_active",
				Help:      "Number of active state machines",
			},
			[]string{"machine"},
		),
		MachinesCompleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "machines_completed_total",
				Help:      "Total number of machines that reached a final state",
			},
			[]string{"machine", "final_state"},
		),
	}

	if reg != nil {
		reg.MustRegister(
			m.TransitionsTotal,
			m.EventsTotal,
			m.TransitionDuration,
			m.CurrentState,
			m.ErrorsTotal,
			m.MachinesActive,
			m.MachinesCompleted,
		)
	}

	return m
}

// DefaultMetrics creates metrics registered with the default Prometheus registry.
func DefaultMetrics() *Metrics {
	return NewMetrics(prometheus.DefaultRegisterer)
}

// MetricsInterpreter wraps an Interpreter with Prometheus metrics.
type MetricsInterpreter[C any] struct {
	interpreter *statekit.Interpreter[C]
	metrics     *Metrics
	machineID   string
	started     bool
}

// MetricsOption configures a MetricsInterpreter.
type MetricsOption[C any] func(*MetricsInterpreter[C])

// NewMetricsInterpreter creates a metrics wrapper around an interpreter.
func NewMetricsInterpreter[C any](
	interp *statekit.Interpreter[C],
	machineID string,
	metrics *Metrics,
	opts ...MetricsOption[C],
) *MetricsInterpreter[C] {
	mi := &MetricsInterpreter[C]{
		interpreter: interp,
		machineID:   machineID,
		metrics:     metrics,
	}

	for _, opt := range opts {
		opt(mi)
	}

	return mi
}

// Start starts the interpreter and records the initial state.
func (mi *MetricsInterpreter[C]) Start() {
	mi.interpreter.Start()
	mi.started = true

	state := mi.interpreter.State()
	mi.metrics.MachinesActive.WithLabelValues(mi.machineID).Inc()
	mi.metrics.CurrentState.WithLabelValues(mi.machineID, string(state.Value)).Set(1)
}

// Send processes an event and records metrics.
func (mi *MetricsInterpreter[C]) Send(event statekit.Event) {
	stateBefore := mi.interpreter.State().Value
	start := time.Now()

	mi.interpreter.Send(event)

	duration := time.Since(start)
	stateAfter := mi.interpreter.State().Value
	transitioned := stateBefore != stateAfter

	// Record event
	transitionedLabel := "false"
	if transitioned {
		transitionedLabel = "true"
	}
	mi.metrics.EventsTotal.WithLabelValues(
		mi.machineID,
		string(event.Type),
		transitionedLabel,
	).Inc()

	// Record transition if state changed
	if transitioned {
		mi.metrics.TransitionsTotal.WithLabelValues(
			mi.machineID,
			string(stateBefore),
			string(stateAfter),
			string(event.Type),
		).Inc()

		mi.metrics.TransitionDuration.WithLabelValues(
			mi.machineID,
			string(stateBefore),
			string(stateAfter),
		).Observe(duration.Seconds())

		// Update current state gauge
		mi.metrics.CurrentState.WithLabelValues(mi.machineID, string(stateBefore)).Set(0)
		mi.metrics.CurrentState.WithLabelValues(mi.machineID, string(stateAfter)).Set(1)
	}

	// Check if machine completed
	if mi.interpreter.Done() {
		mi.metrics.MachinesCompleted.WithLabelValues(
			mi.machineID,
			string(stateAfter),
		).Inc()
		mi.metrics.MachinesActive.WithLabelValues(mi.machineID).Dec()
	}
}

// SendAll processes multiple events.
func (mi *MetricsInterpreter[C]) SendAll(events ...statekit.Event) {
	for _, event := range events {
		mi.Send(event)
	}
}

// State returns the current state.
func (mi *MetricsInterpreter[C]) State() statekit.State[C] {
	return mi.interpreter.State()
}

// Done returns true if in a final state.
func (mi *MetricsInterpreter[C]) Done() bool {
	return mi.interpreter.Done()
}

// Matches checks if current state matches or is descendant of given state.
func (mi *MetricsInterpreter[C]) Matches(stateID statekit.StateID) bool {
	return mi.interpreter.Matches(stateID)
}

// Stop stops the interpreter and updates metrics.
func (mi *MetricsInterpreter[C]) Stop() {
	if mi.started && !mi.interpreter.Done() {
		mi.metrics.MachinesActive.WithLabelValues(mi.machineID).Dec()
	}

	state := mi.interpreter.State()
	mi.metrics.CurrentState.WithLabelValues(mi.machineID, string(state.Value)).Set(0)

	mi.interpreter.Stop()
	mi.started = false
}

// Interpreter returns the underlying interpreter.
func (mi *MetricsInterpreter[C]) Interpreter() *statekit.Interpreter[C] {
	return mi.interpreter
}

// RecordError records an error metric.
func (mi *MetricsInterpreter[C]) RecordError(errorType string) {
	mi.metrics.ErrorsTotal.WithLabelValues(mi.machineID, errorType).Inc()
}
