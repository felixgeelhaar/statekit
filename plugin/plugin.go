// Package plugin provides a hook system for extending interpreter behavior.
//
// Plugins allow observing and modifying interpreter execution without changing
// core code. Implement one or more hook interfaces to receive callbacks.
//
// Example usage:
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Name() string { return "my-plugin" }
//
//	func (p *MyPlugin) OnEnter(ctx plugin.Context[MyCtx], state statekit.StateID) {
//	    log.Printf("entered state: %s", state)
//	}
//
//	interp := statekit.NewInterpreter(machine)
//	interp.Use(&MyPlugin{})
package plugin

import "go.klarlabs.de/statekit/internal/ir"

// Re-export types from internal/ir to ensure type compatibility with statekit
type (
	// StateID uniquely identifies a state within a machine
	StateID = ir.StateID
	// EventType is a named event identifier
	EventType = ir.EventType
	// ActionType identifies a named action
	ActionType = ir.ActionType
	// Event represents a runtime event with optional payload
	Event = ir.Event
)

// Plugin is the base interface that all plugins must implement.
type Plugin[C any] interface {
	// Name returns the plugin's identifier for debugging and logging.
	Name() string
}

// Context provides access to interpreter state during hook execution.
type Context[C any] struct {
	// MachineID is the identifier of the state machine.
	MachineID string
	// CurrentState is the current state ID.
	CurrentState StateID
	// Context is the current machine context (read-only snapshot).
	Context C
}

// OnEventHook is called when events are received by the interpreter.
type OnEventHook[C any] interface {
	Plugin[C]
	// OnEvent is called before an event is processed.
	// The returned event is used for processing (allows modification).
	// Errors are logged but do not stop execution.
	OnEvent(ctx Context[C], event Event) Event
}

// OnTransitionHook is called before and after state transitions.
type OnTransitionHook[C any] interface {
	Plugin[C]
	// BeforeTransition is called before a transition is executed.
	BeforeTransition(ctx Context[C], from, to StateID, event Event)
	// AfterTransition is called after a transition completes.
	AfterTransition(ctx Context[C], from, to StateID, event Event)
}

// OnStateHook is called when entering and exiting states.
type OnStateHook[C any] interface {
	Plugin[C]
	// OnEnter is called when a state is entered.
	OnEnter(ctx Context[C], state StateID)
	// OnExit is called when a state is exited.
	OnExit(ctx Context[C], state StateID)
}

// OnActionHook is called before and after action execution.
type OnActionHook[C any] interface {
	Plugin[C]
	// BeforeAction is called before an action is executed.
	BeforeAction(ctx Context[C], action ActionType, event Event)
	// AfterAction is called after an action completes.
	AfterAction(ctx Context[C], action ActionType, event Event)
}

// OnStartStopHook is called when the interpreter starts and stops.
type OnStartStopHook[C any] interface {
	Plugin[C]
	// OnStart is called when the interpreter starts.
	OnStart(ctx Context[C])
	// OnStop is called when the interpreter stops.
	OnStop(ctx Context[C])
}

// OnErrorHook is called when errors occur during execution.
type OnErrorHook[C any] interface {
	Plugin[C]
	// OnError is called when an error occurs.
	// This includes action panics (recovered) and plugin errors.
	OnError(ctx Context[C], err error)
}

// Composite allows combining multiple plugins into one.
type Composite[C any] struct {
	plugins []Plugin[C]
}

// NewComposite creates a composite plugin from multiple plugins.
func NewComposite[C any](plugins ...Plugin[C]) *Composite[C] {
	return &Composite[C]{plugins: plugins}
}

// Name returns "composite" for the combined plugin.
func (c *Composite[C]) Name() string {
	return "composite"
}

// Plugins returns the underlying plugins.
func (c *Composite[C]) Plugins() []Plugin[C] {
	return c.plugins
}
