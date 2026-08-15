package statekit

import "testing"

// TestBuilderAliases ensures the new builder type aliases (introduced
// for naming clarity) resolve to the original types and the new
// EndMachine() terminator behaves identically to Done().
func TestBuilderAliases(t *testing.T) {
	t.Parallel()
	// Type-equivalence checks via direct assignment — these compile
	// only if the aliases resolve to the same underlying type.
	var (
		ib  *InvokeBuilder[struct{}]
		isb *InvokeServiceBuilder[struct{}]
	)
	isb = ib
	_ = isb

	var (
		mib *MachineInvokeBuilder[struct{}]
		imb *InvokeMachineBuilder[struct{}]
	)
	imb = mib
	_ = imb
}

func TestStateBuilder_EndMachineEquivalentToDone(t *testing.T) {
	t.Parallel()
	machineDone, err := NewMachine[struct{}]("done").
		WithInitial("a").
		State("a").On("X").Target("b").Done().
		State("b").Done().
		Build()
	if err != nil {
		t.Fatalf("Done build: %v", err)
	}

	machineEnd, err := NewMachine[struct{}]("endmachine").
		WithInitial("a").
		State("a").On("X").Target("b").EndMachine().
		State("b").EndMachine().
		Build()
	if err != nil {
		t.Fatalf("EndMachine build: %v", err)
	}

	if len(machineDone.States) != len(machineEnd.States) {
		t.Errorf("state count mismatch: Done=%d EndMachine=%d",
			len(machineDone.States), len(machineEnd.States))
	}
}
