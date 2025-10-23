package executor

import (
	"fmt"
	"log"
	"strings"
)

// Docker command strings
const (
	// DockerCommand is the base docker CLI command
	DockerCommand = "docker"
)

// getContainerName extracts container name from service definition
func getContainerName(service interface{}) (string, error) {
	switch s := service.(type) {
	case string:
		return s, nil
	case map[string]interface{}:
		for name := range s {
			return name, nil
		}
		return "", fmt.Errorf("empty service configuration")
	default:
		return "", fmt.Errorf("service must be a string or object")
	}
}

// StartDockerService starts a Docker service container
func StartDockerService(service interface{}) ExecutionResult {
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
				return ExecutionResult{
					Success: false,
					Stderr:  fmt.Sprintf("Invalid service configuration for %s", name),
				}
			}

			image, ok := configMap["image"].(string)
			if !ok {
				return ExecutionResult{
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
		return ExecutionResult{
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
			return ExecutionResult{Success: true, Stdout: "Container already running"}
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

	// Check if container exists and is running
	checkCmd := fmt.Sprintf("docker ps --format '{{.Names}}' --filter name=^%s$", containerName)
	log.Printf("Checking if container is running: %s", checkCmd)
	checkResult := ExecuteShellCommand(ExecuteOptions{
		Command:       checkCmd,
		CaptureOutput: true,
	})

	if !checkResult.Success || !strings.Contains(checkResult.Stdout, containerName) {
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
