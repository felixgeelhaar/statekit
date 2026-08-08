package viz

import "go.klarlabs.de/statekit/internal/ir"

// FromMachine builds a visualization model directly from a compiled machine,
// with no JSON serialization and no Go source parsing in between. Pass the
// *statekit.MachineConfig[C] returned by MachineBuilder.Build; the generic
// parameter is inferred.
//
// This is the programmatic entry point to every renderer — a machine assembled
// at runtime (from a table, a config file, a database) renders the same way a
// machine written as a source literal does:
//
//	machine, _ := statekit.NewMachine[Ctx]("lifecycle"). /* ... */ .Build()
//	diagram := mermaid.NewRenderer().Render(viz.FromMachine(machine))
//
// Pairing it with a golden-file test keeps a published diagram and the machine
// the runtime executes from drifting apart.
//
// The two other routes remain: ParseNativeJSON for a machine already
// serialized to statekit's native JSON, and viz/goparser for a machine read
// out of Go source without compiling it. export.NewNativeExporter wraps
// FromMachine when the JSON form is what you want.
//
// The returned model is a snapshot. Mutating it does not affect the machine,
// and later changes to the machine are not reflected in it.
func FromMachine[C any](m *ir.MachineConfig[C]) *VizMachine {
	if m == nil {
		return nil
	}

	vm := &VizMachine{
		ID:      m.ID,
		Initial: string(m.Initial),
		States:  make(map[string]*VizState, len(m.States)),
	}

	for id, state := range m.States {
		vm.States[string(id)] = stateFromIR(state)
	}

	for _, state := range vm.States {
		state.Depth = len(m.GetAncestors(ir.StateID(state.ID)))
	}

	return vm
}

// stateFromIR maps an IR state node to its visualization form.
func stateFromIR(state *ir.StateConfig) *VizState {
	vs := &VizState{
		ID:          string(state.ID),
		Type:        VizStateType(state.Type.String()),
		Initial:     string(state.Initial),
		Parent:      string(state.Parent),
		Children:    make([]string, len(state.Children)),
		Entry:       make([]string, len(state.Entry)),
		Exit:        make([]string, len(state.Exit)),
		Transitions: make([]VizTransition, 0, len(state.Transitions)),
	}

	for i, child := range state.Children {
		vs.Children[i] = string(child)
	}
	for i, action := range state.Entry {
		vs.Entry[i] = string(action)
	}
	for i, action := range state.Exit {
		vs.Exit[i] = string(action)
	}

	if state.Type == ir.StateTypeHistory {
		vs.HistoryType = state.HistoryType.String()
		vs.HistoryDefault = string(state.HistoryDefault)
	}

	for _, t := range state.Transitions {
		vs.Transitions = append(vs.Transitions, transitionFromIR(t))
	}

	// Eventless ("always") transitions and tags (v1.x)
	for _, t := range state.Always {
		vs.Always = append(vs.Always, transitionFromIR(t))
	}
	if len(state.Tags) > 0 {
		vs.Tags = append(vs.Tags, state.Tags...)
	}

	for _, inv := range state.Invocations {
		vi := VizInvoke{
			ID:  inv.ID,
			Src: string(inv.Src),
		}
		if inv.OnDone != nil {
			vi.OnDoneTarget = string(inv.OnDone.Target)
		}
		if inv.OnError != nil {
			vi.OnErrTarget = string(inv.OnError.Target)
		}
		vs.Invocations = append(vs.Invocations, vi)
	}

	return vs
}

// transitionFromIR maps an IR transition to its visualization form,
// including delayed metadata and raised internal events (v1.x).
func transitionFromIR(t *ir.TransitionConfig) VizTransition {
	vt := VizTransition{
		Event:   string(t.Event),
		Target:  string(t.Target),
		Guard:   string(t.Guard),
		Actions: make([]string, len(t.Actions)),
	}
	for i, a := range t.Actions {
		vt.Actions[i] = string(a)
	}
	if t.IsDelayed() {
		vt.IsDelayed = true
		vt.DelayMs = t.Delay.Milliseconds()
	}
	for _, r := range t.Raise {
		vt.Raise = append(vt.Raise, string(r))
	}
	vt.Internal = t.Internal
	return vt
}
