// Package otel provides OpenTelemetry tracing integration for statekit state machines.
package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.klarlabs.de/statekit"
)

const (
	// TracerName is the name of the tracer used by this package.
	TracerName = "go.klarlabs.de/statekit/otel"
)

// TracingInterpreter wraps an Interpreter with OpenTelemetry tracing.
type TracingInterpreter[C any] struct {
	interpreter *statekit.Interpreter[C]
	tracer      trace.Tracer
	machineID   string
	spanCtx     context.Context
	rootSpan    trace.Span
}

// TracingOption configures a TracingInterpreter.
type TracingOption[C any] func(*TracingInterpreter[C])

// WithTracer sets a custom tracer.
func WithTracer[C any](tracer trace.Tracer) TracingOption[C] {
	return func(ti *TracingInterpreter[C]) {
		ti.tracer = tracer
	}
}

// NewTracingInterpreter creates a tracing wrapper around an interpreter.
func NewTracingInterpreter[C any](
	interp *statekit.Interpreter[C],
	machineID string,
	opts ...TracingOption[C],
) *TracingInterpreter[C] {
	ti := &TracingInterpreter[C]{
		interpreter: interp,
		machineID:   machineID,
		tracer:      otel.Tracer(TracerName),
	}

	for _, opt := range opts {
		opt(ti)
	}

	return ti
}

// Start starts the interpreter and creates a root span for the machine lifecycle.
func (ti *TracingInterpreter[C]) Start(ctx context.Context) context.Context {
	ti.spanCtx, ti.rootSpan = ti.tracer.Start(ctx, fmt.Sprintf("statemachine/%s", ti.machineID),
		trace.WithAttributes(
			attribute.String("statekit.machine.id", ti.machineID),
		),
	)

	ti.interpreter.Start()

	state := ti.interpreter.State()
	ti.rootSpan.AddEvent("state.entered",
		trace.WithAttributes(
			attribute.String("statekit.state.id", string(state.Value)),
		),
	)

	return ti.spanCtx
}

// Send processes an event and records a span for the transition.
func (ti *TracingInterpreter[C]) Send(ctx context.Context, event statekit.Event) {
	stateBefore := ti.interpreter.State().Value

	_, span := ti.tracer.Start(ctx, fmt.Sprintf("event/%s", event.Type),
		trace.WithAttributes(
			attribute.String("statekit.machine.id", ti.machineID),
			attribute.String("statekit.event.type", string(event.Type)),
			attribute.String("statekit.state.before", string(stateBefore)),
		),
	)
	defer span.End()

	ti.interpreter.Send(event)

	stateAfter := ti.interpreter.State().Value
	transitioned := stateBefore != stateAfter

	span.SetAttributes(
		attribute.String("statekit.state.after", string(stateAfter)),
		attribute.Bool("statekit.transitioned", transitioned),
	)

	if transitioned {
		span.AddEvent("state.transition",
			trace.WithAttributes(
				attribute.String("statekit.state.from", string(stateBefore)),
				attribute.String("statekit.state.to", string(stateAfter)),
			),
		)
	}

	if ti.interpreter.Done() {
		span.AddEvent("state.final",
			trace.WithAttributes(
				attribute.String("statekit.state.id", string(stateAfter)),
			),
		)
	}
}

// SendAll processes multiple events.
func (ti *TracingInterpreter[C]) SendAll(ctx context.Context, events []statekit.Event) {
	for _, event := range events {
		ti.Send(ctx, event)
	}
}

// State returns the current state.
func (ti *TracingInterpreter[C]) State() statekit.State[C] {
	return ti.interpreter.State()
}

// Context returns the current context.
func (ti *TracingInterpreter[C]) Context() C {
	return ti.interpreter.State().Context
}

// Done returns true if in a final state.
func (ti *TracingInterpreter[C]) Done() bool {
	return ti.interpreter.Done()
}

// Matches checks if current state matches or is descendant of given state.
func (ti *TracingInterpreter[C]) Matches(stateID statekit.StateID) bool {
	return ti.interpreter.Matches(stateID)
}

// Stop stops the interpreter and ends the root span.
func (ti *TracingInterpreter[C]) Stop() {
	state := ti.interpreter.State()

	if ti.rootSpan != nil {
		ti.rootSpan.SetAttributes(
			attribute.String("statekit.state.final", string(state.Value)),
			attribute.Bool("statekit.completed", ti.interpreter.Done()),
		)

		if ti.interpreter.Done() {
			ti.rootSpan.SetStatus(codes.Ok, "completed")
		}

		ti.rootSpan.End()
	}

	ti.interpreter.Stop()
}

// Interpreter returns the underlying interpreter.
func (ti *TracingInterpreter[C]) Interpreter() *statekit.Interpreter[C] {
	return ti.interpreter
}

// --- Middleware-style Hook ---

// TransitionHook is a hook that can be called on state transitions.
type TransitionHook func(ctx context.Context, machineID string, event statekit.Event, stateBefore, stateAfter string)

// TracingHook returns a hook that records transitions to OpenTelemetry.
func TracingHook(tracer trace.Tracer) TransitionHook {
	if tracer == nil {
		tracer = otel.Tracer(TracerName)
	}

	return func(ctx context.Context, machineID string, event statekit.Event, stateBefore, stateAfter string) {
		_, span := tracer.Start(ctx, fmt.Sprintf("statemachine/%s/event/%s", machineID, event.Type),
			trace.WithAttributes(
				attribute.String("statekit.machine.id", machineID),
				attribute.String("statekit.event.type", string(event.Type)),
				attribute.String("statekit.state.before", stateBefore),
				attribute.String("statekit.state.after", stateAfter),
				attribute.Bool("statekit.transitioned", stateBefore != stateAfter),
			),
		)
		span.End()
	}
}

// --- Span Attributes Helper ---

// StateAttributes returns common attributes for a state.
func StateAttributes(machineID string, state statekit.StateID) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("statekit.machine.id", machineID),
		attribute.String("statekit.state.id", string(state)),
	}
}

// EventAttributes returns common attributes for an event.
func EventAttributes(machineID string, event statekit.Event) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("statekit.machine.id", machineID),
		attribute.String("statekit.event.type", string(event.Type)),
	}
	return attrs
}

// TransitionAttributes returns attributes for a state transition.
func TransitionAttributes(machineID string, event statekit.Event, from, to statekit.StateID) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("statekit.machine.id", machineID),
		attribute.String("statekit.event.type", string(event.Type)),
		attribute.String("statekit.state.from", string(from)),
		attribute.String("statekit.state.to", string(to)),
		attribute.Bool("statekit.transitioned", from != to),
	}
}
