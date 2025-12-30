// Package viz provides visualization models and renderers for state machines.
package viz

// VizMachine represents a state machine for visualization purposes.
// It is format-agnostic and can be constructed from XState JSON or ir.MachineConfig.
type VizMachine struct {
	ID      string
	Initial string
	States  map[string]*VizState
}

// VizState represents a state node for visualization.
type VizState struct {
	ID          string
	Type        VizStateType
	Initial     string   // For compound states
	Parent      string   // Parent state ID (empty for root)
	Children    []string // Child state IDs (ordered)
	Transitions []VizTransition
	Entry       []string // Entry action names
	Exit        []string // Exit action names

	// History-specific fields
	HistoryType    string // "shallow" or "deep"
	HistoryDefault string // Default target if no history

	// Invoked services
	Invocations []VizInvoke

	// Computed during traversal
	Depth int
}

// VizStateType represents the type of state.
type VizStateType string

const (
	VizStateAtomic   VizStateType = "atomic"
	VizStateCompound VizStateType = "compound"
	VizStateParallel VizStateType = "parallel"
	VizStateFinal    VizStateType = "final"
	VizStateHistory  VizStateType = "history"
)

// VizTransition represents a transition between states.
type VizTransition struct {
	Event   string
	Target  string
	Guard   string
	Actions []string

	// Delayed transition fields
	IsDelayed bool
	DelayMs   int64
}

// VizInvoke represents an invoked service.
type VizInvoke struct {
	ID           string
	Src          string
	OnDoneTarget string
	OnErrTarget  string
}

// GetRootStates returns all root-level state IDs (states without parents).
func (m *VizMachine) GetRootStates() []string {
	var roots []string
	for id, state := range m.States {
		if state.Parent == "" {
			roots = append(roots, id)
		}
	}
	return roots
}

// GetChildren returns the child states of a given state.
func (m *VizMachine) GetChildren(stateID string) []*VizState {
	state := m.States[stateID]
	if state == nil {
		return nil
	}

	children := make([]*VizState, 0, len(state.Children))
	for _, childID := range state.Children {
		if child := m.States[childID]; child != nil {
			children = append(children, child)
		}
	}
	return children
}

// IsInitial checks if a state is the initial state of its parent (or machine).
func (m *VizMachine) IsInitial(stateID string) bool {
	state := m.States[stateID]
	if state == nil {
		return false
	}

	if state.Parent == "" {
		return stateID == m.Initial
	}

	parent := m.States[state.Parent]
	if parent == nil {
		return false
	}

	return stateID == parent.Initial
}
