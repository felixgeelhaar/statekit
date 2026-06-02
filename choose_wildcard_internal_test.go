package statekit

import "testing"

// --- Choose (conditional action combinator) ---

func TestChoose_RunsFirstMatchingBranch(t *testing.T) {
	type Ctx struct {
		N    int
		Tier string
	}

	pick := Choose(
		ChooseBranch[Ctx]{
			When: func(c Ctx, _ Event) bool { return c.N >= 100 },
			Then: func(c *Ctx, _ Event) { c.Tier = "gold" },
		},
		ChooseBranch[Ctx]{
			When: func(c Ctx, _ Event) bool { return c.N >= 10 },
			Then: func(c *Ctx, _ Event) { c.Tier = "silver" },
		},
		ChooseBranch[Ctx]{
			// nil When = else branch
			Then: func(c *Ctx, _ Event) { c.Tier = "bronze" },
		},
	)

	cases := []struct {
		n    int
		want string
	}{
		{150, "gold"},
		{50, "silver"},
		{1, "bronze"},
	}
	for _, tc := range cases {
		c := Ctx{N: tc.n}
		pick(&c, Event{})
		if c.Tier != tc.want {
			t.Errorf("N=%d tier=%q want %q", tc.n, c.Tier, tc.want)
		}
	}
}

func TestChoose_NoBranchMatchesIsNoop(t *testing.T) {
	type Ctx struct{ Hit bool }
	pick := Choose(ChooseBranch[Ctx]{
		When: func(c Ctx, _ Event) bool { return false },
		Then: func(c *Ctx, _ Event) { c.Hit = true },
	})
	c := Ctx{}
	pick(&c, Event{})
	if c.Hit {
		t.Errorf("expected no branch to run")
	}
}

func TestChoose_WiresIntoTransition(t *testing.T) {
	type Ctx struct {
		Score int
		Label string
	}
	m, err := NewMachine[Ctx]("choose").
		WithInitial("idle").
		WithContext(Ctx{Score: 42}).
		WithAction("classify", Choose(
			ChooseBranch[Ctx]{
				When: func(c Ctx, _ Event) bool { return c.Score >= 40 },
				Then: func(c *Ctx, _ Event) { c.Label = "pass" },
			},
			ChooseBranch[Ctx]{Then: func(c *Ctx, _ Event) { c.Label = "fail" }},
		)).
		State("idle").On("GRADE").Target("done").Do("classify").End().Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "GRADE"})
	if got := i.State().Context.Label; got != "pass" {
		t.Errorf("label = %q, want pass", got)
	}
}

// --- Wildcard event ---

func TestWildcard_MatchesUnhandledEvent(t *testing.T) {
	m, err := NewMachine[struct{}]("wild").
		WithInitial("a").
		State("a").
		On("KNOWN").Target("known").End().
		On("*").Target("fallback").End().
		Done().
		State("known").Final().Done().
		State("fallback").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "SOMETHING_ELSE"})
	if got := i.State().Value; got != "fallback" {
		t.Errorf("state = %q, want fallback", got)
	}
}

func TestWildcard_ExactMatchTakesPriority(t *testing.T) {
	m, _ := NewMachine[struct{}]("wild2").
		WithInitial("a").
		State("a").
		On("KNOWN").Target("known").End().
		On("*").Target("fallback").End().
		Done().
		State("known").Final().Done().
		State("fallback").Final().Done().
		Build()
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "KNOWN"})
	if got := i.State().Value; got != "known" {
		t.Errorf("state = %q, want known (exact beats wildcard)", got)
	}
}

func TestWildcard_RespectsGuard(t *testing.T) {
	m, _ := NewMachine[struct{ Allow bool }]("wild3").
		WithInitial("a").
		WithGuard("allow", func(c struct{ Allow bool }, _ Event) bool { return c.Allow }).
		WithContext(struct{ Allow bool }{Allow: false}).
		State("a").On("*").Target("b").Guard("allow").End().Done().
		State("b").Final().Done().
		Build()
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "ANY"})
	if got := i.State().Value; got != "a" {
		t.Errorf("state = %q, want a (guard blocks wildcard)", got)
	}
}

// --- Internal transitions ---

func TestInternal_NoExitEntryOrStateChange(t *testing.T) {
	type Ctx struct {
		Entries int
		Exits   int
		Pings   int
	}
	m, err := NewMachine[Ctx]("internal").
		WithInitial("active").
		WithAction("onEntry", func(c *Ctx, _ Event) { c.Entries++ }).
		WithAction("onExit", func(c *Ctx, _ Event) { c.Exits++ }).
		WithAction("onPing", func(c *Ctx, _ Event) { c.Pings++ }).
		State("active").
		OnEntry("onEntry").
		OnExit("onExit").
		On("PING").Internal().Do("onPing").End().
		Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	i := NewInterpreter(m)
	i.Start() // 1 entry
	i.Send(Event{Type: "PING"})
	i.Send(Event{Type: "PING"})

	st := i.State()
	if st.Value != "active" {
		t.Errorf("state = %q, want active", st.Value)
	}
	if st.Context.Pings != 2 {
		t.Errorf("pings = %d, want 2", st.Context.Pings)
	}
	if st.Context.Entries != 1 {
		t.Errorf("entries = %d, want 1 (internal must not re-enter)", st.Context.Entries)
	}
	if st.Context.Exits != 0 {
		t.Errorf("exits = %d, want 0 (internal must not exit)", st.Context.Exits)
	}
}

func TestInternal_ContrastsWithExternalSelfTransition(t *testing.T) {
	type Ctx struct{ Entries, Exits int }
	m, _ := NewMachine[Ctx]("ext").
		WithInitial("active").
		WithAction("onEntry", func(c *Ctx, _ Event) { c.Entries++ }).
		WithAction("onExit", func(c *Ctx, _ Event) { c.Exits++ }).
		State("active").
		OnEntry("onEntry").
		OnExit("onExit").
		On("SELF").Target("active").End(). // external self-transition (re-enters)
		Done().
		Build()
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "SELF"})
	st := i.State()
	if st.Context.Exits != 1 || st.Context.Entries != 2 {
		t.Errorf("external self: entries=%d exits=%d, want entries=2 exits=1", st.Context.Entries, st.Context.Exits)
	}
}

func TestInternal_AllowsEmptyTarget(t *testing.T) {
	type Ctx struct{ Count int }
	m, err := NewMachine[Ctx]("internal2").
		WithInitial("s").
		WithAction("bump", func(c *Ctx, _ Event) { c.Count++ }).
		State("s").On("BUMP").Internal().Do("bump").End().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	i := NewInterpreter(m)
	i.Start()
	i.Send(Event{Type: "BUMP"})
	if i.State().Context.Count != 1 {
		t.Errorf("count = %d, want 1", i.State().Context.Count)
	}
	if i.State().Value != "s" {
		t.Errorf("state = %q, want s", i.State().Value)
	}
}
