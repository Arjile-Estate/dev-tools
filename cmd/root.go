package cmd

import (
	"context"
	"dev-tools/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"dev-tools/internal/colors"
	"dev-tools/internal/commands"
	"dev-tools/internal/config"
	"dev-tools/internal/executor"
)

// CommandConfig holds runtime configuration for command execution
type CommandConfig struct {
	Verbose    bool
	ProjectDir string
	NoColor    bool
	Format     string
	Watch      bool
}

// version is the application version, injected at build time via ldflags
var version = "1.6.1"

// claudeCodeEnvVar is the environment variable Claude Code sets in
// subprocesses spawned by its Bash tool. See
// https://code.claude.com/docs/en/env-vars.md
const claudeCodeEnvVar = "CLAUDE_CODE"

// defaultOutputFormat returns the format to use when --format is not
// explicitly passed. When running inside a Claude Code session
// (CLAUDE_CODE=1), default to "json" so an LLM gets structured output;
// otherwise default to "text".
func defaultOutputFormat() string {
	if os.Getenv(claudeCodeEnvVar) == "1" {
		return "json"
	}
	return "text"
}

var (
	// Dependency injection variables (used for testing)
	configLoader config.ConfigLoader
	exec         executor.Executor
)

// ExitError is an error that includes a specific exit code
// This allows proper cleanup (defer statements) while preserving exit codes
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	return e.Message
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
		Args:               cobra.MinimumNArgs(1),
		RunE:               runCommand,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true, // Don't parse flags - pass all args through
	}

	rootCmd.Version = version

	// Override help command to show available commands
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Parse flags for help display
		_, devToolsFlags := extractDevToolsFlags(args)
		cfg := configFromFlags(devToolsFlags)

		// Initialize color support for help display
		colors.InitializeColorSupport(cfg.NoColor)

		_, _ = fmt.Fprint(cmd.OutOrStdout(), generateDynamicHelp(cfg.ProjectDir))
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), `
Usage:
  %s [global flags] <command> [args...]

Global flags (must be placed BEFORE the command name):
      --format string        Output format: text or json (default "text")
  -h, --help                 help for dev-tools
      --no-color             Disable colored output
  -p, --project-dir string   Project directory to run commands in (default ".")
  -v, --verbose              Enable verbose logging to stdout
  -w, --watch                Watch mode: re-run command on file changes
      --version              version for dev-tools

Any arguments after the command name are passed through to the command, not
interpreted by dev-tools. For example, use 'dev-tools -p ./svc validate',
not 'dev-tools validate -p ./svc'.
`, cmd.CommandPath())
	})

	return rootCmd
}

// configFromFlags creates a CommandConfig from parsed flag values.
// When --format is not explicitly present in flags, the default is
// determined by defaultOutputFormat (CLAUDE_CODE=1 → "json", else "text").
// An explicit --format value always wins.
func configFromFlags(flags map[string]string) CommandConfig {
	cfg := CommandConfig{
		ProjectDir: ".", // default
		Format:     defaultOutputFormat(),
	}

	if v, ok := flags["verbose"]; ok && v == "true" {
		cfg.Verbose = true
	}
	if v, ok := flags["watch"]; ok && v == "true" {
		cfg.Watch = true
	}
	if v, ok := flags["no-color"]; ok && v == "true" {
		cfg.NoColor = true
	}
	if v, ok := flags["project-dir"]; ok {
		cfg.ProjectDir = v
	}
	if v, ok := flags["format"]; ok {
		cfg.Format = v
	}

	return cfg
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
			} else if arg == "-w" || arg == "--watch" {
				flags["watch"] = "true"
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
			} else if arg == "--format" {
				if i+1 < len(args) {
					flags["format"] = args[i+1]
					i++      // Skip the next arg (the value)
					continue // Don't add to filtered
				}
			} else if strings.HasPrefix(arg, "--format=") {
				flags["format"] = strings.TrimPrefix(arg, "--format=")
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
				return printVersion(cmd, cmdVersion, commands.FormatFromContext(cmd))
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

// printVersion writes the version string in the requested format. JSON mode
// emits {"version":"X.Y.Z"} so an LLM can parse it programmatically; the
// text path preserves the existing human-readable line.
func printVersion(cmd *cobra.Command, v, format string) error {
	if format == "json" {
		return commands.EmitJSON(cmd, map[string]any{"version": v})
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "dev-tools version", v)
	return nil
}

// formatResultAsJSON converts an ExecutionResult to JSON output
func formatResultAsJSON(result executor.ExecutionResult) (string, error) {
	output := map[string]interface{}{
		"command":          result.CommandName,
		"success":          result.Success,
		"return_code":      result.ReturnCode,
		"duration_ms":      result.DurationMs,
		"services_started": result.ServicesStarted,
	}

	// Only include stdout if not empty
	if result.Stdout != "" {
		output["stdout"] = result.Stdout
	}

	// Only include stderr if not empty
	if result.Stderr != "" {
		output["stderr"] = result.Stderr
	}

	// Only include PID if present (for daemon commands)
	if result.PID > 0 {
		output["daemon_pid"] = result.PID
	}

	// Only include warnings if any exist
	if len(result.Warnings) > 0 {
		output["warnings"] = result.Warnings
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func runCommand(cmd *cobra.Command, args []string) error {
	// With DisableFlagParsing=true, we need to manually extract dev-tools flags
	// Dev-tools flags must come BEFORE the command name
	filteredArgs, devToolsFlags := extractDevToolsFlags(args)

	// Resolve configuration first so --version and --help can honor --format.
	cmdCfg := configFromFlags(devToolsFlags)

	// Handle --version flag
	if v, ok := devToolsFlags["version"]; ok && v == "true" {
		return printVersion(cmd, cmd.Version, cmdCfg.Format)
	}

	// Handle --help flag
	if v, ok := devToolsFlags["help"]; ok && v == "true" {
		return cmd.Help()
	}

	// Propagate the resolved output format to built-in command handlers via
	// the cobra command context. They read it back with
	// commands.FormatFromContext(cmd) to decide between text and JSON output.
	parentCtx := cmd.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	cmd.SetContext(context.WithValue(parentCtx, commands.FormatCtxKey, cmdCfg.Format))

	// Initialize colors and logging with the parsed flags
	colors.InitializeColorSupport(cmdCfg.NoColor)
	cleanupLog := setupLogging(cmdCfg.Verbose, cmdCfg.ProjectDir)
	defer cleanupLog()

	parsedArgs := parseArgs(filteredArgs)
	commandName := parsedArgs.CommandName

	absProjectDir, err := filepath.Abs(cmdCfg.ProjectDir)
	if err != nil {
		absProjectDir = cmdCfg.ProjectDir
	}
	logger.SetContext(absProjectDir, commandName)

	logger.Infof("Starting dev-tools with command: %s, passthrough args: %v", commandName, parsedArgs.PassthroughArgs)

	// Load configuration early so user-defined commands can override built-in commands
	var cfg *config.Config
	var configErr error
	if configLoader != nil {
		cfg, configErr = configLoader.LoadConfig(cmdCfg.ProjectDir)
	} else {
		cfg, configErr = config.LoadConfigurationForProject(cmdCfg.ProjectDir)
	}

	// User-defined commands take precedence over built-in commands
	var steps []config.CommandStep
	userDefined := false
	if configErr == nil {
		if s, exists := cfg.Commands[commandName]; exists {
			steps = s
			userDefined = true
		}
	}

	if userDefined {
		// Load environment variables for user-defined commands
		envFile := filepath.Join(cmdCfg.ProjectDir, ".env")
		if exec != nil {
			if err := exec.LoadEnvironmentVariables(envFile); err != nil {
				return fmt.Errorf("%s: %w", colors.Error("failed to load environment variables"), err)
			}
		} else {
			if err := executor.LoadEnvironmentVariables(envFile); err != nil {
				return fmt.Errorf("%s: %w", colors.Error("failed to load environment variables"), err)
			}
		}
	} else {
		// Fall back to built-in commands
		builtInCommands := getBuiltInCommands(cmdCfg.ProjectDir, cmd.Version)
		if commandFunc, exists := builtInCommands[commandName]; exists {
			return commandFunc(cmd, filteredArgs)
		}

		// No built-in match either — report the appropriate error
		if configErr != nil {
			return fmt.Errorf("%s: %w", colors.Error("failed to load configuration"), configErr)
		}

		var availableCommands []string
		for cmd := range cfg.Commands {
			availableCommands = append(availableCommands, cmd)
		}
		return fmt.Errorf("%s", colors.Error("unknown command '%s'. Available commands: %s",
			commandName, strings.Join(availableCommands, ", ")))
	}

	// Handle watch mode
	if cmdCfg.Watch {
		// Create context with cancellation for watch mode
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle Ctrl+C gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			logger.Infof("Received interrupt signal, stopping watch mode")
			cancel()
		}()

		// Run in watch mode
		if err := executor.WatchAndExecute(ctx, commandName, steps, cmdCfg.ProjectDir, parsedArgs.PassthroughArgs); err != nil {
			return fmt.Errorf("%s: %w", colors.Error("watch mode failed"), err)
		}
		return nil
	}

	// Execute command (non-watch mode)
	execOpts := executor.CommandExecutionOptions{
		CommandName:     commandName,
		Steps:           steps,
		WorkingDir:      cmdCfg.ProjectDir,
		PassthroughArgs: parsedArgs.PassthroughArgs,
	}

	var result executor.ExecutionResult
	if exec != nil {
		result = exec.ExecuteCommandWithOptions(execOpts)
	} else {
		result = executor.ExecuteCommandWithOptions(execOpts)
	}

	// Output based on format
	if cmdCfg.Format == "json" {
		// JSON output mode
		jsonOutput, err := formatResultAsJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON output: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), jsonOutput)
	} else {
		// Text output mode (default)
		if result.Stdout != "" {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
		}
	}

	if !result.Success {
		// Determine which command label to use in the error message
		failedLabel := result.FailedCommand
		if failedLabel == "" {
			failedLabel = result.CommandName
		}

		// Log the full output for debugging
		if result.Stdout != "" || result.Stderr != "" {
			logger.Warnf("Command '%s' failed output - stdout: %s, stderr: %s", failedLabel, result.Stdout, result.Stderr)
		}

		// Safety net: ensure non-zero exit code on failure. ReturnCode defaults to 0 in Go,
		// so if an error path forgot to set it, we correct it here to avoid exiting with 0.
		returnCode := result.ReturnCode
		if returnCode == 0 {
			returnCode = 1
		}
		logger.Warnf("Command '%s' failed with exit code %d", failedLabel, returnCode)

		return &ExitError{
			Code:    returnCode,
			Message: fmt.Sprintf("Command '%s' failed with error code: %d", failedLabel, returnCode),
		}
	}

	logger.Info("Command completed successfully")
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
