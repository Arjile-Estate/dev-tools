package executor

import (
	"bytes"
	"context"
	"dev-tools/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
)

// ExecuteOptions contains options for command execution
type ExecuteOptions struct {
	Command       string
	Background    bool
	CaptureOutput bool
	WorkingDir    string
	Daemon        bool
	CommandName   string
}

// DirectExecuteOptions contains options for direct command execution (no shell)
type DirectExecuteOptions struct {
	Command       string   // Command name (e.g., "docker")
	Args          []string // Command arguments
	CaptureOutput bool
	WorkingDir    string
}

// CommandExecutionOptions contains options for command execution with steps
type CommandExecutionOptions struct {
	Context         context.Context      // Cancellation context; defaults to context.Background
	CommandName     string               // Name of the command being executed
	Steps           []config.CommandStep // Steps to execute
	WorkingDir      string               // Working directory
	PassthroughArgs []string             // Arguments to pass through to commands
	CaptureOutput   bool                 // Capture distinct stdout/stderr for DEV-RESULT
}

// ExecuteCommandDirect executes a command directly without shell interpretation
// This is safer than ExecuteShellCommand for commands with user-provided arguments
// as it prevents shell injection attacks. Context allows for cancellation and timeouts.
func ExecuteCommandDirect(ctx context.Context, opts DirectExecuteOptions) ExecutionResult {
	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	if opts.CaptureOutput {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		cancelledBySignal := false
		if err == nil {
			err, cancelledBySignal = waitForProcessWithSignalHandlingStatus(cmd)
		}
		success := err == nil
		returnCode := 0

		if err != nil {
			// Check if context was cancelled
			if ctx.Err() == context.Canceled || cancelledBySignal {
				return ExecutionResult{
					Success:    false,
					Stdout:     stdout.String(),
					Stderr:     stderr.String() + "command cancelled",
					ReturnCode: -1,
					Cancelled:  true,
				}
			}
			if ctx.Err() == context.DeadlineExceeded {
				return ExecutionResult{
					Success:    false,
					Stdout:     stdout.String(),
					Stderr:     stderr.String() + "command timeout exceeded",
					ReturnCode: -1,
					TimedOut:   true,
				}
			}

			if exitError, ok := err.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			if stderr.Len() == 0 {
				_, _ = stderr.WriteString(err.Error())
			}
		}

		return ExecutionResult{
			Success:    success,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			ReturnCode: returnCode,
		}
	}

	// Stream output directly to stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		logger.Warnf("Failed to start command: %v", err)
		return ExecutionResult{
			Success:    false,
			Stderr:     err.Error(),
			ReturnCode: -1,
		}
	}

	waitError := waitForProcessWithSignalHandling(cmd)

	success := waitError == nil
	returnCode := 0

	if waitError != nil {
		// Check if context was cancelled
		if ctx.Err() == context.Canceled {
			return ExecutionResult{
				Success:    false,
				Stderr:     "command cancelled",
				ReturnCode: -1,
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return ExecutionResult{
				Success:    false,
				Stderr:     "command timeout exceeded",
				ReturnCode: -1,
			}
		}

		if exitError, ok := waitError.(*exec.ExitError); ok {
			returnCode = exitError.ExitCode()
		} else {
			returnCode = -1
		}
		logger.Warnf("Command failed with return code %d", returnCode)
	} else {
		logger.Info("Command completed successfully")
	}

	return ExecutionResult{
		Success:    success,
		ReturnCode: returnCode,
	}
}

// ExecuteShellCommand executes a shell command with the given options
// Context allows for cancellation and timeouts
func ExecuteShellCommand(ctx context.Context, opts ExecuteOptions) ExecutionResult {
	cmd := exec.CommandContext(ctx, "sh", "-c", opts.Command)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	if opts.Background {
		setProcessGroupAttr(cmd)

		// Suppress stdout/stderr so background daemons don't produce terminal output
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			logger.Warnf("Failed to open /dev/null: %v", err)
			return ExecutionResult{
				Success:    false,
				Stderr:     err.Error(),
				ReturnCode: -1,
			}
		}
		defer func() { _ = devNull.Close() }()
		cmd.Stdout = devNull
		cmd.Stderr = devNull

		err = cmd.Start()
		if err != nil {
			logger.Warnf("Failed to start background command: %v", err)
			return ExecutionResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		logger.Infof("Started background process with PID %d", cmd.Process.Pid)
		return ExecutionResult{
			Success: true,
			PID:     cmd.Process.Pid,
		}
	}

	if opts.CaptureOutput {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Start()
		cancelledBySignal := false
		if err == nil {
			err, cancelledBySignal = waitForProcessWithSignalHandlingStatus(cmd)
		}
		success := err == nil
		returnCode := 0

		if err != nil {
			// Check if context was cancelled
			if ctx.Err() == context.Canceled || cancelledBySignal {
				return ExecutionResult{
					Success:    false,
					Stdout:     stdout.String(),
					Stderr:     stderr.String() + "command cancelled",
					ReturnCode: -1,
					Cancelled:  true,
				}
			}
			if ctx.Err() == context.DeadlineExceeded {
				return ExecutionResult{
					Success:    false,
					Stdout:     stdout.String(),
					Stderr:     stderr.String() + "command timeout exceeded",
					ReturnCode: -1,
					TimedOut:   true,
				}
			}

			if exitError, ok := err.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			if stderr.Len() == 0 {
				_, _ = stderr.WriteString(err.Error())
			}
		}

		return ExecutionResult{
			Success:    success,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			ReturnCode: returnCode,
		}
	}

	// For daemon processes running in foreground, track PID
	if opts.Daemon {
		// Give the foreground daemon its own process group so that
		// stop/restart from another terminal can kill the entire tree
		setProcessGroupAttr(cmd)

		// Stream output directly to stdout/stderr for foreground daemon
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			logger.Warnf("Failed to start daemon command: %v", err)
			return ExecutionResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		logger.Infof("Started daemon process with PID %d", cmd.Process.Pid)

		// Create enhanced PID file for daemon tracking
		pidFile := GeneratePIDFilename(opts.CommandName, opts.Command)
		if pidErr := CreateEnhancedPIDFile(pidFile, cmd.Process.Pid, opts.CommandName, opts.Command); pidErr != nil {
			// Log with proper error type but don't fail - daemon is running
			logger.Warnf("Warning: %v", NewDaemonError(cmd.Process.Pid, pidFile, pidErr))
		} else {
			logger.Infof("Created enhanced PID file %s for daemon process", pidFile)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the foreground. PID: %d, PID file: %s",
				opts.Command, cmd.Process.Pid, pidFile))
		}

		waitErr := waitForProcessWithSignalHandling(cmd)

		success := waitErr == nil
		returnCode := 0

		if waitErr != nil {
			// Check if context was cancelled
			if ctx.Err() == context.Canceled {
				// Clean up PID file only if we still own it
				_, _ = RemovePIDFileIfOwned(pidFile, cmd.Process.Pid)
				return ExecutionResult{
					Success:    false,
					Stderr:     "command cancelled",
					ReturnCode: -1,
					PID:        cmd.Process.Pid,
				}
			}
			if ctx.Err() == context.DeadlineExceeded {
				// Clean up PID file only if we still own it
				_, _ = RemovePIDFileIfOwned(pidFile, cmd.Process.Pid)
				return ExecutionResult{
					Success:    false,
					Stderr:     "command timeout exceeded",
					ReturnCode: -1,
					PID:        cmd.Process.Pid,
				}
			}

			if exitError, ok := waitErr.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
				// Detect external signal termination and inform the user
				if sigName := describeExitSignal(exitError); sigName != "" {
					logger.Infof("Daemon '%s' was terminated by signal: %s", opts.CommandName, sigName)
					fmt.Printf("%s\n", colors.Warning("Daemon '%s' was terminated by signal: %s",
						opts.CommandName, sigName))
				}
			} else {
				returnCode = -1
			}
			logger.Warnf("Daemon command failed with return code %d", returnCode)
		} else {
			logger.Info("Daemon command completed successfully")
		}

		// Clean up PID file only if we still own it (avoids race with restart)
		if removed, pidErr := RemovePIDFileIfOwned(pidFile, cmd.Process.Pid); pidErr != nil {
			logger.Warnf("Warning: %v", NewDaemonError(cmd.Process.Pid, pidFile, fmt.Errorf("failed to remove PID file: %w", pidErr)))
		} else if removed {
			logger.Infof("Removed PID file %s after daemon completion", pidFile)
		} else {
			logger.Infof("PID file %s belongs to a restarted daemon, leaving in place", pidFile)
		}

		return ExecutionResult{
			Success:    success,
			ReturnCode: returnCode,
			PID:        cmd.Process.Pid,
		}
	}

	// Stream output directly to stdout/stderr for regular commands
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command
	err := cmd.Start()
	if err != nil {
		logger.Warnf("Failed to start command: %v", err)
		return ExecutionResult{
			Success:    false,
			Stderr:     err.Error(),
			ReturnCode: -1,
		}
	}

	waitError := waitForProcessWithSignalHandling(cmd)

	success := waitError == nil
	returnCode := 0

	if waitError != nil {
		// Check if context was cancelled
		if ctx.Err() == context.Canceled {
			return ExecutionResult{
				Success:    false,
				Stderr:     "command cancelled",
				ReturnCode: -1,
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return ExecutionResult{
				Success:    false,
				Stderr:     "command timeout exceeded",
				ReturnCode: -1,
			}
		}

		if exitError, ok := waitError.(*exec.ExitError); ok {
			returnCode = exitError.ExitCode()
		} else {
			returnCode = -1
		}
		logger.Warnf("Command failed with return code %d", returnCode)
	} else {
		logger.Info("Command completed successfully")
	}

	return ExecutionResult{
		Success:    success,
		ReturnCode: returnCode,
	}
}

// GeneratePIDFilename generates a PID filename using SHA1 hash

// appendPassthroughArgs appends passthrough arguments to a base command
func appendPassthroughArgs(baseCommand string, passthroughArgs []string) string {
	if len(passthroughArgs) == 0 {
		return baseCommand
	}

	// Safely quote and append arguments using POSIX shell escaping
	var quotedArgs []string
	for _, arg := range passthroughArgs {
		quotedArgs = append(quotedArgs, shellEscape(arg))
	}

	return fmt.Sprintf("%s %s", baseCommand, strings.Join(quotedArgs, " "))
}

// shellEscape escapes a string for safe use as a shell argument using POSIX shell quoting rules.
// It wraps the argument in single quotes and escapes any single quotes within the string.
// Single quotes prevent all shell expansions: variables ($), command substitution (`), etc.
func shellEscape(arg string) string {
	// Special characters that require quoting
	specialChars := " \t\n$`!&|;<>()[]{}*?\\\"'"

	// Check if the argument needs quoting
	needsQuoting := false
	for _, char := range specialChars {
		if strings.ContainsRune(arg, char) {
			needsQuoting = true
			break
		}
	}

	// If no special characters, return as-is
	if !needsQuoting {
		return arg
	}

	// Use single quotes and escape any embedded single quotes
	// The technique: replace ' with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(arg, "'", "'\\''")
	return fmt.Sprintf("'%s'", escaped)
}

// handleServicesStartup starts configured services and returns the result directly,
// preserving all fields including ReturnCode from the underlying command execution.
func handleServicesStartup(services config.ServicesConfig) ExecutionResult {
	if services.Compose == nil && len(services.Containers) == 0 {
		return ExecutionResult{Success: true}
	}

	return HandleServicesConfiguration(services)
}

// checkDaemonAlreadyRunning reports the PID of an already-running daemon for
// this command, or 0 if none is running. A PID file whose process is no longer
// alive is treated as stale and removed so a fresh daemon can start.
func checkDaemonAlreadyRunning(commandName, command string) int {
	pidFile := GeneratePIDFilename(commandName, command)
	if _, err := os.Stat(pidFile); err == nil {
		if existingPID, err := ReadPIDFile(pidFile); err == nil && IsProcessRunning(existingPID) {
			return existingPID
		}
		// Clean up stale PID file
		logger.Infof("Removing stale PID file %s", pidFile)
		_ = RemovePIDFile(pidFile)
	}
	return 0
}

// executeWithRetry executes a command with retry logic based on step configuration
// ExecuteCommandStep executes a single command step with all its components
func ExecuteCommandStep(step config.CommandStep, commandName, workingDir string, passthroughArgs []string) ExecutionResult {
	return executeCommandStep(context.Background(), step, commandName, workingDir, passthroughArgs, false)
}

func executeCommandStep(ctx context.Context, step config.CommandStep, commandName, workingDir string, passthroughArgs []string, captureOutput bool) (result ExecutionResult) {
	logger.Infof("Executing command step (background=%t, daemon=%t)", step.Background, step.Daemon)

	// Validate and resolve execution directory
	executionDir, err := validateAndResolveDirectory(step.Directory, workingDir)
	if err != nil {
		// Don't double-log: caller will log if needed
		return ExecutionResult{Success: false, Stderr: err.Error(), ReturnCode: 1}
	}

	// Start services if configured
	serviceResult := handleServicesStartup(step.Services)
	if !serviceResult.Success {
		return serviceResult
	}
	servicesStarted := serviceResult.ServicesStarted

	// Defer cleanup if services were started and cleanup is enabled
	if len(servicesStarted) > 0 && step.Services.Cleanup {
		defer func() {
			logger.Infof("Cleaning up services after command execution")
			cleanupResult := StopServices(step.Services)
			if !cleanupResult.Success {
				warning := fmt.Sprintf("Service cleanup failed: %s", cleanupResult.Stderr)
				logger.Warnf("%s", warning)
				result.Warnings = append(result.Warnings, warning)
			} else {
				logger.Infof("Services cleaned up successfully")
			}
		}()
	}

	// Execute run commands
	if len(step.Run) == 0 {
		return ExecutionResult{Success: true, ServicesStarted: servicesStarted}
	}
	var stdout, stderr strings.Builder
	var warnings []string
	var attemptedCommands []string
	attempts := 0

	for _, baseCommand := range []string(step.Run) {
		command := appendPassthroughArgs(baseCommand, passthroughArgs)
		logger.Infof("Executing command: %s", command)

		// If the daemon is already running, starting a second copy is neither
		// wanted nor possible — treat it as an idempotent no-op that succeeds
		// (exit 0) with a clear, actionable message rather than failing with a
		// cryptic "error code: 1". The existing PID file is left untouched.
		if step.Daemon {
			if existingPID := checkDaemonAlreadyRunning(commandName, command); existingPID != 0 {
				logger.Infof("Daemon '%s' already running with PID %d; skipping start", commandName, existingPID)
				fmt.Printf("%s\n", colors.Success("✓ Daemon '%s' is already running (PID %d).", commandName, existingPID))
				fmt.Printf("  Use 'dev-tools restart %s' to restart it, or\n", commandName)
				fmt.Printf("  'dev-tools stop %s' to stop it.\n", commandName)
				return ExecutionResult{Success: true, ServicesStarted: servicesStarted}
			}
		}

		// Execute with retry logic
		result = executeWithRetry(ctx, step, command, executionDir, commandName, captureOutput)
		if captureOutput {
			stdout.WriteString(result.Stdout)
			stderr.WriteString(result.Stderr)
		}
		warnings = append(warnings, result.Warnings...)
		attemptedCommands = append(attemptedCommands, result.AttemptedCommands...)
		attempts += result.Attempts

		if !result.Success && !step.Background && !step.ContinueOnError {
			result.FailedCommand = command
			result.ServicesStarted = servicesStarted
			if captureOutput {
				result.Stdout = stdout.String()
				result.Stderr = stderr.String()
			}
			result.Warnings = warnings
			result.AttemptedCommands = attemptedCommands
			result.Attempts = attempts
			return result
		}

		// Handle background daemon process
		if result.PID != 0 && step.Daemon && step.Background {
			logger.Infof("Background daemon process with PID %d", result.PID)
			pidFile := GeneratePIDFilename(commandName, command)
			if err := CreateEnhancedPIDFile(pidFile, result.PID, commandName, command); err != nil {
				warning := fmt.Sprintf("Failed to create PID file: %v", NewDaemonError(result.PID, pidFile, err))
				logger.Warnf("%s", warning)
				warnings = append(warnings, warning)
			} else {
				logger.Infof("Created enhanced PID file %s for background daemon process", pidFile)
				fmt.Printf("%s\n", colors.Success("Running job '%s' in the background. PID: %d, PID file: %s",
					command, result.PID, pidFile))
			}
			result.ServicesStarted = servicesStarted
			result.Warnings = warnings
			result.AttemptedCommands = attemptedCommands
			result.Attempts = attempts
			return result
		}

		// Handle background process
		if result.PID != 0 && step.Background {
			logger.Infof("Command started with PID %d", result.PID)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the background", command))
			result.ServicesStarted = servicesStarted
			result.Warnings = warnings
			result.AttemptedCommands = attemptedCommands
			result.Attempts = attempts
			return result
		}
	}

	result.Success = true
	result.ServicesStarted = servicesStarted
	if captureOutput {
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
	}
	result.Warnings = warnings
	result.AttemptedCommands = attemptedCommands
	result.Attempts = attempts
	return result
}

// ExecuteCommandWithOptions executes a command using the options struct pattern
func ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult {
	startTime := time.Now()
	logger.Infof("Executing command '%s' with %d steps and passthrough args: %v", opts.CommandName, len(opts.Steps), opts.PassthroughArgs)

	var servicesStarted []string
	var finalResult ExecutionResult
	var stdout, stderr strings.Builder
	var warnings []string
	var attemptedCommands []string
	attempts := 0
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	for i, step := range opts.Steps {
		logger.Infof("Executing step %d/%d", i+1, len(opts.Steps))
		result := executeCommandStep(ctx, step, opts.CommandName, opts.WorkingDir, opts.PassthroughArgs, opts.CaptureOutput)
		if opts.CaptureOutput {
			stdout.WriteString(result.Stdout)
			stderr.WriteString(result.Stderr)
		}
		warnings = append(warnings, result.Warnings...)
		attemptedCommands = append(attemptedCommands, result.AttemptedCommands...)
		attempts += result.Attempts

		// Track services started in this step
		if len(result.ServicesStarted) > 0 {
			servicesStarted = append(servicesStarted, result.ServicesStarted...)
		}

		if !result.Success {
			logger.Warnf("Step %d failed, aborting command execution", i+1)
			result.CommandName = opts.CommandName
			result.DurationMs = time.Since(startTime).Milliseconds()
			result.ServicesStarted = servicesStarted
			if opts.CaptureOutput {
				result.Stdout = stdout.String()
				result.Stderr = stderr.String()
			}
			result.Warnings = warnings
			result.AttemptedCommands = attemptedCommands
			result.Attempts = attempts
			result.StartTime = startTime
			return result
		}

		// Keep track of the final result (PID, stdout, etc from last step)
		finalResult = result
	}

	duration := time.Since(startTime).Milliseconds()
	logger.Infof("Command '%s' completed successfully in %dms", opts.CommandName, duration)

	result := ExecutionResult{
		Success:           true,
		CommandName:       opts.CommandName,
		DurationMs:        duration,
		ServicesStarted:   servicesStarted,
		StartTime:         startTime,
		Stdout:            finalResult.Stdout,
		Stderr:            finalResult.Stderr,
		PID:               finalResult.PID,
		Warnings:          warnings,
		AttemptedCommands: attemptedCommands,
		Attempts:          attempts,
	}
	if opts.CaptureOutput {
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
	}
	return result
}

// getContainerName extracts container name from service definition

// LoadEnvironmentVariables loads environment variables from a .env file
func LoadEnvironmentVariables(envFile string) error {
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		logger.Infof("No .env file found at %s", envFile)
		return nil
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("failed to read .env file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		if err := os.Setenv(key, value); err != nil {
			// Log with proper error type - not critical, other vars may still load
			logger.Warnf("%v", NewValidationError("environment_variable", key, err))
		}
	}

	logger.Infof("Loaded environment variables from %s", envFile)
	return nil
}

// HandleServicesConfiguration handles the new services configuration
