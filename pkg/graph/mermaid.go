package graph

import (
	"fmt"
	"strings"
)

// ToMermaid exports the DAG to Mermaid.js format
func ToMermaid(g DAG) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Class definitions for styling
	sb.WriteString("    classDef open fill:#fff,stroke:#333,stroke-width:2px\n")
	sb.WriteString("    classDef in_progress fill:#e1f5fe,stroke:#01579b,stroke-width:2px\n")
	sb.WriteString("    classDef closed fill:#eeeeee,stroke:#9e9e9e,stroke-width:2px,color:#9e9e9e\n")
	sb.WriteString("    classDef blocked fill:#fff9c4,stroke:#fbc02d,stroke-width:2px\n")

	nodes := g.GetAllNodes()
	for _, node := range nodes {
		// Replace characters that Mermaid might find problematic
		cleanTitle := strings.ReplaceAll(node.Title, "\"", "'")
		cleanTitle = strings.ReplaceAll(cleanTitle, "[", "(")
		cleanTitle = strings.ReplaceAll(cleanTitle, "]", ")")

		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>(%s)\"]\n", node.ID, cleanTitle, node.ID))

		// Apply styling based on status
		switch node.Status {
		case StatusInProgress:
			sb.WriteString(fmt.Sprintf("    class %s in_progress\n", node.ID))
		case StatusClosed:
			sb.WriteString(fmt.Sprintf("    class %s closed\n", node.ID))
		case StatusBlocked:
			sb.WriteString(fmt.Sprintf("    class %s blocked\n", node.ID))
		default:
			sb.WriteString(fmt.Sprintf("    class %s open\n", node.ID))
		}
	}

	for _, edge := range g.GetAllEdges() {
		arrow := "-->"
		if !edge.Type.IsBlockingType() {
			arrow = "-.->" // Dashed line for non-blocking
		}

		// Optional: add edge type as label
		sb.WriteString(fmt.Sprintf("    %s %s %s\n", edge.FromID, arrow, edge.ToID))
	}

	return sb.String()
}
