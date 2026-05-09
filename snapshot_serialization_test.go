package statekit

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
)

type counterCtx struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

func newCounterMachine(t *testing.T) *MachineConfig[counterCtx] {
	t.Helper()
	machine, err := NewMachine[counterCtx]("counter").
		WithInitial("idle").
		WithAction("incr", func(c *counterCtx, _ Event) { c.Count++ }).
		State("idle").
		On("TICK").Target("running").Do("incr").
		Done().
		State("running").
		On("TICK").Target("running").Do("incr").
		On("STOP").Target("done").
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return machine
}

// TestSnapshot_JSON_RoundTrip exercises the assertion that frustrated
// users in https://github.com/looplab/fsm/issues/40 — that a state
// machine's snapshot must be publicly serializable for production
// persistence (DB, gob, JSON) without resorting to reflection over
// unexported fields.
//
// statekit.Snapshot ships with explicit MarshalJSON / UnmarshalJSON
// implementations, so this round-trip is the public guarantee.
func TestSnapshot_JSON_RoundTrip(t *testing.T) {
	t.Parallel()
	machine := newCounterMachine(t)

	original := NewInterpreter(machine)
	defer func() { _ = original.Close() }()
	original.Start()
	original.Send(Event{Type: "TICK"})
	original.Send(Event{Type: "TICK"})
	original.Send(Event{Type: "TICK"})

	snap := original.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty JSON")
	}

	var restored Snapshot[counterCtx]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	rebuilt := NewInterpreter(machine)
	defer func() { _ = rebuilt.Close() }()
	if err := rebuilt.Restore(restored); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if got, want := string(rebuilt.State().Value), "running"; got != want {
		t.Errorf("restored state = %q, want %q", got, want)
	}
	if got, want := rebuilt.State().Context.Count, 3; got != want {
		t.Errorf("restored Count = %d, want %d", got, want)
	}

	// Continue driving — restored interpreter behaves identically to
	// the original.
	rebuilt.Send(Event{Type: "STOP"})
	if !rebuilt.Done() {
		t.Error("restored interpreter did not reach Done after STOP")
	}
}

// TestSnapshot_Gob_RoundTrip is the looplab-#40 direct counter-test:
// the user there asked for `(f *FSM) Save() []byte / Load([]byte)`
// because gob couldn't see the FSM's unexported fields. statekit's
// Snapshot is a value type with public fields plus explicit MarshalJSON
// — gob can therefore handle it directly.
func TestSnapshot_Gob_RoundTrip(t *testing.T) {
	t.Parallel()
	machine := newCounterMachine(t)

	original := NewInterpreter(machine)
	defer func() { _ = original.Close() }()
	original.Start()
	original.Send(Event{Type: "TICK"})
	original.Send(Event{Type: "TICK"})

	snap := original.Snapshot()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(snap); err != nil {
		t.Fatalf("gob.Encode: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty gob")
	}

	var restored Snapshot[counterCtx]
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&restored); err != nil {
		t.Fatalf("gob.Decode: %v", err)
	}

	rebuilt := NewInterpreter(machine)
	defer func() { _ = rebuilt.Close() }()
	if err := rebuilt.Restore(restored); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if got, want := rebuilt.State().Context.Count, 2; got != want {
		t.Errorf("restored Count after gob round-trip = %d, want %d", got, want)
	}
}
