package executor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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

// waitForProcessWithSignalHandling waits for a process to complete while handling signals
func waitForProcessWithSignalHandling(cmd *exec.Cmd) error {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case sig := <-signalChan:
		log.Printf("Received signal %v, terminating process", sig)
		if cmd.Process != nil {
			if sigErr := cmd.Process.Signal(sig); sigErr != nil {
				log.Printf("Failed to forward signal to child process: %v", sigErr)
				if killErr := cmd.Process.Kill(); killErr != nil {
					log.Printf("Failed to kill child process: %v", killErr)
				}
			}
		}
		return <-done
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

// ExecuteCommandStep executes a single command step with all its components
func ExecuteCommandStep(step config.CommandStep, commandName, workingDir string, passthroughArgs []string) ExecutionResult {
	log.Printf("Executing command step (background=%t, daemon=%t)", step.Background, step.Daemon)

	// Handle directory option
	executionDir := workingDir
	if step.Directory != "" {
		stepDir := step.Directory
		if !filepath.IsAbs(stepDir) && workingDir != "" {
			stepDir = filepath.Join(workingDir, stepDir)
		}

		// Validate directory exists and is accessible
		if info, err := os.Stat(stepDir); err != nil {
			if os.IsNotExist(err) {
				errorMsg := fmt.Sprintf("Directory '%s' does not exist", stepDir)
				log.Print(errorMsg)
				return ExecutionResult{Success: false, Stderr: errorMsg}
			} else {
				errorMsg := fmt.Sprintf("Directory '%s' is not accessible: %v", stepDir, err)
				log.Print(errorMsg)
				return ExecutionResult{Success: false, Stderr: errorMsg}
			}
		} else if !info.IsDir() {
			errorMsg := fmt.Sprintf("Path '%s' is not a directory", stepDir)
			log.Print(errorMsg)
			return ExecutionResult{Success: false, Stderr: errorMsg}
		}

		// Test directory accessibility
		if _, err := os.ReadDir(stepDir); err != nil {
			errorMsg := fmt.Sprintf("Directory '%s' is not accessible: %v", stepDir, err)
			log.Print(errorMsg)
			return ExecutionResult{Success: false, Stderr: errorMsg}
		}

		executionDir = stepDir
		log.Printf("Using directory: %s", stepDir)
	}

	// Handle services configuration
	servicesStarted := false
	if step.Services.Compose != nil || len(step.Services.Containers) > 0 {
		result := HandleServicesConfiguration(step.Services)
		if !result.Success {
			log.Printf("Failed to handle services configuration")
			return result
		}
		servicesStarted = true
	}

	// Defer cleanup if services were started and cleanup is enabled
	if servicesStarted && step.Services.Cleanup {
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

	// Handle run commands
	if len(step.Run) > 0 {
		for _, baseCommand := range []string(step.Run) {
			// Append passthrough arguments to the command
			command := appendPassthroughArgs(baseCommand, passthroughArgs)
			log.Printf("Executing command: %s", command)

			// Check if daemon instance is already running
			if step.Daemon {
				pidFile := GeneratePIDFilename(commandName, command)
				if _, err := os.Stat(pidFile); err == nil {
					if existingPID, err := ReadPIDFile(pidFile); err == nil && IsProcessRunning(existingPID) {
						errorMsg := fmt.Sprintf("Daemon process already running with PID %d (pid file: %s)",
							existingPID, pidFile)
						log.Print(errorMsg)
						return ExecutionResult{Success: false, Stderr: errorMsg}
					} else {
						// Clean up stale PID file
						log.Printf("Removing stale PID file %s", pidFile)
						_ = RemovePIDFile(pidFile)
					}
				}
			}

			result := ExecuteShellCommand(ExecuteOptions{
				Command:       command,
				Background:    step.Background,
				CaptureOutput: step.Background, // Capture output for background commands for PID tracking
				WorkingDir:    executionDir,
				Daemon:        step.Daemon,
				CommandName:   commandName,
			})

			if !result.Success && !step.Background {
				return result
			}

			if result.PID != 0 && step.Daemon && step.Background {
				// Handle background daemon processes
				log.Printf("Background daemon process with PID %d", result.PID)
				pidFile := GeneratePIDFilename(commandName, command)
				if err := CreateEnhancedPIDFile(pidFile, result.PID, commandName, command); err != nil {
					log.Printf("Failed to create enhanced PID file: %v", err)
				} else {
					log.Printf("Created enhanced PID file %s for background daemon process", pidFile)
					fmt.Printf("%s\n", colors.Success("Running job '%s' in the background. PID: %d, PID file: %s",
						command, result.PID, pidFile))
				}
				return result
			} else if result.PID != 0 && step.Background {
				log.Printf("Command started with PID %d", result.PID)
				fmt.Printf("%s\n", colors.Success("Running job '%s' in the background", command))
				return result
			}
		}
	}

	return ExecutionResult{Success: true}
}

// ExecuteCommandWithSteps executes a command consisting of multiple steps
func ExecuteCommandWithSteps(commandName string, steps []config.CommandStep, workingDir string, passthroughArgs []string) ExecutionResult {
	log.Printf("Executing command '%s' with %d steps and passthrough args: %v", commandName, len(steps), passthroughArgs)

	for i, step := range steps {
		log.Printf("Executing step %d/%d", i+1, len(steps))
		result := ExecuteCommandStep(step, commandName, workingDir, passthroughArgs)
		if !result.Success {
			log.Printf("Step %d failed, aborting command execution", i+1)
			return result
		}
	}

	log.Printf("Command '%s' completed successfully", commandName)
	return ExecutionResult{Success: true}
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
