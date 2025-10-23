package commands

import (
	"sync"

	"github.com/spf13/cobra"
)

// BuiltInCommand represents a built-in command
type BuiltInCommand struct {
	Name        string
	Description string
	Handler     func(*cobra.Command, []string, string) error
}

var (
	builtInCommandRegistry     []BuiltInCommand
	builtInCommandRegistryOnce sync.Once
)

// initBuiltInCommandRegistry initializes the built-in command registry (called once)
func initBuiltInCommandRegistry() {
	builtInCommandRegistryOnce.Do(func() {
		builtInCommandRegistry = []BuiltInCommand{
			{
				Name:        "logs",
				Description: "Show recent activity logs",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleLogsCommand(cmd, projectDir)
				},
			},
			{
				Name:        "status",
				Description: "Show daemon process status",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleStatusCommand(cmd, projectDir)
				},
			},
			{
				Name:        "cleanup-pids",
				Description: "Clean up stale PID files",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleCleanupPidsCommand(cmd, projectDir)
				},
			},
			{
				Name:        "cleanup-all",
				Description: "Clean up all daemon processes and PID files",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleCleanupAllCommand(cmd, projectDir)
				},
			},
			{
				Name:        "restart",
				Description: "Restart a daemon process",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleRestartCommand(cmd, args, projectDir)
				},
			},
			{
				Name:        "stop",
				Description: "Stop a daemon process",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleStopCommand(cmd, args, projectDir)
				},
			},
			{
				Name:        "version",
				Description: "Show version information",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					// Version is handled in cmd/root.go as it needs access to cmd.Version
					return nil
				},
			},
			{
				Name:        "completion",
				Description: "Generate shell completion script",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleCompletionCommand(cmd, args)
				},
			},
			{
				Name:        "__dev_complete",
				Description: "Internal completion command",
				Handler: func(cmd *cobra.Command, args []string, projectDir string) error {
					return HandleCompleteCommand(cmd, args, projectDir)
				},
			},
		}
	})
}

// GetBuiltInCommandNames returns all built-in command names
func GetBuiltInCommandNames() []string {
	initBuiltInCommandRegistry()
	names := make([]string, 0, len(builtInCommandRegistry))
	for _, cmd := range builtInCommandRegistry {
		// Skip internal commands
		if cmd.Name == "__dev_complete" {
			continue
		}
		names = append(names, cmd.Name)
	}
	return names
}

// GetBuiltInCommandMap returns a map of command names to handlers
// Compatible with the format used in cmd/root.go
func GetBuiltInCommandMap() map[string]func(*cobra.Command, []string, string) error {
	initBuiltInCommandRegistry()
	commandMap := make(map[string]func(*cobra.Command, []string, string) error)
	for _, cmd := range builtInCommandRegistry {
		commandMap[cmd.Name] = cmd.Handler
	}
	return commandMap
}

// GetBuiltInCommand retrieves a specific built-in command by name
func GetBuiltInCommand(name string) *BuiltInCommand {
	initBuiltInCommandRegistry()
	for _, cmd := range builtInCommandRegistry {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}

// IsBuiltInCommand checks if a command name is a built-in command
func IsBuiltInCommand(name string) bool {
	return GetBuiltInCommand(name) != nil
}
