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
		Args:               cobra.MinimumNArgs(1),
		PersistentPreRun:   preRun,
		RunE:               runCommand,
		SilenceUsage:       false,
		SilenceErrors:      false,
		DisableFlagParsing: true, // Don't parse flags - pass all args through
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging to stdout")
	rootCmd.PersistentFlags().StringVarP(&projectDir, "project-dir", "p", ".", "Project directory to run commands in")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	rootCmd.Version = "0.16.0"

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

// parseArgs separates the command name from passthrough arguments
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

	// If we found a -- separator, use everything after it
	if separatorIndex > 0 && separatorIndex < len(args)-1 {
		passthroughArgs = args[separatorIndex+1:]
	} else if separatorIndex == -1 && len(args) > 1 {
		// No separator found - treat all args after command as passthrough
		passthroughArgs = args[1:]
	}

	return CommandArgs{
		CommandName:     commandName,
		PassthroughArgs: passthroughArgs,
	}
}

// extractDevToolsFlags extracts dev-tools flags from args
// Returns filtered args (without dev-tools flags) and a map of flag values
// Only extracts flags that appear BEFORE the command name
func extractDevToolsFlags(args []string) ([]string, map[string]string) {
	flags := make(map[string]string)
	var filtered []string

	commandFound := false
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check if this looks like a command name (doesn't start with -)
		if !strings.HasPrefix(arg, "-") {
			commandFound = true
		}

		// Only extract dev-tools flags before the command name
		if !commandFound {
			// Check for dev-tools flags
			if arg == "-v" || arg == "--verbose" {
				flags["verbose"] = "true"
				continue // Don't add to filtered
			} else if arg == "--no-color" {
				flags["no-color"] = "true"
				continue // Don't add to filtered
			} else if arg == "--version" {
				flags["version"] = "true"
				continue // Don't add to filtered
			} else if arg == "-h" || arg == "--help" {
				flags["help"] = "true"
				continue // Don't add to filtered
			} else if arg == "-p" || arg == "--project-dir" {
				if i+1 < len(args) {
					flags["project-dir"] = args[i+1]
					i++      // Skip the next arg (the value)
					continue // Don't add to filtered
				}
			} else if strings.HasPrefix(arg, "--project-dir=") {
				flags["project-dir"] = strings.TrimPrefix(arg, "--project-dir=")
				continue // Don't add to filtered
			}
		}

		// Add to filtered (either not a dev-tools flag, or after command name)
		filtered = append(filtered, arg)
	}

	return filtered, flags
}

// getBuiltInCommands creates a map of built-in commands with proper closure of projectDir and cmdVersion
func getBuiltInCommands(projectDir string, cmdVersion string) map[string]CommandFunc {
	// Get handlers from registry
	registryMap := commands.GetBuiltInCommandMap()

	// Convert to the format expected by runCommand
	builtInMap := make(map[string]CommandFunc)
	for name, handler := range registryMap {
		// Special handling for version command which needs access to cmd.Version
		if name == "version" {
			builtInMap[name] = func(cmd *cobra.Command, args []string) error {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", cmdVersion)
				return nil
			}
		} else {
			// Capture handler and projectDir in closure
			capturedHandler := handler
			capturedProjectDir := projectDir
			builtInMap[name] = func(cmd *cobra.Command, args []string) error {
				return capturedHandler(cmd, args, capturedProjectDir)
			}
		}
	}

	return builtInMap
}

func runCommand(cmd *cobra.Command, args []string) error {
	// With DisableFlagParsing=true, we need to manually extract dev-tools flags
	// Dev-tools flags must come BEFORE the command name
	filteredArgs, devToolsFlags := extractDevToolsFlags(args)

	// Handle --version flag
	if v, ok := devToolsFlags["version"]; ok && v == "true" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", cmd.Version)
		return nil
	}

	// Handle --help flag
	if v, ok := devToolsFlags["help"]; ok && v == "true" {
		return cmd.Help()
	}

	// Apply dev-tools flags
	if v, ok := devToolsFlags["verbose"]; ok && v == "true" {
		verbose = true
	}
	if v, ok := devToolsFlags["no-color"]; ok && v == "true" {
		noColor = true
	}
	if v, ok := devToolsFlags["project-dir"]; ok {
		projectDir = v
	}

	// Re-initialize colors and logging with the parsed flags
	colors.InitializeColorSupport(noColor)
	setupLogging(verbose, projectDir)

	parsedArgs := parseArgs(filteredArgs)
	commandName := parsedArgs.CommandName

	log.Printf("Starting dev-tools with command: %s, passthrough args: %v", commandName, parsedArgs.PassthroughArgs)

	// Handle special built-in commands
	builtInCommands := getBuiltInCommands(projectDir, cmd.Version)
	if commandFunc, exists := builtInCommands[commandName]; exists {
		return commandFunc(cmd, filteredArgs)
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
