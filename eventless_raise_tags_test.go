package statekit

import "testing"

// --- Tags ---

func TestTags_HasTagForActiveAndAncestors(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{}]("tags").
		WithInitial("active").
		State("active").
		Tags("running").
		WithInitial("loading").
		State("loading").Tags("busy", "io").End().
		State("ready").Tags("idle").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()

	// Leaf "loading" tags
	if !i.HasTag("busy") {
		t.Errorf("expected HasTag(busy) on leaf")
	}
	if !i.HasTag("io") {
		t.Errorf("expected HasTag(io) on leaf")
	}
	// Ancestor "active" tag visible from leaf
	if !i.HasTag("running") {
		t.Errorf("expected HasTag(running) from ancestor")
	}
	// Tag of a sibling (not active) must not match
	if i.HasTag("idle") {
		t.Errorf("did not expect HasTag(idle) — sibling not active")
	}
	// Unknown tag
	if i.HasTag("nope") {
		t.Errorf("did not expect HasTag(nope)")
	}
}

// --- Always (eventless transitions) ---

func TestAlways_FiresOnEntryWhenGuardPasses(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{ Ok bool }]("always").
		WithInitial("check").
		WithGuard("ok", func(c struct{ Ok bool }, e Event) bool { return c.Ok }).
		WithContext(struct{ Ok bool }{Ok: true}).
		State("check").
		Always().Target("approved").Guard("ok").End().
		Always().Target("rejected").End(). // fallback (guardless)
		Done().
		State("approved").Final().Done().
		State("rejected").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	if got := i.State().Value; got != "approved" {
		t.Errorf("state = %q, want approved", got)
	}
}

func TestAlways_FallbackWhenGuardFails(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{ Ok bool }]("always").
		WithInitial("check").
		WithGuard("ok", func(c struct{ Ok bool }, e Event) bool { return c.Ok }).
		WithContext(struct{ Ok bool }{Ok: false}).
		State("check").
		Always().Target("approved").Guard("ok").End().
		Always().Target("rejected").End().
		Done().
		State("approved").Final().Done().
		State("rejected").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	if got := i.State().Value; got != "rejected" {
		t.Errorf("state = %q, want rejected", got)
	}
}

func TestAlways_ChainsThroughMultipleStates(t *testing.T) {
	t.Parallel()
	// a -> b -> c all via eventless transitions in a single macrostep
	m, err := NewMachine[struct{}]("chain").
		WithInitial("a").
		State("a").Always().Target("b").End().Done().
		State("b").Always().Target("c").End().Done().
		State("c").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	if got := i.State().Value; got != "c" {
		t.Errorf("state = %q, want c", got)
	}
}

func TestAlways_FiresAfterEventTransition(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{}]("evt").
		WithInitial("idle").
		State("idle").On("GO").Target("transient").End().Done().
		State("transient").Always().Target("done").End().Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	if got := i.State().Value; got != "idle" {
		t.Fatalf("state = %q, want idle", got)
	}
	i.Send(Event{Type: "GO"})
	if got := i.State().Value; got != "done" {
		t.Errorf("state = %q, want done (transient should auto-advance)", got)
	}
}

// --- Raise (internal self-event) ---

func TestRaise_ProcessedInSameMacrostep(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{}]("raise").
		WithInitial("idle").
		State("idle").On("START").Target("middle").Raise("NEXT").End().Done().
		State("middle").On("NEXT").Target("done").End().Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "START"})
	if got := i.State().Value; got != "done" {
		t.Errorf("state = %q, want done (raised NEXT should be processed)", got)
	}
}

func TestRaise_FromAlwaysTransition(t *testing.T) {
	t.Parallel()
	m, err := NewMachine[struct{}]("raise2").
		WithInitial("start").
		State("start").Always().Target("waiting").Raise("PING").End().Done().
		State("waiting").On("PING").Target("done").End().Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	i := NewInterpreter(m)
	i.Start()
	if got := i.State().Value; got != "done" {
		t.Errorf("state = %q, want done (always→raise→PING)", got)
	}
}

// --- Validation ---

func TestAlways_RequiresTarget(t *testing.T) {
	t.Parallel()
	_, err := NewMachine[struct{}]("bad").
		WithInitial("a").
		State("a").Always().End().Done().
		State("b").Final().Done().
		Build()
	if err == nil {
		t.Errorf("expected validation error for always transition without target")
	}
}
