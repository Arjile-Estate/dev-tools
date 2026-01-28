package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev-tools/internal/logger"
)

var osExecutable = os.Executable

// isRunningViaGoRun detects if the application is running via 'go run'
func isRunningViaGoRun(executable string) bool {
	// 'go run' creates temporary executables in paths like:
	// /tmp/go-build123456789/b001/exe/main (Linux)
	// /var/folders/xy/abcdef/T/go-build987654321/b001/exe/main (macOS)
	return strings.Contains(executable, "go-build") &&
		(strings.Contains(executable, "/tmp/") || strings.Contains(executable, "/T/"))
}

// getLogFilePath determines the appropriate log file path.
func getLogFilePath(projectDir string) (string, error) {
	executable, err := osExecutable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	if isRunningViaGoRun(executable) {
		return filepath.Join(projectDir, "activity.log"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, "Library", "Logs", "dev-tools.log"), nil
}

// setupLogging initializes the structured logger for the application.
func setupLogging(verbose bool, projectDir string) {
	// Determine log output destination
	var logWriter *os.File
	if verbose {
		logWriter = os.Stdout
	} else {
		logFile, err := getLogFilePath(projectDir)
		if err != nil {
			// Fallback to project directory (use fmt since logger not yet initialized)
			fmt.Fprintf(os.Stderr, "Warning: Error getting log file path: %v. Defaulting to project directory.\n", err)
			logFile = filepath.Join(projectDir, "activity.log")
		}

		// Create log directory
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			// Fallback to stdout
			fmt.Fprintf(os.Stderr, "Warning: Error creating log directory: %v. Defaulting to stdout.\n", err)
			logWriter = os.Stdout
		} else {
			// Open log file
			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error opening log file: %v. Defaulting to stdout.\n", err)
				logWriter = os.Stdout
			} else {
				logWriter = file
			}
		}
	}

	// Initialize structured logger with appropriate level
	level := logger.InfoLevel
	if verbose {
		level = logger.DebugLevel
	}

	logger.Init(logWriter, level, verbose)
}
