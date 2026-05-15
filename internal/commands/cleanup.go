package commands

import (
	"fmt"

	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleCleanupPidsCommand cleans up stale PID files. Honors --format json.
func HandleCleanupPidsCommand(cmd *cobra.Command, projectDir string) error {
	result, summary := executor.CleanupStalePIDFiles(projectDir)
	return finishCleanup(cmd, "cleanup-pids", result, summary)
}

// HandleCleanupAllCommand cleans up all daemon processes and PID files. Honors --format json.
func HandleCleanupAllCommand(cmd *cobra.Command, projectDir string) error {
	logger.Info("Cleaning up all daemon processes and PID files")

	result, summary := executor.CleanupStalePIDFilesWithTermination(projectDir, true)
	return finishCleanup(cmd, "cleanup-all", result, summary)
}

func finishCleanup(cmd *cobra.Command, action string, result executor.ExecutionResult, summary executor.CleanupSummary) error {
	jsonMode := FormatFromContext(cmd) == "json"

	if !result.Success {
		if jsonMode {
			// Best-effort: emit a structured failure even when the executor
			// short-circuited without a populated summary.
			_ = EmitJSON(cmd, map[string]any{
				"action":  action,
				"success": false,
				"error":   result.Stderr,
			})
			return fmt.Errorf("%s", result.Stderr)
		}
		return fmt.Errorf("%s failed: %s", action, result.Stderr)
	}

	if jsonMode {
		return EmitJSON(cmd, map[string]any{
			"action":               action,
			"success":              true,
			"cleaned":              len(summary.CleanedFiles),
			"cleaned_files":        nonNilStrings(summary.CleanedFiles),
			"terminated_processes": nonNilStrings(summary.TerminatedProcesses),
			"active_processes":     nonNilStrings(summary.ActiveProcesses),
			"errors":               nonNilStrings(summary.Errors),
		})
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

// nonNilStrings normalises a nil slice to an empty slice so JSON output
// always has the field shaped as [] rather than null — easier for consumers.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
