package statekit

import (
	"go.klarlabs.de/statekit/plugin"
)

// Use registers a plugin with the interpreter.
// Plugins receive callbacks during interpreter execution.
// Multiple plugins can be registered; they are called in registration order.
func (i *Interpreter[C]) Use(p plugin.Plugin[C]) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Expand composite plugins
	if composite, ok := p.(*plugin.Composite[C]); ok {
		i.plugins = append(i.plugins, composite.Plugins()...)
		return
	}

	i.plugins = append(i.plugins, p)
}

// pluginContext creates a plugin.Context from current interpreter state.
// Caller must hold mu.
func (i *Interpreter[C]) pluginContext() plugin.Context[C] {
	return plugin.Context[C]{
		MachineID:    i.machine.ID,
		CurrentState: i.state.Value,
		Context:      i.state.Context,
	}
}

// callOnEvent calls OnEvent hooks on all plugins that implement OnEventHook.
// Returns the (potentially modified) event.
// Caller must hold mu.
func (i *Interpreter[C]) callOnEvent(event Event) Event {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnEventHook[C]); ok {
			event = hook.OnEvent(ctx, event)
		}
	}
	return event
}

// callBeforeTransition calls BeforeTransition hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callBeforeTransition(from, to StateID, event Event) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnTransitionHook[C]); ok {
			hook.BeforeTransition(ctx, from, to, event)
		}
	}
}

// callAfterTransition calls AfterTransition hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callAfterTransition(from, to StateID, event Event) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnTransitionHook[C]); ok {
			hook.AfterTransition(ctx, from, to, event)
		}
	}
}

// callOnEnter calls OnEnter hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callOnEnter(state StateID) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnStateHook[C]); ok {
			hook.OnEnter(ctx, state)
		}
	}
}

// callOnExit calls OnExit hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callOnExit(state StateID) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnStateHook[C]); ok {
			hook.OnExit(ctx, state)
		}
	}
}

// callBeforeAction calls BeforeAction hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callBeforeAction(action ActionType, event Event) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnActionHook[C]); ok {
			hook.BeforeAction(ctx, action, event)
		}
	}
}

// callAfterAction calls AfterAction hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callAfterAction(action ActionType, event Event) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnActionHook[C]); ok {
			hook.AfterAction(ctx, action, event)
		}
	}
}

// callOnStart calls OnStart hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callOnStart() {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnStartStopHook[C]); ok {
			hook.OnStart(ctx)
		}
	}
}

// callOnStop calls OnStop hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callOnStop() {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnStartStopHook[C]); ok {
			hook.OnStop(ctx)
		}
	}
}

// callOnError calls OnError hooks on all plugins.
// Caller must hold mu.
func (i *Interpreter[C]) callOnError(err error) {
	ctx := i.pluginContext()
	for _, p := range i.plugins {
		if hook, ok := p.(plugin.OnErrorHook[C]); ok {
			hook.OnError(ctx, err)
		}
	}
}
