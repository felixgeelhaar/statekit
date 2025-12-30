package statekit

import (
	"testing"
	"time"
)

// BenchmarkMachineBuild benchmarks machine construction
func BenchmarkMachineBuild(b *testing.B) {
	b.Run("simple_3_states", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = NewMachine[struct{}]("traffic").
				WithInitial("green").
				State("green").On("TIMER").Target("yellow").Done().
				State("yellow").On("TIMER").Target("red").Done().
				State("red").On("TIMER").Target("green").Done().
				Build()
		}
	})

	b.Run("hierarchical_5_states", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = NewMachine[struct{}]("editor").
				WithInitial("editing").
				State("editing").
				WithInitial("idle").
				On("SAVE").Target("saved").End().
				State("idle").On("TYPE").Target("dirty").End().End().
				State("dirty").On("CLEAR").Target("idle").End().End().
				Done().
				State("saved").Final().Done().
				Build()
		}
	})

	b.Run("with_actions_guards", func(b *testing.B) {
		type Ctx struct{ Count int }
		for i := 0; i < b.N; i++ {
			_, _ = NewMachine[Ctx]("counter").
				WithInitial("idle").
				WithContext(Ctx{Count: 0}).
				WithAction("inc", func(ctx *Ctx, e Event) { ctx.Count++ }).
				WithGuard("hasCount", func(ctx Ctx, e Event) bool { return ctx.Count > 0 }).
				State("idle").OnEntry("inc").On("GO").Target("running").Guard("hasCount").Done().
				State("running").On("STOP").Target("idle").Done().
				Build()
		}
	})
}

// BenchmarkInterpreterStart benchmarks interpreter initialization
func BenchmarkInterpreterStart(b *testing.B) {
	machine, _ := NewMachine[struct{}]("traffic").
		WithInitial("green").
		State("green").On("TIMER").Target("yellow").Done().
		State("yellow").On("TIMER").Target("red").Done().
		State("red").On("TIMER").Target("green").Done().
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp := NewInterpreter(machine)
		interp.Start()
	}
}

// BenchmarkInterpreterSend benchmarks event processing
func BenchmarkInterpreterSend(b *testing.B) {
	b.Run("simple_transition", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("traffic").
			WithInitial("green").
			State("green").On("TIMER").Target("yellow").Done().
			State("yellow").On("TIMER").Target("red").Done().
			State("red").On("TIMER").Target("green").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		event := Event{Type: "TIMER"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(event)
		}
	})

	b.Run("with_guard_pass", func(b *testing.B) {
		type Ctx struct{ Count int }
		machine, _ := NewMachine[Ctx]("guarded").
			WithInitial("idle").
			WithContext(Ctx{Count: 10}).
			WithGuard("hasCount", func(ctx Ctx, e Event) bool { return ctx.Count > 0 }).
			State("idle").On("GO").Target("running").Guard("hasCount").Done().
			State("running").On("STOP").Target("idle").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		goEvent := Event{Type: "GO"}
		stopEvent := Event{Type: "STOP"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				interp.Send(goEvent)
			} else {
				interp.Send(stopEvent)
			}
		}
	})

	b.Run("with_entry_action", func(b *testing.B) {
		type Ctx struct{ Count int }
		machine, _ := NewMachine[Ctx]("counter").
			WithInitial("idle").
			WithAction("inc", func(ctx *Ctx, e Event) { ctx.Count++ }).
			State("idle").OnEntry("inc").On("GO").Target("idle").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		event := Event{Type: "GO"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(event)
		}
	})

	b.Run("hierarchical_bubble", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("nested").
			WithInitial("parent").
			State("parent").
			WithInitial("child").
			On("RESET").Target("parent").End().
			State("child").On("LOCAL").Target("child").End().End().
			Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		resetEvent := Event{Type: "RESET"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(resetEvent)
		}
	})

	b.Run("no_matching_transition", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("simple").
			WithInitial("idle").
			State("idle").On("GO").Target("running").Done().
			State("running").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		event := Event{Type: "UNKNOWN"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(event)
		}
	})
}

// BenchmarkInterpreterMatches benchmarks state matching
func BenchmarkInterpreterMatches(b *testing.B) {
	b.Run("direct_match", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("simple").
			WithInitial("idle").
			State("idle").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = interp.Matches("idle")
		}
	})

	b.Run("ancestor_match", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("nested").
			WithInitial("parent").
			State("parent").
			WithInitial("child").
			State("child").End().
			Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = interp.Matches("parent")
		}
	})

	b.Run("deep_ancestor_match", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("deep").
			WithInitial("l1").
			State("l1").
			WithInitial("l2").
			State("l2").
			WithInitial("l3").
			State("l3").
			WithInitial("l4").
			State("l4").End().
			End().
			End().
			Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = interp.Matches("l1")
		}
	})
}

// BenchmarkInterpreterState benchmarks state retrieval
func BenchmarkInterpreterState(b *testing.B) {
	type Ctx struct {
		Count int
		Name  string
		Items []string
	}

	machine, _ := NewMachine[Ctx]("bench").
		WithInitial("idle").
		WithContext(Ctx{Count: 42, Name: "test", Items: []string{"a", "b", "c"}}).
		State("idle").Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = interp.State()
	}
}

// BenchmarkDelayedTransition benchmarks delayed transition setup/teardown
func BenchmarkDelayedTransition(b *testing.B) {
	b.Run("setup_and_cancel", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("delayed").
			WithInitial("waiting").
			State("waiting").
			After(time.Hour).Target("timeout"). // Long delay, won't fire
			On("CANCEL").Target("cancelled").
			Done().
			State("timeout").Done().
			State("cancelled").Done().
			Build()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp := NewInterpreter(machine)
			interp.Start()
			interp.Send(Event{Type: "CANCEL"}) // Cancels timer
			interp.Stop()
		}
	})
}

// BenchmarkParallelStates benchmarks parallel state operations
func BenchmarkParallelStates(b *testing.B) {
	b.Run("enter_parallel", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("parallel").
			WithInitial("active").
			State("active").Parallel().
			Region("r1").WithInitial("a").
			State("a").On("X").Target("b").EndState().
			State("b").EndState().
			EndRegion().
			Region("r2").WithInitial("c").
			State("c").On("Y").Target("d").EndState().
			State("d").EndState().
			EndRegion().
			Done().
			Build()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp := NewInterpreter(machine)
			interp.Start()
		}
	})

	b.Run("send_to_parallel", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("parallel").
			WithInitial("active").
			State("active").Parallel().
			Region("r1").WithInitial("a").
			State("a").On("TOGGLE").Target("b").EndState().
			State("b").On("TOGGLE").Target("a").EndState().
			EndRegion().
			Region("r2").WithInitial("c").
			State("c").On("FLIP").Target("d").EndState().
			State("d").On("FLIP").Target("c").EndState().
			EndRegion().
			Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		toggle := Event{Type: "TOGGLE"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(toggle)
		}
	})
}

// BenchmarkPluginHooks benchmarks plugin hook overhead (v0.14)
func BenchmarkPluginHooks(b *testing.B) {
	b.Run("no_plugins", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("simple").
			WithInitial("idle").
			State("idle").On("GO").Target("running").Done().
			State("running").On("STOP").Target("idle").Done().
			Build()

		interp := NewInterpreter(machine)
		interp.Start()
		event := Event{Type: "GO"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(event)
			interp.Send(Event{Type: "STOP"})
		}
	})

	b.Run("with_logging_plugin", func(b *testing.B) {
		machine, _ := NewMachine[struct{}]("simple").
			WithInitial("idle").
			State("idle").On("GO").Target("running").Done().
			State("running").On("STOP").Target("idle").Done().
			Build()

		interp := NewInterpreter(machine)
		// Add a minimal logging plugin
		interp.Use(&benchLoggingPlugin{})
		interp.Start()
		event := Event{Type: "GO"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			interp.Send(event)
			interp.Send(Event{Type: "STOP"})
		}
	})
}

// benchLoggingPlugin is a minimal plugin for benchmarking
type benchLoggingPlugin struct{}

func (p *benchLoggingPlugin) Name() string { return "bench-logger" }

// BenchmarkUpdateContext benchmarks context update performance
func BenchmarkUpdateContext(b *testing.B) {
	type Ctx struct {
		Count int
		Items []int
	}

	machine, _ := NewMachine[Ctx]("bench").
		WithInitial("idle").
		WithContext(Ctx{Count: 0, Items: make([]int, 0, 100)}).
		State("idle").Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp.UpdateContext(func(ctx *Ctx) {
			ctx.Count++
		})
	}
}

// BenchmarkSnapshot benchmarks snapshot creation (v0.5+)
func BenchmarkSnapshot(b *testing.B) {
	type Ctx struct {
		Count int
		Data  string
	}

	machine, _ := NewMachine[Ctx]("bench").
		WithInitial("idle").
		WithContext(Ctx{Count: 42, Data: "benchmark"}).
		State("idle").On("GO").Target("running").Done().
		State("running").On("STOP").Target("idle").Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = interp.Snapshot()
	}
}
