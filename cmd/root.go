package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/executor"
)

var (
	verbose    bool
	projectDir string
	noColor    bool
)

// NewRootCommand creates the root command for the CLI
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dev-tools [command]",
		Short: "Dev Tools - A command runner for development workflows",
		Long: `dev-tools is a command runner that reads configuration from .dev-config.yaml
and provides consistent commands across different project types.

It automatically detects project types (Go, Python, Node.js, Rust) and provides
sensible defaults, while allowing customization through configuration files.`,
		Args:             cobra.MinimumNArgs(1),
		PersistentPreRun: preRun,
		RunE:             runCommand,
		SilenceUsage:     false,
		SilenceErrors:    false,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging to stdout")
	rootCmd.PersistentFlags().StringVarP(&projectDir, "project-dir", "p", ".", "Project directory to run commands in")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	rootCmd.Version = "0.11.0"

	// Override help command to show available commands
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Initialize color support for help display
		colors.InitializeColorSupport(noColor)

		_, _ = fmt.Fprint(cmd.OutOrStdout(), generateDynamicHelp(projectDir))
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `
Usage:
  %s [flags] [command]

Flags:
  -h, --help                 help for dev-tools
      --no-color             Disable colored output
  -p, --project-dir string   Project directory to run commands in (default ".")
  -v, --verbose              Enable verbose logging to stdout
      --version              version for dev-tools
`, cmd.CommandPath())
	})

	return rootCmd
}

// isRunningViaGoRun detects if the application is running via 'go run'
func preRun(cmd *cobra.Command, args []string) {
	colors.InitializeColorSupport(noColor)
	setupLogging(verbose, projectDir)
}

type CommandFunc func(*cobra.Command, []string) error

var builtInCommands = map[string]CommandFunc{
	"logs": func(cmd *cobra.Command, args []string) error {
		return handleLogsCommand(cmd)
	},
	"cleanup-pids": func(cmd *cobra.Command, args []string) error {
		return handleCleanupPidsCommand(cmd)
	},
	"cleanup-all": func(cmd *cobra.Command, args []string) error {
		return handleCleanupAllCommand(cmd)
	},
	"status": func(cmd *cobra.Command, args []string) error {
		return handleStatusCommand(cmd)
	},
	"restart": func(cmd *cobra.Command, args []string) error {
		return handleRestartCommand(cmd, args)
	},
	"stop": func(cmd *cobra.Command, args []string) error {
		return handleStopCommand(cmd, args)
	},
	"version": func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", cmd.Root().Version)
		return nil
	},
	"completion": func(cmd *cobra.Command, args []string) error {
		return handleCompletionCommand(cmd, args)
	},
	"__dev_complete": func(cmd *cobra.Command, args []string) error {
		return handleCompleteCommand(cmd, args)
	},
}

func runCommand(cmd *cobra.Command, args []string) error {
	commandName := args[0]

	log.Printf("Starting dev-tools with command: %s", commandName)

	// Handle special built-in commands
	if commandFunc, exists := builtInCommands[commandName]; exists {
		return commandFunc(cmd, args)
	}

	// Load environment variables
	envFile := filepath.Join(projectDir, ".env")
	if err := executor.LoadEnvironmentVariables(envFile); err != nil {
		return fmt.Errorf("%s: %w", colors.Error("failed to load environment variables"), err)
	}

	// Load configuration
	config, err := config.LoadConfigurationForProject(projectDir)
	if err != nil {
		return fmt.Errorf("%s: %w", colors.Error("failed to load configuration"), err)
	}

	// Check if command exists
	steps, exists := config.Commands[commandName]
	if !exists {
		var availableCommands []string
		for cmd := range config.Commands {
			availableCommands = append(availableCommands, cmd)
		}
		return fmt.Errorf("%s", colors.Error("unknown command '%s'. Available commands: %s",
			commandName, strings.Join(availableCommands, ", ")))
	}

	// Execute command
	result := executor.ExecuteCommandWithSteps(commandName, steps, projectDir)

	// For most commands, we want to show output and return the exit code
	// rather than treating non-zero exit codes as errors
	if result.Stdout != "" {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	}

	if !result.Success {
		log.Printf("Command completed with exit code %d", result.ReturnCode)
		os.Exit(result.ReturnCode)
	}

	log.Print("Command completed successfully")
	return nil
}
