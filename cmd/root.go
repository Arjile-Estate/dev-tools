package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"dev-tools/internal/colors"
	"dev-tools/internal/commands"
	"dev-tools/internal/config"
	"dev-tools/internal/executor"
)

var (
	verbose      bool
	projectDir   string
	noColor      bool
	configLoader config.ConfigLoader
	exec         executor.Executor
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

	rootCmd.Version = "0.12.2"

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

// CommandArgs represents parsed command arguments
type CommandArgs struct {
	CommandName     string
	PassthroughArgs []string
}

// parseArgsFromOsArgs parses arguments from os.Args to handle Cobra's -- consumption
func parseArgsFromOsArgs(args []string) CommandArgs {
	if len(args) == 0 {
		return CommandArgs{}
	}

	commandName := args[0]
	var passthroughArgs []string

	// Parse from os.Args since Cobra consumes the -- separator
	// Find the command in os.Args and then look for -- after it
	commandFound := false
	separatorIndex := -1
	for i, arg := range os.Args {
		if commandFound && arg == "--" {
			separatorIndex = i
			break
		}
		if arg == commandName {
			commandFound = true
		}
	}

	if separatorIndex > 0 && separatorIndex < len(os.Args)-1 {
		passthroughArgs = os.Args[separatorIndex+1:]
	}

	return CommandArgs{
		CommandName:     commandName,
		PassthroughArgs: passthroughArgs,
	}
}

// parseArgs separates the command name from passthrough arguments using -- separator
// This version is used for testing with provided arguments
func parseArgs(args []string) CommandArgs {
	if len(args) == 0 {
		return CommandArgs{}
	}

	commandName := args[0]
	var passthroughArgs []string

	// Find "--" separator
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	if separatorIndex > 0 && separatorIndex < len(args)-1 {
		passthroughArgs = args[separatorIndex+1:]
	}

	return CommandArgs{
		CommandName:     commandName,
		PassthroughArgs: passthroughArgs,
	}
}

var builtInCommands = map[string]CommandFunc{
	"logs": func(cmd *cobra.Command, args []string) error {
		return commands.HandleLogsCommand(cmd, projectDir)
	},
	"cleanup-pids": func(cmd *cobra.Command, args []string) error {
		return commands.HandleCleanupPidsCommand(cmd, projectDir)
	},
	"cleanup-all": func(cmd *cobra.Command, args []string) error {
		return commands.HandleCleanupAllCommand(cmd, projectDir)
	},
	"status": func(cmd *cobra.Command, args []string) error {
		return commands.HandleStatusCommand(cmd, projectDir)
	},
	"restart": func(cmd *cobra.Command, args []string) error {
		return commands.HandleRestartCommand(cmd, args, projectDir)
	},
	"stop": func(cmd *cobra.Command, args []string) error {
		return commands.HandleStopCommand(cmd, args, projectDir)
	},
	"version": func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", cmd.Root().Version)
		return nil
	},
	"completion": func(cmd *cobra.Command, args []string) error {
		return commands.HandleCompletionCommand(cmd, args)
	},
	"__dev_complete": func(cmd *cobra.Command, args []string) error {
		return commands.HandleCompleteCommand(cmd, args, projectDir)
	},
}

func runCommand(cmd *cobra.Command, args []string) error {
	parsedArgs := parseArgsFromOsArgs(args)
	commandName := parsedArgs.CommandName

	log.Printf("Starting dev-tools with command: %s, passthrough args: %v", commandName, parsedArgs.PassthroughArgs)

	// Handle special built-in commands
	if commandFunc, exists := builtInCommands[commandName]; exists {
		return commandFunc(cmd, args)
	}

	// Load environment variables
	envFile := filepath.Join(projectDir, ".env")
	if exec != nil {
		if err := exec.LoadEnvironmentVariables(envFile); err != nil {
			return fmt.Errorf("%s: %w", colors.Error("failed to load environment variables"), err)
		}
	} else {
		if err := executor.LoadEnvironmentVariables(envFile); err != nil {
			return fmt.Errorf("%s: %w", colors.Error("failed to load environment variables"), err)
		}
	}

	// Load configuration
	var cfg *config.Config
	var err error
	if configLoader != nil {
		cfg, err = configLoader.LoadConfig(projectDir)
	} else {
		cfg, err = config.LoadConfigurationForProject(projectDir)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", colors.Error("failed to load configuration"), err)
	}

	// Check if command exists
	steps, exists := cfg.Commands[commandName]
	if !exists {
		var availableCommands []string
		for cmd := range cfg.Commands {
			availableCommands = append(availableCommands, cmd)
		}
		return fmt.Errorf("%s", colors.Error("unknown command '%s'. Available commands: %s",
			commandName, strings.Join(availableCommands, ", ")))
	}

	// Execute command
	var result executor.ExecutionResult
	if exec != nil {
		result = exec.ExecuteCommandWithSteps(commandName, steps, projectDir, parsedArgs.PassthroughArgs)
	} else {
		result = executor.ExecuteCommandWithSteps(commandName, steps, projectDir, parsedArgs.PassthroughArgs)
	}

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

// SetConfigLoader sets the config loader for the root command.
func SetConfigLoader(loader config.ConfigLoader) {
	configLoader = loader
}

// SetExecutor sets the exec for the root command.
func SetExecutor(e executor.Executor) {
	exec = e
}
