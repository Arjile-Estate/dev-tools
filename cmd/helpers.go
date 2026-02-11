package cmd

import (
	"fmt"
	"strings"

	"dev-tools/internal/colors"
	"dev-tools/internal/commands"
	"dev-tools/internal/config"
)

// generateDynamicHelp creates help text that includes available commands
func generateDynamicHelp(dir string) string {
	baseHelp := `dev-tools is a command runner that reads configuration from .dev-config.yaml
and provides consistent commands across different project types.

It automatically detects project types (Go, Python, Node.js, Rust) and provides
sensible defaults, while allowing customization through configuration files.`

	// Try to load configuration to show available commands
	config, err := config.LoadConfigurationForProject(dir)
	if err != nil {
		return baseHelp + `

Examples:
  dev-tools test         # Run tests
  dev-tools lint         # Run linting
  dev-tools build        # Build project
  dev-tools logs         # Show recent logs
  dev-tools cleanup-pids # Clean up stale PID files`
	}

	// Add available commands
	var commandList []string
	commandSet := make(map[string]bool)

	// Add commands from config
	for cmdName := range config.Commands {
		commandList = append(commandList, cmdName)
		commandSet[cmdName] = true
	}

	// Add built-in commands from registry (avoid duplicates)
	builtInCommandNames := commands.GetBuiltInCommandNames()
	for _, cmdName := range builtInCommandNames {
		if !commandSet[cmdName] {
			commandList = append(commandList, cmdName)
		}
	}

	if len(commandList) > 0 {
		baseHelp += fmt.Sprintf(`

Available commands: %s

Examples:`, colors.Highlight(strings.Join(commandList, ", ")))

		// Show examples for first few commands
		exampleCount := 0
		for cmd := range config.Commands {
			if exampleCount >= 3 {
				break
			}
			baseHelp += fmt.Sprintf(`
  dev-tools %s`, cmd)
			exampleCount++
		}
		baseHelp += `
  dev-tools logs        # Show recent logs
  dev-tools --verbose test # Run with verbose logging`
	}

	return baseHelp
}
