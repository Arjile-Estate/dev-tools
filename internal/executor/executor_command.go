package executor

import (
	"fmt"
	"log"
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

// ExecuteCommandDirect executes a command directly without shell interpretation
// This is safer than ExecuteShellCommand for commands with user-provided arguments
// as it prevents shell injection attacks
func ExecuteCommandDirect(opts DirectExecuteOptions) ExecutionResult {
	cmd := exec.Command(opts.Command, opts.Args...)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	if opts.CaptureOutput {
		output, err := cmd.CombinedOutput()
		success := err == nil
		returnCode := 0
		stderr := ""

		if err != nil {
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
		log.Printf("Failed to start command: %v", err)
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
		if exitError, ok := waitError.(*exec.ExitError); ok {
			returnCode = exitError.ExitCode()
		} else {
			returnCode = -1
		}
		log.Printf("Command failed with return code %d", returnCode)
	} else {
		log.Print("Command completed successfully")
	}

	return ExecutionResult{
		Success:    success,
		ReturnCode: returnCode,
	}
}

// ExecuteShellCommand executes a shell command with the given options
func ExecuteShellCommand(opts ExecuteOptions) ExecutionResult {
	cmd := exec.Command("sh", "-c", opts.Command)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	if opts.Background {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to start background command: %v", err)
			return ExecutionResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		log.Printf("Started background process with PID %d", cmd.Process.Pid)
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
			log.Printf("Failed to start daemon command: %v", err)
			return ExecutionResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		log.Printf("Started daemon process with PID %d", cmd.Process.Pid)

		// Create enhanced PID file for daemon tracking
		pidFile := GeneratePIDFilename(opts.CommandName, opts.Command)
		if pidErr := CreateEnhancedPIDFile(pidFile, cmd.Process.Pid, opts.CommandName, opts.Command); pidErr != nil {
			log.Printf("Failed to create enhanced PID file: %v", pidErr)
		} else {
			log.Printf("Created enhanced PID file %s for daemon process", pidFile)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the foreground. PID: %d, PID file: %s",
				opts.Command, cmd.Process.Pid, pidFile))
		}

		waitErr := waitForProcessWithSignalHandling(cmd)

		success := waitErr == nil
		returnCode := 0

		if waitErr != nil {
			if exitError, ok := waitErr.(*exec.ExitError); ok {
				returnCode = exitError.ExitCode()
			} else {
				returnCode = -1
			}
			log.Printf("Daemon command failed with return code %d", returnCode)
		} else {
			log.Print("Daemon command completed successfully")
		}

		// Clean up PID file
		if pidErr := RemovePIDFile(pidFile); pidErr != nil {
			log.Printf("Failed to remove PID file: %v", pidErr)
		} else {
			log.Printf("Removed PID file %s after daemon completion", pidFile)
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
		log.Printf("Failed to start command: %v", err)
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
		if exitError, ok := waitError.(*exec.ExitError); ok {
			returnCode = exitError.ExitCode()
		} else {
			returnCode = -1
		}
		log.Printf("Command failed with return code %d", returnCode)
	} else {
		log.Print("Command completed successfully")
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
		log.Printf("Removing stale PID file %s", pidFile)
		_ = RemovePIDFile(pidFile)
	}
	return nil
}

// executeWithRetry executes a command with retry logic based on step configuration
// ExecuteCommandStep executes a single command step with all its components
func ExecuteCommandStep(step config.CommandStep, commandName, workingDir string, passthroughArgs []string) ExecutionResult {
	log.Printf("Executing command step (background=%t, daemon=%t)", step.Background, step.Daemon)

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
			log.Printf("Cleaning up services after command execution")
			cleanupResult := StopServices(step.Services)
			if !cleanupResult.Success {
				log.Printf("Warning: Service cleanup failed: %s", cleanupResult.Stderr)
			} else {
				log.Printf("Services cleaned up successfully")
			}
		}()
	}

	// Execute run commands
	if len(step.Run) == 0 {
		return ExecutionResult{Success: true, ServicesStarted: servicesStarted}
	}

	for _, baseCommand := range []string(step.Run) {
		command := appendPassthroughArgs(baseCommand, passthroughArgs)
		log.Printf("Executing command: %s", command)

		// Check if daemon already running
		if step.Daemon {
			if err := checkDaemonAlreadyRunning(commandName, command); err != nil {
				// Don't double-log: caller will log if needed
				return ExecutionResult{Success: false, Stderr: err.Error()}
			}
		}

		// Execute with retry logic
		result := executeWithRetry(step, command, executionDir, commandName)

		if !result.Success && !step.Background {
			result.ServicesStarted = servicesStarted
			return result
		}

		// Handle background daemon process
		if result.PID != 0 && step.Daemon && step.Background {
			log.Printf("Background daemon process with PID %d", result.PID)
			pidFile := GeneratePIDFilename(commandName, command)
			if err := CreateEnhancedPIDFile(pidFile, result.PID, commandName, command); err != nil {
				log.Printf("Failed to create enhanced PID file: %v", err)
			} else {
				log.Printf("Created enhanced PID file %s for background daemon process", pidFile)
				fmt.Printf("%s\n", colors.Success("Running job '%s' in the background. PID: %d, PID file: %s",
					command, result.PID, pidFile))
			}
			result.ServicesStarted = servicesStarted
			return result
		}

		// Handle background process
		if result.PID != 0 && step.Background {
			log.Printf("Command started with PID %d", result.PID)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the background", command))
			result.ServicesStarted = servicesStarted
			return result
		}
	}

	return ExecutionResult{Success: true, ServicesStarted: servicesStarted}
}

// ExecuteCommandWithSteps executes a command consisting of multiple steps
func ExecuteCommandWithSteps(commandName string, steps []config.CommandStep, workingDir string, passthroughArgs []string) ExecutionResult {
	startTime := time.Now()
	log.Printf("Executing command '%s' with %d steps and passthrough args: %v", commandName, len(steps), passthroughArgs)

	var servicesStarted []string
	var finalResult ExecutionResult

	for i, step := range steps {
		log.Printf("Executing step %d/%d", i+1, len(steps))
		result := ExecuteCommandStep(step, commandName, workingDir, passthroughArgs)

		// Track services started in this step
		if len(result.ServicesStarted) > 0 {
			servicesStarted = append(servicesStarted, result.ServicesStarted...)
		}

		if !result.Success {
			log.Printf("Step %d failed, aborting command execution", i+1)
			result.CommandName = commandName
			result.DurationMs = time.Since(startTime).Milliseconds()
			result.ServicesStarted = servicesStarted
			result.StartTime = startTime
			return result
		}

		// Keep track of the final result (PID, stdout, etc from last step)
		finalResult = result
	}

	duration := time.Since(startTime).Milliseconds()
	log.Printf("Command '%s' completed successfully in %dms", commandName, duration)

	return ExecutionResult{
		Success:         true,
		CommandName:     commandName,
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
		log.Printf("No .env file found at %s", envFile)
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
			log.Printf("Failed to set environment variable %s: %v", key, err)
		}
	}

	log.Printf("Loaded environment variables from %s", envFile)
	return nil
}

// HandleServicesConfiguration handles the new services configuration
