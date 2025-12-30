package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/felixgeelhaar/statekit"
)

func TestTracingInterpreter_Basic(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "test-machine", WithTracer[struct{}](tracer))

	ctx := ti.Start(context.Background())

	ti.Send(ctx, statekit.Event{Type: "START"})

	if ti.State().Value != "running" {
		t.Errorf("expected running, got %s", ti.State().Value)
	}

	ti.Stop()

	// Check spans were recorded
	spans := exporter.GetSpans()
	if len(spans) < 2 {
		t.Errorf("expected at least 2 spans, got %d", len(spans))
	}

	// Find root span
	var rootSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "statemachine/test-machine" {
			rootSpan = &spans[i]
			break
		}
	}
	if rootSpan == nil {
		t.Error("expected root span")
	}

	// Find event span
	var eventSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "event/START" {
			eventSpan = &spans[i]
			break
		}
	}
	if eventSpan == nil {
		t.Error("expected event span")
	}
}

func TestTracingInterpreter_TransitionAttributes(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("attrs-test").
		WithInitial("a").
		State("a").On("NEXT").Target("b").Done().
		State("b").Done().
		Build()

	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "attrs-test", WithTracer[struct{}](tracer))

	ctx := ti.Start(context.Background())
	ti.Send(ctx, statekit.Event{Type: "NEXT"})
	ti.Stop()

	spans := exporter.GetSpans()

	// Find the event span
	var eventSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "event/NEXT" {
			eventSpan = &spans[i]
			break
		}
	}
	if eventSpan == nil {
		t.Fatal("expected event span")
	}

	// Check attributes
	attrs := eventSpan.Attributes
	checkAttribute(t, attrs, "statekit.machine.id", "attrs-test")
	checkAttribute(t, attrs, "statekit.event.type", "NEXT")
	checkAttribute(t, attrs, "statekit.state.before", "a")
	checkAttribute(t, attrs, "statekit.state.after", "b")
	checkAttributeBool(t, attrs, "statekit.transitioned", true)
}

func TestTracingInterpreter_NoTransition(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("no-trans").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "no-trans", WithTracer[struct{}](tracer))

	ctx := ti.Start(context.Background())
	ti.Send(ctx, statekit.Event{Type: "UNKNOWN"}) // No matching transition
	ti.Stop()

	spans := exporter.GetSpans()

	// Find the event span
	var eventSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "event/UNKNOWN" {
			eventSpan = &spans[i]
			break
		}
	}
	if eventSpan == nil {
		t.Fatal("expected event span")
	}

	// Check transitioned is false
	checkAttributeBool(t, eventSpan.Attributes, "statekit.transitioned", false)
}

func TestTracingInterpreter_FinalState(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("final-test").
		WithInitial("active").
		State("active").On("DONE").Target("completed").Done().
		State("completed").Final().Done().
		Build()

	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "final-test", WithTracer[struct{}](tracer))

	ctx := ti.Start(context.Background())
	ti.Send(ctx, statekit.Event{Type: "DONE"})

	if !ti.Done() {
		t.Error("expected Done() to be true")
	}

	ti.Stop()

	spans := exporter.GetSpans()

	// Find event span and check for final event
	var eventSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "event/DONE" {
			eventSpan = &spans[i]
			break
		}
	}
	if eventSpan == nil {
		t.Fatal("expected event span")
	}

	// Check that state.final event was recorded
	hasFinalEvent := false
	for _, event := range eventSpan.Events {
		if event.Name == "state.final" {
			hasFinalEvent = true
			break
		}
	}
	if !hasFinalEvent {
		t.Error("expected state.final event")
	}
}

func TestTracingInterpreter_SendAll(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("sendall").
		WithInitial("a").
		State("a").On("NEXT").Target("b").Done().
		State("b").On("NEXT").Target("c").Done().
		State("c").Done().
		Build()

	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "sendall", WithTracer[struct{}](tracer))

	ctx := ti.Start(context.Background())
	ti.SendAll(ctx, []statekit.Event{
		{Type: "NEXT"},
		{Type: "NEXT"},
	})
	ti.Stop()

	if ti.State().Value != "c" {
		t.Errorf("expected c, got %s", ti.State().Value)
	}

	spans := exporter.GetSpans()

	// Count event spans
	eventSpans := 0
	for _, span := range spans {
		if span.Name == "event/NEXT" {
			eventSpans++
		}
	}
	if eventSpans != 2 {
		t.Errorf("expected 2 event spans, got %d", eventSpans)
	}
}

func TestTracingInterpreter_Matches(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("matches").
		WithInitial("parent").
		State("parent").
		WithInitial("child").
		State("child").End().
		Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	ti := NewTracingInterpreter(interp, "matches")

	ti.Start(context.Background())

	if !ti.Matches("parent") {
		t.Error("expected to match parent")
	}
	if !ti.Matches("child") {
		t.Error("expected to match child")
	}

	ti.Stop()
}

func TestTracingHook(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	tracer := tp.Tracer(TracerName)

	hook := TracingHook(tracer)

	ctx := context.Background()
	hook(ctx, "test-machine", statekit.Event{Type: "START"}, "idle", "running")

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "statemachine/test-machine/event/START" {
		t.Errorf("unexpected span name: %s", span.Name)
	}

	checkAttribute(t, span.Attributes, "statekit.machine.id", "test-machine")
	checkAttribute(t, span.Attributes, "statekit.event.type", "START")
	checkAttribute(t, span.Attributes, "statekit.state.before", "idle")
	checkAttribute(t, span.Attributes, "statekit.state.after", "running")
	checkAttributeBool(t, span.Attributes, "statekit.transitioned", true)
}

func TestStateAttributes(t *testing.T) {
	attrs := StateAttributes("machine-1", "running")

	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(attrs))
	}

	checkAttribute(t, attrs, "statekit.machine.id", "machine-1")
	checkAttribute(t, attrs, "statekit.state.id", "running")
}

func TestEventAttributes(t *testing.T) {
	attrs := EventAttributes("machine-1", statekit.Event{Type: "START"})

	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(attrs))
	}

	checkAttribute(t, attrs, "statekit.machine.id", "machine-1")
	checkAttribute(t, attrs, "statekit.event.type", "START")
}

func TestTransitionAttributes(t *testing.T) {
	attrs := TransitionAttributes("machine-1", statekit.Event{Type: "START"}, "idle", "running")

	checkAttribute(t, attrs, "statekit.machine.id", "machine-1")
	checkAttribute(t, attrs, "statekit.event.type", "START")
	checkAttribute(t, attrs, "statekit.state.from", "idle")
	checkAttribute(t, attrs, "statekit.state.to", "running")
	checkAttributeBool(t, attrs, "statekit.transitioned", true)
}

// Helper functions

func checkAttribute(t *testing.T, attrs []attribute.KeyValue, key, expected string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsString() != expected {
				t.Errorf("attribute %s: expected %q, got %q", key, expected, attr.Value.AsString())
			}
			return
		}
	}
	t.Errorf("attribute %s not found", key)
}

func checkAttributeBool(t *testing.T, attrs []attribute.KeyValue, key string, expected bool) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsBool() != expected {
				t.Errorf("attribute %s: expected %v, got %v", key, expected, attr.Value.AsBool())
			}
			return
		}
	}
	t.Errorf("attribute %s not found", key)
}
