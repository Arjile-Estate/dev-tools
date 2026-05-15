package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// DockerService represents a running Docker container
type DockerService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image"`
	Ports  string `json:"ports"`
}

// HandleStatusCommand displays comprehensive system status
func HandleStatusCommand(cmd *cobra.Command, args []string, projectDir string) error {
	logger.Info("Displaying comprehensive system status")

	// Output format is propagated via the cobra context by cmd/root.go.
	// Fallback to "text" keeps this safe for direct invocation in tests.
	useJSONFormat := FormatFromContext(cmd) == "json"

	// Gather status information
	daemons, err := executor.ListDaemonProcesses(projectDir)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to list daemon processes: %v", err)))
	}

	// Get running Docker containers
	services := getRunningDockerContainers()

	// Get project information
	projectType := config.DetectProjectType(projectDir)
	configFile := filepath.Join(projectDir, ".dev-config.yaml")
	hasConfig := fileExists(configFile)

	if useJSONFormat {
		return outputStatusJSON(cmd, daemons, services, projectType, configFile, hasConfig)
	}

	return outputStatusText(cmd, daemons, services, projectType, configFile, hasConfig, projectDir)
}

func outputStatusJSON(cmd *cobra.Command, daemons []executor.DaemonInfo, services []DockerService, projectType config.ProjectType, configFile string, hasConfig bool) error {
	status := map[string]interface{}{
		"daemons":      daemons,
		"services":     services,
		"project_type": projectType,
	}

	if hasConfig {
		status["config_file"] = configFile
	}

	jsonBytes, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status to JSON: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	return nil
}

func outputStatusText(cmd *cobra.Command, daemons []executor.DaemonInfo, services []DockerService, projectType config.ProjectType, configFile string, hasConfig bool, projectDir string) error {
	// Display project information
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Highlight("PROJECT INFORMATION"))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Project Type: %s\n", colors.Info(projectTypeToString(projectType)))
	if hasConfig {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Config File:  %s\n", colors.Info(configFile))
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Config File:  %s\n", colors.Warning("Not found (using defaults)"))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

	// Display daemon processes
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Highlight("DAEMON PROCESSES"))
	if len(daemons) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("  No daemon processes found in %s", projectDir)))
	} else {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
		header := fmt.Sprintf("  %-20s %-10s %-8s %-12s %s",
			"COMMAND NAME", "STATUS", "PID", "UPTIME", "COMMAND")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(header))
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  "+strings.Repeat("-", 78))

		for _, daemon := range daemons {
			var status, statusColor string
			if daemon.IsRunning {
				status = "Running"
				statusColor = colors.Success(status)
			} else {
				status = "Stopped"
				statusColor = colors.Warning(status)
			}

			commandName := daemon.CommandName
			if commandName == "" {
				commandName = "(unknown)"
			}

			uptime := daemon.Uptime
			if uptime == "" {
				uptime = "N/A"
			}

			command := daemon.Command
			if command == "" {
				command = "(unknown)"
			}
			if len(command) > StatusCommandMaxLen {
				command = command[:StatusCommandMaxLen-3] + "..."
			}

			row := fmt.Sprintf("  %-20s %-10s %-8d %-12s %s",
				commandName,
				statusColor,
				daemon.PID,
				uptime,
				command)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), row)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Total: %s\n", colors.Info(fmt.Sprintf("%d daemon(s)", len(daemons))))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

	// Display Docker services
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Highlight("DOCKER SERVICES"))
	if len(services) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info("  No Docker containers found"))
	} else {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
		header := fmt.Sprintf("  %-25s %-15s %-15s %s",
			"CONTAINER NAME", "STATUS", "IMAGE", "PORTS")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(header))
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  "+strings.Repeat("-", 78))

		for _, service := range services {
			statusColor := colors.Success(service.Status)
			if service.Status != "running" {
				statusColor = colors.Warning(service.Status)
			}

			image := service.Image
			if len(image) > StatusImageMaxLen {
				image = image[:StatusImageMaxLen-3] + "..."
			}

			ports := service.Ports
			if ports == "" {
				ports = "none"
			}
			if len(ports) > StatusPortsMaxLen {
				ports = ports[:StatusPortsMaxLen-3] + "..."
			}

			row := fmt.Sprintf("  %-25s %-15s %-15s %s",
				service.Name,
				statusColor,
				image,
				ports)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), row)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Total: %s\n", colors.Info(fmt.Sprintf("%d container(s)", len(services))))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

	return nil
}

func getRunningDockerContainers() []DockerService {
	// Check if Docker is available
	checkCmd := executor.ExecuteShellCommand(context.Background(), executor.ExecuteOptions{
		Command:       "docker ps --version",
		CaptureOutput: true,
	})
	if !checkCmd.Success {
		return []DockerService{}
	}

	// Get running containers
	psCmd := executor.ExecuteShellCommand(context.Background(), executor.ExecuteOptions{
		Command:       "docker ps --format '{{.Names}}|{{.Status}}|{{.Image}}|{{.Ports}}'",
		CaptureOutput: true,
	})

	if !psCmd.Success || psCmd.Stdout == "" {
		return []DockerService{}
	}

	var services []DockerService
	lines := strings.Split(strings.TrimSpace(psCmd.Stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= DockerPSFieldCount {
			services = append(services, DockerService{
				Name:   parts[0],
				Status: parts[1],
				Image:  parts[2],
				Ports:  parts[3],
			})
		}
	}

	return services
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func projectTypeToString(pt config.ProjectType) string {
	switch pt {
	case config.ProjectTypeGo:
		return "Go"
	case config.ProjectTypePython:
		return "Python"
	case config.ProjectTypeNodeJS:
		return "Node.js"
	case config.ProjectTypeRust:
		return "Rust"
	case config.ProjectTypeMaven:
		return "Java/Maven"
	case config.ProjectTypeDotNet:
		return ".NET"
	case config.ProjectTypePHP:
		return "PHP"
	case config.ProjectTypeRuby:
		return "Ruby"
	default:
		return "Unknown"
	}
}
