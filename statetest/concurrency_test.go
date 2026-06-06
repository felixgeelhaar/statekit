package statetest_test

import (
	"sync"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/statetest"
)

func TestActionCounter_Concurrent(t *testing.T) {
	counter := statetest.NewActionCounter()
	action := statetest.ActionFor[struct{}](counter, "concurrent")

	const goroutines = 100
	const increments = 100
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				action(nil, statekit.Event{})
			}
		}()
	}
	wg.Wait()

	expected := goroutines * increments
	if counter.Count("concurrent") != expected {
		t.Errorf("expected count %d, got %d", expected, counter.Count("concurrent"))
	}
}

func TestActionCounter_ConcurrentMultipleActions(t *testing.T) {
	counter := statetest.NewActionCounter()
	action1 := statetest.ActionFor[struct{}](counter, "action1")
	action2 := statetest.ActionFor[struct{}](counter, "action2")

	const goroutines = 50
	const increments = 50
	var wg sync.WaitGroup

	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				action1(nil, statekit.Event{})
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				action2(nil, statekit.Event{})
			}
		}()
	}
	wg.Wait()

	expected := goroutines * increments
	if counter.Count("action1") != expected {
		t.Errorf("action1: expected count %d, got %d", expected, counter.Count("action1"))
	}
	if counter.Count("action2") != expected {
		t.Errorf("action2: expected count %d, got %d", expected, counter.Count("action2"))
	}
	if counter.Total() != expected*2 {
		t.Errorf("total: expected %d, got %d", expected*2, counter.Total())
	}
}

func TestGuardResult_Concurrent(t *testing.T) {
	guards := statetest.NewGuardResult()
	guard := statetest.GuardFor[struct{}](guards, "canProceed")

	const goroutines = 100
	var wg sync.WaitGroup

	// Concurrent reads and writes
	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			// Writer
			guards.Set("canProceed", i%2 == 0)
		}(i)
		go func() {
			defer wg.Done()
			// Reader
			_ = guard(struct{}{}, statekit.Event{})
		}()
	}
	wg.Wait()

	// Verify no panic and guard still works
	guards.Set("canProceed", true)
	if !guard(struct{}{}, statekit.Event{}) {
		t.Error("expected guard to return true")
	}
	guards.Set("canProceed", false)
	if guard(struct{}{}, statekit.Event{}) {
		t.Error("expected guard to return false")
	}
}

func TestRecorder_ConcurrentRead(t *testing.T) {
	machine := buildTestMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetest.NewRecorder(interp)
	rec.Start()

	const goroutines = 100
	var wg sync.WaitGroup

	// Send some events first
	rec.Send(statekit.Event{Type: "START"})
	rec.Send(statekit.Event{Type: "PAUSE"})

	// Concurrent reads should not panic
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = rec.State()
			_ = rec.Matches("running")
			_ = rec.Done()
			_ = rec.Transitions()
			_ = rec.TransitionCount()
			_ = rec.States()
			_ = rec.UniqueStates()
			_ = rec.Events()
			_ = rec.EventTypes()
		}()
	}
	wg.Wait()

	// Verify state is still correct
	if rec.State().Value != "paused" {
		t.Errorf("expected 'paused', got %q", rec.State().Value)
	}
}

func TestRecorder_ConcurrentSendAndRead(t *testing.T) {
	machine := buildCycleMachine()
	interp := statekit.NewInterpreter(machine)
	rec := statetest.NewRecorder(interp)
	rec.Start()

	const sends = 50
	const readers = 10
	var wg sync.WaitGroup

	wg.Add(1 + readers)

	// Single sender to avoid race on machine state
	go func() {
		defer wg.Done()
		for i := 0; i < sends; i++ {
			rec.Send(statekit.Event{Type: "NEXT"})
		}
	}()

	// Multiple concurrent readers
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < sends*2; j++ {
				_ = rec.State()
				_ = rec.Transitions()
				_ = rec.TransitionCount()
			}
		}()
	}
	wg.Wait()

	// Verify transitions were recorded
	// Start + sends
	expectedTransitions := 1 + sends
	if rec.TransitionCount() != expectedTransitions {
		t.Errorf("expected %d transitions, got %d", expectedTransitions, rec.TransitionCount())
	}
}

// buildCycleMachine creates a machine that cycles through states for testing
func buildCycleMachine() *statekit.MachineConfig[struct{}] {
	machine, err := statekit.NewMachine[struct{}]("cycle").
		WithInitial("a").
		State("a").On("NEXT").Target("b").Done().
		State("b").On("NEXT").Target("c").Done().
		State("c").On("NEXT").Target("a").Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}
