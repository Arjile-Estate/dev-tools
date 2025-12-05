package executor

import (
	"fmt"
	"log"
	"strings"
)

// buildDockerPsCommand builds a docker ps command with the given format and filters
func buildDockerPsCommand(containerName string, allContainers bool) string {
	flags := "--format '{{.Names}}'"
	if allContainers {
		flags = "-a " + flags
	}
	return fmt.Sprintf("docker ps %s --filter name=^%s$", flags, containerName)
}

// containerExists checks if a container exists (running or stopped)
func containerExists(containerName string) bool {
	checkCmd := buildDockerPsCommand(containerName, true)
	log.Printf("Checking if container exists: %s", checkCmd)

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       checkCmd,
		CaptureOutput: true,
	})

	return result.Success && strings.Contains(result.Stdout, containerName)
}

// containerIsRunning checks if a container is currently running
func containerIsRunning(containerName string) bool {
	statusCmd := buildDockerPsCommand(containerName, false)
	log.Printf("Checking container status: %s", statusCmd)

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       statusCmd,
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

// startExistingContainer starts a stopped container
func startExistingContainer(containerName string) ExecutionResult {
	startCmd := fmt.Sprintf("docker start %s", containerName)
	log.Printf("Starting existing container: %s", startCmd)
	return ExecuteShellCommand(ExecuteOptions{
		Command:       startCmd,
		CaptureOutput: true,
	})
}

// createAndStartContainer creates a new container using the docker run command
func createAndStartContainer(runCmd string) ExecutionResult {
	log.Printf("Creating new container: %s", runCmd)
	return ExecuteShellCommand(ExecuteOptions{
		Command:       runCmd,
		CaptureOutput: true,
	})
}
