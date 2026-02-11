package executor

import (
	"context"
	"dev-tools/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
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
	CommandName     string               // Name of the command being executed
	Steps           []config.CommandStep // Steps to execute
	WorkingDir      string               // Working directory
	PassthroughArgs []string             // Arguments to pass through to commands
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
		output, err := cmd.CombinedOutput()
		success := err == nil
		returnCode := 0
		stderr := ""

		if err != nil {
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

			if exitError, ok := err.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			stderr = err.Error()
		}

		return ExecutionResult{
			Success:    success,
			Stdout:     string(output),
			Stderr:     stderr,
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
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		err := cmd.Start()
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
		output, err := cmd.CombinedOutput()
		success := err == nil
		returnCode := 0
		stderr := ""

		if err != nil {
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

			if exitError, ok := err.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			stderr = err.Error()
		}

		return ExecutionResult{
			Success:    success,
			Stdout:     string(output),
			Stderr:     stderr,
			ReturnCode: returnCode,
		}
	}

	// For daemon processes running in foreground, track PID
	if opts.Daemon {
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
				// Clean up PID file
				_ = RemovePIDFile(pidFile)
				return ExecutionResult{
					Success:    false,
					Stderr:     "command cancelled",
					ReturnCode: -1,
					PID:        cmd.Process.Pid,
				}
			}
			if ctx.Err() == context.DeadlineExceeded {
				// Clean up PID file
				_ = RemovePIDFile(pidFile)
				return ExecutionResult{
					Success:    false,
					Stderr:     "command timeout exceeded",
					ReturnCode: -1,
					PID:        cmd.Process.Pid,
				}
			}

			if exitError, ok := waitErr.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			logger.Warnf("Daemon command failed with return code %d", returnCode)
		} else {
			logger.Info("Daemon command completed successfully")
		}

		// Clean up PID file
		if pidErr := RemovePIDFile(pidFile); pidErr != nil {
			// Log with proper error type but don't fail - daemon completed
			logger.Warnf("Warning: %v", NewDaemonError(cmd.Process.Pid, pidFile, fmt.Errorf("failed to remove PID file: %w", pidErr)))
		} else {
			logger.Infof("Removed PID file %s after daemon completion", pidFile)
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

// handleServicesStartup starts configured services and returns their names
func handleServicesStartup(services config.ServicesConfig) ([]string, error) {
	if services.Compose == nil && len(services.Containers) == 0 {
		return nil, nil
	}

	result := HandleServicesConfiguration(services)
	if !result.Success {
		// Wrap in ServiceError for better error categorization
		return nil, NewServiceError("multiple", "start", fmt.Errorf("%s", result.Stderr))
	}

	return result.ServicesStarted, nil
}

// checkDaemonAlreadyRunning checks if a daemon is already running and cleans up stale PID files
func checkDaemonAlreadyRunning(commandName, command string) error {
	pidFile := GeneratePIDFilename(commandName, command)
	if _, err := os.Stat(pidFile); err == nil {
		if existingPID, err := ReadPIDFile(pidFile); err == nil && IsProcessRunning(existingPID) {
			return NewDaemonError(existingPID, pidFile, fmt.Errorf("process already running"))
		}
		// Clean up stale PID file
		logger.Infof("Removing stale PID file %s", pidFile)
		_ = RemovePIDFile(pidFile)
	}
	return nil
}

// executeWithRetry executes a command with retry logic based on step configuration
// ExecuteCommandStep executes a single command step with all its components
func ExecuteCommandStep(step config.CommandStep, commandName, workingDir string, passthroughArgs []string) (result ExecutionResult) {
	logger.Infof("Executing command step (background=%t, daemon=%t)", step.Background, step.Daemon)

	// Validate and resolve execution directory
	executionDir, err := validateAndResolveDirectory(step.Directory, workingDir)
	if err != nil {
		// Don't double-log: caller will log if needed
		return ExecutionResult{Success: false, Stderr: err.Error()}
	}

	// Start services if configured
	servicesStarted, err := handleServicesStartup(step.Services)
	if err != nil {
		// Don't double-log: caller will log if needed
		return ExecutionResult{Success: false, Stderr: err.Error()}
	}

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

	for _, baseCommand := range []string(step.Run) {
		command := appendPassthroughArgs(baseCommand, passthroughArgs)
		logger.Infof("Executing command: %s", command)

		// Check if daemon already running
		if step.Daemon {
			if err := checkDaemonAlreadyRunning(commandName, command); err != nil {
				// Don't double-log: caller will log if needed
				return ExecutionResult{Success: false, Stderr: err.Error()}
			}
		}

		// Execute with retry logic
		result := executeWithRetry(context.Background(), step, command, executionDir, commandName)

		if !result.Success && !step.Background && !step.ContinueOnError {
			result.FailedCommand = command
			result.ServicesStarted = servicesStarted
			return result
		}

		// Handle background daemon process
		if result.PID != 0 && step.Daemon && step.Background {
			logger.Infof("Background daemon process with PID %d", result.PID)
			pidFile := GeneratePIDFilename(commandName, command)
			if err := CreateEnhancedPIDFile(pidFile, result.PID, commandName, command); err != nil {
				warning := fmt.Sprintf("Failed to create PID file: %v", NewDaemonError(result.PID, pidFile, err))
				logger.Warnf("%s", warning)
				result.Warnings = append(result.Warnings, warning)
			} else {
				logger.Infof("Created enhanced PID file %s for background daemon process", pidFile)
				fmt.Printf("%s\n", colors.Success("Running job '%s' in the background. PID: %d, PID file: %s",
					command, result.PID, pidFile))
			}
			result.ServicesStarted = servicesStarted
			return result
		}

		// Handle background process
		if result.PID != 0 && step.Background {
			logger.Infof("Command started with PID %d", result.PID)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the background", command))
			result.ServicesStarted = servicesStarted
			return result
		}
	}

	return ExecutionResult{Success: true, ServicesStarted: servicesStarted}
}

// ExecuteCommandWithOptions executes a command using the options struct pattern
func ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult {
	startTime := time.Now()
	logger.Infof("Executing command '%s' with %d steps and passthrough args: %v", opts.CommandName, len(opts.Steps), opts.PassthroughArgs)

	var servicesStarted []string
	var finalResult ExecutionResult

	for i, step := range opts.Steps {
		logger.Infof("Executing step %d/%d", i+1, len(opts.Steps))
		result := ExecuteCommandStep(step, opts.CommandName, opts.WorkingDir, opts.PassthroughArgs)

		// Track services started in this step
		if len(result.ServicesStarted) > 0 {
			servicesStarted = append(servicesStarted, result.ServicesStarted...)
		}

		if !result.Success {
			logger.Warnf("Step %d failed, aborting command execution", i+1)
			result.CommandName = opts.CommandName
			result.DurationMs = time.Since(startTime).Milliseconds()
			result.ServicesStarted = servicesStarted
			result.StartTime = startTime
			return result
		}

		// Keep track of the final result (PID, stdout, etc from last step)
		finalResult = result
	}

	duration := time.Since(startTime).Milliseconds()
	logger.Infof("Command '%s' completed successfully in %dms", opts.CommandName, duration)

	return ExecutionResult{
		Success:         true,
		CommandName:     opts.CommandName,
		DurationMs:      duration,
		ServicesStarted: servicesStarted,
		StartTime:       startTime,
		Stdout:          finalResult.Stdout,
		Stderr:          finalResult.Stderr,
		PID:             finalResult.PID,
	}
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
