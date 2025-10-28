package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"wilson/core/registry"
)

// TerminalConfirmation implements confirmation via terminal I/O
// This is the default confirmation handler for Wilson's CLI interface
type TerminalConfirmation struct{}

// RequestConfirmation displays confirmation prompt in terminal and waits for user response
func (t *TerminalConfirmation) RequestConfirmation(req registry.ConfirmationRequest) bool {
	fmt.Println()
	fmt.Printf("⚠️  Confirmation Required\n")
	fmt.Printf("Tool: %s\n", req.ToolName)
	fmt.Printf("Risk Level: %s\n", req.RiskLevel)
	fmt.Printf("Description: %s\n", req.Description)
	fmt.Printf("Arguments: %s\n", formatArgs(req.Arguments))
	fmt.Println()
	fmt.Print("Allow execution? (y/n): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return response == "y" || response == "yes"
	}

	// If we can't read input (EOF, error), default to deny
	return false
}

// formatArgs formats arguments for display
func formatArgs(args map[string]interface{}) string {
	data, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(data)
}
