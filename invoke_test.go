package statekit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/internal/ir"
)

// TestInvoke_ServiceStartsOnEntry verifies that a service is started when entering a state
func TestInvoke_ServiceStartsOnEntry(t *testing.T) {
	t.Parallel()
	serviceStarted := false
	var wg sync.WaitGroup
	wg.Add(1)

	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[struct{}]) error {
			serviceStarted = true
			wg.Done()
			// Block until cancelled
			<-ctx.Context.(context.Context).Done()
			return nil
		}).
		State("loading").
		Invoke("fetchData").End().
		Done().
		State("done").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for service to start
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Service started
	case <-time.After(time.Second):
		t.Fatal("Service did not start")
	}

	if !serviceStarted {
		t.Error("Expected service to be started")
	}

	interp.Stop()
}

// TestInvoke_ServiceCancelledOnExit verifies that a service is cancelled when exiting a state
func TestInvoke_ServiceCancelledOnExit(t *testing.T) {
	t.Parallel()
	serviceCancelled := false
	serviceStarted := make(chan struct{})
	serviceDone := make(chan struct{})

	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[struct{}]) error {
			close(serviceStarted)
			<-ctx.Context.(context.Context).Done()
			serviceCancelled = true
			close(serviceDone)
			return nil
		}).
		State("loading").
		Invoke("fetchData").End().
		On("CANCEL").Target("idle").
		Done().
		State("idle").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for service to start
	select {
	case <-serviceStarted:
	case <-time.After(time.Second):
		t.Fatal("Service did not start")
	}

	// Transition away from loading - should cancel the service
	interp.Send(statekit.Event{Type: "CANCEL"})

	// Wait for service to be cancelled
	select {
	case <-serviceDone:
	case <-time.After(time.Second):
		t.Fatal("Service was not cancelled")
	}

	if !serviceCancelled {
		t.Error("Expected service to be cancelled")
	}
}

// TestInvoke_OnDoneTransition verifies that OnDone triggers a transition when service completes
func TestInvoke_OnDoneTransition(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[struct{}]) error {
			// Complete immediately
			return nil
		}).
		State("loading").
		Invoke("fetchData").OnDone("success").End().
		Done().
		State("success").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	waitUntil(t, time.Second, func() bool { return interp.State().Value == "success" })

	state := interp.State()
	if state.Value != "success" {
		t.Errorf("Expected state 'success', got '%s'", state.Value)
	}

	if !interp.Done() {
		t.Error("Expected machine to be done")
	}
}

// TestInvoke_OnErrorTransition verifies that OnError triggers a transition when service fails
func TestInvoke_OnErrorTransition(t *testing.T) {
	t.Parallel()
	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[struct{}]) error {
			return errors.New("network error")
		}).
		State("loading").
		Invoke("fetchData").OnDone("success").OnError("failure").End().
		Done().
		State("success").Final().
		Done().
		State("failure").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	waitUntil(t, time.Second, func() bool { return interp.State().Value == "failure" })

	state := interp.State()
	if state.Value != "failure" {
		t.Errorf("Expected state 'failure', got '%s'", state.Value)
	}
}

// TestInvoke_OnDoneAction verifies that OnDoneAction executes when service completes
func TestInvoke_OnDoneAction(t *testing.T) {
	t.Parallel()
	type Context struct {
		ActionExecuted bool
	}

	machine, err := statekit.NewMachine[Context]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[Context]) error {
			return nil
		}).
		WithAction("setDone", func(ctx *Context, e statekit.Event) {
			ctx.ActionExecuted = true
		}).
		State("loading").
		Invoke("fetchData").OnDone("success").OnDoneAction("setDone").End().
		Done().
		State("success").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for service to complete
	waitUntil(t, time.Second, func() bool { return interp.State().Context.ActionExecuted })

	state := interp.State()
	if !state.Context.ActionExecuted {
		t.Error("Expected action to be executed")
	}
}

// TestInvoke_ServiceSendsEvents verifies that services can send events back to the machine
func TestInvoke_ServiceSendsEvents(t *testing.T) {
	t.Parallel()
	type Context struct {
		ReceivedEvent bool
	}

	machine, err := statekit.NewMachine[Context]("test").
		WithInitial("loading").
		WithService("fetchData", func(ctx ir.ServiceContext[Context]) error {
			// Send an event back to the machine
			ctx.Send(statekit.Event{Type: "DATA_RECEIVED"})
			// Block until cancelled
			<-ctx.Context.(context.Context).Done()
			return nil
		}).
		WithAction("markReceived", func(ctx *Context, e statekit.Event) {
			ctx.ReceivedEvent = true
		}).
		State("loading").
		Invoke("fetchData").End().
		On("DATA_RECEIVED").Target("processing").Do("markReceived").
		Done().
		State("processing").
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for event to be processed
	waitUntil(t, time.Second, func() bool { return interp.State().Value == "processing" })

	state := interp.State()
	if state.Value != "processing" {
		t.Errorf("Expected state 'processing', got '%s'", state.Value)
	}
	if !state.Context.ReceivedEvent {
		t.Error("Expected action to be executed from sent event")
	}

	interp.Stop()
}

// TestInvoke_MultipleServices verifies that multiple services can be invoked on a single state
func TestInvoke_MultipleServices(t *testing.T) {
	t.Parallel()
	service1Started := make(chan struct{})
	service2Started := make(chan struct{})

	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("service1", func(ctx ir.ServiceContext[struct{}]) error {
			close(service1Started)
			<-ctx.Context.(context.Context).Done()
			return nil
		}).
		WithService("service2", func(ctx ir.ServiceContext[struct{}]) error {
			close(service2Started)
			<-ctx.Context.(context.Context).Done()
			return nil
		}).
		State("loading").
		Invoke("service1").ID("svc1").End().
		Invoke("service2").ID("svc2").End().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for both services to start
	select {
	case <-service1Started:
	case <-time.After(time.Second):
		t.Fatal("Service 1 did not start")
	}

	select {
	case <-service2Started:
	case <-time.After(time.Second):
		t.Fatal("Service 2 did not start")
	}

	interp.Stop()
}

// TestInvoke_StopCancelsAllServices verifies that Stop() cancels all active services
func TestInvoke_StopCancelsAllServices(t *testing.T) {
	t.Parallel()
	var cancelled sync.WaitGroup
	cancelled.Add(2)
	started := make(chan struct{}, 2)

	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("service1", func(ctx ir.ServiceContext[struct{}]) error {
			started <- struct{}{}
			<-ctx.Context.(context.Context).Done()
			cancelled.Done()
			return nil
		}).
		WithService("service2", func(ctx ir.ServiceContext[struct{}]) error {
			started <- struct{}{}
			<-ctx.Context.(context.Context).Done()
			cancelled.Done()
			return nil
		}).
		State("loading").
		Invoke("service1").End().
		Invoke("service2").End().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	waitUntil(t, time.Second, func() bool {
		return len(started) == 2
	})

	// Stop should cancel all services
	interp.Stop()

	// Wait for both to be cancelled
	done := make(chan struct{})
	go func() {
		cancelled.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All services cancelled
	case <-time.After(time.Second):
		t.Fatal("Not all services were cancelled")
	}
}

// TestInvoke_ServiceContextHasMachineContext verifies service receives current machine context
func TestInvoke_ServiceContextHasMachineContext(t *testing.T) {
	t.Parallel()
	type Context struct {
		Value string
	}

	receivedValue := ""
	done := make(chan struct{})

	machine, err := statekit.NewMachine[Context]("test").
		WithInitial("loading").
		WithContext(Context{Value: "test-value"}).
		WithService("fetchData", func(ctx ir.ServiceContext[Context]) error {
			receivedValue = ctx.MachineContext.Value
			close(done)
			return nil
		}).
		State("loading").
		Invoke("fetchData").End().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Service did not complete")
	}

	if receivedValue != "test-value" {
		t.Errorf("Expected service to receive 'test-value', got '%s'", receivedValue)
	}
}

// TestInvoke_DoesNotTransitionIfStateChanged verifies completed service doesn't transition if state changed
func TestInvoke_DoesNotTransitionIfStateChanged(t *testing.T) {
	t.Parallel()
	serviceStarted := make(chan struct{})
	serviceCanComplete := make(chan struct{})

	machine, err := statekit.NewMachine[struct{}]("test").
		WithInitial("loading").
		WithService("slowFetch", func(ctx ir.ServiceContext[struct{}]) error {
			close(serviceStarted)
			// Wait until we're told we can complete
			select {
			case <-serviceCanComplete:
				return nil
			case <-ctx.Context.(context.Context).Done():
				return nil
			}
		}).
		State("loading").
		Invoke("slowFetch").OnDone("success").End().
		On("CANCEL").Target("cancelled").
		Done().
		State("success").Final().
		Done().
		State("cancelled").Final().
		Done().
		Build()

	if err != nil {
		t.Fatalf("Failed to build machine: %v", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Wait for service to start
	<-serviceStarted

	// Transition away before service completes
	interp.Send(statekit.Event{Type: "CANCEL"})

	// Verify we're in cancelled state
	if interp.State().Value != "cancelled" {
		t.Errorf("Expected state 'cancelled', got '%s'", interp.State().Value)
	}

	// Now let the service complete
	close(serviceCanComplete)

	// Service completion must not move us out of cancelled — poll briefly.
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if interp.State().Value != "cancelled" {
			t.Fatalf("Expected state 'cancelled' after service complete, got '%s'", interp.State().Value)
		}
		time.Sleep(time.Millisecond)
	}
}
