package viz

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/felixgeelhaar/statekit/export"
)

// ParseXStateJSON converts XState JSON to VizMachine.
func ParseXStateJSON(data []byte) (*VizMachine, error) {
	var xstate export.XStateMachine
	if err := json.Unmarshal(data, &xstate); err != nil {
		return nil, fmt.Errorf("unmarshal XState JSON: %w", err)
	}
	return FromXStateMachine(&xstate), nil
}

// FromXStateMachine converts an XStateMachine to VizMachine.
func FromXStateMachine(xm *export.XStateMachine) *VizMachine {
	vm := &VizMachine{
		ID:      xm.ID,
		Initial: xm.Initial,
		States:  make(map[string]*VizState),
	}

	// Convert all states recursively
	for id, node := range xm.States {
		convertXStateNode(vm, id, "", node)
	}

	// Calculate depths
	calculateDepths(vm)

	return vm
}

// convertXStateNode recursively converts an XStateNode and its children.
func convertXStateNode(vm *VizMachine, id, parentID string, node export.XStateNode) {
	vs := &VizState{
		ID:     id,
		Parent: parentID,
		Entry:  node.Entry,
		Exit:   node.Exit,
	}

	// Determine type
	switch node.Type {
	case "final":
		vs.Type = VizStateFinal
	case "parallel":
		vs.Type = VizStateParallel
	case "history":
		vs.Type = VizStateHistory
		vs.HistoryType = node.History
		vs.HistoryDefault = node.Target
	default:
		if len(node.States) > 0 {
			vs.Type = VizStateCompound
			vs.Initial = node.Initial
		} else {
			vs.Type = VizStateAtomic
		}
	}

	// Convert event-based transitions
	for event, trans := range node.On {
		vs.Transitions = append(vs.Transitions, VizTransition{
			Event:   event,
			Target:  trans.Target,
			Guard:   trans.Guard,
			Actions: trans.Actions,
		})
	}

	// Convert delayed transitions
	for delayMs, trans := range node.After {
		delay, _ := strconv.ParseInt(delayMs, 10, 64)
		vs.Transitions = append(vs.Transitions, VizTransition{
			Event:     fmt.Sprintf("after(%dms)", delay),
			Target:    trans.Target,
			Guard:     trans.Guard,
			Actions:   trans.Actions,
			IsDelayed: true,
			DelayMs:   delay,
		})
	}

	// Sort transitions for deterministic output
	sort.Slice(vs.Transitions, func(i, j int) bool {
		return vs.Transitions[i].Event < vs.Transitions[j].Event
	})

	// Convert invoked services
	for _, inv := range node.Invoke {
		vizInv := VizInvoke{
			ID:  inv.ID,
			Src: inv.Src,
		}
		if inv.OnDone != nil {
			vizInv.OnDoneTarget = inv.OnDone.Target
		}
		if inv.OnError != nil {
			vizInv.OnErrTarget = inv.OnError.Target
		}
		vs.Invocations = append(vs.Invocations, vizInv)
	}

	// Add state to machine
	vm.States[id] = vs

	// Process children
	var childIDs []string
	for childID := range node.States {
		childIDs = append(childIDs, childID)
	}
	sort.Strings(childIDs) // Deterministic order

	vs.Children = childIDs

	// Recursively convert children
	for _, childID := range childIDs {
		convertXStateNode(vm, childID, id, node.States[childID])
	}
}

// calculateDepths sets the Depth field for all states.
func calculateDepths(vm *VizMachine) {
	for id, state := range vm.States {
		state.Depth = getDepth(vm, id)
	}
}

func getDepth(vm *VizMachine, stateID string) int {
	state := vm.States[stateID]
	if state == nil || state.Parent == "" {
		return 0
	}
	return 1 + getDepth(vm, state.Parent)
}
