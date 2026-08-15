package statekit

import (
	"context"
	"fmt"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
)

// --- Invoked services management (v3.0) ---

// startInvokedServices starts all services for the given state
// Caller must hold mu.
func (i *Interpreter[C]) startInvokedServices(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil || len(stateConfig.Invocations) == 0 {
		return
	}

	for _, invoke := range stateConfig.Invocations {
		service := i.machine.GetService(invoke.Src)
		if service == nil {
			continue
		}

		// Create cancellation context for this invocation
		ctx, cancel := context.WithCancel(context.Background())
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)
		i.activeServices[invokeKey] = cancel

		// Capture invoke config for closure
		capturedInvoke := invoke
		capturedStateID := stateID

		// Start service in goroutine
		go func() {
			// Create service context
			svcCtx := ir.ServiceContext[C]{
				Context:        ctx,
				MachineContext: i.state.Context,
				Send: func(event Event) {
					i.Send(event)
				},
			}

			// Run the service
			err := service(svcCtx)

			// Handle completion (back on the interpreter goroutine)
			i.mu.Lock()
			defer i.mu.Unlock()

			// Check if we're still in the same state and service wasn't cancelled
			if !i.started || !i.matchesUnlocked(capturedStateID) {
				return
			}

			// Clean up
			delete(i.activeServices, invokeKey)

			// Handle result
			if err != nil {
				// Service failed
				if capturedInvoke.OnError != nil {
					i.executeServiceTransition(capturedStateID, capturedInvoke.OnError, Event{
						Type:    "error",
						Payload: err,
					})
				}
			} else {
				// Service succeeded
				if capturedInvoke.OnDone != nil {
					i.executeServiceTransition(capturedStateID, capturedInvoke.OnDone, Event{
						Type: "done",
					})
				}
			}
		}()
	}
}

// cancelInvokedServices cancels all services for the given state
// Caller must hold mu.
func (i *Interpreter[C]) cancelInvokedServices(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	for _, invoke := range stateConfig.Invocations {
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)
		if cancel, ok := i.activeServices[invokeKey]; ok {
			cancel()
			delete(i.activeServices, invokeKey)
		}
	}
}

// executeServiceTransition executes an OnDone or OnError transition
// Caller must hold mu.
func (i *Interpreter[C]) executeServiceTransition(sourceStateID ir.StateID, trans *ir.TransitionConfig, event Event) {
	if trans.Target == "" {
		// No target, just execute actions
		i.executeActions(trans.Actions, event)
		return
	}

	sourceState := i.machine.GetState(sourceStateID)
	if sourceState == nil {
		return
	}

	source := &transitionSource[C]{
		state:      sourceState,
		transition: trans,
	}
	i.executeTransitionHierarchical(source, event)
}

// --- Invoked machine management (v0.14) ---

// startInvokedMachines starts all child machines for the given state
// Caller must hold mu.
func (i *Interpreter[C]) startInvokedMachines(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil || len(stateConfig.MachineInvocations) == 0 {
		return
	}

	for _, invoke := range stateConfig.MachineInvocations {
		factory := i.machine.GetChildMachine(invoke.MachineRef)
		if factory == nil {
			continue
		}

		// Create unique key for this invocation
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)

		// Create parent send function for child-to-parent communication
		parentSend := func(event Event) error {
			i.Send(event)
			return nil
		}

		// Create child interpreter via factory
		child := factory(i.state.Context, parentSend)
		i.activeInvokedMachines[invokeKey] = child

		// Start child in goroutine to handle its lifecycle
		capturedInvoke := invoke
		capturedStateID := stateID
		go func() {
			// Start the child machine
			child.Start()

			// Wait for child to complete (reach final state)
			// Poll periodically - in production this would use channels
			for !child.Done() {
				// Small sleep to avoid busy-waiting
				time.Sleep(10 * time.Millisecond)

				// Check if we've been stopped
				i.mu.Lock()
				_, stillActive := i.activeInvokedMachines[invokeKey]
				i.mu.Unlock()
				if !stillActive {
					return // We were stopped externally
				}
			}

			// Child completed - handle OnDone
			i.mu.Lock()
			defer i.mu.Unlock()

			// Check if we're still in the same state and still active
			if !i.started || !i.matchesUnlocked(capturedStateID) {
				return
			}

			// Clean up
			delete(i.activeInvokedMachines, invokeKey)

			// Execute OnDone transition if configured
			if capturedInvoke.OnDone != nil && capturedInvoke.OnDone.Target != "" {
				doneEvent := Event{
					Type: EventType(fmt.Sprintf("statekit.done.invoke.%s", capturedInvoke.ID)),
				}
				i.executeServiceTransition(capturedStateID, capturedInvoke.OnDone, doneEvent)
			}
		}()
	}
}

// stopInvokedMachines stops all child machines for the given state
// Caller must hold mu.
func (i *Interpreter[C]) stopInvokedMachines(stateID ir.StateID) {
	stateConfig := i.machine.GetState(stateID)
	if stateConfig == nil {
		return
	}

	for _, invoke := range stateConfig.MachineInvocations {
		invokeKey := fmt.Sprintf("%s:%s", stateID, invoke.ID)
		if child, ok := i.activeInvokedMachines[invokeKey]; ok {
			child.Stop()
			delete(i.activeInvokedMachines, invokeKey)
		}
	}
}
