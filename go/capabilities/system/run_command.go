package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"wilson/core/registry"
	. "wilson/core/types"
)

type RunCommandTool struct{}

func (t *RunCommandTool) Metadata() ToolMetadata {
	return ToolMetadata{
		Name:            "run_command",
		Description:     "Run a shell command in the workspace directory (use with caution)",
		Category:        CategorySystem,
		RiskLevel:       RiskDangerous,
		RequiresConfirm: true, // Always confirm before running commands
		Enabled:         true,
		Parameters: []Parameter{
			{
				Name:        "command",
				Type:        "string",
				Required:    true,
				Description: "shell command to execute",
				Example:     "ls -la",
			},
		},
		Examples: []string{
			`{"tool": "run_command", "arguments": {"command": "ls -la"}}`,
			`{"tool": "run_command", "arguments": {"command": "git status"}}`,
		},
	}
}

func (t *RunCommandTool) Validate(args map[string]interface{}) error {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return fmt.Errorf("command parameter is required")
	}

	// Security: block dangerous commands
	dangerous := []string{"rm -rf", "sudo", "curl", "wget", "dd", "mkfs", "> /dev"}
	for _, d := range dangerous {
		if strings.Contains(strings.ToLower(cmdStr), d) {
			return fmt.Errorf("command blocked for safety: contains '%s'", d)
		}
	}

	return nil
}

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command parameter required")
	}

	// Validate first
	if err := t.Validate(args); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = GetSafeWorkspace()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func init() {
	registry.Register(&RunCommandTool{})
}
