package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// GraphJSON represents the JSON-serializable graph format
type GraphJSON struct {
	Nodes []NodeJSON `json:"nodes"`
	Edges []EdgeJSON `json:"edges"`
}

// NodeJSON is a JSON-serializable node
type NodeJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
}

// EdgeJSON is a JSON-serializable edge
type EdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// RenderOptions controls rendering behavior
type RenderOptions struct {
	Format    string // "ascii", "dot", "json"
	RootID    string // If set, render only subgraph from this node
	ExcludeFn func(*Node) bool // Optional filter function
}

// Render renders the graph in the specified format
func (g *AdjacencyDAG) Render(opts RenderOptions) (string, error) {
	if opts.RootID != "" {
		if _, err := g.GetNode(opts.RootID); err != nil {
			return "", fmt.Errorf("root node %q not found in graph", opts.RootID)
		}
	}
	switch opts.Format {
	case "ascii":
		return g.RenderASCII(opts)
	case "dot":
		return g.RenderDOT(opts)
	case "json":
		return g.RenderJSON(opts)
	default:
		return "", fmt.Errorf("unsupported format: %s", opts.Format)
	}
}

// RenderASCII returns an ASCII-art tree representation of the graph
func (g *AdjacencyDAG) RenderASCII(opts RenderOptions) (string, error) {
	var sb strings.Builder

	allNodes := g.GetAllNodes()
	allEdges := g.GetAllEdges()

	// Filter nodes based on opts
	nodesToShow := make(map[string]*Node)
	edgesToShow := make([]*Edge, 0)

	if opts.RootID != "" {
		// Subgraph: only show nodes reachable from root
		reachable := g.getReachableNodes(opts.RootID)
		for _, nodeID := range reachable {
			if node, err := g.GetNode(nodeID); err == nil {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[nodeID] = node
				}
			}
		}
		// Include root node itself
		if root, err := g.GetNode(opts.RootID); err == nil {
			if opts.ExcludeFn == nil || !opts.ExcludeFn(root) {
				nodesToShow[opts.RootID] = root
			}
		}
	} else {
		// Full graph: include all non-archived nodes
		for _, node := range allNodes {
			if node.Status != StatusArchived {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[node.ID] = node
				}
			}
		}
	}

	// Collect edges between shown nodes
	for _, edge := range allEdges {
		if _, fromOK := nodesToShow[edge.FromID]; fromOK {
			if _, toOK := nodesToShow[edge.ToID]; toOK {
				edgesToShow = append(edgesToShow, edge)
			}
		}
	}

	// Simple edge list format
	sb.WriteString("Dependency Graph\n")
	sb.WriteString("================\n\n")

	// Group edges by type for readability
	edgesByType := make(map[DependencyType][]*Edge)
	for _, edge := range edgesToShow {
		edgesByType[edge.Type] = append(edgesByType[edge.Type], edge)
	}

	// Sort types for consistent output
	var types []DependencyType
	for t := range edgesByType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	// Render edges grouped by type
	for _, depType := range types {
		edges := edgesByType[depType]
		sort.Slice(edges, func(i, j int) bool {
			if edges[i].FromID != edges[j].FromID {
				return edges[i].FromID < edges[j].FromID
			}
			return edges[i].ToID < edges[j].ToID
		})

		sb.WriteString(fmt.Sprintf("%s:\n", depType))
		for _, edge := range edges {
			fromNode := nodesToShow[edge.FromID]
			toNode := nodesToShow[edge.ToID]
			sb.WriteString(fmt.Sprintf("  [%s] %s --> [%s] %s\n",
				edge.FromID, fromNode.Title,
				edge.ToID, toNode.Title))
		}
		sb.WriteString("\n")
	}

	// List isolated nodes (nodes with no edges)
	isolated := make([]*Node, 0)
	for nodeID, node := range nodesToShow {
		hasEdges := false
		for _, edge := range edgesToShow {
			if edge.FromID == nodeID || edge.ToID == nodeID {
				hasEdges = true
				break
			}
		}
		if !hasEdges {
			isolated = append(isolated, node)
		}
	}

	if len(isolated) > 0 {
		sb.WriteString("Isolated Nodes:\n")
		sort.Slice(isolated, func(i, j int) bool {
			return isolated[i].ID < isolated[j].ID
		})
		for _, node := range isolated {
			sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", node.ID, node.Title, node.Type))
		}
	}

	return sb.String(), nil
}

// RenderDOT returns a Graphviz DOT representation
func (g *AdjacencyDAG) RenderDOT(opts RenderOptions) (string, error) {
	var sb strings.Builder

	allNodes := g.GetAllNodes()
	allEdges := g.GetAllEdges()

	// Filter nodes
	nodesToShow := make(map[string]*Node)

	if opts.RootID != "" {
		reachable := g.getReachableNodes(opts.RootID)
		for _, nodeID := range reachable {
			if node, err := g.GetNode(nodeID); err == nil {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[nodeID] = node
				}
			}
		}
		if root, err := g.GetNode(opts.RootID); err == nil {
			if opts.ExcludeFn == nil || !opts.ExcludeFn(root) {
				nodesToShow[opts.RootID] = root
			}
		}
	} else {
		for _, node := range allNodes {
			if node.Status != StatusArchived {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[node.ID] = node
				}
			}
		}
	}

	// Collect edges between shown nodes
	edgesToShow := make([]*Edge, 0)
	for _, edge := range allEdges {
		if _, fromOK := nodesToShow[edge.FromID]; fromOK {
			if _, toOK := nodesToShow[edge.ToID]; toOK {
				edgesToShow = append(edgesToShow, edge)
			}
		}
	}

	// Write DOT format
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=rounded];\n\n")

	// Write nodes
	for _, node := range nodesToShow {
		color := "white"
		switch node.Status {
		case StatusClosed:
			color = "gray"
		case StatusInProgress:
			color = "lightblue"
		case StatusBlocked:
			color = "lightcoral"
		}
		label := strings.ReplaceAll(node.Title, "\"", "\\\"")
		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", fillcolor=\"%s\", style=\"filled,rounded\"];\n",
			node.ID, label, color))
	}

	sb.WriteString("\n")

	// Write edges
	for _, edge := range edgesToShow {
		style := "solid"
		if edge.Type.IsSoftDependency() {
			style = "dashed"
		}
		sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\", style=\"%s\"];\n",
			edge.FromID, edge.ToID, edge.Type, style))
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

// RenderJSON returns a JSON representation of the graph
func (g *AdjacencyDAG) RenderJSON(opts RenderOptions) (string, error) {
	allNodes := g.GetAllNodes()
	allEdges := g.GetAllEdges()

	// Filter nodes
	nodesToShow := make(map[string]*Node)

	if opts.RootID != "" {
		reachable := g.getReachableNodes(opts.RootID)
		for _, nodeID := range reachable {
			if node, err := g.GetNode(nodeID); err == nil {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[nodeID] = node
				}
			}
		}
		if root, err := g.GetNode(opts.RootID); err == nil {
			if opts.ExcludeFn == nil || !opts.ExcludeFn(root) {
				nodesToShow[opts.RootID] = root
			}
		}
	} else {
		for _, node := range allNodes {
			if node.Status != StatusArchived {
				if opts.ExcludeFn == nil || !opts.ExcludeFn(node) {
					nodesToShow[node.ID] = node
				}
			}
		}
	}

	// Collect edges between shown nodes
	edgesToShow := make([]*Edge, 0)
	for _, edge := range allEdges {
		if _, fromOK := nodesToShow[edge.FromID]; fromOK {
			if _, toOK := nodesToShow[edge.ToID]; toOK {
				edgesToShow = append(edgesToShow, edge)
			}
		}
	}

	// Build JSON structure
	nodeArray := make([]NodeJSON, 0, len(nodesToShow))
	for _, node := range nodesToShow {
		nodeArray = append(nodeArray, NodeJSON{
			ID:       node.ID,
			Title:    node.Title,
			Type:     node.Type,
			Status:   string(node.Status),
			Priority: int(node.Priority),
		})
	}

	// Sort for consistent output
	sort.Slice(nodeArray, func(i, j int) bool {
		return nodeArray[i].ID < nodeArray[j].ID
	})

	edgeArray := make([]EdgeJSON, 0, len(edgesToShow))
	for _, edge := range edgesToShow {
		edgeArray = append(edgeArray, EdgeJSON{
			From: edge.FromID,
			To:   edge.ToID,
			Type: string(edge.Type),
		})
	}

	result := GraphJSON{
		Nodes: nodeArray,
		Edges: edgeArray,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// getReachableNodes returns all nodes reachable from the given node (BFS)
func (g *AdjacencyDAG) getReachableNodes(startID string) []string {
	visited := make(map[string]bool)
	queue := []string{startID}
	var result []string

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true
		result = append(result, current)

		// Add successors
		if successors, err := g.GetSuccessors(current); err == nil {
			for _, succ := range successors {
				if !visited[succ] {
					queue = append(queue, succ)
				}
			}
		}
	}

	return result
}
