// Package viz provides visualization models and renderers for state machines.
package viz

// VizMachine represents a state machine for visualization purposes.
// It is format-agnostic and can be constructed from ir.MachineConfig.
type VizMachine struct {
	ID      string               `json:"id"`
	Initial string               `json:"initial"`
	States  map[string]*VizState `json:"states"`
}

// VizState represents a state node for visualization.
type VizState struct {
	ID          string          `json:"id"`
	Type        VizStateType    `json:"type"`
	Initial     string          `json:"initial,omitempty"`
	Parent      string          `json:"parent,omitempty"`
	Children    []string        `json:"children,omitempty"`
	Transitions []VizTransition `json:"transitions,omitempty"`
	Always      []VizTransition `json:"always,omitempty"` // eventless transitions (v1.x)
	Entry       []string        `json:"entry,omitempty"`
	Exit        []string        `json:"exit,omitempty"`
	Tags        []string        `json:"tags,omitempty"` // state tags (v1.x)

	// History-specific fields
	HistoryType    string `json:"historyType,omitempty"`    // "shallow" or "deep"
	HistoryDefault string `json:"historyDefault,omitempty"` // Default target if no history

	// Invoked services
	Invocations []VizInvoke `json:"invocations,omitempty"`

	// Computed during traversal
	Depth int `json:"depth,omitempty"`
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
	Event   string   `json:"event"`
	Target  string   `json:"target"`
	Guard   string   `json:"guard,omitempty"`
	Actions []string `json:"actions,omitempty"`

	// Delayed transition fields
	IsDelayed bool  `json:"isDelayed,omitempty"`
	DelayMs   int64 `json:"delayMs,omitempty"`

	// Raised internal events (v1.x)
	Raise []string `json:"raise,omitempty"`

	// Internal transition: runs actions without exit/entry (v1.x)
	Internal bool `json:"internal,omitempty"`
}

// VizInvoke represents an invoked service.
type VizInvoke struct {
	ID           string `json:"id"`
	Src          string `json:"src"`
	OnDoneTarget string `json:"onDoneTarget,omitempty"`
	OnErrTarget  string `json:"onErrTarget,omitempty"`
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
