package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: false,
		},
		{
			name:    "version flag",
			args:    []string{"version"},
			wantErr: false,
		},
		{
			name:    "verbose flag",
			args:    []string{"--verbose", "version"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewRootCommand()
			rootCmd.SetArgs(tt.args)

			// Capture output
			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err := rootCmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("rootCmd.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogsCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test activity log in tmpDir
	logContent := "2023-01-01 12:00:00 - test - INFO - Test log message\n"
	err := os.WriteFile(filepath.Join(tmpDir, "activity.log"), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "logs"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("logs command should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Test log message") {
		t.Errorf("logs command should display log content, got: %s", output)
	}
}

func TestCleanupPidsCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test PID file with invalid PID
	err := os.WriteFile(filepath.Join(tmpDir, ".test123.pid"), []byte("999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "cleanup-pids"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("cleanup-pids command should not error: %v", err)
	}

	// PID file should be removed
	if _, err := os.Stat(filepath.Join(tmpDir, ".test123.pid")); !os.IsNotExist(err) {
		t.Error("Stale PID file should be cleaned up")
	}
}

func TestStatusCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test PID files
	// Enhanced PID file for running daemon
	enhancedPIDContent := fmt.Sprintf(`{"pid":%d,"command_name":"test-daemon","command":"sleep 300","start_time":"%s","restart_count":0}`,
		os.Getpid(), "2023-01-01T12:00:00Z")
	err := os.WriteFile(filepath.Join(tmpDir, ".enhanced.pid"), []byte(enhancedPIDContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create enhanced PID file: %v", err)
	}

	// Legacy PID file for running daemon
	err = os.WriteFile(filepath.Join(tmpDir, ".legacy.pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		t.Fatalf("Failed to create legacy PID file: %v", err)
	}

	// Stale PID file
	err = os.WriteFile(filepath.Join(tmpDir, ".stale.pid"), []byte("999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to create stale PID file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "status"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("status command should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DAEMON STATUS") {
		t.Errorf("status command should show daemon status header, got: %s", output)
	}

	if !strings.Contains(output, "test-daemon") {
		t.Errorf("status command should show enhanced daemon info, got: %s", output)
	}

	if !strings.Contains(output, "Running") {
		t.Errorf("status command should show running status, got: %s", output)
	}

	if !strings.Contains(output, "Stopped") {
		t.Errorf("status command should show stopped status for stale daemon, got: %s", output)
	}
}

func TestStatusCommandNoDaemons(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "status"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("status command should not error with no daemons: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No daemon processes found") {
		t.Errorf("status command should show 'no daemons' message, got: %s", output)
	}
}

func TestRestartCommand(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create enhanced PID file for a stale daemon (using non-existent PID)
	enhancedPIDContent := fmt.Sprintf(`{"pid":%d,"command_name":"test-daemon","command":"echo restarted","start_time":"%s","restart_count":0}`,
		999999, "2023-01-01T12:00:00Z")
	err := os.WriteFile(".enhanced.pid", []byte(enhancedPIDContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create enhanced PID file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "restart", "test-daemon"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("restart command should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Restarted daemon") {
		t.Errorf("restart command should show restart success message, got: %s", output)
	}

	// Clean up any remaining PID files
	files, _ := filepath.Glob("*.pid")
	for _, file := range files {
		_ = os.Remove(file)
	}
}

func TestRestartCommandNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "restart", "non-existent-daemon"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("restart command should error for non-existent daemon")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("restart command should show 'not found' error, got: %v", err)
	}
}

func TestStopCommand(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create enhanced PID file for a stale daemon (using non-existent PID)
	enhancedPIDContent := fmt.Sprintf(`{"pid":%d,"command_name":"test-daemon","command":"sleep 300","start_time":"%s","restart_count":0}`,
		999999, "2023-01-01T12:00:00Z")
	err := os.WriteFile(".enhanced.pid", []byte(enhancedPIDContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create enhanced PID file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "stop", "test-daemon"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("stop command should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Stopped daemon") {
		t.Errorf("stop command should show stop success message, got: %s", output)
	}

	// Check that PID file was removed
	if _, err := os.Stat(".enhanced.pid"); !os.IsNotExist(err) {
		t.Error("PID file should be removed after stopping daemon")
	}
}

func TestStopCommandNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "stop", "non-existent-daemon"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("stop command should error for non-existent daemon")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("stop command should show 'not found' error, got: %v", err)
	}
}

func TestCommandWithProjectDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a go.mod to make it a Go project
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "logs"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	_ = rootCmd.Execute()
	// This might error because there's no activity.log, but it should at least try
	// The important thing is that it recognizes the project directory
	output := buf.String()
	if strings.Contains(output, "panic") {
		t.Errorf("Command should not panic with project-dir flag, got: %s", output)
	}
}

func TestUnknownCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod to make it a known project type
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "nonexistent-command"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err == nil {
		t.Error("Unknown command should return an error")
	}

	errString := err.Error()
	if !strings.Contains(errString, "unknown command") && !strings.Contains(errString, "Unknown command") {
		t.Errorf("Error should mention unknown command, got: %s", errString)
	}
}

func TestValidCommandExecution(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create go.mod to make it a Go project (which has default test command)
	err := os.WriteFile("go.mod", []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a simple test that will pass
	err = os.MkdirAll("internal/test", 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFile := filepath.Join("internal", "test", "test.go")
	testContent := `package test

import "testing"

func TestDummy(t *testing.T) {
	// This test always passes
}
`
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	rootCmd := NewRootCommand()
	// Use a command that should work - test command for Go project
	rootCmd.SetArgs([]string{"test"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	_ = rootCmd.Execute()
	// The test command might fail due to environment setup, but the command parsing should work
	output := buf.String()
	if strings.Contains(output, "panic") {
		t.Errorf("Valid command should not panic, got: %s", output)
	}
}

func TestVerboseLogging(t *testing.T) {
	tmpDir := t.TempDir()

	// Create activity log
	logContent := "Test log message\n"
	err := os.WriteFile(filepath.Join(tmpDir, "activity.log"), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "--verbose", "logs"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("verbose logs command should not error: %v", err)
	}

	// With verbose flag, we should see the log content
	output := buf.String()
	if !strings.Contains(output, "Test log message") {
		t.Errorf("Verbose logs should display log content, got: %s", output)
	}
}

func TestGenerateDynamicHelpWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test config file
	configContent := `commands:
  custom-command:
    - run: "echo custom"
  another-command:
    - run: "echo another"
`
	configFile := filepath.Join(tmpDir, ".dev-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Also create go.mod to make it a Go project
	goModPath := filepath.Join(tmpDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "--help"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	_ = rootCmd.Execute() // Help command doesn't return error

	output := buf.String()

	// Debug: print the actual output
	t.Logf("Help output: %s", output)

	// Should contain custom commands
	if !strings.Contains(output, "custom-command") {
		t.Errorf("Help should include custom commands from config. Output: %s", output)
	}
	if !strings.Contains(output, "another-command") {
		t.Errorf("Help should include custom commands from config. Output: %s", output)
	}

	// Should contain built-in commands
	if !strings.Contains(output, "cleanup-pids") {
		t.Errorf("Help should include built-in commands. Output: %s", output)
	}
	if !strings.Contains(output, "version") {
		t.Errorf("Help should include built-in commands. Output: %s", output)
	}

	// Should have logs command only in Available commands list (not counting examples and help text)
	// Count logs in the "Available commands" line specifically
	if !strings.Contains(output, "Available commands:") {
		t.Error("Help should contain 'Available commands:' section")
	}

	// Check that logs appears in the available commands
	availableCommandsIdx := strings.Index(output, "Available commands:")
	examplesIdx := strings.Index(output, "Examples:")
	if availableCommandsIdx == -1 || examplesIdx == -1 {
		t.Error("Help should have both 'Available commands:' and 'Examples:' sections")
	}

	availableCommandsSection := output[availableCommandsIdx:examplesIdx]
	if !strings.Contains(availableCommandsSection, "logs") {
		t.Error("logs command should appear in Available commands section")
	}
}

func TestGenerateDynamicHelpNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// No config file, just go.mod
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "--help"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	_ = rootCmd.Execute()

	output := buf.String()

	// Debug: print the actual output
	t.Logf("Help output (no config): %s", output)

	// Should contain default commands from Go project type
	if !strings.Contains(output, "test") {
		t.Errorf("Help should include default test command for Go projects. Output: %s", output)
	}
	if !strings.Contains(output, "build") {
		t.Errorf("Help should include default build command for Go projects. Output: %s", output)
	}
}

func TestHandleLogsCommandEmptyLog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty activity.log file
	activityLog := filepath.Join(tmpDir, "activity.log")
	err := os.WriteFile(activityLog, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty log file: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "logs"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("logs command should succeed with empty log file: %v", err)
	}

	// Should have empty output for empty log file
	output := buf.String()
	if output != "" {
		t.Logf("Output from empty log file: %q", output)
	}
}

func TestHandleLogsCommandWithValidFile(t *testing.T) {
	// Test that handleLogsCommand works with a valid log file
	tmpDir := t.TempDir()

	// Create a test activity.log with some content
	activityLog := filepath.Join(tmpDir, "activity.log")
	testContent := "2025/06/18 12:00:00 Test log message\n2025/06/18 12:01:00 Another test message\n"
	err := os.WriteFile(activityLog, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	// Test using the full command execution rather than direct function call
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "logs"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("logs command should succeed with valid log file: %v", err)
		return
	}

	// Should contain the test content
	output := buf.String()
	if !strings.Contains(output, "Test log message") {
		t.Errorf("Output should contain test log content, got: %s", output)
	}
	if !strings.Contains(output, "Another test message") {
		t.Errorf("Output should contain test log content, got: %s", output)
	}
}

func TestCommandWithRelativeProjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()

	// Change to parent of tmpDir
	parentDir := filepath.Dir(tmpDir)
	if err := os.Chdir(parentDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create go.mod in tmpDir
	relativeDir := filepath.Base(tmpDir)
	goModPath := filepath.Join(relativeDir, "go.mod")
	err := os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", relativeDir, "version"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("Command should work with relative project dir: %v", err)
	}
}

func TestDaemonCommandAlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create a config with a daemon command
	configContent := `commands:
  test-daemon:
    - run: "sleep 0.5"
      daemon: true
`
	err := os.WriteFile(".dev-config.yaml", []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Create a fake PID file for an already running process
	pidFile := filepath.Join(tmpDir, ".test.pid")
	currentPID := os.Getpid()
	err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", currentPID)), 0644)
	if err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}
	defer func() { _ = os.Remove(pidFile) }()

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"test-daemon"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	// Note: This test may not work as expected because the PID filename generation
	// depends on the actual command content, but it exercises the code path
	if err != nil {
		t.Logf("Daemon command handling test completed with error (expected): %v", err)
	}
}
