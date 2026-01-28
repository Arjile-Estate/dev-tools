package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleLogsCommand displays recent activity logs
func HandleLogsCommand(cmd *cobra.Command, projectDir string) error {
	logger.Info("Displaying recent activity logs")

	logFile := filepath.Join(projectDir, "activity.log")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s", logFile)
	}

	result := executor.ExecuteShellCommand(context.Background(), executor.ExecuteOptions{
		Command:       fmt.Sprintf("tail -n 50 %s", logFile),
		CaptureOutput: true,
	})

	if !result.Success {
		return fmt.Errorf("failed to read logs: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}
