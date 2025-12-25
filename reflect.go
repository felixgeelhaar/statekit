package statekit

import (
	"fmt"
	"reflect"

	"github.com/felixgeelhaar/statekit/internal/ir"
	"github.com/felixgeelhaar/statekit/internal/parser"
)

// MachineDef is a marker type that must be embedded in a struct
// to define a state machine using the reflection DSL.
//
// Use struct tags to configure the machine:
//   - id:"machineId" - Required machine identifier
//   - initial:"stateName" - Required initial state name
//
// Example:
//
//	type MyMachine struct {
//	    statekit.MachineDef `id:"myMachine" initial:"idle"`
//	    Idle    statekit.StateNode `on:"START->running"`
//	    Running statekit.StateNode `on:"STOP->idle"`
//	}
type MachineDef struct{}

// StateNode is a marker type for defining atomic states in the reflection DSL.
//
// Use struct tags to configure the state:
//   - on:"EVENT->target" - Define a transition (can specify multiple with comma)
//   - on:"EVENT->target:guard" - Transition with guard condition
//   - on:"EVENT->target/action1;action2" - Transition with actions
//   - on:"EVENT->target/action:guard" - Transition with action and guard
//   - entry:"action1,action2" - Entry actions
//   - exit:"action1,action2" - Exit actions
//
// Example:
//
//	Idle statekit.StateNode `on:"START->running:canStart" entry:"logIdle"`
type StateNode struct{}

// CompoundNode is a marker type for defining compound (nested) states.
//
// Use struct tags to configure the compound state:
//   - initial:"childState" - Required initial child state
//   - on:"EVENT->target" - Parent-level transitions
//   - entry:"action" - Parent entry actions
//   - exit:"action" - Parent exit actions
//
// Child states are defined as fields within the struct that embeds CompoundNode.
//
// Example:
//
//	type ActiveState struct {
//	    statekit.CompoundNode `initial:"idle" on:"RESET->done"`
//	    Idle    statekit.StateNode `on:"START->working"`
//	    Working statekit.StateNode `on:"STOP->idle"`
//	}
type CompoundNode struct{}

// FinalNode is a marker type for defining final states.
//
// Final states indicate the machine has completed. They typically
// have no outgoing transitions.
//
// Example:
//
//	Completed statekit.FinalNode
type FinalNode struct{}

// ActionRegistry holds action and guard function implementations
// that are referenced by name in the reflection DSL.
//
// ActionRegistry is not safe for concurrent use. It should be fully
// configured before calling FromStruct or FromStructWithContext.
type ActionRegistry[C any] struct {
	actions map[ActionType]Action[C]
	guards  map[GuardType]Guard[C]
}

// NewActionRegistry creates a new empty action registry.
func NewActionRegistry[C any]() *ActionRegistry[C] {
	return &ActionRegistry[C]{
		actions: make(map[ActionType]Action[C]),
		guards:  make(map[GuardType]Guard[C]),
	}
}

// WithAction registers an action function by name.
// Returns the registry for method chaining.
func (r *ActionRegistry[C]) WithAction(name ActionType, action Action[C]) *ActionRegistry[C] {
	r.actions[name] = action
	return r
}

// WithGuard registers a guard function by name.
// Returns the registry for method chaining.
func (r *ActionRegistry[C]) WithGuard(name GuardType, guard Guard[C]) *ActionRegistry[C] {
	r.guards[name] = guard
	return r
}

// FromStruct builds a MachineConfig from a struct definition using the reflection DSL.
//
// The struct M must embed MachineDef and define states using StateNode,
// CompoundNode, or FinalNode marker types with appropriate struct tags.
//
// Actions and guards referenced in tags must be registered in the provided ActionRegistry.
//
// Example:
//
//	type MyMachine struct {
//	    statekit.MachineDef `id:"example" initial:"idle"`
//	    Idle    statekit.StateNode `on:"START->running:canStart" entry:"logStart"`
//	    Running statekit.StateNode `on:"STOP->idle"`
//	}
//
//	registry := statekit.NewActionRegistry[MyContext]().
//	    WithAction("logStart", func(ctx *MyContext, e statekit.Event) { ... }).
//	    WithGuard("canStart", func(ctx MyContext, e statekit.Event) bool { ... })
//
//	machine, err := statekit.FromStruct[MyMachine, MyContext](registry)
func FromStruct[M any, C any](registry *ActionRegistry[C]) (*ir.MachineConfig[C], error) {
	var zero C
	return FromStructWithContext[M, C](registry, zero)
}

// FromStructWithContext builds a MachineConfig with an initial context value.
func FromStructWithContext[M any, C any](registry *ActionRegistry[C], ctx C) (*ir.MachineConfig[C], error) {
	var m M
	t := reflect.TypeOf(m)

	schema, err := parser.ParseMachineStruct(t)
	if err != nil {
		return nil, fmt.Errorf("parse struct: %w", err)
	}

	machine, err := buildMachineFromSchema[C](schema, registry)
	if err != nil {
		return nil, err
	}

	machine.Context = ctx
	return machine, nil
}

// buildMachineFromSchema converts a parsed schema into a MachineConfig.
func buildMachineFromSchema[C any](schema *parser.MachineSchema, registry *ActionRegistry[C]) (*ir.MachineConfig[C], error) {
	var ctx C
	machine := ir.NewMachineConfig[C](schema.ID, ir.StateID(schema.Initial), ctx)

	// Copy actions and guards from registry
	if registry != nil {
		for name, action := range registry.actions {
			machine.Actions[name] = action
		}
		for name, guard := range registry.guards {
			machine.Guards[name] = guard
		}
	}

	// Build states recursively
	for _, stateSchema := range schema.States {
		if err := buildStateFromSchema(machine, stateSchema, ""); err != nil {
			return nil, err
		}
	}

	// Validate the machine
	if err := ir.Validate(machine); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return machine, nil
}

// buildStateFromSchema recursively builds states from schema.
func buildStateFromSchema[C any](machine *ir.MachineConfig[C], schema *parser.StateSchema, parentID ir.StateID) error {
	stateID := ir.StateID(schema.Name)

	// Determine state type
	var stateType ir.StateType
	switch schema.Type {
	case parser.StateSchemaAtomic:
		stateType = ir.StateTypeAtomic
	case parser.StateSchemaCompound:
		stateType = ir.StateTypeCompound
	case parser.StateSchemaFinal:
		stateType = ir.StateTypeFinal
	default:
		return fmt.Errorf("unknown state schema type: %d", schema.Type)
	}

	// Create state config
	state := ir.NewStateConfig(stateID, stateType)
	state.Parent = parentID
	state.Initial = ir.StateID(schema.Initial)

	// Add entry actions
	for _, action := range schema.Entry {
		state.Entry = append(state.Entry, ir.ActionType(action))
	}

	// Add exit actions
	for _, action := range schema.Exit {
		state.Exit = append(state.Exit, ir.ActionType(action))
	}

	// Add transitions
	for _, trans := range schema.Transitions {
		transition := ir.NewTransitionConfig(
			ir.EventType(trans.Event),
			ir.StateID(trans.Target),
		)
		transition.Guard = ir.GuardType(trans.Guard)
		for _, action := range trans.Actions {
			transition.Actions = append(transition.Actions, ir.ActionType(action))
		}
		state.Transitions = append(state.Transitions, transition)
	}

	// Register state
	machine.States[stateID] = state

	// Build children
	for _, childSchema := range schema.Children {
		if err := buildStateFromSchema(machine, childSchema, stateID); err != nil {
			return err
		}
		state.Children = append(state.Children, ir.StateID(childSchema.Name))
	}

	return nil
}
