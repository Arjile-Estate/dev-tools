package executor

import (
	"context"
	"dev-tools/internal/logger"
	"fmt"
	"strings"
)

// IsDockerRunning reports whether the Docker daemon is reachable. It runs
// `docker info`, which (unlike `docker --version`) returns non-zero when the
// daemon is not running. It is a package-level var so tests can stub it.
var IsDockerRunning = func() bool {
	result := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       "docker",
		Args:          []string{"info", "--format", "{{.ServerVersion}}"},
		CaptureOutput: true,
	})
	return result.Success && strings.TrimSpace(result.Stdout) != ""
}

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
	logger.Infof("Checking if container exists: docker %v", args)

	result := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       "docker",
		Args:          args,
		CaptureOutput: true,
	})

	return result.Success && strings.Contains(result.Stdout, containerName)
}

// containerIsRunning checks if a container is currently running using direct execution
func containerIsRunning(containerName string) bool {
	args := buildDockerPsArgs(containerName, false)
	logger.Infof("Checking container status: docker %v", args)

	result := ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
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
	logger.Infof("Starting existing container: docker start %s", containerName)
	return ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       "docker",
		Args:          []string{"start", containerName},
		CaptureOutput: true,
	})
}

// createAndStartContainer creates a new container using direct execution (no shell)
// Accepts command name and arguments separately for security
func createAndStartContainer(command string, args []string) ExecutionResult {
	logger.Infof("Creating new container: %s %v", command, args)
	return ExecuteCommandDirect(context.Background(), DirectExecuteOptions{
		Command:       command,
		Args:          args,
		CaptureOutput: true,
	})
}
