package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/statekit"
)

func TestMachineHandler_GetState(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	w := httptest.NewRecorder()

	handler.HandleGetState(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp StateResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.CurrentState != "idle" {
		t.Errorf("expected idle, got %s", resp.CurrentState)
	}
	if resp.Done {
		t.Error("expected Done=false")
	}
}

func TestMachineHandler_SendEvent(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	body := bytes.NewBufferString(`{"type": "START"}`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp EventResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.PreviousState != "idle" {
		t.Errorf("expected prev=idle, got %s", resp.PreviousState)
	}
	if resp.CurrentState != "running" {
		t.Errorf("expected curr=running, got %s", resp.CurrentState)
	}
	if !resp.Transitioned {
		t.Error("expected Transitioned=true")
	}
}

func TestMachineHandler_SendEvent_NoTransition(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("START").Target("running").Done().
		State("running").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	body := bytes.NewBufferString(`{"type": "UNKNOWN"}`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	var resp EventResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Transitioned {
		t.Error("expected Transitioned=false")
	}
	if resp.CurrentState != "idle" {
		t.Errorf("expected idle, got %s", resp.CurrentState)
	}
}

func TestMachineHandler_SendEvent_InvalidBody(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMachineHandler_SendEvent_MissingType(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	body := bytes.NewBufferString(`{"payload": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMachineHandler_GetContext(t *testing.T) {
	type Context struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}

	machine, _ := statekit.NewMachine[Context]("test").
		WithInitial("idle").
		WithContext(Context{Count: 42, Name: "test"}).
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	req := httptest.NewRequest(http.MethodGet, "/context", nil)
	w := httptest.NewRecorder()

	handler.HandleGetContext(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var ctx Context
	_ = json.Unmarshal(w.Body.Bytes(), &ctx)

	if ctx.Count != 42 {
		t.Errorf("expected count=42, got %d", ctx.Count)
	}
	if ctx.Name != "test" {
		t.Errorf("expected name=test, got %s", ctx.Name)
	}
}

func TestMachineHandler_ServeHTTP(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").On("GO").Target("done").Done().
		State("done").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	// Test GET /state
	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /state: expected 200, got %d", w.Code)
	}

	// Test POST /event
	body := bytes.NewBufferString(`{"type": "GO"}`)
	req = httptest.NewRequest(http.MethodPost, "/event", body)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST /event: expected 200, got %d", w.Code)
	}

	// Test GET /context
	req = httptest.NewRequest(http.MethodGet, "/context", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /context: expected 200, got %d", w.Code)
	}

	// Test 404
	req = httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown path: expected 404, got %d", w.Code)
	}
}

func TestMachineRegistry(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	factory := func(id string) (*statekit.Interpreter[struct{}], error) {
		interp := statekit.NewInterpreter(machine)
		interp.Start()
		return interp, nil
	}

	registry := NewMachineRegistry(factory)

	// Get creates new machine
	interp1, err := registry.Get("machine-1")
	if err != nil {
		t.Fatal(err)
	}
	if interp1 == nil {
		t.Fatal("expected interpreter")
	}

	// Get returns same machine
	interp2, _ := registry.Get("machine-1")
	if interp1 != interp2 {
		t.Error("expected same interpreter instance")
	}

	// Different ID returns different machine
	interp3, _ := registry.Get("machine-2")
	if interp1 == interp3 {
		t.Error("expected different interpreter instance")
	}

	// List returns all IDs
	ids := registry.List()
	if len(ids) != 2 {
		t.Errorf("expected 2 machines, got %d", len(ids))
	}

	// Remove stops and removes
	registry.Remove("machine-1")
	ids = registry.List()
	if len(ids) != 1 {
		t.Errorf("expected 1 machine after remove, got %d", len(ids))
	}
}

func TestWithMachine_MachineFromContext(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	ctx := context.Background()
	ctx = WithMachine(ctx, interp)

	retrieved, ok := MachineFromContext[struct{}](ctx)
	if !ok {
		t.Error("expected interpreter in context")
	}
	if retrieved != interp {
		t.Error("expected same interpreter")
	}
}

func TestMachineMiddleware(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	middleware := MachineMiddleware(interp)

	var capturedInterp *statekit.Interpreter[struct{}]
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInterp, _ = MachineFromContext[struct{}](r.Context())
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if capturedInterp != interp {
		t.Error("expected interpreter in request context")
	}
}

func TestRegistryMiddleware(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	factory := func(id string) (*statekit.Interpreter[struct{}], error) {
		interp := statekit.NewInterpreter(machine)
		interp.Start()
		return interp, nil
	}

	registry := NewMachineRegistry(factory)

	idExtractor := func(r *http.Request) string {
		return r.URL.Query().Get("id")
	}

	middleware := RegistryMiddleware(registry, idExtractor)

	var capturedInterp *statekit.Interpreter[struct{}]
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedInterp, _ = MachineFromContext[struct{}](r.Context())
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware(handler)

	// Test with ID
	req := httptest.NewRequest(http.MethodGet, "/?id=machine-1", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if capturedInterp == nil {
		t.Error("expected interpreter in request context")
	}

	// Test without ID
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without ID, got %d", w.Code)
	}
}

func TestNewServeMux(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("idle").
		State("idle").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	mux := NewServeMux(interp, "/api/machine")

	// Test state endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/machine/state", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMachineHandler_FinalState(t *testing.T) {
	machine, _ := statekit.NewMachine[struct{}]("test").
		WithInitial("active").
		State("active").On("COMPLETE").Target("done").Done().
		State("done").Final().Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	// Send event to final state
	body := bytes.NewBufferString(`{"type": "COMPLETE"}`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	var resp EventResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Done {
		t.Error("expected Done=true after reaching final state")
	}
	if resp.CurrentState != "done" {
		t.Errorf("expected state=done, got %s", resp.CurrentState)
	}
}

func TestMachineHandler_WithPayload(t *testing.T) {
	type Context struct {
		Value string
	}

	machine, _ := statekit.NewMachine[Context]("test").
		WithInitial("idle").
		WithAction("setVal", func(ctx *Context, e statekit.Event) {
			if payload, ok := e.Payload.(map[string]any); ok {
				if v, ok := payload["value"].(string); ok {
					ctx.Value = v
				}
			}
		}).
		State("idle").On("SET").Target("idle").Do("setVal").Done().
		Build()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	handler := NewMachineHandler(interp)

	body := bytes.NewBufferString(`{"type": "SET", "payload": {"value": "hello"}}`)
	req := httptest.NewRequest(http.MethodPost, "/event", body)
	w := httptest.NewRecorder()

	handler.HandleSendEvent(w, req)

	// Check context was updated
	req = httptest.NewRequest(http.MethodGet, "/context", nil)
	w = httptest.NewRecorder()
	handler.HandleGetContext(w, req)

	var ctx Context
	_ = json.Unmarshal(w.Body.Bytes(), &ctx)

	if ctx.Value != "hello" {
		t.Errorf("expected value=hello, got %s", ctx.Value)
	}
}
