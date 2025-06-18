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
	builtInCommands := []string{"logs", "cleanup-pids", "version"}
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

// NewRootCommand creates the root command for the CLI
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dev-tools [command]",
		Short: "Dev Tools - A command runner for development workflows",
		Long: `dev-tools is a command runner that reads configuration from .dev-config.yaml
and provides consistent commands across different project types.

It automatically detects project types (Go, Python, Node.js, Rust) and provides
sensible defaults, while allowing customization through configuration files.`,
		Args:              cobra.MinimumNArgs(1),
		PersistentPreRun:  setupLogging,
		RunE:              runCommand,
		SilenceUsage:      false,
		SilenceErrors:     false,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging to stdout")
	rootCmd.PersistentFlags().StringVarP(&projectDir, "project-dir", "p", ".", "Project directory to run commands in")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	rootCmd.Version = "0.7.1"

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
func isRunningViaGoRun(executable string) bool {
	// 'go run' creates temporary executables in paths like:
	// /tmp/go-build123456789/b001/exe/main (Linux)
	// /var/folders/xy/abcdef/T/go-build987654321/b001/exe/main (macOS)
	return strings.Contains(executable, "go-build") &&
		   (strings.Contains(executable, "/tmp/") || strings.Contains(executable, "/T/"))
}

// getHomeDirectory returns the user's home directory
func getHomeDirectory() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME environment variable not set")
	}
	return home, nil
}

// ensureLogDirectory creates the log directory if it doesn't exist
func ensureLogDirectory(logFilePath string) error {
	logDir := filepath.Dir(logFilePath)
	return os.MkdirAll(logDir, 0755)
}

// getLogFilePath returns the appropriate log file path based on execution method
func getLogFilePath(executable, homeDir, projectDir string) (string, bool) {
	isGoRun := isRunningViaGoRun(executable)

	if isGoRun {
		// When running via 'go run', use project directory
		return filepath.Join(projectDir, "activity.log"), true
	} else {
		// When running compiled binary, use ~/Library/Logs/dev-tools.log
		return filepath.Join(homeDir, "Library", "Logs", "dev-tools.log"), false
	}
}

func setupLogging(cmd *cobra.Command, args []string) {
	// Initialize color support
	colors.InitializeColorSupport(noColor)

	// Setup basic logging
	if verbose {
		log.SetOutput(os.Stdout)
	} else {
		// Get executable path
		executable, err := os.Executable()
		if err != nil {
			// Fallback to project directory if we can't determine executable
			logFile := filepath.Join(projectDir, "activity.log")
			if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				log.SetOutput(file)
			}
			log.SetFlags(log.LstdFlags)
			return
		}

		// Get home directory
		homeDir, err := getHomeDirectory()
		if err != nil {
			// Fallback to project directory if we can't get home directory
			logFile := filepath.Join(projectDir, "activity.log")
			if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				log.SetOutput(file)
			}
			log.SetFlags(log.LstdFlags)
			return
		}

		// Determine log file path based on execution method
		logFile, _ := getLogFilePath(executable, homeDir, projectDir)

		// Ensure log directory exists
		if err := ensureLogDirectory(logFile); err != nil {
			// Fallback to project directory if we can't create log directory
			logFile = filepath.Join(projectDir, "activity.log")
		}

		if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(file)
		}
	}
	log.SetFlags(log.LstdFlags)
}

func runCommand(cmd *cobra.Command, args []string) error {
	commandName := args[0]

	log.Printf("Starting dev-tools with command: %s", commandName)

	// Handle special built-in commands
	switch commandName {
	case "logs":
		return handleLogsCommand(cmd)
	case "cleanup-pids":
		return handleCleanupPidsCommand(cmd)
	case "version":
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", cmd.Root().Version)
		return nil
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

func handleLogsCommand(cmd *cobra.Command) error {
	log.Print("Displaying recent activity logs")

	// Get executable path
	executable, err := os.Executable()
	if err != nil {
		// Fallback to project directory if we can't determine executable
		logFile := filepath.Join(projectDir, "activity.log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("%s", colors.Error("no log file found at %s", logFile))
		}

		result := executor.ExecuteShellCommand(executor.ExecuteOptions{
			Command:       fmt.Sprintf("tail -n 50 %s", logFile),
			CaptureOutput: true,
		})

		if !result.Success {
			return fmt.Errorf("%s", colors.Error("failed to read logs: %s", result.Stderr))
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
		return nil
	}

	// Get home directory
	homeDir, err := getHomeDirectory()
	if err != nil {
		// Fallback to project directory if we can't get home directory
		logFile := filepath.Join(projectDir, "activity.log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("%s", colors.Error("no log file found at %s", logFile))
		}

		result := executor.ExecuteShellCommand(executor.ExecuteOptions{
			Command:       fmt.Sprintf("tail -n 50 %s", logFile),
			CaptureOutput: true,
		})

		if !result.Success {
			return fmt.Errorf("%s", colors.Error("failed to read logs: %s", result.Stderr))
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
		return nil
	}

	// Determine log file path based on execution method
	logFile, _ := getLogFilePath(executable, homeDir, projectDir)

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s", logFile)
	}

	result := executor.ExecuteShellCommand(executor.ExecuteOptions{
		Command:       fmt.Sprintf("tail -n 50 %s", logFile),
		CaptureOutput: true,
	})

	if !result.Success {
		return fmt.Errorf("failed to read logs: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func handleCleanupPidsCommand(cmd *cobra.Command) error {
	result := executor.CleanupStalePIDFiles(projectDir)
	if !result.Success {
		return fmt.Errorf("cleanup failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}
