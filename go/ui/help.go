package ui

import (
	"fmt"

	. "wilson/core/types"
)

// PrintToolHelp displays available tools grouped by category
func PrintToolHelp(tools []Tool) {
	fmt.Println("\n=== Available Tools ===")

	// Group by category
	categories := make(map[ToolCategory][]Tool)
	for _, tool := range tools {
		meta := tool.Metadata()
		categories[meta.Category] = append(categories[meta.Category], tool)
	}

	// Print by category
	categoryOrder := []ToolCategory{"filesystem", "web", "context", "agent", "system"}
	for _, cat := range categoryOrder {
		if tools, ok := categories[cat]; ok && len(tools) > 0 {
			fmt.Printf("\n%s (%d):\n", cat, len(tools))
			for _, tool := range tools {
				meta := tool.Metadata()
				fmt.Printf("  %-20s %s\n", meta.Name, meta.Description)
			}
		}
	}
	fmt.Println()
}
