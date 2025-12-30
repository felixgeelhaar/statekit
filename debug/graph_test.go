package debug_test

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/debug"
)

func buildGraphTestMachine() *statekit.MachineConfig[struct{}] {
	machine, err := statekit.NewMachine[struct{}]("graph_test").
		WithInitial("idle").
		State("idle").
		On("START").Target("running").
		On("SKIP").Target("done").
		Done().
		State("running").
		On("PAUSE").Target("paused").
		On("STOP").Target("done").
		Done().
		State("paused").
		On("RESUME").Target("running").
		On("STOP").Target("done").
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}

func TestStateGraph_NewStateGraph(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	if graph.ID != "graph_test" {
		t.Errorf("expected ID 'graph_test', got %q", graph.ID)
	}

	if graph.Initial != "idle" {
		t.Errorf("expected initial 'idle', got %q", graph.Initial)
	}

	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(graph.Nodes))
	}

	if len(graph.Edges) != 6 {
		t.Errorf("expected 6 edges, got %d", len(graph.Edges))
	}
}

func TestStateGraph_GetNode(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	node := graph.GetNode("idle")
	if node == nil {
		t.Fatal("expected node for 'idle'")
	}

	if node.Type != "atomic" {
		t.Errorf("expected type 'atomic', got %q", node.Type)
	}

	if graph.GetNode("nonexistent") != nil {
		t.Error("expected nil for nonexistent node")
	}
}

func TestStateGraph_GetEdgesFrom(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	edges := graph.GetEdgesFrom("idle")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges from 'idle', got %d", len(edges))
	}
}

func TestStateGraph_GetEdgesTo(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	edges := graph.GetEdgesTo("done")
	if len(edges) != 3 {
		t.Errorf("expected 3 edges to 'done', got %d", len(edges))
	}
}

func TestStateGraph_RootNodes(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	roots := graph.RootNodes()
	if len(roots) != 4 {
		t.Errorf("expected 4 root nodes (flat machine), got %d", len(roots))
	}
}

func TestStateGraph_LeafNodes(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	leaves := graph.LeafNodes()
	if len(leaves) != 4 {
		t.Errorf("expected 4 leaf nodes (flat machine), got %d", len(leaves))
	}
}

func TestStateGraph_FinalNodes(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	finals := graph.FinalNodes()
	if len(finals) != 1 {
		t.Errorf("expected 1 final node, got %d", len(finals))
	}

	if finals[0].ID != "done" {
		t.Errorf("expected final node 'done', got %q", finals[0].ID)
	}
}

func TestStateGraph_GetPath(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	path := graph.GetPath("idle")
	if len(path) != 1 || path[0] != "idle" {
		t.Errorf("expected path [idle], got %v", path)
	}
}

func TestStateGraph_Reachable(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	reachable := graph.Reachable("idle")
	if len(reachable) != 4 {
		t.Errorf("expected 4 reachable states from 'idle', got %d", len(reachable))
	}
}

func TestStateGraph_UnreachableStates(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	unreachable := graph.UnreachableStates()
	if len(unreachable) != 0 {
		t.Errorf("expected 0 unreachable states, got %d: %v", len(unreachable), unreachable)
	}
}

func TestStateGraph_DeadEndStates(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	deadEnds := graph.DeadEndStates()
	// Final state is not a dead end
	if len(deadEnds) != 0 {
		t.Errorf("expected 0 dead end states, got %d: %v", len(deadEnds), deadEnds)
	}
}

func TestStateGraph_String(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	str := graph.String()

	if !strings.Contains(str, "StateGraph: graph_test") {
		t.Error("string should contain graph ID")
	}

	if !strings.Contains(str, "Initial: idle") {
		t.Error("string should contain initial state")
	}

	if !strings.Contains(str, "States: 4") {
		t.Error("string should contain state count")
	}

	if !strings.Contains(str, "Transitions: 6") {
		t.Error("string should contain transition count")
	}
}

func TestStateGraph_ToMermaid(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	mermaid := graph.ToMermaid()

	if !strings.Contains(mermaid, "stateDiagram-v2") {
		t.Error("mermaid should start with stateDiagram-v2")
	}

	if !strings.Contains(mermaid, "[*] --> idle") {
		t.Error("mermaid should contain initial state transition")
	}

	if !strings.Contains(mermaid, "idle --> running : START") {
		t.Error("mermaid should contain transitions")
	}

	if !strings.Contains(mermaid, "done --> [*]") {
		t.Error("mermaid should mark final state")
	}
}

func TestStateGraph_ToDOT(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	dot := graph.ToDOT()

	if !strings.Contains(dot, "digraph graph_test") {
		t.Error("DOT should start with digraph")
	}

	if !strings.Contains(dot, "__start__") {
		t.Error("DOT should contain start node")
	}

	if !strings.Contains(dot, "idle -> running") {
		t.Error("DOT should contain transitions")
	}

	if !strings.Contains(dot, "done [shape=doublecircle]") {
		t.Error("DOT should mark final state with doublecircle")
	}
}

func TestStateGraph_Analyze(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)

	analysis := graph.Analyze()

	if len(analysis.Unreachable) != 0 {
		t.Errorf("expected 0 unreachable, got %v", analysis.Unreachable)
	}

	if len(analysis.DeadEnds) != 0 {
		t.Errorf("expected 0 dead ends, got %v", analysis.DeadEnds)
	}

	if analysis.LeafCount != 4 {
		t.Errorf("expected 4 leaves, got %d", analysis.LeafCount)
	}

	if analysis.FinalCount != 1 {
		t.Errorf("expected 1 final, got %d", analysis.FinalCount)
	}

	if analysis.MaxDepth != 0 {
		t.Errorf("expected max depth 0 (flat), got %d", analysis.MaxDepth)
	}

	// This machine has cycles (running <-> paused)
	if !analysis.HasCycles {
		t.Error("expected to detect cycles")
	}
}

func TestStateGraph_HierarchicalMachine(t *testing.T) {
	machine, err := statekit.NewMachine[struct{}]("nested").
		WithInitial("active").
		State("active").
		WithInitial("idle").
		On("EXIT").Target("done").End(). // TransitionBuilder.End() → StateBuilder[active]
		State("idle").
		On("START").Target("working").
		End(). // TransitionBuilder.End() → StateBuilder[idle]
		End(). // StateBuilder[idle].End() → StateBuilder[active]
		State("working").
		On("STOP").Target("idle").
		End(). // TransitionBuilder.End() → StateBuilder[working]
		End(). // StateBuilder[working].End() → StateBuilder[active]
		Done().
		State("done").Final().Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	graph := debug.NewStateGraph(machine)

	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(graph.Nodes))
	}

	// Check depth
	activeNode := graph.GetNode("active")
	if activeNode.Depth != 0 {
		t.Errorf("expected 'active' depth 0, got %d", activeNode.Depth)
	}

	idleNode := graph.GetNode("idle")
	if idleNode.Depth != 1 {
		t.Errorf("expected 'idle' depth 1, got %d", idleNode.Depth)
	}

	// Check path
	path := graph.GetPath("idle")
	if len(path) != 2 || path[0] != "active" || path[1] != "idle" {
		t.Errorf("expected path [active, idle], got %v", path)
	}

	// Analysis
	analysis := graph.Analyze()
	if analysis.MaxDepth != 1 {
		t.Errorf("expected max depth 1, got %d", analysis.MaxDepth)
	}
}

func TestStateGraph_WithUnreachableState(t *testing.T) {
	// Build machine with an unreachable state
	machine, err := statekit.NewMachine[struct{}]("unreachable_test").
		WithInitial("a").
		State("a").
		On("NEXT").Target("b").
		Done().
		State("b").Final().Done().
		State("orphan"). // No transitions lead here
		On("X").Target("a").
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	graph := debug.NewStateGraph(machine)
	unreachable := graph.UnreachableStates()

	if len(unreachable) != 1 || unreachable[0] != "orphan" {
		t.Errorf("expected [orphan], got %v", unreachable)
	}
}

func TestStateGraph_WithDeadEnd(t *testing.T) {
	// Build machine with a dead end (non-final state with no outgoing transitions)
	machine, err := statekit.NewMachine[struct{}]("deadend_test").
		WithInitial("a").
		State("a").
		On("NEXT").Target("deadend").
		Done().
		State("deadend"). // No transitions, not final = dead end
		Done().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	graph := debug.NewStateGraph(machine)
	deadEnds := graph.DeadEndStates()

	if len(deadEnds) != 1 || deadEnds[0] != "deadend" {
		t.Errorf("expected [deadend], got %v", deadEnds)
	}
}

func TestAnalysis_String(t *testing.T) {
	machine := buildGraphTestMachine()
	graph := debug.NewStateGraph(machine)
	analysis := graph.Analyze()

	str := analysis.String()

	if !strings.Contains(str, "Analysis Results:") {
		t.Error("should contain header")
	}

	if !strings.Contains(str, "Leaf States: 4") {
		t.Error("should contain leaf count")
	}

	if !strings.Contains(str, "Final States: 1") {
		t.Error("should contain final count")
	}
}
