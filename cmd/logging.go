package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// setupLogging initializes the logger for the application.
func setupLogging(verbose bool, projectDir string) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if verbose {
		log.SetOutput(os.Stdout)
		return
	}

	logFile, err := getLogFilePath(projectDir)
	if err != nil {
		log.Printf("Error getting log file path: %v. Defaulting to project directory.", err)
		logFile = filepath.Join(projectDir, "activity.log")
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		log.Printf("Error creating log directory: %v. Defaulting to stdout.", err)
		log.SetOutput(os.Stdout)
		return
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Error opening log file: %v. Defaulting to stdout.", err)
		log.SetOutput(os.Stdout)
		return
	}

	log.SetOutput(file)
}
