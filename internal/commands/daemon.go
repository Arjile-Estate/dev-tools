package commands

import (
	"fmt"

	"dev-tools/internal/colors"
	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleRestartCommand restarts a daemon process
func HandleRestartCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("restart command requires a daemon name"))
	}

	daemonName := args[1]
	logger.Infof("Restarting daemon: %s", daemonName)

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

// HandleStopCommand stops a daemon process
func HandleStopCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("stop command requires a daemon name"))
	}

	daemonName := args[1]
	logger.Infof("Stopping daemon: %s", daemonName)

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
