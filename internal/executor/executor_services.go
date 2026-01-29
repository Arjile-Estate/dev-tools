package executor

import (
	"context"
	"fmt"
	"dev-tools/internal/logger"
	"os"
	"strings"

	"dev-tools/internal/config"
)

// Docker Compose command (built into modern docker CLI)
const DockerComposeCmd = "docker compose"

// HandleServicesConfiguration handles the new services configuration
func HandleServicesConfiguration(services config.ServicesConfig) ExecutionResult {
	logger.Infof("Handling services configuration (compose: %v, containers: %d)",
		services.Compose != nil, len(services.Containers))

	var servicesStarted []string

	// Handle Docker Compose services
	if services.Compose != nil {
		result := StartDockerCompose(*services.Compose)
		if !result.Success {
			return result
		}

		// Track compose services
		if services.Compose.File != "" {
			servicesStarted = append(servicesStarted, "compose:"+services.Compose.File)
		}

		// Wait for health checks if enabled
		if services.WaitForHealth {
			logger.Infof("Waiting for compose services to be healthy")
			// For compose services, we'll check general container health
			// This is a simplified approach since compose services don't have individual names
		}
	}

	// Handle individual container services
	for _, container := range services.Containers {
		result := StartDockerServiceTyped(container)
		if !result.Success {
			return result
		}

		// Track the container service
		containerName, err := getContainerName(container)
		if err == nil {
			servicesStarted = append(servicesStarted, containerName)
		}

		// Wait for health checks if enabled
		if services.WaitForHealth {
			healthResult := WaitForServiceHealth(container, services.Timeout)
			if !healthResult.Success {
				logger.Infof("Health check failed for service %v: %s", container, healthResult.Stderr)
				// Continue with other services but log the failure
			}
		}
	}

	// Cleanup is now handled via defer in ExecuteCommandStep
	if services.Cleanup {
		logger.Infof("Cleanup enabled for services - will be cleaned up after command execution")
	}

	return ExecutionResult{
		Success:         true,
		ServicesStarted: servicesStarted,
	}
}

// StartDockerCompose starts services using Docker Compose
func StartDockerCompose(compose config.ComposeConfig) ExecutionResult {
	logger.Infof("Starting Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		logger.Info(errorMsg)
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	composeCmd := DockerComposeCmd

	// Build command arguments (no shell, direct execution for security)
	args := []string{"-f", compose.File}

	// Add profiles if specified
	for _, profile := range compose.Profiles {
		args = append(args, "--profile", profile)
	}

	args = append(args, "up", "-d")

	// Add specific services if specified
	if len(compose.Services) > 0 {
		args = append(args, compose.Services...)
	}

	logger.Infof("Running compose command: %s %v", composeCmd, args)

	// Use direct execution to avoid shell injection vulnerabilities
	result := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       composeCmd,
		Args:          args,
		CaptureOutput: true,
	})

	if !result.Success {
		logger.Infof("Docker Compose command failed: %s", result.Stderr)
		return result
	}

	logger.Info("Docker Compose services started successfully")
	return result
}

// StopServices stops and cleans up services based on configuration
func StopServices(services config.ServicesConfig) ExecutionResult {
	logger.Infof("Stopping services (compose: %v, containers: %d)",
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
		logger.Infof("Service cleanup completed with errors: %s", errorMsg)
		return ExecutionResult{
			Success: false,
			Stderr:  errorMsg,
		}
	}

	logger.Info("Service cleanup completed successfully")
	return ExecutionResult{Success: true}
}

// StopDockerCompose stops services using Docker Compose
func StopDockerCompose(compose config.ComposeConfig) ExecutionResult {
	logger.Infof("Stopping Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		logger.Info(errorMsg)
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	composeCmd := DockerComposeCmd

	// Build command arguments (no shell, direct execution for security)
	args := []string{"-f", compose.File}

	// Add profiles if specified
	for _, profile := range compose.Profiles {
		args = append(args, "--profile", profile)
	}

	args = append(args, "down")

	// Add specific services if specified
	if len(compose.Services) > 0 {
		args = append(args, compose.Services...)
	}

	logger.Infof("Running compose down command: %s %v", composeCmd, args)

	// Use direct execution to avoid shell injection vulnerabilities
	result := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       composeCmd,
		Args:          args,
		CaptureOutput: true,
	})

	if !result.Success {
		logger.Infof("Docker Compose down command failed: %s", result.Stderr)
		return result
	}

	logger.Info("Docker Compose services stopped successfully")
	return result
}
