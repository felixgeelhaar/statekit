package main

import (
	"testing"

	"go.klarlabs.de/statekit"
)

func TestWorkerMachine(t *testing.T) {
	machine := buildWorkerMachine()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "idle" {
		t.Errorf("expected initial state 'idle', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "TASK", Payload: "test"})
	if interp.State().Value != "working" {
		t.Errorf("expected state 'working', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "COMPLETE"})
	if interp.State().Value != "done" {
		t.Errorf("expected state 'done', got %s", interp.State().Value)
	}

	if !interp.Done() {
		t.Error("expected interpreter to be done")
	}

	interp.Stop()
}

func TestSupervisorMachine(t *testing.T) {
	machine := buildSupervisor()

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	if interp.State().Value != "idle" {
		t.Errorf("expected initial state 'idle', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "BEGIN"})
	if interp.State().Value != "supervising" {
		t.Errorf("expected state 'supervising', got %s", interp.State().Value)
	}

	interp.Send(statekit.Event{Type: "STOP"})
	if interp.State().Value != "stopped" {
		t.Errorf("expected state 'stopped', got %s", interp.State().Value)
	}

	interp.Stop()
}
