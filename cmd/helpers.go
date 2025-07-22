package cmd

import (
	"fmt"
	"strings"

	"dev-tools/internal/colors"
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
  dev-tools test        # Run tests
  dev-tools lint        # Run linting
  dev-tools build       # Build project
  dev-tools logs        # Show recent logs
  dev-tools cleanup-pids # Clean up stale PID files`
	}

	// Add available commands
	var commands []string
	commandSet := make(map[string]bool)

	// Add commands from config
	for cmd := range config.Commands {
		commands = append(commands, cmd)
		commandSet[cmd] = true
	}

	// Add built-in commands (avoid duplicates)
	builtInCommands := []string{"logs", "cleanup-pids", "cleanup-all", "status", "restart", "stop", "version"}
	for _, cmd := range builtInCommands {
		if !commandSet[cmd] {
			commands = append(commands, cmd)
		}
	}

	if len(commands) > 0 {
		baseHelp += fmt.Sprintf(`

Available commands: %s

Examples:`, colors.Highlight(strings.Join(commands, ", ")))

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
