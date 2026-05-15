package commands

import (
	"fmt"

	"dev-tools/internal/colors"
	"dev-tools/internal/executor"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleRestartCommand restarts a daemon process. Emits a JSON result when
// the command was invoked with --format json (propagated via context).
func HandleRestartCommand(cmd *cobra.Command, args []string, projectDir string) error {
	return handleDaemonAction(cmd, args, projectDir, daemonAction{
		Key:       "restart",
		Gerund:    "Restarting",
		PastTense: "Restarted",
		Op:        executor.RestartDaemonProcess,
	})
}

// HandleStopCommand stops a daemon process. Emits a JSON result when
// the command was invoked with --format json (propagated via context).
func HandleStopCommand(cmd *cobra.Command, args []string, projectDir string) error {
	return handleDaemonAction(cmd, args, projectDir, daemonAction{
		Key:       "stop",
		Gerund:    "Stopping",
		PastTense: "Stopped",
		Op:        executor.StopDaemonProcess,
	})
}

// daemonAction bundles the variants of an action's name (machine key,
// gerund for log messages, past tense for user output) and the operation
// itself. Pre-computing all the morphology avoids fragile string concat.
type daemonAction struct {
	Key       string
	Gerund    string
	PastTense string
	Op        func(string, *executor.DaemonInfo) error
}

func handleDaemonAction(cmd *cobra.Command, args []string, projectDir string, a daemonAction) error {
	if len(args) < 2 {
		// Argument-validation errors stay as plain Go errors so they reach
		// stderr via main.go regardless of --format.
		return fmt.Errorf("%s", colors.Error("%s command requires a daemon name", a.Key))
	}

	daemonName := args[1]
	jsonMode := FormatFromContext(cmd) == "json"
	logger.Infof("%s daemon: %s", a.Gerund, daemonName)

	daemon, err := executor.FindDaemonByCommandName(projectDir, daemonName)
	if err != nil {
		return reportDaemonFailure(cmd, jsonMode, a.Key, daemonName,
			fmt.Sprintf("daemon '%s' not found: %v", daemonName, err))
	}

	if err := a.Op(projectDir, daemon); err != nil {
		return reportDaemonFailure(cmd, jsonMode, a.Key, daemonName,
			fmt.Sprintf("failed to %s daemon '%s': %v", a.Key, daemonName, err))
	}

	if jsonMode {
		return EmitJSON(cmd, map[string]any{
			"action":  a.Key,
			"daemon":  daemonName,
			"success": true,
		})
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success("%s daemon '%s'", a.PastTense, daemonName))
	return nil
}

// reportDaemonFailure emits a structured JSON failure record in JSON mode,
// and always returns a non-nil error so main.go produces a non-zero exit
// code. The error message is plain (no ANSI) in JSON mode to keep stderr
// machine-friendly.
func reportDaemonFailure(cmd *cobra.Command, jsonMode bool, action, daemonName, msg string) error {
	if jsonMode {
		_ = EmitJSON(cmd, map[string]any{
			"action":  action,
			"daemon":  daemonName,
			"success": false,
			"error":   msg,
		})
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("%s", colors.Error("%s", msg))
}
