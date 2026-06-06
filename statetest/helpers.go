package statetest

import (
	"sync"

	"go.klarlabs.de/statekit"
)

// SendEvents sends multiple events to an interpreter in sequence.
func SendEvents[C any](interp *statekit.Interpreter[C], events ...statekit.Event) {
	for _, event := range events {
		interp.Send(event)
	}
}

// SendEventTypes sends multiple events by type (with no payload) to an interpreter.
func SendEventTypes[C any](interp *statekit.Interpreter[C], types ...statekit.EventType) {
	for _, t := range types {
		interp.Send(statekit.Event{Type: t})
	}
}

// MakeEvent creates an event with the given type and optional payload.
func MakeEvent(eventType statekit.EventType, payload ...any) statekit.Event {
	e := statekit.Event{Type: eventType}
	if len(payload) > 0 {
		e.Payload = payload[0]
	}
	return e
}

// MakeEvents creates multiple events from event types.
func MakeEvents(types ...statekit.EventType) []statekit.Event {
	events := make([]statekit.Event, len(types))
	for i, t := range types {
		events[i] = statekit.Event{Type: t}
	}
	return events
}

// StartAndSend starts an interpreter and sends events in one call.
func StartAndSend[C any](interp *statekit.Interpreter[C], events ...statekit.Event) {
	interp.Start()
	SendEvents(interp, events...)
}

// StartAndSendTypes starts an interpreter and sends events by type.
func StartAndSendTypes[C any](interp *statekit.Interpreter[C], types ...statekit.EventType) {
	interp.Start()
	SendEventTypes(interp, types...)
}

// RunMachine creates an interpreter, starts it, and sends events.
// Returns the interpreter for further assertions.
func RunMachine[C any](machine *statekit.MachineConfig[C], events ...statekit.Event) *statekit.Interpreter[C] {
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	SendEvents(interp, events...)
	return interp
}

// RunMachineTypes creates an interpreter, starts it, and sends events by type.
// Returns the interpreter for further assertions.
func RunMachineTypes[C any](machine *statekit.MachineConfig[C], types ...statekit.EventType) *statekit.Interpreter[C] {
	interp := statekit.NewInterpreter(machine)
	interp.Start()
	SendEventTypes(interp, types...)
	return interp
}

// RecordMachine creates a recorder-wrapped interpreter, starts it, and returns the recorder.
func RecordMachine[C any](machine *statekit.MachineConfig[C]) *Recorder[C] {
	interp := statekit.NewInterpreter(machine)
	rec := NewRecorder(interp)
	rec.Start()
	return rec
}

// RecordAndRun creates a recorder, starts it, and sends events.
// Returns the recorder for assertions.
func RecordAndRun[C any](machine *statekit.MachineConfig[C], events ...statekit.Event) *Recorder[C] {
	rec := RecordMachine(machine)
	rec.SendAll(events...)
	return rec
}

// RecordAndRunTypes creates a recorder, starts it, and sends events by type.
// Returns the recorder for assertions.
func RecordAndRunTypes[C any](machine *statekit.MachineConfig[C], types ...statekit.EventType) *Recorder[C] {
	rec := RecordMachine(machine)
	for _, t := range types {
		rec.Send(statekit.Event{Type: t})
	}
	return rec
}

// MustBuild builds a machine and panics on error.
// Useful for test setup where build errors should fail immediately.
func MustBuild[C any](builder interface {
	Build() (*statekit.MachineConfig[C], error)
}) *statekit.MachineConfig[C] {
	machine, err := builder.Build()
	if err != nil {
		panic("MustBuild: " + err.Error())
	}
	return machine
}

// QuickMachine creates a simple linear state machine for quick tests.
// States transition in order using events named "NEXT".
// The last state is final.
func QuickMachine[C any](states ...string) *statekit.MachineConfig[C] {
	if len(states) == 0 {
		panic("QuickMachine: at least one state required")
	}

	builder := statekit.NewMachine[C]("quick_test").
		WithInitial(statekit.StateID(states[0]))

	for i, state := range states {
		stateBuilder := builder.State(statekit.StateID(state))

		if i == len(states)-1 {
			// Last state is final
			stateBuilder.Final()
		} else {
			// Transition to next state
			stateBuilder.On("NEXT").Target(statekit.StateID(states[i+1]))
		}

		builder = stateBuilder.Done()
	}

	machine, err := builder.Build()
	if err != nil {
		panic("QuickMachine: " + err.Error())
	}
	return machine
}

// QuickMachineWithEvents creates a simple state machine where each state
// transitions to the next via the provided event types.
// events slice should be len(states)-1 since last state is final.
func QuickMachineWithEvents[C any](states []string, events []statekit.EventType) *statekit.MachineConfig[C] {
	if len(states) == 0 {
		panic("QuickMachineWithEvents: at least one state required")
	}
	if len(events) != len(states)-1 {
		panic("QuickMachineWithEvents: events length must be len(states)-1")
	}

	builder := statekit.NewMachine[C]("quick_test").
		WithInitial(statekit.StateID(states[0]))

	for i, state := range states {
		stateBuilder := builder.State(statekit.StateID(state))

		if i == len(states)-1 {
			stateBuilder.Final()
		} else {
			stateBuilder.On(events[i]).Target(statekit.StateID(states[i+1]))
		}

		builder = stateBuilder.Done()
	}

	machine, err := builder.Build()
	if err != nil {
		panic("QuickMachineWithEvents: " + err.Error())
	}
	return machine
}

// ToggleMachine creates a two-state toggle machine.
// Useful for testing simple on/off or active/inactive scenarios.
func ToggleMachine[C any](stateA, stateB string, eventAtoB, eventBtoA statekit.EventType) *statekit.MachineConfig[C] {
	machine, err := statekit.NewMachine[C]("toggle").
		WithInitial(statekit.StateID(stateA)).
		State(statekit.StateID(stateA)).
		On(eventAtoB).Target(statekit.StateID(stateB)).
		Done().
		State(statekit.StateID(stateB)).
		On(eventBtoA).Target(statekit.StateID(stateA)).
		Done().
		Build()

	if err != nil {
		panic("ToggleMachine: " + err.Error())
	}
	return machine
}

// CycleMachine creates a machine that cycles through states.
// The last state transitions back to the first state.
func CycleMachine[C any](states ...string) *statekit.MachineConfig[C] {
	if len(states) < 2 {
		panic("CycleMachine: at least two states required")
	}

	builder := statekit.NewMachine[C]("cycle").
		WithInitial(statekit.StateID(states[0]))

	for i, state := range states {
		nextState := states[(i+1)%len(states)]
		builder = builder.State(statekit.StateID(state)).
			On("NEXT").Target(statekit.StateID(nextState)).
			Done()
	}

	machine, err := builder.Build()
	if err != nil {
		panic("CycleMachine: " + err.Error())
	}
	return machine
}

// BranchMachine creates a machine with a decision point.
// From startState, event1 goes to branch1 and event2 goes to branch2.
// Both branches are final states.
func BranchMachine[C any](startState, branch1, branch2 string, event1, event2 statekit.EventType) *statekit.MachineConfig[C] {
	machine, err := statekit.NewMachine[C]("branch").
		WithInitial(statekit.StateID(startState)).
		State(statekit.StateID(startState)).
		On(event1).Target(statekit.StateID(branch1)).
		On(event2).Target(statekit.StateID(branch2)).
		Done().
		State(statekit.StateID(branch1)).Final().Done().
		State(statekit.StateID(branch2)).Final().Done().
		Build()

	if err != nil {
		panic("BranchMachine: " + err.Error())
	}
	return machine
}

// ActionCounter is a helper for counting action invocations in tests.
// It is safe for concurrent use.
type ActionCounter struct {
	mu     sync.Mutex
	counts map[statekit.ActionType]int
}

// NewActionCounter creates a new action counter.
func NewActionCounter() *ActionCounter {
	return &ActionCounter{
		counts: make(map[statekit.ActionType]int),
	}
}

// Action returns an action that increments the counter for the given name.
// Deprecated: Use ActionFor[C] for type-safe actions.
func (c *ActionCounter) Action(name statekit.ActionType) func(ctx *any, e statekit.Event) {
	return func(_ *any, _ statekit.Event) {
		c.mu.Lock()
		c.counts[name]++
		c.mu.Unlock()
	}
}

// ActionFor returns a typed action that increments the counter.
func ActionFor[C any](c *ActionCounter, name statekit.ActionType) statekit.Action[C] {
	return func(_ *C, _ statekit.Event) {
		c.mu.Lock()
		c.counts[name]++
		c.mu.Unlock()
	}
}

// Count returns the number of times the action was invoked.
func (c *ActionCounter) Count(name statekit.ActionType) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[name]
}

// Reset clears all counts.
func (c *ActionCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = make(map[statekit.ActionType]int)
}

// Total returns the total number of action invocations.
func (c *ActionCounter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, count := range c.counts {
		total += count
	}
	return total
}

// GuardResult is a helper for controlling guard behavior in tests.
// It is safe for concurrent use.
type GuardResult struct {
	mu      sync.RWMutex
	results map[statekit.GuardType]bool
}

// NewGuardResult creates a new guard result controller.
func NewGuardResult() *GuardResult {
	return &GuardResult{
		results: make(map[statekit.GuardType]bool),
	}
}

// Set sets the return value for a guard.
func (g *GuardResult) Set(name statekit.GuardType, result bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.results[name] = result
}

// GuardFor returns a typed guard that returns the configured result.
func GuardFor[C any](g *GuardResult, name statekit.GuardType) statekit.Guard[C] {
	return func(_ C, _ statekit.Event) bool {
		g.mu.RLock()
		defer g.mu.RUnlock()
		result, ok := g.results[name]
		if !ok {
			return true // Default to true if not configured
		}
		return result
	}
}

// SetAll sets all guards to the same value.
func (g *GuardResult) SetAll(result bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for name := range g.results {
		g.results[name] = result
	}
}

// Reset clears all configured guard results.
func (g *GuardResult) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.results = make(map[statekit.GuardType]bool)
}
