package statekit_test

import (
	"fmt"
	"testing"

	"go.klarlabs.de/statekit"
)

// FuzzBuilderAndInterpreter builds small machines from fuzz bytes and
// exercises Start/Send/Snapshot without panicking.
func FuzzBuilderAndInterpreter(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3})
	f.Add([]byte{5, 0, 1, 2, 3, 4})
	f.Add([]byte{2, 7, 1, 0})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		n := 2 + int(dataByte(data, 0)%4) // 2–5 states
		machineID := fmt.Sprintf("fuzz-%d", n)

		b := statekit.NewMachine[struct{}](machineID).WithInitial("s0")
		for i := 0; i < n; i++ {
			id := statekit.StateID(fmt.Sprintf("s%d", i))
			sb := b.State(id)
			if i == n-1 && dataByte(data, 1)%2 == 0 {
				sb.Final()
			} else {
				target := statekit.StateID(fmt.Sprintf("s%d", (i+1)%n))
				evt := statekit.EventType(fmt.Sprintf("E%d", i%3))
				sb.On(evt).Target(target)
			}
			sb.Done()
		}

		machine, err := b.Build()
		if err != nil {
			return // invalid configs are fine; must not panic
		}

		interp := statekit.NewInterpreter(machine)
		defer func() { _ = interp.Close() }()
		interp.Start()

		for i := 0; i < 8; i++ {
			evt := statekit.Event{Type: statekit.EventType(fmt.Sprintf("E%d", dataByte(data, 2+i)%3))}
			interp.Send(evt)
			_ = interp.State()
			_ = interp.Done()
			_ = interp.Snapshot()
		}
	})
}

func dataByte(data []byte, i int) byte {
	if len(data) == 0 {
		return 0
	}
	return data[i%len(data)]
}
