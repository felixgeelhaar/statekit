package debug

import (
	"fmt"
	"sort"
	"strings"

	"github.com/felixgeelhaar/statekit"
	"github.com/felixgeelhaar/statekit/internal/ir"
)

// StateGraph represents the state machine as a navigable graph structure.
// It provides methods to traverse and visualize the machine topology.
type StateGraph struct {
	ID       string
	Initial  statekit.StateID
	Nodes    map[statekit.StateID]*GraphNode
	Edges    []*GraphEdge
	nodeList []statekit.StateID // Ordered list of nodes
}

// GraphNode represents a state in the graph.
type GraphNode struct {
	ID       statekit.StateID
	Type     string
	Parent   statekit.StateID
	Initial  statekit.StateID
	Children []statekit.StateID
	Entry    []string
	Exit     []string
	Depth    int
}

// GraphEdge represents a transition in the graph.
type GraphEdge struct {
	From    statekit.StateID
	To      statekit.StateID
	Event   statekit.EventType
	Guard   statekit.GuardType
	Actions []string
	IsDelay bool
}

// NewStateGraph creates a state graph from a machine configuration.
// This works with any context type by using type erasure.
func NewStateGraph[C any](machine *ir.MachineConfig[C]) *StateGraph {
	graph := &StateGraph{
		ID:      machine.ID,
		Initial: machine.Initial,
		Nodes:   make(map[statekit.StateID]*GraphNode),
		Edges:   make([]*GraphEdge, 0),
	}

	// Build nodes
	for id, config := range machine.States {
		depth := 0
		parent := config.Parent
		for parent != "" {
			depth++
			if parentConfig := machine.GetState(parent); parentConfig != nil {
				parent = parentConfig.Parent
			} else {
				break
			}
		}

		node := &GraphNode{
			ID:       id,
			Type:     config.Type.String(),
			Parent:   config.Parent,
			Initial:  config.Initial,
			Children: config.Children,
			Entry:    actionsToStrings(config.Entry),
			Exit:     actionsToStrings(config.Exit),
			Depth:    depth,
		}
		graph.Nodes[id] = node
		graph.nodeList = append(graph.nodeList, id)
	}

	// Sort nodes for consistent ordering
	sort.Slice(graph.nodeList, func(a, b int) bool {
		return graph.nodeList[a] < graph.nodeList[b]
	})

	// Build edges
	for id, config := range machine.States {
		for _, t := range config.Transitions {
			edge := &GraphEdge{
				From:    id,
				To:      t.Target,
				Event:   t.Event,
				Guard:   t.Guard,
				Actions: actionsToStrings(t.Actions),
				IsDelay: t.Delay > 0,
			}
			graph.Edges = append(graph.Edges, edge)
		}
	}

	// Sort edges for consistent ordering
	sort.Slice(graph.Edges, func(a, b int) bool {
		if graph.Edges[a].From != graph.Edges[b].From {
			return graph.Edges[a].From < graph.Edges[b].From
		}
		return graph.Edges[a].Event < graph.Edges[b].Event
	})

	return graph
}

// StateGraphFrom creates a state graph from an inspector.
func StateGraphFrom[C any](inspector *Inspector[C]) *StateGraph {
	return NewStateGraph(inspector.machine)
}

// GetNode returns the node for the given state ID.
func (g *StateGraph) GetNode(id statekit.StateID) *GraphNode {
	return g.Nodes[id]
}

// GetEdgesFrom returns all edges originating from the given state.
func (g *StateGraph) GetEdgesFrom(id statekit.StateID) []*GraphEdge {
	var edges []*GraphEdge
	for _, e := range g.Edges {
		if e.From == id {
			edges = append(edges, e)
		}
	}
	return edges
}

// GetEdgesTo returns all edges leading to the given state.
func (g *StateGraph) GetEdgesTo(id statekit.StateID) []*GraphEdge {
	var edges []*GraphEdge
	for _, e := range g.Edges {
		if e.To == id {
			edges = append(edges, e)
		}
	}
	return edges
}

// RootNodes returns all top-level states (no parent).
func (g *StateGraph) RootNodes() []*GraphNode {
	var roots []*GraphNode
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if node.Parent == "" {
			roots = append(roots, node)
		}
	}
	return roots
}

// LeafNodes returns all states that have no children.
func (g *StateGraph) LeafNodes() []*GraphNode {
	var leaves []*GraphNode
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if len(node.Children) == 0 {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// FinalNodes returns all final states.
func (g *StateGraph) FinalNodes() []*GraphNode {
	var finals []*GraphNode
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if node.Type == "final" {
			finals = append(finals, node)
		}
	}
	return finals
}

// GetPath returns the path from root to the given state.
func (g *StateGraph) GetPath(id statekit.StateID) []statekit.StateID {
	var path []statekit.StateID
	current := id
	for current != "" {
		path = append([]statekit.StateID{current}, path...)
		node := g.Nodes[current]
		if node == nil {
			break
		}
		current = node.Parent
	}
	return path
}

// Reachable returns all states reachable from the given state via transitions.
func (g *StateGraph) Reachable(id statekit.StateID) []statekit.StateID {
	visited := make(map[statekit.StateID]bool)
	var result []statekit.StateID

	var dfs func(statekit.StateID)
	dfs = func(current statekit.StateID) {
		if visited[current] {
			return
		}
		visited[current] = true
		result = append(result, current)

		for _, edge := range g.GetEdgesFrom(current) {
			dfs(edge.To)
		}
	}

	dfs(id)
	return result
}

// UnreachableStates returns states that cannot be reached from the initial state.
func (g *StateGraph) UnreachableStates() []statekit.StateID {
	reachable := make(map[statekit.StateID]bool)
	for _, id := range g.Reachable(g.Initial) {
		reachable[id] = true
	}

	var unreachable []statekit.StateID
	for _, id := range g.nodeList {
		if !reachable[id] {
			unreachable = append(unreachable, id)
		}
	}
	return unreachable
}

// DeadEndStates returns non-final states with no outgoing transitions.
func (g *StateGraph) DeadEndStates() []statekit.StateID {
	var deadEnds []statekit.StateID
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if node.Type == "final" {
			continue
		}
		edges := g.GetEdgesFrom(id)
		if len(edges) == 0 && len(node.Children) == 0 {
			deadEnds = append(deadEnds, id)
		}
	}
	return deadEnds
}

// String returns a text representation of the graph.
func (g *StateGraph) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "StateGraph: %s\n", g.ID)
	fmt.Fprintf(&sb, "Initial: %s\n", g.Initial)
	fmt.Fprintf(&sb, "States: %d, Transitions: %d\n", len(g.Nodes), len(g.Edges))
	sb.WriteString("\n")

	// Print states hierarchically
	sb.WriteString("States:\n")
	g.printNodesHierarchically(&sb, "", 2)
	sb.WriteString("\n")

	// Print transitions
	sb.WriteString("Transitions:\n")
	for _, edge := range g.Edges {
		fmt.Fprintf(&sb, "  %s --%s--> %s", edge.From, edge.Event, edge.To)
		if edge.Guard != "" {
			fmt.Fprintf(&sb, " [%s]", edge.Guard)
		}
		if edge.IsDelay {
			sb.WriteString(" (delayed)")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// printNodesHierarchically prints nodes in a tree format
func (g *StateGraph) printNodesHierarchically(sb *strings.Builder, parentID statekit.StateID, indent int) {
	var children []statekit.StateID
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if node.Parent == parentID {
			children = append(children, id)
		}
	}

	for _, id := range children {
		node := g.Nodes[id]
		prefix := strings.Repeat(" ", indent)

		var marker string
		switch node.Type {
		case "final":
			marker = "◉"
		case "compound", "parallel":
			marker = "◆"
		default:
			marker = "○"
		}

		if id == g.Initial {
			marker = "→" + marker
		} else {
			marker = " " + marker
		}

		fmt.Fprintf(sb, "%s%s %s (%s)\n", prefix, marker, id, node.Type)

		// Recurse for children
		g.printNodesHierarchically(sb, id, indent+2)
	}
}

// ToMermaid exports the graph as Mermaid diagram syntax.
func (g *StateGraph) ToMermaid() string {
	var sb strings.Builder

	sb.WriteString("stateDiagram-v2\n")

	// Add initial state
	fmt.Fprintf(&sb, "    [*] --> %s\n", sanitizeMermaidID(g.Initial))

	// Add all transitions
	for _, edge := range g.Edges {
		from := sanitizeMermaidID(edge.From)
		to := sanitizeMermaidID(edge.To)
		label := string(edge.Event)
		if edge.Guard != "" {
			label = fmt.Sprintf("%s [%s]", label, edge.Guard)
		}
		fmt.Fprintf(&sb, "    %s --> %s : %s\n", from, to, label)
	}

	// Mark final states
	for _, node := range g.FinalNodes() {
		fmt.Fprintf(&sb, "    %s --> [*]\n", sanitizeMermaidID(node.ID))
	}

	return sb.String()
}

// ToDOT exports the graph as GraphViz DOT syntax.
func (g *StateGraph) ToDOT() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "digraph %s {\n", sanitizeDOTID(g.ID))
	sb.WriteString("    rankdir=LR;\n")
	sb.WriteString("    node [shape=box, style=rounded];\n")
	sb.WriteString("\n")

	// Add invisible start node
	sb.WriteString("    __start__ [shape=point, width=0.1];\n")
	fmt.Fprintf(&sb, "    __start__ -> %s;\n", sanitizeDOTID(g.Initial))
	sb.WriteString("\n")

	// Style final states
	for _, node := range g.FinalNodes() {
		fmt.Fprintf(&sb, "    %s [shape=doublecircle];\n", sanitizeDOTID(node.ID))
	}

	// Add all states
	for _, id := range g.nodeList {
		node := g.Nodes[id]
		if node.Type == "final" {
			continue // Already handled
		}
		fmt.Fprintf(&sb, "    %s;\n", sanitizeDOTID(id))
	}
	sb.WriteString("\n")

	// Add transitions
	for _, edge := range g.Edges {
		from := sanitizeDOTID(edge.From)
		to := sanitizeDOTID(edge.To)
		label := string(edge.Event)
		if edge.Guard != "" {
			label = fmt.Sprintf("%s\\n[%s]", label, edge.Guard)
		}
		style := ""
		if edge.IsDelay {
			style = ", style=dashed"
		}
		fmt.Fprintf(&sb, "    %s -> %s [label=\"%s\"%s];\n", from, to, label, style)
	}

	sb.WriteString("}\n")
	return sb.String()
}

// sanitizeMermaidID makes an ID safe for Mermaid
func sanitizeMermaidID(id statekit.StateID) string {
	s := string(id)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// sanitizeDOTID makes an ID safe for GraphViz DOT
func sanitizeDOTID(id any) string {
	s := fmt.Sprintf("%v", id)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// Analysis provides static analysis of the graph
type Analysis struct {
	Unreachable []statekit.StateID
	DeadEnds    []statekit.StateID
	HasCycles   bool
	LeafCount   int
	FinalCount  int
	MaxDepth    int
}

// Analyze performs static analysis on the graph.
func (g *StateGraph) Analyze() *Analysis {
	analysis := &Analysis{
		Unreachable: g.UnreachableStates(),
		DeadEnds:    g.DeadEndStates(),
		LeafCount:   len(g.LeafNodes()),
		FinalCount:  len(g.FinalNodes()),
	}

	// Check for cycles
	analysis.HasCycles = g.hasCycles()

	// Calculate max depth
	for _, node := range g.Nodes {
		if node.Depth > analysis.MaxDepth {
			analysis.MaxDepth = node.Depth
		}
	}

	return analysis
}

// hasCycles detects if the transition graph has cycles
func (g *StateGraph) hasCycles() bool {
	visited := make(map[statekit.StateID]bool)
	recStack := make(map[statekit.StateID]bool)

	var dfs func(statekit.StateID) bool
	dfs = func(id statekit.StateID) bool {
		visited[id] = true
		recStack[id] = true

		for _, edge := range g.GetEdgesFrom(id) {
			if !visited[edge.To] {
				if dfs(edge.To) {
					return true
				}
			} else if recStack[edge.To] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	for _, id := range g.nodeList {
		if !visited[id] {
			if dfs(id) {
				return true
			}
		}
	}

	return false
}

// String returns a summary of the analysis.
func (a *Analysis) String() string {
	var sb strings.Builder

	sb.WriteString("Analysis Results:\n")
	fmt.Fprintf(&sb, "  Leaf States: %d\n", a.LeafCount)
	fmt.Fprintf(&sb, "  Final States: %d\n", a.FinalCount)
	fmt.Fprintf(&sb, "  Max Depth: %d\n", a.MaxDepth)
	fmt.Fprintf(&sb, "  Has Cycles: %v\n", a.HasCycles)

	if len(a.Unreachable) > 0 {
		fmt.Fprintf(&sb, "  ⚠ Unreachable States: %v\n", a.Unreachable)
	}

	if len(a.DeadEnds) > 0 {
		fmt.Fprintf(&sb, "  ⚠ Dead End States: %v\n", a.DeadEnds)
	}

	return sb.String()
}
