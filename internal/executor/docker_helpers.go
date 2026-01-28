package executor

import (
	"fmt"
	"log"
	"strings"
)

// buildDockerPsArgs builds docker ps arguments with the given format and filters
func buildDockerPsArgs(containerName string, allContainers bool) []string {
	args := []string{"ps", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=^%s$", containerName)}
	if allContainers {
		// Insert -a after ps
		args = []string{"ps", "-a", "--format", "{{.Names}}", "--filter", fmt.Sprintf("name=^%s$", containerName)}
	}
	return args
}

// containerExists checks if a container exists (running or stopped) using direct execution
func containerExists(containerName string) bool {
	args := buildDockerPsArgs(containerName, true)
	log.Printf("Checking if container exists: docker %v", args)

	result := ExecuteCommandDirect(DirectExecuteOptions{
		Command:       "docker",
		Args:          args,
		CaptureOutput: true,
	})

	return result.Success && strings.Contains(result.Stdout, containerName)
}

// containerIsRunning checks if a container is currently running using direct execution
func containerIsRunning(containerName string) bool {
	args := buildDockerPsArgs(containerName, false)
	log.Printf("Checking container status: docker %v", args)

	result := ExecuteCommandDirect(DirectExecuteOptions{
		Command:       "docker",
		Args:          args,
		CaptureOutput: true,
	})

	return result.Success && strings.Contains(result.Stdout, containerName)
}

// getContainerStatus returns existence and running status of a container
func getContainerStatus(containerName string) (exists bool, running bool) {
	exists = containerExists(containerName)
	if exists {
		running = containerIsRunning(containerName)
	}
	return
}

// startExistingContainer starts a stopped container using direct execution (no shell)
func startExistingContainer(containerName string) ExecutionResult {
	log.Printf("Starting existing container: docker start %s", containerName)
	return ExecuteCommandDirect(DirectExecuteOptions{
		Command:       "docker",
		Args:          []string{"start", containerName},
		CaptureOutput: true,
	})
}

// createAndStartContainer creates a new container using direct execution (no shell)
// Accepts command name and arguments separately for security
func createAndStartContainer(command string, args []string) ExecutionResult {
	log.Printf("Creating new container: %s %v", command, args)
	return ExecuteCommandDirect(DirectExecuteOptions{
		Command:       command,
		Args:          args,
		CaptureOutput: true,
	})
}
