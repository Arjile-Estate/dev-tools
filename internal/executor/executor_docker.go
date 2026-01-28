package executor

import (
	"fmt"
	"log"
	"os"
	"strings"

	"dev-tools/internal/config"
)

// Docker command strings
const (
	// DockerCommand is the base docker CLI command
	DockerCommand = "docker"
)

// getPasswordFromEnv gets a password from environment variable or returns a default
// Logs a warning when using default passwords for security awareness
func getPasswordFromEnv(envVar, defaultPassword, serviceName string) string {
	if password := os.Getenv(envVar); password != "" {
		return password
	}
	log.Printf("WARNING: Using default password for %s. Set %s environment variable for production use.", serviceName, envVar)
	return defaultPassword
}

// buildDockerRunCommand builds a docker run command from a ContainerConfig
func buildDockerRunCommand(cfg *config.ContainerConfig) string {
	cmdParts := []string{"docker", "run", "-d"}

	// Add environment variables
	for key, value := range cfg.Environment {
		cmdParts = append(cmdParts, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volumes
	for _, vol := range cfg.Volumes {
		cmdParts = append(cmdParts, "-v", vol)
	}

	// Add ports
	for _, port := range cfg.Ports {
		cmdParts = append(cmdParts, "-p", port)
	}

	// Add networks
	for _, network := range cfg.Networks {
		cmdParts = append(cmdParts, "--network", network)
	}

	// Add restart policy
	if cfg.Restart != "" {
		cmdParts = append(cmdParts, "--restart", cfg.Restart)
	}

	// Add memory limit
	if cfg.Memory != "" {
		cmdParts = append(cmdParts, "--memory", cfg.Memory)
	}

	// Add CPU limit
	if cfg.CPUs != "" {
		cmdParts = append(cmdParts, "--cpus", cfg.CPUs)
	}

	// Add health check
	if cfg.HealthCheck != nil {
		if cfg.HealthCheck.Test != "" {
			cmdParts = append(cmdParts, "--health-cmd", cfg.HealthCheck.Test)
		}
		if cfg.HealthCheck.Interval != "" {
			cmdParts = append(cmdParts, "--health-interval", cfg.HealthCheck.Interval)
		}
		if cfg.HealthCheck.Timeout != "" {
			cmdParts = append(cmdParts, "--health-timeout", cfg.HealthCheck.Timeout)
		}
		if cfg.HealthCheck.Retries != "" {
			cmdParts = append(cmdParts, "--health-retries", cfg.HealthCheck.Retries)
		}
	}

	// Add container name and image
	cmdParts = append(cmdParts, "--name", cfg.Name, cfg.Image)

	// Add custom command if specified
	if cfg.Command != "" {
		cmdParts = append(cmdParts, "sh", "-c", cfg.Command)
	}

	return strings.Join(cmdParts, " ")
}

// getContainerName extracts container name from service definition
func getContainerName(service interface{}) (string, error) {
	switch s := service.(type) {
	case string:
		return s, nil
	case config.ContainerReference:
		return s.GetName(), nil
	case *config.ContainerReference:
		return s.GetName(), nil
	case map[string]interface{}:
		for name := range s {
			return name, nil
		}
		return "", fmt.Errorf("empty service configuration")
	default:
		return "", fmt.Errorf("service must be a string or ContainerReference")
	}
}

// StartDockerServiceTyped starts a Docker service container using typed configuration
func StartDockerServiceTyped(ref config.ContainerReference) ExecutionResult {
	log.Printf("Starting Docker service: %s", ref.GetName())

	var containerName string
	var runCmd string

	if ref.IsSimple() {
		// Simple string service name
		containerName = ref.Simple

		// Predefined service configurations with environment variable support
		// SECURITY NOTE: Default passwords are "password" for development only.
		// ALWAYS set environment variables in production:
		//   - POSTGRES_PASSWORD for postgres
		//   - MYSQL_ROOT_PASSWORD for mysql
		switch ref.Simple {
		case "redis":
			runCmd = "docker run -d --name redis -p 6379:6379 redis:latest"
		case "postgres":
			postgresPassword := getPasswordFromEnv("POSTGRES_PASSWORD", "password", "postgres")
			runCmd = fmt.Sprintf("docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=%s postgres:latest", postgresPassword)
		case "mysql":
			mysqlPassword := getPasswordFromEnv("MYSQL_ROOT_PASSWORD", "password", "mysql")
			runCmd = fmt.Sprintf("docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=%s mysql:latest", mysqlPassword)
		default:
			// Default format for unknown services
			runCmd = fmt.Sprintf("docker run -d --name %s %s:latest", containerName, ref.Simple)
		}
	} else if ref.Complex != nil {
		// Complex service configuration
		if err := ref.Complex.Validate(); err != nil {
			return ExecutionResult{
				Success: false,
				Stderr:  err.Error(),
			}
		}

		containerName = ref.Complex.Name
		runCmd = buildDockerRunCommand(ref.Complex)
	} else {
		return ExecutionResult{
			Success: false,
			Stderr:  "Invalid container reference: neither simple nor complex",
		}
	}

	// Check container status and handle accordingly
	exists, running := getContainerStatus(containerName)

	if exists {
		if running {
			log.Printf("Container %s is already running", containerName)
			return ExecutionResult{Success: true, Stdout: "Container already running"}
		}
		// Container exists but is stopped, start it
		return startExistingContainer(containerName)
	}

	// Container doesn't exist, create and start it
	return createAndStartContainer(runCmd)
}

// StartDockerService starts a Docker service container (legacy interface{} support)
// Deprecated: Use StartDockerServiceTyped with config.ContainerReference for type safety
func StartDockerService(service interface{}) ExecutionResult {
	// Convert interface{} to ContainerReference for backward compatibility
	switch s := service.(type) {
	case string:
		return StartDockerServiceTyped(config.ContainerReference{Simple: s})
	case config.ContainerReference:
		return StartDockerServiceTyped(s)
	case *config.ContainerReference:
		return StartDockerServiceTyped(*s)
	default:
		return ExecutionResult{
			Success: false,
			Stderr:  "Service must be a string or ContainerReference",
		}
	}
}

// StopDockerService stops a Docker service container
func StopDockerService(service interface{}) ExecutionResult {
	log.Printf("Stopping Docker service: %v", service)

	containerName, err := getContainerName(service)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  err.Error(),
		}
	}

	// Check if container is running
	if !containerIsRunning(containerName) {
		log.Printf("Container %s is not running", containerName)
		return ExecutionResult{Success: true, Stdout: "Container not running"}
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
	return ExecutionResult{Success: true}
}

// WaitForServiceHealth waits for a service to become healthy
func WaitForServiceHealth(service interface{}, timeout int) ExecutionResult {
	log.Printf("Waiting for service health check: %v (timeout: %d seconds)", service, timeout)

	containerName, err := getContainerName(service)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  err.Error(),
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
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	log.Printf("Container %s is healthy", containerName)
	return ExecutionResult{Success: true}
}
