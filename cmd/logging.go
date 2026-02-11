package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"dev-tools/internal/logger"
	"dev-tools/internal/logpath"
)

// setupLogging initializes the structured logger for the application.
// Returns a cleanup function that closes the log file handle.
func setupLogging(verbose bool, projectDir string) func() {
	noop := func() {}

	// Determine log output destination
	var logWriter *os.File
	if verbose {
		logWriter = os.Stdout
	} else {
		logFile := logpath.GetLogFilePath(projectDir)

		// Create log directory
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error creating log directory: %v. Defaulting to stdout.\n", err)
			logWriter = os.Stdout
		} else {
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

	// Return cleanup function that closes the file handle (no-op for stdout)
	if logWriter != os.Stdout {
		return func() { logWriter.Close() }
	}
	return noop
}
