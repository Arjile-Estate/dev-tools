package executor

import (
	"context"
	"dev-tools/internal/logger"
	"fmt"
	"os"

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
	logger.Infof("WARNING: Using default password for %s. Set %s environment variable for production use.", serviceName, envVar)
	return defaultPassword
}

// buildDockerRunCommand builds docker run command arguments from a ContainerConfig
// Returns command name and arguments separately for secure execution without shell
func buildDockerRunCommand(cfg *config.ContainerConfig) (string, []string) {
	args := []string{"run", "-d"}

	// Add environment variables
	for key, value := range cfg.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volumes
	for _, vol := range cfg.Volumes {
		args = append(args, "-v", vol)
	}

	// Add ports
	for _, port := range cfg.Ports {
		args = append(args, "-p", port)
	}

	// Add networks
	for _, network := range cfg.Networks {
		args = append(args, "--network", network)
	}

	// Add restart policy
	if cfg.Restart != "" {
		args = append(args, "--restart", cfg.Restart)
	}

	// Add memory limit
	if cfg.Memory != "" {
		args = append(args, "--memory", cfg.Memory)
	}

	// Add CPU limit
	if cfg.CPUs != "" {
		args = append(args, "--cpus", cfg.CPUs)
	}

	// Add health check
	if cfg.HealthCheck != nil {
		if cfg.HealthCheck.Test != "" {
			args = append(args, "--health-cmd", cfg.HealthCheck.Test)
		}
		if cfg.HealthCheck.Interval != "" {
			args = append(args, "--health-interval", cfg.HealthCheck.Interval)
		}
		if cfg.HealthCheck.Timeout != "" {
			args = append(args, "--health-timeout", cfg.HealthCheck.Timeout)
		}
		if cfg.HealthCheck.Retries != "" {
			args = append(args, "--health-retries", cfg.HealthCheck.Retries)
		}
	}

	// Add container name and image
	args = append(args, "--name", cfg.Name, cfg.Image)

	// Add custom command if specified
	if cfg.Command != "" {
		args = append(args, "sh", "-c", cfg.Command)
	}

	return "docker", args
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
	logger.Infof("Starting Docker service: %s", ref.GetName())

	var containerName string
	var dockerCmd string
	var dockerArgs []string

	if ref.IsSimple() {
		// Simple string service name
		containerName = ref.Simple
		dockerCmd = "docker"

		// Predefined service configurations with environment variable support
		// SECURITY NOTE: Default passwords are "password" for development only.
		// ALWAYS set environment variables in production:
		//   - POSTGRES_PASSWORD for postgres
		//   - MYSQL_ROOT_PASSWORD for mysql
		switch ref.Simple {
		case "redis":
			dockerArgs = []string{"run", "-d", "--name", "redis", "-p", "6379:6379", "redis:latest"}
		case "postgres":
			postgresPassword := getPasswordFromEnv("POSTGRES_PASSWORD", "password", "postgres")
			dockerArgs = []string{"run", "-d", "--name", "postgres", "-p", "5432:5432", "-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", postgresPassword), "postgres:latest"}
		case "mysql":
			mysqlPassword := getPasswordFromEnv("MYSQL_ROOT_PASSWORD", "password", "mysql")
			dockerArgs = []string{"run", "-d", "--name", "mysql", "-p", "3306:3306", "-e", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", mysqlPassword), "mysql:latest"}
		default:
			// Default format for unknown services
			dockerArgs = []string{"run", "-d", "--name", containerName, fmt.Sprintf("%s:latest", ref.Simple)}
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
		dockerCmd, dockerArgs = buildDockerRunCommand(ref.Complex)
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
			logger.Infof("Container %s is already running", containerName)
			return ExecutionResult{Success: true, Stdout: "Container already running"}
		}
		// Container exists but is stopped, start it
		return startExistingContainer(containerName)
	}

	// Container doesn't exist, create and start it using direct execution (no shell)
	return createAndStartContainer(dockerCmd, dockerArgs)
}

// StopDockerService stops a Docker service container
func StopDockerService(service interface{}) ExecutionResult {
	logger.Infof("Stopping Docker service: %v", service)

	containerName, err := getContainerName(service)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  err.Error(),
		}
	}

	// Check if container is running
	if !containerIsRunning(containerName) {
		logger.Infof("Container %s is not running", containerName)
		return ExecutionResult{Success: true, Stdout: "Container not running"}
	}

	// Stop the container using direct execution (no shell)
	logger.Infof("Stopping container: docker stop %s", containerName)
	stopResult := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       "docker",
		Args:          []string{"stop", containerName},
		CaptureOutput: true,
	})

	if !stopResult.Success {
		logger.Infof("Failed to stop container %s: %s", containerName, stopResult.Stderr)
		return stopResult
	}

	logger.Infof("Container %s stopped successfully", containerName)
	return ExecutionResult{Success: true}
}

// WaitForServiceHealth waits for a service to become healthy
func WaitForServiceHealth(service interface{}, timeout int) ExecutionResult {
	logger.Infof("Waiting for service health check: %v (timeout: %d seconds)", service, timeout)

	containerName, err := getContainerName(service)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  err.Error(),
		}
	}

	// Wait for container to be healthy
	healthCmd := fmt.Sprintf("timeout %d bash -c 'while [ \"$(docker inspect --format=\"{{.State.Health.Status}}\" %s 2>/dev/null || echo \"no-health\")\" != \"healthy\" ]; do sleep 1; done'", timeout, containerName)
	logger.Infof("Running health check: %s", healthCmd)

	result := ExecuteShellCommand(context.Background(), ExecuteOptions{
		Command:       healthCmd,
		CaptureOutput: true,
	})

	if !result.Success {
		errorMsg := fmt.Sprintf("Health check failed for container %s: %s", containerName, result.Stderr)
		logger.Info(errorMsg)
		return ExecutionResult{Success: false, Stderr: errorMsg}
	}

	logger.Infof("Container %s is healthy", containerName)
	return ExecutionResult{Success: true}
}
