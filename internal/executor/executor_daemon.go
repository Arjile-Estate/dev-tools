package executor

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"dev-tools/internal/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dev-tools/internal/colors"
)

// PIDFileInfo contains metadata about a daemon process
type PIDFileInfo struct {
	PID          int       `json:"pid"`
	CommandName  string    `json:"command_name"`
	Command      string    `json:"command"`
	StartTime    time.Time `json:"start_time"`
	RestartCount int       `json:"restart_count"`
}

// DaemonInfo contains information about a daemon process including runtime status
type DaemonInfo struct {
	PIDFileInfo
	PIDFile   string `json:"pid_file"`
	IsRunning bool   `json:"is_running"`
	Uptime    string `json:"uptime,omitempty"`
}

// GeneratePIDFilename generates a PID filename using SHA1 hash
func GeneratePIDFilename(commandName, command string) string {
	combined := commandName + command
	hash := sha1.Sum([]byte(combined))
	return fmt.Sprintf(".%x.pid", hash[:4]) // Use first 8 hex characters (4 bytes)
}

// CreatePIDFile creates a PID file in legacy format (for backward compatibility in tests)
func CreatePIDFile(pidFile string, pid int) error {
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// CreateEnhancedPIDFile creates a PID file with full metadata
func CreateEnhancedPIDFile(pidFile string, pid int, commandName, command string) error {
	pidInfo := PIDFileInfo{
		PID:          pid,
		CommandName:  commandName,
		Command:      command,
		StartTime:    time.Now(),
		RestartCount: 0,
	}

	data, err := json.Marshal(pidInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal PID info: %w", err)
	}

	return os.WriteFile(pidFile, data, 0644)
}

// ReadPIDFile reads a PID from a PID file (supports both enhanced and legacy formats)
func ReadPIDFile(pidFile string) (int, error) {
	pidInfo, err := ReadEnhancedPIDFile(pidFile)
	if err != nil {
		return 0, err
	}
	return pidInfo.PID, nil
}

// ReadEnhancedPIDFile reads enhanced PID file information (supports both formats)
func ReadEnhancedPIDFile(pidFile string) (*PIDFileInfo, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON (enhanced format)
	var pidInfo PIDFileInfo
	if err := json.Unmarshal(data, &pidInfo); err == nil {
		return &pidInfo, nil
	}

	// Fall back to legacy format (just PID number)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid PID in file %s: %w", pidFile, err)
	}

	// Return minimal info for legacy format
	return &PIDFileInfo{
		PID:          pid,
		CommandName:  "",
		Command:      "",
		StartTime:    time.Time{},
		RestartCount: 0,
	}, nil
}

// RemovePIDFile removes a PID file
func RemovePIDFile(pidFile string) error {
	return os.Remove(pidFile)
}

// IsProcessRunning checks if a process with given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// ListDaemonProcesses returns information about all daemon processes in the project directory
func ListDaemonProcesses(projectDir string) ([]DaemonInfo, error) {
	logger.Infof("Listing daemon processes in %s", projectDir)

	pidFiles, err := filepath.Glob(filepath.Join(projectDir, "*.pid"))
	if err != nil {
		return nil, fmt.Errorf("failed to find PID files: %w", err)
	}

	var daemons []DaemonInfo

	for _, pidFile := range pidFiles {
		pidInfo, err := ReadEnhancedPIDFile(pidFile)
		if err != nil {
			logger.Infof("Could not read PID file %s: %v", pidFile, err)
			continue
		}

		isRunning := IsProcessRunning(pidInfo.PID)
		var uptime string
		if isRunning && !pidInfo.StartTime.IsZero() {
			uptime = time.Since(pidInfo.StartTime).Round(time.Second).String()
		}

		daemon := DaemonInfo{
			PIDFileInfo: *pidInfo,
			PIDFile:     filepath.Base(pidFile),
			IsRunning:   isRunning,
			Uptime:      uptime,
		}

		daemons = append(daemons, daemon)
	}

	return daemons, nil
}

// FindDaemonByCommandName finds a daemon by its command name
func FindDaemonByCommandName(projectDir, commandName string) (*DaemonInfo, error) {
	daemons, err := ListDaemonProcesses(projectDir)
	if err != nil {
		return nil, err
	}

	for _, daemon := range daemons {
		if daemon.CommandName == commandName {
			return &daemon, nil
		}
	}

	return nil, fmt.Errorf("daemon with command name '%s' not found", commandName)
}

// StopDaemonProcess stops a daemon process by PID and removes its PID file
func StopDaemonProcess(projectDir string, daemon *DaemonInfo) error {
	logger.Infof("Stopping daemon process %s (PID %d)", daemon.CommandName, daemon.PID)

	if !daemon.IsRunning {
		logger.Infof("Daemon %s is not running, removing stale PID file", daemon.CommandName)
		pidFilePath := filepath.Join(projectDir, daemon.PIDFile)
		return RemovePIDFile(pidFilePath)
	}

	// Send SIGTERM to the process
	process, err := os.FindProcess(daemon.PID)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", daemon.PID, err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", daemon.PID, err)
	}

	// Wait for process to terminate (with timeout)
	for i := 0; i < 30; i++ {
		if !IsProcessRunning(daemon.PID) {
			logger.Infof("Daemon %s (PID %d) stopped successfully", daemon.CommandName, daemon.PID)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill if still running
	if IsProcessRunning(daemon.PID) {
		logger.Infof("Daemon %s (PID %d) did not stop gracefully, force killing", daemon.CommandName, daemon.PID)
		err = process.Signal(syscall.SIGKILL)
		if err != nil {
			return fmt.Errorf("failed to send SIGKILL to process %d: %w", daemon.PID, err)
		}
		// Wait a bit more for SIGKILL to take effect
		time.Sleep(500 * time.Millisecond)
	}

	// Remove PID file
	pidFilePath := filepath.Join(projectDir, daemon.PIDFile)
	err = RemovePIDFile(pidFilePath)
	if err != nil {
		// Log with proper error type but don't fail - process is stopped
		logger.Infof("Warning: %v", NewDaemonError(daemon.PID, pidFilePath, fmt.Errorf("failed to remove PID file: %w", err)))
		// Don't return error for PID file removal failure
	}

	return nil
}

// RestartDaemonProcess restarts a daemon process
func RestartDaemonProcess(projectDir string, daemon *DaemonInfo) error {
	logger.Infof("Restarting daemon process %s", daemon.CommandName)

	// Stop the existing process if running
	if daemon.IsRunning {
		if err := StopDaemonProcess(projectDir, daemon); err != nil {
			return fmt.Errorf("failed to stop daemon for restart: %w", err)
		}
		// Wait a moment for the process to fully terminate
		time.Sleep(100 * time.Millisecond)
	} else {
		// If the process is not running, just remove the stale PID file
		pidFilePath := filepath.Join(projectDir, daemon.PIDFile)
		if err := RemovePIDFile(pidFilePath); err != nil {
			// Log with proper error type - not critical since process not running
			logger.Infof("Warning: %v", NewDaemonError(0, pidFilePath, fmt.Errorf("failed to remove stale PID file: %w", err)))
		}
	}

	// Start the daemon again with the same command
	if daemon.Command == "" {
		return fmt.Errorf("cannot restart daemon %s: no command information available (legacy PID file)", daemon.CommandName)
	}

	result := ExecuteShellCommand(context.Background(), ExecuteOptions{
		Command:     daemon.Command,
		Background:  true,
		Daemon:      true,
		CommandName: daemon.CommandName,
	})

	if !result.Success {
		return fmt.Errorf("failed to restart daemon %s: %s", daemon.CommandName, result.Stderr)
	}

	// Create a new PID file for the restarted process
	pidFile := GeneratePIDFilename(daemon.CommandName, daemon.Command)
	pidFilePath := filepath.Join(projectDir, pidFile)
	if err := CreateEnhancedPIDFile(pidFilePath, result.PID, daemon.CommandName, daemon.Command); err != nil {
		// Log with proper error type but don't fail - process is running
		logger.Infof("Warning: %v", NewDaemonError(result.PID, pidFilePath, fmt.Errorf("failed to create PID file: %w", err)))
		// Don't return an error, as the process is running, but log it.
	}

	logger.Infof("Daemon %s restarted successfully with PID %d", daemon.CommandName, result.PID)
	return nil
}

// CleanupStalePIDFilesWithTermination cleans up PID files and optionally terminates running processes
func CleanupStalePIDFilesWithTermination(projectDir string, terminateRunning bool) ExecutionResult {
	logger.Info("Starting PID file cleanup")

	pidFiles, err := filepath.Glob(filepath.Join(projectDir, "*.pid"))
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  fmt.Sprintf("Failed to find PID files: %v", err),
		}
	}

	if len(pidFiles) == 0 {
		message := "No PID files found to clean up"
		logger.Info(message)
		return ExecutionResult{
			Success: true,
			Stdout:  message,
		}
	}

	daemons, err := ListDaemonProcesses(projectDir)
	if err != nil {
		return ExecutionResult{
			Success: false,
			Stderr:  fmt.Sprintf("Failed to list daemon processes: %v", err),
		}
	}

	var cleanedFiles []string
	var terminatedProcesses []string
	var activeProcesses []string
	var errors []string

	for _, daemon := range daemons {
		if daemon.IsRunning {
			if terminateRunning {
				logger.Infof("Terminating running daemon %s (PID %d)", daemon.CommandName, daemon.PID)
				err := StopDaemonProcess(projectDir, &daemon)
				if err != nil {
					errorMsg := fmt.Sprintf("Failed to terminate %s (PID %d): %v", daemon.CommandName, daemon.PID, err)
					logger.Info(errorMsg)
					errors = append(errors, errorMsg)
				} else {
					terminatedProcesses = append(terminatedProcesses, fmt.Sprintf("%s (PID %d)", daemon.CommandName, daemon.PID))
				}
			} else {
				logger.Infof("Process %d from %s is still running", daemon.PID, daemon.PIDFile)
				activeProcesses = append(activeProcesses, fmt.Sprintf("%s (PID %d)", daemon.CommandName, daemon.PID))
			}
		} else {
			logger.Infof("Process %d from %s is not running, removing PID file", daemon.PID, daemon.PIDFile)
			pidFilePath := filepath.Join(projectDir, daemon.PIDFile)
			if err := RemovePIDFile(pidFilePath); err != nil {
				errorMsg := fmt.Sprintf("Failed to remove %s: %v", daemon.PIDFile, err)
				logger.Info(errorMsg)
				errors = append(errors, errorMsg)
			} else {
				cleanedFiles = append(cleanedFiles, fmt.Sprintf("%s (PID %d)", daemon.CommandName, daemon.PID))
			}
		}
	}

	// Prepare summary message
	var summary strings.Builder

	if len(cleanedFiles) > 0 {
		fmt.Fprintf(&summary, "%s\n", colors.Info("Cleaned up %d stale PID file(s):", len(cleanedFiles)))
		for _, file := range cleanedFiles {
			fmt.Fprintf(&summary, "  - %s\n", file)
		}
	}

	if len(terminatedProcesses) > 0 {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		fmt.Fprintf(&summary, "%s\n", colors.Success("Terminated %d running daemon(s):", len(terminatedProcesses)))
		for _, process := range terminatedProcesses {
			fmt.Fprintf(&summary, "  - %s\n", process)
		}
	}

	if len(activeProcesses) > 0 {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		fmt.Fprintf(&summary, "%s\n", colors.Info("Found %d active process(es):", len(activeProcesses)))
		for _, process := range activeProcesses {
			fmt.Fprintf(&summary, "  - %s\n", process)
		}
	}

	if len(errors) > 0 {
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		fmt.Fprintf(&summary, "%s\n", colors.Warning("Encountered %d error(s):", len(errors)))
		for _, error := range errors {
			fmt.Fprintf(&summary, "  - %s\n", error)
		}
	}

	if len(cleanedFiles) == 0 && len(terminatedProcesses) == 0 && len(activeProcesses) == 0 && len(errors) == 0 {
		summary.WriteString(colors.Info("No PID files found to process"))
	}

	logger.Infof("PID cleanup completed. Summary: %s", summary.String())

	// Return success if we cleaned files or terminated processes, error only if all operations failed
	success := len(errors) == 0 || len(cleanedFiles) > 0 || len(terminatedProcesses) > 0 || len(activeProcesses) > 0

	return ExecutionResult{
		Success: success,
		Stdout:  summary.String(),
		Stderr:  strings.Join(errors, "\n"),
	}
}

// CleanupStalePIDFiles cleans up stale PID files for processes that are no longer running
func CleanupStalePIDFiles(projectDir string) ExecutionResult {
	return CleanupStalePIDFilesWithTermination(projectDir, false)
}
