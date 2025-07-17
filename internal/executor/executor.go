package executor

import (
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
)

// CommandResult represents the result of command execution
type CommandResult struct {
	Success    bool
	Stdout     string
	Stderr     string
	ReturnCode int
	PID        int
}

// ExecuteOptions contains options for command execution
type ExecuteOptions struct {
	Command       string
	Background    bool
	CaptureOutput bool
	WorkingDir    string
	Daemon        bool
	CommandName   string
}

// ExecuteShellCommand executes a shell command with the given options
func ExecuteShellCommand(opts ExecuteOptions) CommandResult {
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
			return CommandResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		log.Printf("Started background process with PID %d", cmd.Process.Pid)
		return CommandResult{
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

		return CommandResult{
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
			return CommandResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		log.Printf("Started daemon process with PID %d", cmd.Process.Pid)

		// Create PID file for daemon tracking
		pidFile := GeneratePIDFilename(opts.CommandName, opts.Command)
		if pidErr := CreatePIDFile(pidFile, cmd.Process.Pid); pidErr != nil {
			log.Printf("Failed to create PID file: %v", pidErr)
		} else {
			log.Printf("Created PID file %s for daemon process", pidFile)
			fmt.Printf("%s\n", colors.Success("Running job '%s' in the foreground. PID: %d, PID file: %s",
				opts.Command, cmd.Process.Pid, pidFile))
		}

		// Set up signal handling for graceful termination
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		// Wait for process to complete or receive signal
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		var waitErr error
		select {
		case waitErr = <-done:
			// Process completed normally
		case sig := <-signalChan:
			log.Printf("Received signal %v, terminating daemon process", sig)
			// Forward signal to child process
			if cmd.Process != nil {
				if sigErr := cmd.Process.Signal(sig); sigErr != nil {
					log.Printf("Failed to forward signal to child process: %v", sigErr)
					// Force kill if signal forwarding fails
					if killErr := cmd.Process.Kill(); killErr != nil {
						log.Printf("Failed to kill child process: %v", killErr)
					}
				}
			}
			waitErr = <-done // Wait for process to actually exit
		}

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

		return CommandResult{
			Success:    success,
			ReturnCode: returnCode,
			PID:        cmd.Process.Pid,
		}
	}

	// Stream output directly to stdout/stderr for regular commands
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up signal handling for graceful termination
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the command
	err := cmd.Start()
	if err != nil {
		log.Printf("Failed to start command: %v", err)
		return CommandResult{
			Success:    false,
			Stderr:     err.Error(),
			ReturnCode: -1,
		}
	}

	// Wait for process to complete or receive signal
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitError error
	select {
	case waitError = <-done:
		// Process completed normally
	case sig := <-signalChan:
		log.Printf("Received signal %v, terminating command process", sig)
		// Forward signal to child process
		if cmd.Process != nil {
			if sigErr := cmd.Process.Signal(sig); sigErr != nil {
				log.Printf("Failed to forward signal to child process: %v", sigErr)
				// Force kill if signal forwarding fails
				if killErr := cmd.Process.Kill(); killErr != nil {
					log.Printf("Failed to kill child process: %v", killErr)
				}
			}
		}
		waitError = <-done // Wait for process to actually exit
	}

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

	return CommandResult{
		Success:    success,
		ReturnCode: returnCode,
	}
}

// GeneratePIDFilename generates a PID filename using SHA1 hash
func GeneratePIDFilename(commandName, command string) string {
	combined := commandName + command
	hash := sha1.Sum([]byte(combined))
	return fmt.Sprintf(".%x.pid", hash[:4]) // Use first 8 hex characters (4 bytes)
}

// CreatePIDFile creates a PID file for daemon process tracking
func CreatePIDFile(pidFile string, pid int) error {
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// ReadPIDFile reads a PID from a PID file
func ReadPIDFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file %s: %w", pidFile, err)
	}

	return pid, nil
}

// RemovePIDFile removes a PID file
func RemovePIDFile(pidFile string) error {
	return os.Remove(pidFile)
}

// IsProcessRunning checks if a process with given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CleanupStalePIDFiles cleans up stale PID files for processes that are no longer running
func CleanupStalePIDFiles(projectDir string) CommandResult {
	log.Print("Starting PID file cleanup")

	pidFiles, err := filepath.Glob(filepath.Join(projectDir, "*.pid"))
	if err != nil {
		return CommandResult{
			Success: false,
			Stderr:  fmt.Sprintf("Failed to find PID files: %v", err),
		}
	}

	if len(pidFiles) == 0 {
		message := "No PID files found to clean up"
		log.Print(message)
		return CommandResult{
			Success: true,
			Stdout:  message,
		}
	}

	var cleanedFiles []string
	var activeProcesses []string
	var errors []string

	for _, pidFile := range pidFiles {
		pid, err := ReadPIDFile(pidFile)
		if err != nil {
			errorMsg := fmt.Sprintf("Could not read PID from %s: %v", filepath.Base(pidFile), err)
			log.Print(errorMsg)
			errors = append(errors, errorMsg)
			continue
		}

		if IsProcessRunning(pid) {
			log.Printf("Process %d from %s is still running", pid, filepath.Base(pidFile))
			activeProcesses = append(activeProcesses, fmt.Sprintf("%s (PID %d)", filepath.Base(pidFile), pid))
		} else {
			log.Printf("Process %d from %s is not running, removing PID file", pid, filepath.Base(pidFile))
			if err := RemovePIDFile(pidFile); err != nil {
				errorMsg := fmt.Sprintf("Failed to remove %s: %v", filepath.Base(pidFile), err)
				log.Print(errorMsg)
				errors = append(errors, errorMsg)
			} else {
				cleanedFiles = append(cleanedFiles, fmt.Sprintf("%s (PID %d)", filepath.Base(pidFile), pid))
			}
		}
	}

	// Prepare summary message
	var summary strings.Builder

	if len(cleanedFiles) > 0 {
		fmt.Fprintf(&summary, "%s\n", colors.Info("Cleaned up %d stale PID file(s):", len(cleanedFiles)))
		for _, file := range cleanedFiles {
			fmt.Fprintf(&summary, "  - %s\n", file)
		}
	}

	if len(activeProcesses) > 0 {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		fmt.Fprintf(&summary, "%s\n", colors.Info("Found %d active process(es):", len(activeProcesses)))
		for _, process := range activeProcesses {
			fmt.Fprintf(&summary, "  - %s\n", process)
		}
	}

	if len(errors) > 0 {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		fmt.Fprintf(&summary, "%s\n", colors.Warning("Encountered %d error(s):", len(errors)))
		for _, error := range errors {
			fmt.Fprintf(&summary, "  - %s\n", error)
		}
	}

	if len(cleanedFiles) == 0 && len(activeProcesses) == 0 && len(errors) == 0 {
		summary.WriteString(colors.Info("No PID files found to process"))
	}

	log.Printf("PID cleanup completed. Summary: %s", summary.String())

	// Return success if we cleaned files or found active processes, error only if all operations failed
	success := len(errors) == 0 || len(cleanedFiles) > 0 || len(activeProcesses) > 0

	return CommandResult{
		Success: success,
		Stdout:  summary.String(),
		Stderr:  strings.Join(errors, "\n"),
	}
}

// ExecuteCommandStep executes a single command step with all its components
func ExecuteCommandStep(step config.CommandStep, commandName, workingDir string) CommandResult {
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
				return CommandResult{Success: false, Stderr: errorMsg}
			} else {
				errorMsg := fmt.Sprintf("Directory '%s' is not accessible: %v", stepDir, err)
				log.Print(errorMsg)
				return CommandResult{Success: false, Stderr: errorMsg}
			}
		} else if !info.IsDir() {
			errorMsg := fmt.Sprintf("Path '%s' is not a directory", stepDir)
			log.Print(errorMsg)
			return CommandResult{Success: false, Stderr: errorMsg}
		}

		// Test directory accessibility
		if _, err := os.ReadDir(stepDir); err != nil {
			errorMsg := fmt.Sprintf("Directory '%s' is not accessible: %v", stepDir, err)
			log.Print(errorMsg)
			return CommandResult{Success: false, Stderr: errorMsg}
		}

		executionDir = stepDir
		log.Printf("Using directory: %s", stepDir)
	}

	// Handle start_services (backward compatibility with deprecation warning)
	if len(step.StartServices) > 0 {
		log.Printf("WARNING: 'start_services' is deprecated and will be removed in a future version. Please use 'services' configuration instead.")
		fmt.Printf("%s\n", colors.Warning("WARNING: 'start_services' is deprecated. Please migrate to 'services' configuration."))

		for _, service := range step.StartServices {
			result := StartDockerService(service)
			if !result.Success {
				log.Printf("Failed to start service %v", service)
				return result
			}
		}
	}

	// Handle new services configuration
	if step.Services.Compose != nil || len(step.Services.Containers) > 0 {
		result := HandleServicesConfiguration(step.Services)
		if !result.Success {
			log.Printf("Failed to handle services configuration")
			return result
		}
	}

	// Handle run commands
	if len(step.Run) > 0 {
		for _, command := range []string(step.Run) {
			log.Printf("Executing command: %s", command)

			// Check if daemon instance is already running
			if step.Daemon {
				pidFile := GeneratePIDFilename(commandName, command)
				if _, err := os.Stat(pidFile); err == nil {
					if existingPID, err := ReadPIDFile(pidFile); err == nil && IsProcessRunning(existingPID) {
						errorMsg := fmt.Sprintf("Daemon process already running with PID %d (pid file: %s)",
							existingPID, pidFile)
						log.Print(errorMsg)
						return CommandResult{Success: false, Stderr: errorMsg}
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
				if err := CreatePIDFile(pidFile, result.PID); err != nil {
					log.Printf("Failed to create PID file: %v", err)
				} else {
					log.Printf("Created PID file %s for background daemon process", pidFile)
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

	return CommandResult{Success: true}
}

// ExecuteCommandWithSteps executes a command consisting of multiple steps
func ExecuteCommandWithSteps(commandName string, steps []config.CommandStep, workingDir string) CommandResult {
	log.Printf("Executing command '%s' with %d steps", commandName, len(steps))

	for i, step := range steps {
		log.Printf("Executing step %d/%d", i+1, len(steps))
		result := ExecuteCommandStep(step, commandName, workingDir)
		if !result.Success {
			log.Printf("Step %d failed, aborting command execution", i+1)
			return result
		}
	}

	log.Printf("Command '%s' completed successfully", commandName)
	return CommandResult{Success: true}
}

// StartDockerService starts a Docker service container
func StartDockerService(service interface{}) CommandResult {
	log.Printf("Starting Docker service: %v", service)

	var containerName string
	var runCmd string

	switch s := service.(type) {
	case string:
		// Simple string service name
		containerName = s
		serviceConfigs := map[string]string{
			"redis":    "docker run -d --name redis -p 6379:6379 redis:latest",
			"postgres": "docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=password postgres:latest",
			"mysql":    "docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password mysql:latest",
		}

		var exists bool
		runCmd, exists = serviceConfigs[s]
		if !exists {
			// Default format for unknown services
			runCmd = fmt.Sprintf("docker run -d --name %s %s:latest", containerName, s)
		}

	case map[string]interface{}:
		// Complex service definition - assume first key is service name
		for name, config := range s {
			containerName = name
			configMap, ok := config.(map[string]interface{})
			if !ok {
				return CommandResult{
					Success: false,
					Stderr:  fmt.Sprintf("Invalid service configuration for %s", name),
				}
			}

			image, ok := configMap["image"].(string)
			if !ok {
				return CommandResult{
					Success: false,
					Stderr:  fmt.Sprintf("Service %s must have an 'image' field", name),
				}
			}

			// Build docker run command
			cmdParts := []string{"docker", "run", "-d"}

			// Add environment variables
			if env, ok := configMap["environment"].(map[string]interface{}); ok {
				for key, value := range env {
					if valueStr, ok := value.(string); ok {
						cmdParts = append(cmdParts, "-e", fmt.Sprintf("%s=%s", key, valueStr))
					}
				}
			}

			// Add volumes
			if volumes, ok := configMap["volumes"].([]interface{}); ok {
				for _, vol := range volumes {
					if volStr, ok := vol.(string); ok {
						cmdParts = append(cmdParts, "-v", volStr)
					}
				}
			}

			// Add ports
			if ports, ok := configMap["ports"].([]interface{}); ok {
				for _, port := range ports {
					if portStr, ok := port.(string); ok {
						cmdParts = append(cmdParts, "-p", portStr)
					}
				}
			}

			// Add networks
			if networks, ok := configMap["networks"].([]interface{}); ok {
				for _, network := range networks {
					if networkStr, ok := network.(string); ok {
						cmdParts = append(cmdParts, "--network", networkStr)
					}
				}
			}

			// Add restart policy
			if restart, ok := configMap["restart"].(string); ok {
				cmdParts = append(cmdParts, "--restart", restart)
			}

			// Add memory limit
			if memory, ok := configMap["memory"].(string); ok {
				cmdParts = append(cmdParts, "--memory", memory)
			}

			// Add CPU limit
			if cpus, ok := configMap["cpus"].(string); ok {
				cmdParts = append(cmdParts, "--cpus", cpus)
			}

			// Add health check
			if healthCheck, ok := configMap["healthcheck"].(map[string]interface{}); ok {
				if test, ok := healthCheck["test"].(string); ok {
					cmdParts = append(cmdParts, "--health-cmd", test)
				}
				if interval, ok := healthCheck["interval"].(string); ok {
					cmdParts = append(cmdParts, "--health-interval", interval)
				}
				if timeout, ok := healthCheck["timeout"].(string); ok {
					cmdParts = append(cmdParts, "--health-timeout", timeout)
				}
				if retries, ok := healthCheck["retries"].(string); ok {
					cmdParts = append(cmdParts, "--health-retries", retries)
				}
			}

			// Add container name and image
			cmdParts = append(cmdParts, "--name", containerName, image)

			// Add custom command if specified
			if command, ok := configMap["command"].(string); ok {
				cmdParts = append(cmdParts, "sh", "-c", command)
			}

			runCmd = strings.Join(cmdParts, " ")
			break // Only process first service for now
		}

	default:
		return CommandResult{
			Success: false,
			Stderr:  "Service must be a string or object",
		}
	}

	// Check if container already exists
	checkCmd := fmt.Sprintf("docker ps -a --format '{{.Names}}' --filter name=^%s$", containerName)
	log.Printf("Checking if container exists: %s", checkCmd)
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkCmd,
		CaptureOutput: true,
	})

	if checkResult.Success && strings.Contains(checkResult.Stdout, containerName) {
		// Container exists, check if it's running
		statusCmd := fmt.Sprintf("docker ps --format '{{.Names}}' --filter name=^%s$", containerName)
		log.Printf("Checking container status: %s", statusCmd)
		statusResult := ExecuteShellCommand(ExecuteOptions{
			Command:       statusCmd,
			CaptureOutput: true,
		})

		if statusResult.Success && strings.Contains(statusResult.Stdout, containerName) {
			log.Printf("Container %s is already running", containerName)
			return CommandResult{Success: true, Stdout: "Container already running"}
		} else {
			// Container exists but is stopped, start it
			startCmd := fmt.Sprintf("docker start %s", containerName)
			log.Printf("Starting existing container: %s", startCmd)
			return ExecuteShellCommand(ExecuteOptions{
				Command:       startCmd,
				CaptureOutput: true,
			})
		}
	}

	// Container doesn't exist, create and start it
	log.Printf("Creating new container: %s", runCmd)
	return ExecuteShellCommand(ExecuteOptions{
		Command:       runCmd,
		CaptureOutput: true,
	})
}

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

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		if err := os.Setenv(key, value); err != nil {
			log.Printf("Failed to set environment variable %s: %v", key, err)
		}
	}

	log.Printf("Loaded environment variables from %s", envFile)
	return nil
}

// HandleServicesConfiguration handles the new services configuration
func HandleServicesConfiguration(services config.ServicesConfig) CommandResult {
	log.Printf("Handling services configuration (compose: %v, containers: %d)",
		services.Compose != nil, len(services.Containers))

	// Handle Docker Compose services
	if services.Compose != nil {
		result := StartDockerCompose(*services.Compose)
		if !result.Success {
			return result
		}

		// Wait for health checks if enabled
		if services.WaitForHealth {
			log.Printf("Waiting for compose services to be healthy")
			// For compose services, we'll check general container health
			// This is a simplified approach since compose services don't have individual names
		}
	}

	// Handle individual container services
	for _, container := range services.Containers {
		result := StartDockerService(container)
		if !result.Success {
			return result
		}

		// Wait for health checks if enabled
		if services.WaitForHealth {
			healthResult := WaitForServiceHealth(container, services.Timeout)
			if !healthResult.Success {
				log.Printf("Health check failed for service %v: %s", container, healthResult.Stderr)
				// Continue with other services but log the failure
			}
		}
	}

	// Store services for cleanup if needed
	if services.Cleanup {
		// TODO: Implement proper cleanup tracking
		log.Printf("Cleanup enabled for services - tracking for future cleanup")
	}

	return CommandResult{Success: true}
}

// StartDockerCompose starts services using Docker Compose
func StartDockerCompose(compose config.ComposeConfig) CommandResult {
	log.Printf("Starting Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		log.Print(errorMsg)
		return CommandResult{Success: false, Stderr: errorMsg}
	}

	// Determine which docker compose command to use
	var composeCmd string

	// Try new docker compose command first
	checkNewCmd := "docker compose version"
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkNewCmd,
		CaptureOutput: true,
	})

	if checkResult.Success {
		composeCmd = "docker compose"
	} else {
		// Fall back to docker-compose
		composeCmd = "docker-compose"
	}

	// Build command
	cmdParts := []string{composeCmd, "-f", compose.File}

	// Add profiles if specified
	for _, profile := range compose.Profiles {
		cmdParts = append(cmdParts, "--profile", profile)
	}

	cmdParts = append(cmdParts, "up", "-d")

	// Add specific services if specified
	if len(compose.Services) > 0 {
		cmdParts = append(cmdParts, compose.Services...)
	}

	finalCmd := strings.Join(cmdParts, " ")
	log.Printf("Running compose command: %s", finalCmd)

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       finalCmd,
		CaptureOutput: true,
	})

	if !result.Success {
		log.Printf("Docker Compose command failed: %s", result.Stderr)
		return result
	}

	log.Print("Docker Compose services started successfully")
	return result
}

// StopServices stops and cleans up services based on configuration
func StopServices(services config.ServicesConfig) CommandResult {
	log.Printf("Stopping services (compose: %v, containers: %d)",
		services.Compose != nil, len(services.Containers))

	var errors []string

	// Stop Docker Compose services
	if services.Compose != nil {
		result := StopDockerCompose(*services.Compose)
		if !result.Success {
			errors = append(errors, fmt.Sprintf("Failed to stop compose services: %s", result.Stderr))
		}
	}

	// Stop individual container services
	for _, container := range services.Containers {
		result := StopDockerService(container)
		if !result.Success {
			errors = append(errors, fmt.Sprintf("Failed to stop container service: %s", result.Stderr))
		}
	}

	if len(errors) > 0 {
		errorMsg := strings.Join(errors, "; ")
		log.Printf("Service cleanup completed with errors: %s", errorMsg)
		return CommandResult{
			Success: false,
			Stderr:  errorMsg,
		}
	}

	log.Print("Service cleanup completed successfully")
	return CommandResult{Success: true}
}

// StopDockerCompose stops services using Docker Compose
func StopDockerCompose(compose config.ComposeConfig) CommandResult {
	log.Printf("Stopping Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		log.Print(errorMsg)
		return CommandResult{Success: false, Stderr: errorMsg}
	}

	// Determine which docker compose command to use
	var composeCmd string

	// Try new docker compose command first
	checkNewCmd := "docker compose version"
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkNewCmd,
		CaptureOutput: true,
	})

	if checkResult.Success {
		composeCmd = "docker compose"
	} else {
		// Fall back to docker-compose
		composeCmd = "docker-compose"
	}

	// Build command
	cmdParts := []string{composeCmd, "-f", compose.File}

	// Add profiles if specified
	for _, profile := range compose.Profiles {
		cmdParts = append(cmdParts, "--profile", profile)
	}

	cmdParts = append(cmdParts, "down")

	// Add specific services if specified
	if len(compose.Services) > 0 {
		cmdParts = append(cmdParts, compose.Services...)
	}

	finalCmd := strings.Join(cmdParts, " ")
	log.Printf("Running compose down command: %s", finalCmd)

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       finalCmd,
		CaptureOutput: true,
	})

	if !result.Success {
		log.Printf("Docker Compose down command failed: %s", result.Stderr)
		return result
	}

	log.Print("Docker Compose services stopped successfully")
	return result
}

// StopDockerService stops a Docker service container
func StopDockerService(service interface{}) CommandResult {
	log.Printf("Stopping Docker service: %v", service)

	var containerName string

	switch s := service.(type) {
	case string:
		containerName = s
	case map[string]interface{}:
		// Complex service definition - assume first key is service name
		for name := range s {
			containerName = name
			break
		}
	default:
		return CommandResult{
			Success: false,
			Stderr:  "Service must be a string or object",
		}
	}

	// Check if container exists and is running
	checkCmd := fmt.Sprintf("docker ps --format '{{.Names}}' --filter name=^%s$", containerName)
	log.Printf("Checking if container is running: %s", checkCmd)
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkCmd,
		CaptureOutput: true,
	})

	if !checkResult.Success || !strings.Contains(checkResult.Stdout, containerName) {
		log.Printf("Container %s is not running", containerName)
		return CommandResult{Success: true, Stdout: "Container not running"}
	}

	// Stop the container
	stopCmd := fmt.Sprintf("docker stop %s", containerName)
	log.Printf("Stopping container: %s", stopCmd)
	stopResult := ExecuteShellCommand(ExecuteOptions{
		Command:       stopCmd,
		CaptureOutput: true,
	})

	if !stopResult.Success {
		log.Printf("Failed to stop container %s: %s", containerName, stopResult.Stderr)
		return stopResult
	}

	log.Printf("Container %s stopped successfully", containerName)
	return CommandResult{Success: true}
}

// WaitForServiceHealth waits for a service to become healthy
func WaitForServiceHealth(service interface{}, timeout int) CommandResult {
	log.Printf("Waiting for service health check: %v (timeout: %d seconds)", service, timeout)

	var containerName string

	switch s := service.(type) {
	case string:
		containerName = s
	case map[string]interface{}:
		// Complex service definition - assume first key is service name
		for name := range s {
			containerName = name
			break
		}
	default:
		return CommandResult{
			Success: false,
			Stderr:  "Service must be a string or object",
		}
	}

	// Wait for container to be healthy
	healthCmd := fmt.Sprintf("timeout %d bash -c 'while [ \"$(docker inspect --format=\"{{.State.Health.Status}}\" %s 2>/dev/null || echo \"no-health\")\" != \"healthy\" ]; do sleep 1; done'", timeout, containerName)
	log.Printf("Running health check: %s", healthCmd)

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       healthCmd,
		CaptureOutput: true,
	})

	if !result.Success {
		errorMsg := fmt.Sprintf("Health check failed for container %s: %s", containerName, result.Stderr)
		log.Print(errorMsg)
		return CommandResult{Success: false, Stderr: errorMsg}
	}

	log.Printf("Container %s is healthy", containerName)
	return CommandResult{Success: true}
}
