package statekit

import (
	"fmt"
	"time"

	"go.klarlabs.de/statekit/internal/ir"
	"go.klarlabs.de/statekit/viz"
)

// FromJSON builds a typed *MachineConfig[C] from a Statekit Native JSON
// definition. It is the typed, core-package counterpart to the fluent builder
// and the reflection DSL — JSON is a first-class machine-definition path, not
// only a codegen artifact.
//
// Named actions and guards referenced in the JSON are resolved against the
// supplied ActionRegistry. Resolution is strict: a referenced name that is not
// registered is an error, so authoring mistakes surface at load time rather
// than as silent no-ops. registry may be nil when the definition references no
// actions or guards (a purely structural state chart).
//
// FromJSON reuses the same Native JSON parser as the export, generate, and viz
// packages, so an exported machine round-trips back into an equivalent typed
// machine. The viz package it depends on is pure-stdlib, so FromJSON adds no
// external dependency to the core module.
//
// Example:
//
//	reg := statekit.NewActionRegistry[MyContext]().
//	    WithAction("inc", func(ctx *MyContext, e statekit.Event) { ctx.Count++ }).
//	    WithGuard("canStart", func(ctx MyContext, e statekit.Event) bool { ... })
//
//	machine, err := statekit.FromJSON[MyContext](data, reg)
func FromJSON[C any](data []byte, registry *ActionRegistry[C]) (*MachineConfig[C], error) {
	var zero C
	return FromJSONWithContext[C](data, registry, zero)
}

// FromJSONWithContext is FromJSON with an explicit initial context value.
func FromJSONWithContext[C any](data []byte, registry *ActionRegistry[C], ctx C) (*MachineConfig[C], error) {
	vm, err := viz.ParseNativeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse native JSON: %w", err)
	}
	return fromViz[C](vm, registry, ctx)
}

// fromViz converts a parsed VizMachine into a typed MachineConfig, resolving
// named actions and guards against the registry.
func fromViz[C any](vm *viz.VizMachine, registry *ActionRegistry[C], ctx C) (*MachineConfig[C], error) {
	machine := ir.NewMachineConfig[C](vm.ID, ir.StateID(vm.Initial), ctx)

	res := &refResolver[C]{registry: registry}

	for _, rootID := range vm.GetRootStates() {
		vs := vm.States[rootID]
		if vs == nil {
			continue
		}
		if err := buildStateFromViz[C](machine, vm, vs, "", res); err != nil {
			return nil, err
		}
	}

	// Install only the actions and guards actually referenced by the machine,
	// pulling implementations from the registry on demand.
	for name := range res.actions {
		machine.Actions[name] = res.actionFns[name]
	}
	for name := range res.guards {
		machine.Guards[name] = res.guardFns[name]
	}

	if err := ir.Validate(machine); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return machine, nil
}

// refResolver resolves named action/guard references against a registry,
// recording which names a machine uses and failing on unknown references.
type refResolver[C any] struct {
	registry  *ActionRegistry[C]
	actions   map[ir.ActionType]struct{}
	guards    map[ir.GuardType]struct{}
	actionFns map[ir.ActionType]ir.Action[C]
	guardFns  map[ir.GuardType]ir.Guard[C]
}

func (r *refResolver[C]) action(name string) (ir.ActionType, error) {
	at := ir.ActionType(name)
	if r.actions == nil {
		r.actions = make(map[ir.ActionType]struct{})
		r.actionFns = make(map[ir.ActionType]ir.Action[C])
	}
	if _, seen := r.actions[at]; seen {
		return at, nil
	}
	if r.registry == nil {
		return "", fmt.Errorf("action %q is referenced but no registry was provided", name)
	}
	fn, ok := r.registry.actions[at]
	if !ok {
		return "", fmt.Errorf("action %q is referenced but not registered", name)
	}
	r.actions[at] = struct{}{}
	r.actionFns[at] = ir.Action[C](fn)
	return at, nil
}

func (r *refResolver[C]) guard(name string) (ir.GuardType, error) {
	gt := ir.GuardType(name)
	if r.guards == nil {
		r.guards = make(map[ir.GuardType]struct{})
		r.guardFns = make(map[ir.GuardType]ir.Guard[C])
	}
	if _, seen := r.guards[gt]; seen {
		return gt, nil
	}
	if r.registry == nil {
		return "", fmt.Errorf("guard %q is referenced but no registry was provided", name)
	}
	fn, ok := r.registry.guards[gt]
	if !ok {
		return "", fmt.Errorf("guard %q is referenced but not registered", name)
	}
	r.guards[gt] = struct{}{}
	r.guardFns[gt] = ir.Guard[C](fn)
	return gt, nil
}

// buildStateFromViz recursively materialises a VizState into the typed IR.
func buildStateFromViz[C any](
	machine *ir.MachineConfig[C],
	vm *viz.VizMachine,
	vs *viz.VizState,
	parentID ir.StateID,
	res *refResolver[C],
) error {
	stateType, err := vizStateType(vs.Type)
	if err != nil {
		return fmt.Errorf("state %q: %w", vs.ID, err)
	}

	state := ir.NewStateConfig(ir.StateID(vs.ID), stateType)
	state.Parent = parentID
	state.Initial = ir.StateID(vs.Initial)
	state.Tags = append(state.Tags, vs.Tags...)

	if stateType == ir.StateTypeHistory {
		state.HistoryType = vizHistoryType(vs.HistoryType)
		state.HistoryDefault = ir.StateID(vs.HistoryDefault)
	}

	for _, a := range vs.Entry {
		at, err := res.action(a)
		if err != nil {
			return fmt.Errorf("state %q entry: %w", vs.ID, err)
		}
		state.Entry = append(state.Entry, at)
	}
	for _, a := range vs.Exit {
		at, err := res.action(a)
		if err != nil {
			return fmt.Errorf("state %q exit: %w", vs.ID, err)
		}
		state.Exit = append(state.Exit, at)
	}

	for _, t := range vs.Transitions {
		tc, err := buildTransitionFromViz[C](vs.ID, t, res)
		if err != nil {
			return err
		}
		state.Transitions = append(state.Transitions, tc)
	}

	for _, t := range vs.Always {
		tc, err := buildTransitionFromViz[C](vs.ID, t, res)
		if err != nil {
			return err
		}
		state.Always = append(state.Always, tc)
	}

	machine.States[state.ID] = state

	for _, childID := range vs.Children {
		child := vm.States[childID]
		if child == nil {
			continue
		}
		if err := buildStateFromViz[C](machine, vm, child, state.ID, res); err != nil {
			return err
		}
		state.Children = append(state.Children, ir.StateID(childID))
	}

	return nil
}

// buildTransitionFromViz materialises a VizTransition, resolving its guard and
// actions and preserving delayed, internal, and raise semantics.
func buildTransitionFromViz[C any](
	stateID string,
	t viz.VizTransition,
	res *refResolver[C],
) (*ir.TransitionConfig, error) {
	tc := ir.NewTransitionConfig(ir.EventType(t.Event), ir.StateID(t.Target))

	if t.Guard != "" {
		gt, err := res.guard(t.Guard)
		if err != nil {
			return nil, fmt.Errorf("state %q transition %q: %w", stateID, t.Event, err)
		}
		tc.Guard = gt
	}

	for _, a := range t.Actions {
		at, err := res.action(a)
		if err != nil {
			return nil, fmt.Errorf("state %q transition %q: %w", stateID, t.Event, err)
		}
		tc.Actions = append(tc.Actions, at)
	}

	if t.IsDelayed {
		tc.Delay = time.Duration(t.DelayMs) * time.Millisecond
	}
	for _, r := range t.Raise {
		tc.Raise = append(tc.Raise, ir.EventType(r))
	}
	tc.Internal = t.Internal

	return tc, nil
}

func vizStateType(t viz.VizStateType) (ir.StateType, error) {
	switch t {
	case viz.VizStateAtomic, "":
		return ir.StateTypeAtomic, nil
	case viz.VizStateCompound:
		return ir.StateTypeCompound, nil
	case viz.VizStateParallel:
		return ir.StateTypeParallel, nil
	case viz.VizStateFinal:
		return ir.StateTypeFinal, nil
	case viz.VizStateHistory:
		return ir.StateTypeHistory, nil
	default:
		return 0, fmt.Errorf("unknown state type %q", t)
	}
}

func vizHistoryType(s string) ir.HistoryType {
	if s == "deep" {
		return ir.HistoryTypeDeep
	}
	return ir.HistoryTypeShallow
}
