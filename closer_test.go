package statekit

import (
	"io"
	"testing"
)

func TestInterpreter_ImplementsCloser(t *testing.T) {
	t.Parallel()
	machine, err := NewMachine[struct{}]("c").
		WithInitial("idle").
		State("idle").Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	interp := NewInterpreter(machine)
	interp.Start()

	var c io.Closer = interp
	if err := c.Close(); err != nil {
		t.Errorf("expected nil error from Close, got: %v", err)
	}

	// Idempotent: second Close should also succeed
	if err := c.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got: %v", err)
	}
}
