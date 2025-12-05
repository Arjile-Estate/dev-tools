package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev-tools/internal/executor"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleLogsCommand(t *testing.T) {
	tests := []struct {
		name           string
		setupLogFile   bool
		logContent     string
		expectedOutput string
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "successful log display",
			setupLogFile:   true,
			logContent:     "line1\nline2\nline3\n",
			expectedOutput: "line1\nline2\nline3\n",
			expectError:    false,
		},
		{
			name:         "log file does not exist",
			setupLogFile: false,
			expectError:  true,
			errorMsg:     "no log file found at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if tt.setupLogFile {
				logFile := filepath.Join(tempDir, "activity.log")
				err := os.WriteFile(logFile, []byte(tt.logContent), 0644)
				require.NoError(t, err)
			}

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleLogsCommand(cmd, tempDir)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Contains(t, buf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestHandleCleanupPidsCommand(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput string
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "successful cleanup",
			expectedOutput: "cleaned up",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleCleanupPidsCommand(cmd, tempDir)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandleCleanupAllCommand(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput string
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "successful cleanup all",
			expectedOutput: "cleaned up",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleCleanupAllCommand(cmd, tempDir)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandleStatusCommand(t *testing.T) {
	t.Run("no daemons found in empty directory", func(t *testing.T) {
		tempDir := t.TempDir()

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "No daemon processes found")
	})

	t.Run("display daemon status with running processes", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a PID file for a running daemon
		pidFile1 := filepath.Join(tempDir, ".daemon1.pid")
		pidInfo1 := executor.PIDFileInfo{
			PID:          os.Getpid(), // Use current process PID (guaranteed to be running)
			CommandName:  "test-daemon",
			Command:      "npm run dev",
			StartTime:    time.Now().Add(-5 * time.Minute),
			RestartCount: 0,
		}
		data1, err := json.Marshal(pidInfo1)
		require.NoError(t, err)
		err = os.WriteFile(pidFile1, data1, 0644)
		require.NoError(t, err)

		// Create a PID file for a stopped daemon
		pidFile2 := filepath.Join(tempDir, ".daemon2.pid")
		pidInfo2 := executor.PIDFileInfo{
			PID:          999999, // Non-existent PID
			CommandName:  "stopped-daemon",
			Command:      "python worker.py",
			StartTime:    time.Now().Add(-10 * time.Minute),
			RestartCount: 1,
		}
		data2, err := json.Marshal(pidInfo2)
		require.NoError(t, err)
		err = os.WriteFile(pidFile2, data2, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()

		// Check for project information header
		assert.Contains(t, output, "PROJECT INFORMATION")
		assert.Contains(t, output, "Project Type")

		// Check for daemon status header
		assert.Contains(t, output, "DAEMON PROCESSES")
		assert.Contains(t, output, "COMMAND NAME")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, "PID")

		// Check for daemon entries
		assert.Contains(t, output, "test-daemon")
		assert.Contains(t, output, "stopped-daemon")
		assert.Contains(t, output, "Running") // For running daemon
		assert.Contains(t, output, "Stopped") // For stopped daemon
		assert.Contains(t, output, "Total:")
		assert.Contains(t, output, "2 daemon(s)")

		// Check for Docker services section
		assert.Contains(t, output, "DOCKER SERVICES")
	})

	t.Run("handle daemon with empty command name", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create PID file with empty command name
		pidFile := filepath.Join(tempDir, ".legacy.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          os.Getpid(),
			CommandName:  "", // Empty command name
			Command:      "some command",
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "(legacy)") // Should show "(legacy)" for empty command name
	})

	t.Run("handle daemon with empty uptime", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create PID file
		pidFile := filepath.Join(tempDir, ".test.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          999999, // Non-existent PID so uptime will be empty
			CommandName:  "test-daemon",
			Command:      "test command",
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "N/A") // Should show "N/A" for empty uptime
	})

	t.Run("handle daemon with empty command", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create PID file with empty command
		pidFile := filepath.Join(tempDir, ".test.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          os.Getpid(),
			CommandName:  "test-daemon",
			Command:      "", // Empty command
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "(unknown)") // Should show "(unknown)" for empty command
	})

	t.Run("handle daemon with long command", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create PID file with very long command
		longCommand := "this is a very long command that exceeds the 38 character limit and should be truncated"
		pidFile := filepath.Join(tempDir, ".test.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          os.Getpid(),
			CommandName:  "test-daemon",
			Command:      longCommand,
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "this is a very long command that ex...") // Should be truncated with "..."
	})
}

func TestHandleRestartCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing daemon name",
			args:        []string{"restart"},
			expectError: true,
			errorMsg:    "restart command requires a daemon name",
		},
		{
			name:        "daemon not found",
			args:        []string{"restart", "nonexistent"},
			expectError: true,
			errorMsg:    "daemon 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleRestartCommand(cmd, tt.args, tempDir)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("successfully find daemon for restart", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a PID file for the daemon to be restarted - use a non-existent PID to avoid terminating test process
		pidFile := filepath.Join(tempDir, ".test-daemon.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          999999, // Non-existent process
			CommandName:  "test-daemon",
			Command:      "sleep 300",
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleRestartCommand(cmd, []string{"restart", "test-daemon"}, tempDir)

		// This should succeed in finding the daemon, though restart might fail due to process not running
		// The important part is that it doesn't fail with "daemon not found"
		if err != nil {
			assert.NotContains(t, err.Error(), "daemon 'test-daemon' not found")
		}
	})
}

func TestHandleStopCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing daemon name",
			args:        []string{"stop"},
			expectError: true,
			errorMsg:    "stop command requires a daemon name",
		},
		{
			name:        "daemon not found",
			args:        []string{"stop", "nonexistent"},
			expectError: true,
			errorMsg:    "daemon 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleStopCommand(cmd, tt.args, tempDir)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("successfully find daemon for stop", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a PID file for the daemon to be stopped - use a non-existent PID to avoid terminating test process
		pidFile := filepath.Join(tempDir, ".test-daemon.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          999999, // Non-existent process
			CommandName:  "test-daemon",
			Command:      "sleep 300",
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStopCommand(cmd, []string{"stop", "test-daemon"}, tempDir)

		// This should succeed in finding the daemon, though stop might fail due to process not running
		// The important part is that it doesn't fail with "daemon not found"
		if err != nil {
			assert.NotContains(t, err.Error(), "daemon 'test-daemon' not found")
		}
	})
}

func TestGetLogFilePath(t *testing.T) {
	tests := []struct {
		name         string
		projectDir   string
		expectedPath string
	}{
		{
			name:         "basic path construction",
			projectDir:   "/home/user/project",
			expectedPath: "/home/user/project/activity.log",
		},
		{
			name:         "relative path",
			projectDir:   "./project",
			expectedPath: "project/activity.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filepath.Join(tt.projectDir, "activity.log")
			assert.Equal(t, tt.expectedPath, result)
		})
	}
}
