package commands

import (
	"fmt"

	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleCleanupPidsCommand cleans up stale PID files
func HandleCleanupPidsCommand(cmd *cobra.Command, projectDir string) error {
	result := executor.CleanupStalePIDFiles(projectDir)
	if !result.Success {
		return fmt.Errorf("cleanup failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

// HandleCleanupAllCommand cleans up all daemon processes and PID files
func HandleCleanupAllCommand(cmd *cobra.Command, projectDir string) error {
	logger.Info("Cleaning up all daemon processes and PID files")

	result := executor.CleanupStalePIDFilesWithTermination(projectDir, true)
	if !result.Success {
		return fmt.Errorf("cleanup-all failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}
