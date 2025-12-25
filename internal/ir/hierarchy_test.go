package ir

import "testing"

// Helper to create a hierarchical test machine:
//
//	root
//	├── active (compound)
//	│   ├── idle (initial)
//	│   └── working (compound)
//	│       ├── loading (initial)
//	│       └── processing
//	└── done (final)
func createHierarchicalMachine() *MachineConfig[struct{}] {
	m := NewMachineConfig[struct{}]("test", "active", struct{}{})

	// Root level states
	active := NewStateConfig("active", StateTypeCompound)
	active.Initial = "idle"
	active.Children = []StateID{"idle", "working"}
	m.States["active"] = active

	done := NewStateConfig("done", StateTypeFinal)
	m.States["done"] = done

	// Children of active
	idle := NewStateConfig("idle", StateTypeAtomic)
	idle.Parent = "active"
	m.States["idle"] = idle

	working := NewStateConfig("working", StateTypeCompound)
	working.Parent = "active"
	working.Initial = "loading"
	working.Children = []StateID{"loading", "processing"}
	m.States["working"] = working

	// Children of working
	loading := NewStateConfig("loading", StateTypeAtomic)
	loading.Parent = "working"
	m.States["loading"] = loading

	processing := NewStateConfig("processing", StateTypeAtomic)
	processing.Parent = "working"
	m.States["processing"] = processing

	return m
}

func TestStateConfig_IsCompound(t *testing.T) {
	m := createHierarchicalMachine()

	if !m.States["active"].IsCompound() {
		t.Error("expected 'active' to be compound")
	}
	if !m.States["working"].IsCompound() {
		t.Error("expected 'working' to be compound")
	}
	if m.States["idle"].IsCompound() {
		t.Error("expected 'idle' not to be compound")
	}
	if m.States["done"].IsCompound() {
		t.Error("expected 'done' not to be compound")
	}
}

func TestStateConfig_IsAtomic(t *testing.T) {
	m := createHierarchicalMachine()

	if !m.States["idle"].IsAtomic() {
		t.Error("expected 'idle' to be atomic")
	}
	if !m.States["loading"].IsAtomic() {
		t.Error("expected 'loading' to be atomic")
	}
	if m.States["active"].IsAtomic() {
		t.Error("expected 'active' not to be atomic")
	}
}

func TestStateConfig_IsFinal(t *testing.T) {
	m := createHierarchicalMachine()

	if !m.States["done"].IsFinal() {
		t.Error("expected 'done' to be final")
	}
	if m.States["idle"].IsFinal() {
		t.Error("expected 'idle' not to be final")
	}
}

func TestMachineConfig_GetAncestors(t *testing.T) {
	m := createHierarchicalMachine()

	// loading -> working -> active
	ancestors := m.GetAncestors("loading")
	expected := []StateID{"working", "active"}
	if len(ancestors) != len(expected) {
		t.Fatalf("expected %d ancestors, got %d: %v", len(expected), len(ancestors), ancestors)
	}
	for i, exp := range expected {
		if ancestors[i] != exp {
			t.Errorf("expected ancestor[%d] = %s, got %s", i, exp, ancestors[i])
		}
	}

	// idle -> active
	ancestors = m.GetAncestors("idle")
	if len(ancestors) != 1 || ancestors[0] != "active" {
		t.Errorf("expected [active], got %v", ancestors)
	}

	// active -> (none)
	ancestors = m.GetAncestors("active")
	if len(ancestors) != 0 {
		t.Errorf("expected no ancestors for root state, got %v", ancestors)
	}
}

func TestMachineConfig_GetPath(t *testing.T) {
	m := createHierarchicalMachine()

	// Path to loading: active -> working -> loading
	path := m.GetPath("loading")
	expected := []StateID{"active", "working", "loading"}
	if len(path) != len(expected) {
		t.Fatalf("expected path length %d, got %d: %v", len(expected), len(path), path)
	}
	for i, exp := range expected {
		if path[i] != exp {
			t.Errorf("expected path[%d] = %s, got %s", i, exp, path[i])
		}
	}

	// Path to active: just active
	path = m.GetPath("active")
	if len(path) != 1 || path[0] != "active" {
		t.Errorf("expected [active], got %v", path)
	}
}

func TestMachineConfig_GetInitialLeaf(t *testing.T) {
	m := createHierarchicalMachine()

	// active -> idle (active's initial)
	leaf := m.GetInitialLeaf("active")
	if leaf != "idle" {
		t.Errorf("expected initial leaf of 'active' to be 'idle', got %s", leaf)
	}

	// working -> loading (working's initial)
	leaf = m.GetInitialLeaf("working")
	if leaf != "loading" {
		t.Errorf("expected initial leaf of 'working' to be 'loading', got %s", leaf)
	}

	// idle -> idle (atomic, returns itself)
	leaf = m.GetInitialLeaf("idle")
	if leaf != "idle" {
		t.Errorf("expected initial leaf of 'idle' to be 'idle', got %s", leaf)
	}
}

func TestMachineConfig_IsDescendantOf(t *testing.T) {
	m := createHierarchicalMachine()

	// loading is descendant of working
	if !m.IsDescendantOf("loading", "working") {
		t.Error("expected 'loading' to be descendant of 'working'")
	}

	// loading is descendant of active
	if !m.IsDescendantOf("loading", "active") {
		t.Error("expected 'loading' to be descendant of 'active'")
	}

	// working is descendant of active
	if !m.IsDescendantOf("working", "active") {
		t.Error("expected 'working' to be descendant of 'active'")
	}

	// active is NOT descendant of working
	if m.IsDescendantOf("active", "working") {
		t.Error("expected 'active' NOT to be descendant of 'working'")
	}

	// idle is NOT descendant of working
	if m.IsDescendantOf("idle", "working") {
		t.Error("expected 'idle' NOT to be descendant of 'working'")
	}

	// loading is NOT descendant of idle (siblings' subtrees)
	if m.IsDescendantOf("loading", "idle") {
		t.Error("expected 'loading' NOT to be descendant of 'idle'")
	}
}

func TestMachineConfig_FindLCA(t *testing.T) {
	m := createHierarchicalMachine()

	// LCA of loading and processing is working
	lca := m.FindLCA("loading", "processing")
	if lca != "working" {
		t.Errorf("expected LCA of loading/processing to be 'working', got %s", lca)
	}

	// LCA of loading and idle is active
	lca = m.FindLCA("loading", "idle")
	if lca != "active" {
		t.Errorf("expected LCA of loading/idle to be 'active', got %s", lca)
	}

	// LCA of idle and done is "" (no common ancestor at root level)
	lca = m.FindLCA("idle", "done")
	if lca != "" {
		t.Errorf("expected LCA of idle/done to be '', got %s", lca)
	}

	// LCA of loading and working is working (one is ancestor of other)
	lca = m.FindLCA("loading", "working")
	if lca != "working" {
		t.Errorf("expected LCA of loading/working to be 'working', got %s", lca)
	}

	// LCA of same state is itself
	lca = m.FindLCA("loading", "loading")
	if lca != "loading" {
		t.Errorf("expected LCA of loading/loading to be 'loading', got %s", lca)
	}
}
