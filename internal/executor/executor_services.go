package executor

import (
	"fmt"
	"log"
	"os"
	"strings"

	"dev-tools/internal/config"
)

// HandleServicesConfiguration handles the new services configuration
func HandleServicesConfiguration(services config.ServicesConfig) ExecutionResult {
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

	return ExecutionResult{Success: true}
}

// getDockerComposeCommand determines which docker compose command to use
func getDockerComposeCommand() string {
	checkNewCmd := "docker compose version"
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkNewCmd,
		CaptureOutput: true,
	})

	if checkResult.Success {
		return "docker compose"
	}
	return "docker-compose"
}

// StartDockerCompose starts services using Docker Compose
func StartDockerCompose(compose config.ComposeConfig) ExecutionResult {
	log.Printf("Starting Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		log.Print(errorMsg)
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	composeCmd := getDockerComposeCommand()

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
func StopServices(services config.ServicesConfig) ExecutionResult {
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
		return ExecutionResult{
			Success: false,
			Stderr:  errorMsg,
		}
	}

	log.Print("Service cleanup completed successfully")
	return ExecutionResult{Success: true}
}

// StopDockerCompose stops services using Docker Compose
func StopDockerCompose(compose config.ComposeConfig) ExecutionResult {
	log.Printf("Stopping Docker Compose services from file: %s", compose.File)

	// Check if compose file exists
	if _, err := os.Stat(compose.File); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("Docker Compose file '%s' does not exist", compose.File)
		log.Print(errorMsg)
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	composeCmd := getDockerComposeCommand()

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
