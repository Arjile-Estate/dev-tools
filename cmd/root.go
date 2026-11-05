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
	"time"

	"github.com/spf13/cobra"

	"dev-tools/internal/colors"
	"dev-tools/internal/commands"
	"dev-tools/internal/config"
	"dev-tools/internal/devresult"
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
var version = "1.6.2"

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
	resultConfig, resultEnabled := devResultCommandConfig(cfg, commandName, steps, cmdCfg.Watch)
	execOpts := executor.CommandExecutionOptions{
		Context:         cmd.Context(),
		CommandName:     commandName,
		Steps:           steps,
		WorkingDir:      cmdCfg.ProjectDir,
		PassthroughArgs: parsedArgs.PassthroughArgs,
		CaptureOutput:   resultEnabled,
	}
	var validationRun devresult.ValidationRun
	var resultWarnings []string
	if resultEnabled {
		validationRun, err = devresult.CreateValidationRun(cmdCfg.ProjectDir, commandName, time.Now())
		if err != nil {
			resultWarnings = append(resultWarnings, "run-scoped diagnostics are unavailable: "+err.Error())
		} else {
			execOpts.Environment = validationRun.Environment()
			defer func() {
				if cleanupErr := os.RemoveAll(validationRun.Dir); cleanupErr != nil {
					logger.Warnf("could not remove validation scratch directory %q: %v", validationRun.Dir, cleanupErr)
				}
			}()
		}
	}

	var result executor.ExecutionResult
	if exec != nil {
		result = exec.ExecuteCommandWithOptions(execOpts)
	} else {
		result = executor.ExecuteCommandWithOptions(execOpts)
	}

	if resultEnabled {
		collected := collectDiagnostics(cmd.Context(), cmdCfg.ProjectDir, commandName, result, resultConfig.DiagnosticsAdapter, validationRun)
		retainedEvidence := collected.evidence
		if collected.warning != "" {
			resultWarnings = append(resultWarnings, collected.warning)
			logger.Warnf("DEV-RESULT %s diagnostics fell back to the rendered log: %s", commandName, collected.warning)
		} else if collected.fallbackReason != "" {
			logger.Infof("DEV-RESULT %s diagnostics fell back to the rendered log: %s", commandName, collected.fallbackReason)
		}
		if cfg.DevResult != nil && validationRun.ID != "" {
			retention, retentionErr := devresult.ApplyRetention(cmdCfg.ProjectDir, validationRun.ID, time.Now(), devresult.RetentionPolicy{
				MaxAge:   cfg.DevResult.Retention.MaxAge,
				MaxBytes: cfg.DevResult.Retention.MaxBytes,
			})
			if retentionErr != nil {
				resultWarnings = append(resultWarnings, "DEV-RESULT retention unavailable: "+retentionErr.Error())
			} else if retention.Warning != "" {
				resultWarnings = append(resultWarnings, "DEV-RESULT retention warning: "+retention.Warning)
			}
		}
		budgets := devResultBudgets(resultConfig)
		job := validationRun.ID
		if job == "" {
			job = devresult.NewJobID(commandName, time.Now())
		}
		warnings := append([]string(nil), result.Warnings...)
		warnings = append(warnings, resultWarnings...)
		model, retained := devresult.Build(devresult.Input{
			Job:                        job,
			CommandName:                commandName,
			NormalizedCommand:          devresult.NormalizeCommand(commandName, parsedArgs.PassthroughArgs),
			Scope:                      devresult.NormalizeScope(parsedArgs.PassthroughArgs),
			ExitCode:                   result.ReturnCode,
			Duration:                   time.Duration(result.DurationMs) * time.Millisecond,
			Attempts:                   result.Attempts,
			Stdout:                     result.Stdout,
			Stderr:                     result.Stderr,
			Warnings:                   warnings,
			TimedOut:                   result.TimedOut,
			Cancelled:                  result.Cancelled,
			Diagnostics:                collected.response,
			DiagnosticsConfigured:      resultConfig.DiagnosticsAdapter != nil,
			DiagnosticsFallbackReason:  collected.fallbackReason,
			AdditionalRedactionMatches: retainedEvidence.RedactionMatches,
		}, budgets, os.Environ())
		compactMetadata := devresult.Compact(model, budgets)
		model.Truncation = compactMetadata.Truncation
		written, writeErr := devresult.WriteArtifactsWithEvidenceForFormat(cmdCfg.ProjectDir, model, retained, retainedEvidence, budgets, cmdCfg.Format == "json")
		if writeErr != nil {
			logger.Warnf("DEV-RESULT artifacts unavailable: %v", writeErr)
			written = devresult.WithArtifactWarning(model, writeErr)
		}
		if cmdCfg.Format == "json" {
			jsonBytes, marshalErr := devresult.MarshalCompactJSON(written, budgets)
			if marshalErr != nil {
				jsonBytes = []byte(fmt.Sprintf("{\"schema\":%q,\"outcome\":%q,\"exit_code\":%d,\"error\":%q}\n", devresult.SchemaV1, written.Outcome, result.ReturnCode, marshalErr.Error()))
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), string(jsonBytes))
		} else {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), devresult.RenderText(written, budgets))
		}
	} else if cmdCfg.Format == "json" {
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
		if !resultEnabled && (result.Stdout != "" || result.Stderr != "") {
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

func devResultCommandConfig(cfg *config.Config, commandName string, steps []config.CommandStep, watch bool) (config.DevResultCommandConfig, bool) {
	if cfg == nil || cfg.DevResult == nil || watch || !devresult.SupportedCommand(commandName) {
		return config.DevResultCommandConfig{}, false
	}
	commandConfig, exists := cfg.DevResult.Commands[commandName]
	if !exists || !commandConfig.Enabled {
		return config.DevResultCommandConfig{}, false
	}
	for _, step := range steps {
		if step.Background || step.Daemon {
			return config.DevResultCommandConfig{}, false
		}
	}
	return commandConfig, true
}

func devResultBudgets(commandConfig config.DevResultCommandConfig) devresult.Budgets {
	budgets := devresult.DefaultBudgets()
	applyBudget := func(target *devresult.Budget, override config.ResultBudgetConfig) {
		if override.Lines > 0 {
			target.Lines = override.Lines
		}
		if override.Bytes > 0 {
			target.Bytes = override.Bytes
		}
	}
	applyBudget(&budgets.Success, commandConfig.Success)
	applyBudget(&budgets.Failure, commandConfig.Failure)
	applyBudget(&budgets.Timeout, commandConfig.Timeout)
	applyBudget(&budgets.Background, commandConfig.Background)
	return budgets
}

// collectedDiagnostics is the outcome of one adapter run: the validated response (nil on
// fallback), the visible warning, the retained evidence, and the short fallback reason the
// result's diagnostics marker carries.
type collectedDiagnostics struct {
	response       *devresult.DiagnosticsResponse
	warning        string
	evidence       devresult.EvidenceBundle
	fallbackReason string
}

func collectDiagnostics(ctx context.Context, projectDir, commandName string, execution executor.ExecutionResult, adapter *config.DiagnosticsAdapterConfig, run devresult.ValidationRun) collectedDiagnostics {
	if adapter == nil {
		return collectedDiagnostics{}
	}
	if run.ID == "" || run.Dir == "" {
		return collectedDiagnostics{}.fallBack("", "run-scoped diagnostics are unavailable")
	}
	fresh, err := run.HasFreshEvidence(adapter.StartWhen)
	if err != nil {
		return collectedDiagnostics{}.fallBack("structured diagnostics are unavailable: "+err.Error(), devresult.ReasonAdapterUnavailable)
	}
	if !fresh {
		if adapter.Required {
			missing := "structured source " + strings.Join(adapter.StartWhen, " or ") + " is missing"
			return collectedDiagnostics{}.fallBack(missing, missing)
		}
		return collectedDiagnostics{}.fallBack("", "no structured evidence was written")
	}

	limits := devresult.Limits{
		Timeout:          adapter.Timeout,
		StdoutBytes:      adapter.StdoutBytes,
		StderrBytes:      adapter.StderrBytes,
		MaxDiagnostics:   adapter.MaxDiagnostics,
		PerSourceBytes:   adapter.MaxSourceBytes,
		TotalSourceBytes: adapter.MaxTotalSourceBytes,
	}
	adapterResult := devresult.ExecuteAdapter(ctx, devresult.AdapterOptions{
		Argv:       adapter.Command,
		WorkingDir: projectDir,
		Request: devresult.DiagnosticsRequest{
			Schema:     devresult.DiagnosticsRequestSchemaV1,
			RunID:      run.ID,
			RunDir:     run.Dir,
			Command:    commandName,
			ExitCode:   execution.ReturnCode,
			DurationMS: execution.DurationMs,
			TimedOut:   execution.TimedOut,
			Cancelled:  execution.Cancelled,
		},
		Limits: limits,
	})
	adapterResponse := ""
	adapterResponseMatches := 0
	if adapterResult.Response != nil {
		redactedResponse, matches := devresult.RedactDiagnosticsResponse(*adapterResult.Response, os.Environ())
		if encodedResponse, encodeErr := json.Marshal(redactedResponse); encodeErr == nil {
			adapterResponse = string(encodedResponse)
			adapterResponseMatches = matches
		}
	}
	adapterStderr, adapterStderrMatches := redactRetainedEvidence(string(adapterResult.Stderr))
	collected := collectedDiagnostics{evidence: devresult.EvidenceBundle{
		AdapterResponse:  adapterResponse,
		AdapterStderr:    adapterStderr,
		RedactionMatches: adapterResponseMatches + adapterStderrMatches,
	}}
	if adapterResult.Err != nil {
		return adapterFallback(collected, adapterResult)
	}
	if adapterResult.Response == nil {
		return collected.fallBack("structured diagnostics are unavailable: adapter returned no response", "adapter returned no response")
	}

	validated, err := run.ValidateSources(adapterResult.Response.Sources, limits)
	if err != nil {
		return collected.fallBack("unsafe structured evidence: "+err.Error(), devresult.ReasonUnsafeEvidence)
	}
	for index, source := range validated {
		path, pathMatches := retainedSourcePath(source.Path, index)
		content, contentMatches := redactRetainedEvidence(string(source.Content))
		collected.evidence.Sources = append(collected.evidence.Sources, devresult.RetainedSource{
			Kind:    source.Kind,
			Path:    path,
			Content: content,
		})
		collected.evidence.RedactionMatches += pathMatches + contentMatches
	}
	collected.response = adapterResult.Response
	return collected
}

func redactRetainedEvidence(value string) (string, int) {
	return devresult.Redact(value, os.Environ())
}

func retainedSourcePath(path string, index int) (string, int) {
	redacted, matches := redactRetainedEvidence(path)
	if redacted == path {
		return path, matches
	}
	return filepath.ToSlash(filepath.Join("redacted", fmt.Sprintf("%04d-%s", index+1, filepath.Base(redacted)))), matches
}

// fallBack records why structured diagnostics were not used: the visible warning (empty for a
// quiet fallback) and the short reason the result's diagnostics marker carries.
func (collected collectedDiagnostics) fallBack(warning, reason string) collectedDiagnostics {
	collected.warning = warning
	collected.fallbackReason = reason
	return collected
}

// adapterFallback turns a failed adapter run into its visible warning and marker reason.
func adapterFallback(collected collectedDiagnostics, result devresult.AdapterResult) collectedDiagnostics {
	switch result.Reason {
	case devresult.ReasonUnsafeEvidence:
		return collected.fallBack("unsafe structured evidence: "+result.Err.Error(), result.Reason)
	case devresult.ReasonMalformedResponse:
		return collected.fallBack("structured diagnostics are unavailable: malformed adapter response: "+result.Err.Error(), result.Reason)
	case devresult.ReasonAdapterTimeout, devresult.ReasonAdapterCancelled:
		return collected.fallBack("structured diagnostics are unavailable: "+result.Reason, result.Reason)
	case "":
		return collected.fallBack("structured diagnostics are unavailable: "+result.Err.Error(), devresult.ReasonAdapterUnavailable)
	}
	if strings.HasPrefix(result.Reason, "expected version") {
		return collected.fallBack("structured diagnostics are unavailable: "+result.Reason, result.Reason)
	}
	return collected.fallBack("structured diagnostics are unavailable: "+result.Err.Error(), result.Reason)
}

// SetConfigLoader sets the config loader for the root command.
func SetConfigLoader(loader config.ConfigLoader) {
	configLoader = loader
}

// SetExecutor sets the exec for the root command.
func SetExecutor(e executor.Executor) {
	exec = e
}
