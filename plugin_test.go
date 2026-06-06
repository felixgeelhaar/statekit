package statekit_test

import (
	"errors"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/plugin"
)

// testPlugin tracks all hook invocations for testing.
type testPlugin[C any] struct {
	name string

	// Tracking
	started   bool
	stopped   bool
	events    []plugin.Event
	enters    []plugin.StateID
	exits     []plugin.StateID
	beforeTx  []txInfo
	afterTx   []txInfo
	beforeAct []actInfo
	afterAct  []actInfo
	errors    []error
}

type txInfo struct {
	from, to plugin.StateID
	event    plugin.Event
}

type actInfo struct {
	action plugin.ActionType
	event  plugin.Event
}

func newTestPlugin[C any](name string) *testPlugin[C] {
	return &testPlugin[C]{name: name}
}

func (p *testPlugin[C]) Name() string { return p.name }

func (p *testPlugin[C]) OnStart(ctx plugin.Context[C]) {
	p.started = true
}

func (p *testPlugin[C]) OnStop(ctx plugin.Context[C]) {
	p.stopped = true
}

func (p *testPlugin[C]) OnEvent(ctx plugin.Context[C], event plugin.Event) plugin.Event {
	p.events = append(p.events, event)
	return event
}

func (p *testPlugin[C]) OnEnter(ctx plugin.Context[C], state plugin.StateID) {
	p.enters = append(p.enters, state)
}

func (p *testPlugin[C]) OnExit(ctx plugin.Context[C], state plugin.StateID) {
	p.exits = append(p.exits, state)
}

func (p *testPlugin[C]) BeforeTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
	p.beforeTx = append(p.beforeTx, txInfo{from, to, event})
}

func (p *testPlugin[C]) AfterTransition(ctx plugin.Context[C], from, to plugin.StateID, event plugin.Event) {
	p.afterTx = append(p.afterTx, txInfo{from, to, event})
}

func (p *testPlugin[C]) BeforeAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event) {
	p.beforeAct = append(p.beforeAct, actInfo{action, event})
}

func (p *testPlugin[C]) AfterAction(ctx plugin.Context[C], action plugin.ActionType, event plugin.Event) {
	p.afterAct = append(p.afterAct, actInfo{action, event})
}

func (p *testPlugin[C]) OnError(ctx plugin.Context[C], err error) {
	p.errors = append(p.errors, err)
}

func TestPlugin_OnStartStop(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)

	if plug.started {
		t.Error("OnStart called before Start()")
	}

	interp.Start()

	if !plug.started {
		t.Error("OnStart not called after Start()")
	}

	if plug.stopped {
		t.Error("OnStop called before Stop()")
	}

	interp.Stop()

	if !plug.stopped {
		t.Error("OnStop not called after Stop()")
	}
}

func TestPlugin_OnEnterExit(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("running").End().
		Done().
		State("running").
		On("STOP").Target("idle").End().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	// Should have entered "idle"
	if len(plug.enters) != 1 || plug.enters[0] != "idle" {
		t.Errorf("expected enters = [idle], got %v", plug.enters)
	}

	// Transition to running
	interp.Send(statekit.Event{Type: "GO"})

	// Should have exited "idle" and entered "running"
	if len(plug.exits) != 1 || plug.exits[0] != "idle" {
		t.Errorf("expected exits = [idle], got %v", plug.exits)
	}
	if len(plug.enters) != 2 || plug.enters[1] != "running" {
		t.Errorf("expected enters = [idle, running], got %v", plug.enters)
	}
}

func TestPlugin_OnEvent(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("running").End().
		Done().
		State("running").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	interp.Send(statekit.Event{Type: "GO", Payload: "test-payload"})

	if len(plug.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(plug.events))
	}
	if plug.events[0].Type != "GO" {
		t.Errorf("expected event type GO, got %s", plug.events[0].Type)
	}
	if plug.events[0].Payload != "test-payload" {
		t.Errorf("expected payload test-payload, got %v", plug.events[0].Payload)
	}
}

func TestPlugin_BeforeAfterTransition(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("running").End().
		Done().
		State("running").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	interp.Send(statekit.Event{Type: "GO"})

	// Check BeforeTransition
	if len(plug.beforeTx) != 1 {
		t.Fatalf("expected 1 beforeTx, got %d", len(plug.beforeTx))
	}
	if plug.beforeTx[0].from != "idle" || plug.beforeTx[0].to != "running" {
		t.Errorf("expected transition idle→running, got %s→%s",
			plug.beforeTx[0].from, plug.beforeTx[0].to)
	}

	// Check AfterTransition
	if len(plug.afterTx) != 1 {
		t.Fatalf("expected 1 afterTx, got %d", len(plug.afterTx))
	}
	if plug.afterTx[0].from != "idle" || plug.afterTx[0].to != "running" {
		t.Errorf("expected transition idle→running, got %s→%s",
			plug.afterTx[0].from, plug.afterTx[0].to)
	}
}

func TestPlugin_BeforeAfterAction(t *testing.T) {
	t.Parallel()
	type ctx struct{ count int }

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		WithAction("increment", func(c *ctx, e statekit.Event) {
			c.count++
		}).
		State("idle").
		OnEntry("increment").
		On("GO").Do("increment").Target("running").End().
		Done().
		State("running").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	// Entry action for "idle"
	if len(plug.beforeAct) != 1 || plug.beforeAct[0].action != "increment" {
		t.Errorf("expected beforeAct = [increment], got %v", plug.beforeAct)
	}
	if len(plug.afterAct) != 1 || plug.afterAct[0].action != "increment" {
		t.Errorf("expected afterAct = [increment], got %v", plug.afterAct)
	}

	// Transition with action
	interp.Send(statekit.Event{Type: "GO"})

	// Should now have 2 actions: initial entry + transition action
	if len(plug.beforeAct) != 2 {
		t.Errorf("expected 2 beforeAct, got %d", len(plug.beforeAct))
	}
	if len(plug.afterAct) != 2 {
		t.Errorf("expected 2 afterAct, got %d", len(plug.afterAct))
	}
}

func TestPlugin_OnError_ActionPanic(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		WithAction("panics", func(c *ctx, e statekit.Event) {
			panic("test panic")
		}).
		State("idle").
		On("PANIC").Do("panics").Target("idle").End().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	// Should not panic externally
	interp.Send(statekit.Event{Type: "PANIC"})

	// Should have received error
	if len(plug.errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(plug.errors))
	}
	if plug.errors[0] == nil || !errors.Is(plug.errors[0], plug.errors[0]) {
		t.Error("expected non-nil error")
	}
}

// eventModifyingPlugin modifies events that pass through.
type eventModifyingPlugin[C any] struct{}

func (p *eventModifyingPlugin[C]) Name() string { return "event-modifier" }

func (p *eventModifyingPlugin[C]) OnEvent(ctx plugin.Context[C], event plugin.Event) plugin.Event {
	// Modify payload
	event.Payload = "modified"
	return event
}

func TestPlugin_EventModification(t *testing.T) {
	t.Parallel()
	type ctx struct{ payload any }

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		WithAction("capture", func(c *ctx, e statekit.Event) {
			c.payload = e.Payload
		}).
		State("idle").
		On("TEST").Do("capture").Target("idle").End().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Use(&eventModifyingPlugin[ctx]{})
	interp.Start()

	interp.Send(statekit.Event{Type: "TEST", Payload: "original"})

	state := interp.State()
	if state.Context.payload != "modified" {
		t.Errorf("expected payload = modified, got %v", state.Context.payload)
	}
}

func TestPlugin_Composite(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug1 := newTestPlugin[ctx]("plugin-1")
	plug2 := newTestPlugin[ctx]("plugin-2")

	composite := plugin.NewComposite[ctx](plug1, plug2)

	interp := statekit.NewInterpreter(machine)
	interp.Use(composite)
	interp.Start()

	if !plug1.started {
		t.Error("plugin-1 OnStart not called")
	}
	if !plug2.started {
		t.Error("plugin-2 OnStart not called")
	}
}

func TestPlugin_MultiplePlugins(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("idle").
		State("idle").
		On("GO").Target("running").End().
		Done().
		State("running").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug1 := newTestPlugin[ctx]("plugin-1")
	plug2 := newTestPlugin[ctx]("plugin-2")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug1)
	interp.Use(plug2)
	interp.Start()

	interp.Send(statekit.Event{Type: "GO"})

	// Both plugins should have received the event
	if len(plug1.events) != 1 {
		t.Errorf("plugin-1 expected 1 event, got %d", len(plug1.events))
	}
	if len(plug2.events) != 1 {
		t.Errorf("plugin-2 expected 1 event, got %d", len(plug2.events))
	}
}

func TestPlugin_HierarchicalStates(t *testing.T) {
	t.Parallel()
	type ctx struct{}

	machine, err := statekit.NewMachine[ctx]("test").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		State("idle").
		On("GO").Target("working").End().
		End().
		State("working").
		On("STOP").Target("idle").End().
		End().
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	plug := newTestPlugin[ctx]("test-plugin")

	interp := statekit.NewInterpreter(machine)
	interp.Use(plug)
	interp.Start()

	// Should have entered both "active" and "idle"
	if len(plug.enters) != 2 {
		t.Fatalf("expected 2 enters, got %d: %v", len(plug.enters), plug.enters)
	}
	if plug.enters[0] != "active" || plug.enters[1] != "idle" {
		t.Errorf("expected enters = [active, idle], got %v", plug.enters)
	}

	// Transition from idle to working
	interp.Send(statekit.Event{Type: "GO"})

	// Should have exited "idle" only, entered "working"
	if len(plug.exits) != 1 || plug.exits[0] != "idle" {
		t.Errorf("expected exits = [idle], got %v", plug.exits)
	}
	if len(plug.enters) != 3 || plug.enters[2] != "working" {
		t.Errorf("expected enters = [active, idle, working], got %v", plug.enters)
	}
}
