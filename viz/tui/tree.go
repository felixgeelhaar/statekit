package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/felixgeelhaar/statekit/viz"
)

// TreeNode represents a node in the tree view.
type TreeNode struct {
	State    *viz.VizState
	Children []*TreeNode
	Expanded bool
	Depth    int
}

// TreeView manages the tree representation of a state machine.
type TreeView struct {
	Machine  *viz.VizMachine
	Root     []*TreeNode
	FlatList []*TreeNode // Flattened visible nodes
	cursor   int
	styles   *Styles
}

// NewTreeView creates a new tree view for a machine.
func NewTreeView(machine *viz.VizMachine, styles *Styles) *TreeView {
	tv := &TreeView{
		Machine: machine,
		styles:  styles,
	}
	tv.buildTree()
	tv.updateFlatList()
	return tv
}

// buildTree constructs the tree structure from the machine.
func (tv *TreeView) buildTree() {
	// Build nodes for all states
	nodeMap := make(map[string]*TreeNode)
	for _, state := range tv.Machine.States {
		nodeMap[state.ID] = &TreeNode{
			State:    state,
			Expanded: true,
			Depth:    state.Depth,
		}
	}

	// Link parents and children
	for _, state := range tv.Machine.States {
		node := nodeMap[state.ID]
		for _, childID := range state.Children {
			if child, ok := nodeMap[childID]; ok {
				node.Children = append(node.Children, child)
			}
		}
		// Sort children for deterministic order
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].State.ID < node.Children[j].State.ID
		})
	}

	// Find root nodes (states without parents)
	for _, state := range tv.Machine.States {
		if state.Parent == "" {
			tv.Root = append(tv.Root, nodeMap[state.ID])
		}
	}

	// Sort root nodes
	sort.Slice(tv.Root, func(i, j int) bool {
		return tv.Root[i].State.ID < tv.Root[j].State.ID
	})
}

// updateFlatList rebuilds the flattened list of visible nodes.
func (tv *TreeView) updateFlatList() {
	tv.FlatList = nil
	for _, root := range tv.Root {
		tv.flattenNode(root)
	}
}

// flattenNode recursively adds visible nodes to the flat list.
func (tv *TreeView) flattenNode(node *TreeNode) {
	tv.FlatList = append(tv.FlatList, node)
	if node.Expanded {
		for _, child := range node.Children {
			tv.flattenNode(child)
		}
	}
}

// CursorUp moves the cursor up.
func (tv *TreeView) CursorUp() {
	if tv.cursor > 0 {
		tv.cursor--
	}
}

// CursorDown moves the cursor down.
func (tv *TreeView) CursorDown() {
	if tv.cursor < len(tv.FlatList)-1 {
		tv.cursor++
	}
}

// Toggle expands/collapses the current node.
func (tv *TreeView) Toggle() {
	if tv.cursor >= 0 && tv.cursor < len(tv.FlatList) {
		node := tv.FlatList[tv.cursor]
		if len(node.Children) > 0 {
			node.Expanded = !node.Expanded
			tv.updateFlatList()
		}
	}
}

// Collapse collapses the current node.
func (tv *TreeView) Collapse() {
	if tv.cursor >= 0 && tv.cursor < len(tv.FlatList) {
		node := tv.FlatList[tv.cursor]
		if len(node.Children) > 0 && node.Expanded {
			node.Expanded = false
			tv.updateFlatList()
		}
	}
}

// Expand expands the current node.
func (tv *TreeView) Expand() {
	if tv.cursor >= 0 && tv.cursor < len(tv.FlatList) {
		node := tv.FlatList[tv.cursor]
		if len(node.Children) > 0 && !node.Expanded {
			node.Expanded = true
			tv.updateFlatList()
		}
	}
}

// ExpandAll expands all nodes.
func (tv *TreeView) ExpandAll() {
	for _, node := range tv.FlatList {
		node.Expanded = true
	}
	tv.updateFlatList()
}

// CollapseAll collapses all nodes.
func (tv *TreeView) CollapseAll() {
	for _, root := range tv.Root {
		tv.collapseRecursive(root)
	}
	tv.updateFlatList()
}

func (tv *TreeView) collapseRecursive(node *TreeNode) {
	node.Expanded = false
	for _, child := range node.Children {
		tv.collapseRecursive(child)
	}
}

// Selected returns the currently selected node.
func (tv *TreeView) Selected() *TreeNode {
	if tv.cursor >= 0 && tv.cursor < len(tv.FlatList) {
		return tv.FlatList[tv.cursor]
	}
	return nil
}

// View renders the tree view.
func (tv *TreeView) View(width, height int) string {
	var sb strings.Builder

	// Calculate visible range
	visibleLines := height
	startIdx := 0
	if tv.cursor >= visibleLines {
		startIdx = tv.cursor - visibleLines + 1
	}
	endIdx := startIdx + visibleLines
	if endIdx > len(tv.FlatList) {
		endIdx = len(tv.FlatList)
	}

	for i := startIdx; i < endIdx; i++ {
		node := tv.FlatList[i]
		line := tv.renderNode(node, i == tv.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderNode renders a single node.
func (tv *TreeView) renderNode(node *TreeNode, selected bool) string {
	var sb strings.Builder

	// Indentation
	indent := strings.Repeat("  ", node.Depth)
	sb.WriteString(tv.styles.TreeBranch.Render(indent))

	// Expansion indicator
	if len(node.Children) > 0 {
		if node.Expanded {
			sb.WriteString(tv.styles.TreeBranch.Render("▼ "))
		} else {
			sb.WriteString(tv.styles.TreeBranch.Render("▶ "))
		}
	} else {
		sb.WriteString("  ")
	}

	// State indicator
	indicator := tv.stateIndicator(node.State)
	sb.WriteString(indicator)
	sb.WriteString(" ")

	// State name
	name := node.State.ID
	if tv.Machine.IsInitial(node.State.ID) {
		name = "● " + name
	}

	if selected {
		sb.WriteString(tv.styles.TreeNodeSelected.Render(name))
	} else {
		style := tv.stateStyle(node.State)
		sb.WriteString(style.Render(name))
	}

	// Transition count
	if len(node.State.Transitions) > 0 {
		transCount := fmt.Sprintf(" (%d transitions)", len(node.State.Transitions))
		sb.WriteString(tv.styles.TreeBranch.Render(transCount))
	}

	return sb.String()
}

// stateIndicator returns a visual indicator for the state type.
func (tv *TreeView) stateIndicator(state *viz.VizState) string {
	switch state.Type {
	case viz.VizStateFinal:
		return tv.styles.TreeFinal.Render("◉")
	case viz.VizStateCompound:
		return tv.styles.TreeCompound.Render("[+]")
	case viz.VizStateParallel:
		return tv.styles.TreeParallel.Render("[||]")
	case viz.VizStateHistory:
		if state.HistoryType == "deep" {
			return tv.styles.TreeHistory.Render("H*")
		}
		return tv.styles.TreeHistory.Render("H")
	default:
		return tv.styles.TreeLeaf.Render("○")
	}
}

// stateStyle returns the style for a state type.
func (tv *TreeView) stateStyle(state *viz.VizState) lipgloss.Style {
	switch state.Type {
	case viz.VizStateFinal:
		return tv.styles.TreeFinal
	case viz.VizStateCompound:
		return tv.styles.TreeCompound
	case viz.VizStateParallel:
		return tv.styles.TreeParallel
	case viz.VizStateHistory:
		return tv.styles.TreeHistory
	default:
		return tv.styles.TreeLeaf
	}
}
