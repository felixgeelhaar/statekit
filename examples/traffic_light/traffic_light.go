package trafficlight

import (
	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// Context holds the state machine context
type Context struct {
	CycleCount int
	Log        []string
}

// Events
const (
	EventTimer statekit.EventType = "TIMER"
	EventReset statekit.EventType = "RESET"
)

// States
const (
	StateGreen  statekit.StateID = "green"
	StateYellow statekit.StateID = "yellow"
	StateRed    statekit.StateID = "red"
)

// NewTrafficLight creates a new traffic light state machine
func NewTrafficLight() (*ir.MachineConfig[Context], error) {
	return statekit.NewMachine[Context]("trafficLight").
		WithInitial(StateGreen).
		WithContext(Context{CycleCount: 0, Log: nil}).

		// Actions
		WithAction("logGreen", func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Entered GREEN")
		}).
		WithAction("logYellow", func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Entered YELLOW")
		}).
		WithAction("logRed", func(ctx *Context, e statekit.Event) {
			ctx.Log = append(ctx.Log, "Entered RED")
		}).
		WithAction("incrementCycle", func(ctx *Context, e statekit.Event) {
			ctx.CycleCount++
		}).

		// States
		State(StateGreen).
			OnEntry("logGreen").
			On(EventTimer).Target(StateYellow).
			On(EventReset).Target(StateGreen).
			Done().

		State(StateYellow).
			OnEntry("logYellow").
			On(EventTimer).Target(StateRed).
			Done().

		State(StateRed).
			OnEntry("logRed").
			On(EventTimer).Target(StateGreen).Do("incrementCycle").
			Done().

		Build()
}
