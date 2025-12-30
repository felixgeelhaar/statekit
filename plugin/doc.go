// Package plugin provides a hook system for extending interpreter behavior.
//
// Plugins allow observing and modifying interpreter execution without changing
// core statekit code. This enables features like logging, metrics, tracing,
// and custom middleware.
//
// # Plugin Interface
//
// All plugins must implement the base Plugin[C] interface with a Name() method.
// To receive callbacks, implement one or more hook interfaces:
//
//   - OnEventHook: Called when events are received (can modify events)
//   - OnTransitionHook: Called before and after state transitions
//   - OnStateHook: Called when entering and exiting states
//   - OnActionHook: Called before and after action execution
//   - OnStartStopHook: Called when interpreter starts and stops
//   - OnErrorHook: Called when errors occur during execution
//
// # Example Usage
//
//	type LoggingPlugin[C any] struct {
//	    logger *slog.Logger
//	}
//
//	func (p *LoggingPlugin[C]) Name() string { return "logging" }
//
//	func (p *LoggingPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
//	    p.logger.Info("entered state", "state", state)
//	}
//
//	func (p *LoggingPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
//	    p.logger.Info("exited state", "state", state)
//	}
//
//	// Register with interpreter
//	interp := statekit.NewInterpreter(machine)
//	interp.Use(&LoggingPlugin[MyContext]{logger: slog.Default()})
//	interp.Start()
//
// # Error Handling
//
// Plugin errors are logged but do not stop interpreter execution. This ensures
// that observability plugins cannot break state machine behavior. Errors are
// reported to any registered OnErrorHook plugins.
//
// # Composite Plugins
//
// Multiple plugins can be combined using NewComposite:
//
//	composite := plugin.NewComposite(loggingPlugin, metricsPlugin)
//	interp.Use(composite)
//
// # Thread Safety
//
// Plugin hooks are called while the interpreter holds its mutex. Plugins should
// avoid long-running operations or blocking calls in hook methods to prevent
// degrading interpreter performance.
package plugin
