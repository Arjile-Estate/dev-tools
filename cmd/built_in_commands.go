package cmd

import (
	"fmt"
	"log"
	"os"

	"strings"

	"dev-tools/internal/colors"
	"dev-tools/internal/executor"
	"github.com/spf13/cobra"
)

func handleLogsCommand(cmd *cobra.Command) error {
	log.Print("Displaying recent activity logs")

	logFile, err := getLogFilePath(projectDir)
	if err != nil {
		return fmt.Errorf("failed to get log file path: %w", err)
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s", logFile)
	}

	result := executor.ExecuteShellCommand(executor.ExecuteOptions{
		Command:       fmt.Sprintf("tail -n 50 %s", logFile),
		CaptureOutput: true,
	})

	if !result.Success {
		return fmt.Errorf("failed to read logs: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func handleCleanupPidsCommand(cmd *cobra.Command) error {
	result := executor.CleanupStalePIDFiles(projectDir)
	if !result.Success {
		return fmt.Errorf("cleanup failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func handleCleanupAllCommand(cmd *cobra.Command) error {
	log.Print("Cleaning up all daemon processes and PID files")

	result := executor.CleanupStalePIDFilesWithTermination(projectDir, true)
	if !result.Success {
		return fmt.Errorf("cleanup-all failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func handleStatusCommand(cmd *cobra.Command) error {
	log.Print("Displaying daemon process status")

	daemons, err := executor.ListDaemonProcesses(projectDir)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to list daemon processes: %v", err)))
	}

	if len(daemons) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("No daemon processes found in %s", projectDir)))
		return nil
	}

	// Display header
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Highlight("DAEMON STATUS"))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

	// Display table header
	header := fmt.Sprintf("%-20s %-10s %-8s %-12s %s",
		"COMMAND NAME", "STATUS", "PID", "UPTIME", "COMMAND")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(header))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))

	// Display each daemon
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
			commandName = "(legacy)"
		}

		uptime := daemon.Uptime
		if uptime == "" {
			uptime = "N/A"
		}

		command := daemon.Command
		if command == "" {
			command = "(unknown)"
		}
		if len(command) > 40 {
			command = command[:37] + "..."
		}

		row := fmt.Sprintf("%-20s %-10s %-8d %-12s %s",
			commandName,
			statusColor,
			daemon.PID,
			uptime,
			command)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), row)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
	_, _ = fmt.Fprint(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Total: %d daemon process(es)\n", len(daemons))))

	return nil
}

func handleRestartCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("restart command requires a daemon name"))
	}

	daemonName := args[1]
	log.Printf("Restarting daemon: %s", daemonName)

	daemon, err := executor.FindDaemonByCommandName(projectDir, daemonName)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("daemon '%s' not found: %v", daemonName, err)))
	}

	err = executor.RestartDaemonProcess(projectDir, daemon)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to restart daemon '%s': %v", daemonName, err)))
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Restarted daemon '%s'", daemonName)))
	return nil
}

func handleStopCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("stop command requires a daemon name"))
	}

	daemonName := args[1]
	log.Printf("Stopping daemon: %s", daemonName)

	daemon, err := executor.FindDaemonByCommandName(projectDir, daemonName)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("daemon '%s' not found: %v", daemonName, err)))
	}

	err = executor.StopDaemonProcess(projectDir, daemon)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to stop daemon '%s': %v", daemonName, err)))
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Stopped daemon '%s'", daemonName)))
	return nil
}
