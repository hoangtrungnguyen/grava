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
	sb.WriteString("    classDef wisp stroke-dasharray: 5 5\n")

	nodes := g.GetAllNodes()
	for _, node := range nodes {
		// Replace characters that Mermaid might find problematic
		cleanTitle := strings.ReplaceAll(node.Title, "\"", "'")
		cleanTitle = strings.ReplaceAll(cleanTitle, "[", "(")
		cleanTitle = strings.ReplaceAll(cleanTitle, "]", ")")

		displayName := cleanTitle
		if node.Ephemeral {
			displayName = "👻 " + cleanTitle
		}

		fmt.Fprintf(&sb, "    %s[\"%s<br/>(%s)\"]\n", node.ID, displayName, node.ID)

		// Apply styling based on status
		switch node.Status {
		case StatusInProgress:
			fmt.Fprintf(&sb, "    class %s in_progress\n", node.ID)
		case StatusClosed:
			fmt.Fprintf(&sb, "    class %s closed\n", node.ID)
		case StatusBlocked:
			fmt.Fprintf(&sb, "    class %s blocked\n", node.ID)
		default:
			fmt.Fprintf(&sb, "    class %s open\n", node.ID)
		}

		if node.Ephemeral {
			fmt.Fprintf(&sb, "    class %s wisp\n", node.ID)
		}
	}

	for _, edge := range g.GetAllEdges() {
		arrow := "-->"
		if !edge.Type.IsBlockingType() {
			arrow = "-.->" // Dashed line for non-blocking
		}

		// Optional: add edge type as label
		fmt.Fprintf(&sb, "    %s %s %s\n", edge.FromID, arrow, edge.ToID)
	}

	return sb.String()
}
