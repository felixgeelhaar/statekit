package statekit

import (
	"testing"
)

// Benchmark context
type BenchContext struct {
	Count int
}

// BenchmarkReflection_BuildTime benchmarks machine construction from struct
func BenchmarkReflection_BuildTime(b *testing.B) {
	type BenchMachine struct {
		MachineDef `id:"bench" initial:"idle"`
		Idle       StateNode `on:"START->running:canStart" entry:"onEntry" exit:"onExit"`
		Running    StateNode `on:"STOP->idle" entry:"onEntry"`
	}

	registry := NewActionRegistry[BenchContext]().
		WithAction("onEntry", func(ctx *BenchContext, e Event) { ctx.Count++ }).
		WithAction("onExit", func(ctx *BenchContext, e Event) { ctx.Count-- }).
		WithGuard("canStart", func(ctx BenchContext, e Event) bool { return ctx.Count > 0 })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FromStruct[BenchMachine, BenchContext](registry)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBuilder_BuildTime benchmarks machine construction with builder
func BenchmarkBuilder_BuildTime(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewMachine[BenchContext]("bench").
			WithInitial("idle").
			WithAction("onEntry", func(ctx *BenchContext, e Event) { ctx.Count++ }).
			WithAction("onExit", func(ctx *BenchContext, e Event) { ctx.Count-- }).
			WithGuard("canStart", func(ctx BenchContext, e Event) bool { return ctx.Count > 0 }).
			State("idle").
			OnEntry("onEntry").
			OnExit("onExit").
			On("START").Target("running").Guard("canStart").
			Done().
			State("running").
			OnEntry("onEntry").
			On("STOP").Target("idle").
			Done().
			Build()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInterpreter_Send_Reflection compares runtime with reflection-built machine
func BenchmarkInterpreter_Send_Reflection(b *testing.B) {
	type BenchMachine struct {
		MachineDef `id:"bench" initial:"idle"`
		Idle       StateNode `on:"START->running"`
		Running    StateNode `on:"STOP->idle"`
	}

	registry := NewActionRegistry[BenchContext]()
	machine, _ := FromStruct[BenchMachine, BenchContext](registry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp := NewInterpreter(machine)
		interp.Start()
		interp.Send(Event{Type: "START"})
		interp.Send(Event{Type: "STOP"})
	}
}

// BenchmarkInterpreter_Send_Builder compares runtime with builder-built machine
func BenchmarkInterpreter_Send_Builder(b *testing.B) {
	machine, _ := NewMachine[BenchContext]("bench").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").
		On("STOP").Target("idle").
		Done().
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp := NewInterpreter(machine)
		interp.Start()
		interp.Send(Event{Type: "START"})
		interp.Send(Event{Type: "STOP"})
	}
}

// BenchmarkInterpreter_Send_HotPath benchmarks the hot path (Send) only
func BenchmarkInterpreter_Send_HotPath(b *testing.B) {
	machine, _ := NewMachine[BenchContext]("bench").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		Done().
		State("running").
		On("STOP").Target("idle").
		Done().
		Build()

	interp := NewInterpreter(machine)
	interp.Start()

	events := []Event{
		{Type: "START"},
		{Type: "STOP"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp.Send(events[i%2])
	}
}

// BenchmarkReflection_Hierarchical tests hierarchical state performance
func BenchmarkReflection_Hierarchical(b *testing.B) {
	type ChildState struct {
		StateNode `on:"NEXT->sibling"`
	}

	type SiblingState struct {
		StateNode `on:"BACK->child"`
	}

	type ParentState struct {
		CompoundNode `initial:"child" on:"RESET->done"`
		Child        ChildState
		Sibling      SiblingState
	}

	type HierarchicalMachine struct {
		MachineDef `id:"hierarchical" initial:"parent"`
		Parent     ParentState
		Done       FinalNode
	}

	registry := NewActionRegistry[BenchContext]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FromStruct[HierarchicalMachine, BenchContext](registry)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParser_ToSnakeCase benchmarks the snake_case conversion
func BenchmarkParser_ToSnakeCase(b *testing.B) {
	testCases := []string{
		"SimpleState",
		"VeryLongStateName",
		"HTTPSConnection",
		"XMLParser",
		"a",
		"AB",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = toSnakeCase(tc)
		}
	}
}

// BenchmarkStringParsing tests transition parsing overhead
func BenchmarkStringParsing_SimpleTransition(b *testing.B) {
	type BenchMachine struct {
		MachineDef `id:"bench" initial:"a"`
		A          StateNode `on:"E->b"`
		B          StateNode
	}

	registry := NewActionRegistry[BenchContext]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FromStruct[BenchMachine, BenchContext](registry)
	}
}

// BenchmarkStringParsing_ComplexTransition tests complex parsing
func BenchmarkStringParsing_ComplexTransition(b *testing.B) {
	type BenchMachine struct {
		MachineDef `id:"bench" initial:"a"`
		A          StateNode `on:"E1->b/action1;action2:guard,E2->c/action3:guard2"`
		B          StateNode
		C          StateNode
	}

	registry := NewActionRegistry[BenchContext]().
		WithAction("action1", func(ctx *BenchContext, e Event) {}).
		WithAction("action2", func(ctx *BenchContext, e Event) {}).
		WithAction("action3", func(ctx *BenchContext, e Event) {}).
		WithGuard("guard", func(ctx BenchContext, e Event) bool { return true }).
		WithGuard("guard2", func(ctx BenchContext, e Event) bool { return true })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FromStruct[BenchMachine, BenchContext](registry)
	}
}

// BenchmarkMemoryAllocations tests memory efficiency
func BenchmarkMemoryAllocations_SmallMachine(b *testing.B) {
	type SmallMachine struct {
		MachineDef `id:"small" initial:"idle"`
		Idle       StateNode `on:"GO->running"`
		Running    StateNode `on:"STOP->idle"`
	}

	registry := NewActionRegistry[BenchContext]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine, _ := FromStruct[SmallMachine, BenchContext](registry)
		_ = machine
	}
}

// BenchmarkMemoryAllocations_LargeMachine tests memory with many states
func BenchmarkMemoryAllocations_LargeMachine(b *testing.B) {
	type LargeMachine struct {
		MachineDef `id:"large" initial:"s1"`
		S1         StateNode `on:"E->s2"`
		S2         StateNode `on:"E->s3"`
		S3         StateNode `on:"E->s4"`
		S4         StateNode `on:"E->s5"`
		S5         StateNode `on:"E->s6"`
		S6         StateNode `on:"E->s7"`
		S7         StateNode `on:"E->s8"`
		S8         StateNode `on:"E->s9"`
		S9         StateNode `on:"E->s10"`
		S10        StateNode `on:"E->s1"`
	}

	registry := NewActionRegistry[BenchContext]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine, _ := FromStruct[LargeMachine, BenchContext](registry)
		_ = machine
	}
}

// Helper for snake_case conversion (copied from internal/reflect/parser.go for benchmarking)
func toSnakeCase(s string) string {
	result := make([]byte, 0, len(s)*2)
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, byte(r+32))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}
