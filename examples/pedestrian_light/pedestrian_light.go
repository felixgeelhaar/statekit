// Package pedestrianlight implements a pedestrian crossing signal with hierarchical states.
//
// This example demonstrates:
// - Compound/nested states
// - Event bubbling to parent states
// - Proper entry/exit action ordering
// - State hierarchy and initial state resolution
//
// State hierarchy:
//
//	pedestrian_signal
//	├── active (compound, initial)
//	│   ├── dont_walk (initial)
//	│   ├── walk
//	│   └── countdown (compound)
//	│       ├── flashing (initial)
//	│       └── warning
//	└── maintenance
package pedestrianlight

import (
	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// State IDs
const (
	StateActive      statekit.StateID = "active"
	StateDontWalk    statekit.StateID = "dont_walk"
	StateWalk        statekit.StateID = "walk"
	StateCountdown   statekit.StateID = "countdown"
	StateFlashing    statekit.StateID = "flashing"
	StateWarning     statekit.StateID = "warning"
	StateMaintenance statekit.StateID = "maintenance"
)

// Event types
const (
	EventPedestrianButton statekit.EventType = "PEDESTRIAN_BUTTON"
	EventTimer            statekit.EventType = "TIMER"
	EventEnterMaintenance statekit.EventType = "ENTER_MAINTENANCE"
	EventExitMaintenance  statekit.EventType = "EXIT_MAINTENANCE"
)

// Action types
const (
	ActionEnterActive      statekit.ActionType = "enterActive"
	ActionExitActive       statekit.ActionType = "exitActive"
	ActionEnterDontWalk    statekit.ActionType = "enterDontWalk"
	ActionExitDontWalk     statekit.ActionType = "exitDontWalk"
	ActionEnterWalk        statekit.ActionType = "enterWalk"
	ActionExitWalk         statekit.ActionType = "exitWalk"
	ActionEnterCountdown   statekit.ActionType = "enterCountdown"
	ActionExitCountdown    statekit.ActionType = "exitCountdown"
	ActionEnterFlashing    statekit.ActionType = "enterFlashing"
	ActionEnterWarning     statekit.ActionType = "enterWarning"
	ActionEnterMaintenance statekit.ActionType = "enterMaintenance"
	ActionExitMaintenance  statekit.ActionType = "exitMaintenance"
	ActionLogTransition    statekit.ActionType = "logTransition"
)

// Context holds the pedestrian signal state
type Context struct {
	// CrossingCount tracks number of complete crossing cycles
	CrossingCount int
	// CountdownSeconds remaining in countdown
	CountdownSeconds int
	// Log of state entries for testing/debugging
	Log []string
	// InMaintenance indicates if currently in maintenance mode
	InMaintenance bool
}

// NewPedestrianLight creates a new pedestrian crossing signal state machine
func NewPedestrianLight() (*ir.MachineConfig[Context], error) {
	return statekit.NewMachine[Context]("pedestrian_signal").
		WithInitial(StateActive).
		WithContext(Context{CountdownSeconds: 10}).
		// Register all actions
		WithAction(ActionEnterActive, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Entered ACTIVE mode")
		}).
		WithAction(ActionExitActive, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Exited ACTIVE mode")
		}).
		WithAction(ActionEnterDontWalk, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "DON'T WALK - Hand symbol displayed")
		}).
		WithAction(ActionExitDontWalk, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "DON'T WALK ended")
		}).
		WithAction(ActionEnterWalk, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "WALK - Walking figure displayed")
		}).
		WithAction(ActionExitWalk, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "WALK ended")
		}).
		WithAction(ActionEnterCountdown, func(ctx *Context, e statekit.Event) {
			ctx.CountdownSeconds = 10
			ctx.Log = append(ctx.Log, "Countdown started")
		}).
		WithAction(ActionExitCountdown, func(ctx *Context, e statekit.Event) {
			ctx.CrossingCount++
			ctx.Log = append(ctx.Log, "Countdown ended, crossing complete")
		}).
		WithAction(ActionEnterFlashing, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Flashing hand symbol")
		}).
		WithAction(ActionEnterWarning, func(ctx *Context, e statekit.Event) {
			ctx.CountdownSeconds = 3
			ctx.Log = append(ctx.Log, "Warning - solid hand, 3 seconds remaining")
		}).
		WithAction(ActionEnterMaintenance, func(ctx *Context, e statekit.Event) {
			ctx.InMaintenance = true
			ctx.Log = append(ctx.Log, "Entered MAINTENANCE mode - all lights off")
		}).
		WithAction(ActionExitMaintenance, func(ctx *Context, e statekit.Event) {
			ctx.InMaintenance = false
			ctx.Log = append(ctx.Log, "Exited MAINTENANCE mode")
		}).
		WithAction(ActionLogTransition, func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Transition action executed")
		}).
		// Define the active compound state with children
		State(StateActive).
			WithInitial(StateDontWalk).
			OnEntry(ActionEnterActive).
			OnExit(ActionExitActive).
			On(EventEnterMaintenance).Target(StateMaintenance).Do(ActionLogTransition).End().
			// Don't Walk state
			State(StateDontWalk).
				OnEntry(ActionEnterDontWalk).
				OnExit(ActionExitDontWalk).
				On(EventPedestrianButton).Target(StateWalk).
			End().
			End().
			// Walk state
			State(StateWalk).
				OnEntry(ActionEnterWalk).
				OnExit(ActionExitWalk).
				On(EventTimer).Target(StateCountdown).
			End().
			End().
			// Countdown compound state
			State(StateCountdown).
				WithInitial(StateFlashing).
				OnEntry(ActionEnterCountdown).
				OnExit(ActionExitCountdown).
				State(StateFlashing).
					OnEntry(ActionEnterFlashing).
					On(EventTimer).Target(StateWarning).
				End().
				End().
				State(StateWarning).
					OnEntry(ActionEnterWarning).
					On(EventTimer).Target(StateDontWalk).
				End().
			End().
		End().
		Done().
		// Maintenance state (sibling of active)
		State(StateMaintenance).
			OnEntry(ActionEnterMaintenance).
			OnExit(ActionExitMaintenance).
			On(EventExitMaintenance).Target(StateActive).
		Done().
		Build()
}
