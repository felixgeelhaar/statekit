package mcp

import (
	"fmt"
	"sync"
	"time"

	"go.klarlabs.de/statekit"

	"go.klarlabs.de/statekit/viz"
)

// Registry manages type-erased machine instances for MCP tools.
// All machines use map[string]any as their context type since MCP
// tools receive JSON at runtime.
type Registry struct {
	mu       sync.RWMutex
	machines map[string]*instance
}

// Ctx is the context type used for all MCP-managed machines.
type Ctx = map[string]any

type instance struct {
	interp    *statekit.Interpreter[Ctx]
	machine   *statekit.MachineConfig[Ctx]
	vizData   *viz.VizMachine
	createdAt time.Time
}

// NewRegistry creates a new empty machine registry.
func NewRegistry() *Registry {
	return &Registry{
		machines: make(map[string]*instance),
	}
}

// Create builds a machine from a VizMachine definition and starts it.
func (r *Registry) Create(vm *viz.VizMachine) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.machines[vm.ID]; exists {
		return fmt.Errorf("machine %q already exists", vm.ID)
	}

	machine, err := buildFromViz(vm)
	if err != nil {
		return fmt.Errorf("build machine: %w", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	r.machines[vm.ID] = &instance{
		interp:    interp,
		machine:   machine,
		vizData:   vm,
		createdAt: time.Now(),
	}
	return nil
}

// Get returns a machine instance by ID.
func (r *Registry) Get(id string) (*instance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.machines[id]
	return inst, ok
}

// List returns all machine IDs with their current state info.
func (r *Registry) List() []MachineInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]MachineInfo, 0, len(r.machines))
	for id, inst := range r.machines {
		result = append(result, MachineInfo{
			ID:           id,
			CurrentState: string(inst.interp.State().Value),
			Done:         inst.interp.Done(),
		})
	}
	return result
}

// Delete stops and removes a machine instance.
func (r *Registry) Delete(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, ok := r.machines[id]
	if !ok {
		return false
	}
	inst.interp.Stop()
	delete(r.machines, id)
	return true
}

// Reset stops a machine and rebuilds it from its stored definition.
func (r *Registry) Reset(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, ok := r.machines[id]
	if !ok {
		return fmt.Errorf("machine %q not found", id)
	}

	inst.interp.Stop()

	machine, err := buildFromViz(inst.vizData)
	if err != nil {
		return fmt.Errorf("rebuild machine: %w", err)
	}

	interp := statekit.NewInterpreter(machine)
	interp.Start()

	r.machines[id] = &instance{
		interp:    interp,
		machine:   machine,
		vizData:   inst.vizData,
		createdAt: inst.createdAt,
	}
	return nil
}

// MachineInfo is a summary of a machine instance.
type MachineInfo struct {
	ID           string `json:"id"`
	CurrentState string `json:"currentState"`
	Done         bool   `json:"done"`
}

// buildFromViz constructs a MachineConfig from a VizMachine.
// Actions and guards are no-ops since MCP is for state tracking/visualization.
func buildFromViz(vm *viz.VizMachine) (*statekit.MachineConfig[Ctx], error) {
	builder := statekit.NewMachine[Ctx](vm.ID).
		WithInitial(statekit.StateID(vm.Initial))

	// Collect and register no-op actions and guards
	actions := make(map[string]bool)
	guards := make(map[string]bool)
	for _, state := range vm.States {
		for _, a := range state.Entry {
			actions[a] = true
		}
		for _, a := range state.Exit {
			actions[a] = true
		}
		for _, t := range state.Transitions {
			if t.Guard != "" {
				guards[t.Guard] = true
			}
			for _, a := range t.Actions {
				actions[a] = true
			}
		}
	}

	for name := range actions {
		builder = builder.WithAction(statekit.ActionType(name), func(_ *Ctx, _ statekit.Event) {})
	}
	for name := range guards {
		builder = builder.WithGuard(statekit.GuardType(name), func(_ Ctx, _ statekit.Event) bool { return true })
	}

	// Build root states
	for _, rootID := range vm.GetRootStates() {
		state := vm.States[rootID]
		if state == nil {
			continue
		}
		sb := builder.State(statekit.StateID(rootID))
		populateState(sb, state, vm)
		sb.Done()
	}

	return builder.Build()
}

func populateState(sb *statekit.StateBuilder[Ctx], vs *viz.VizState, vm *viz.VizMachine) {
	if vs.Type == viz.VizStateFinal {
		sb.Final()
	}

	if vs.Initial != "" {
		sb.WithInitial(statekit.StateID(vs.Initial))
	}

	for _, a := range vs.Entry {
		sb.OnEntry(statekit.ActionType(a))
	}
	for _, a := range vs.Exit {
		sb.OnExit(statekit.ActionType(a))
	}

	for _, t := range vs.Transitions {
		tb := sb.On(statekit.EventType(t.Event)).Target(statekit.StateID(t.Target))
		if t.Guard != "" {
			tb.Guard(statekit.GuardType(t.Guard))
		}
		tb.End()
	}

	for _, childID := range vs.Children {
		child := vm.States[childID]
		if child == nil {
			continue
		}
		csb := sb.State(statekit.StateID(childID))
		populateState(csb, child, vm)
		csb.End()
	}
}
