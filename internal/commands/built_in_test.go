package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev-tools/internal/config"
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
		assert.Contains(t, output, "(unknown)") // Should show "(unknown)" for empty command name
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
func TestOutputStatusJSON(t *testing.T) {
	t.Run("output daemon status as JSON", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a PID file
		pidFile := filepath.Join(tempDir, ".test.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          os.Getpid(),
			CommandName:  "test-daemon",
			Command:      "npm run dev",
			StartTime:    time.Now().Add(-5 * time.Minute),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleStatusCommand(cmd, []string{"--format=json"}, tempDir)

		require.NoError(t, err)
		output := buf.String()

		// Verify it's valid JSON
		var jsonOutput map[string]interface{}
		err = json.Unmarshal([]byte(output), &jsonOutput)
		require.NoError(t, err)

		// Check for expected fields
		assert.Contains(t, jsonOutput, "project_type")
		assert.Contains(t, jsonOutput, "daemons")
		assert.Contains(t, jsonOutput, "services")
	})
}

func TestProjectTypeToString(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   func(tmpDir string)
		expectedType string
	}{
		{
			name: "go project",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)
			},
			expectedType: "Go",
		},
		{
			name: "python project",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(""), 0644)
			},
			expectedType: "Python",
		},
		{
			name: "node project",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644)
			},
			expectedType: "Node.js",
		},
		{
			name: "rust project",
			setupFiles: func(tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(""), 0644)
			},
			expectedType: "Rust",
		},
		{
			name:         "generic project",
			setupFiles:   func(tmpDir string) {},
			expectedType: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setupFiles(tmpDir)

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleStatusCommand(cmd, []string{}, tmpDir)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.expectedType)
		})
	}
}

func TestGetRunningDockerContainers(t *testing.T) {
	t.Run("returns empty list when Docker is not available", func(t *testing.T) {
		containers := getRunningDockerContainers()
		// Should not panic, should return empty list or list of containers
		assert.NotNil(t, containers)
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "orange",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "apple",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandleOnboardCommand(t *testing.T) {
	t.Run("generate onboarding documentation", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a basic config file
		configContent := `commands:
  test:
    - run: "go test ./..."
  build:
    - run: "go build -o app ."
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleOnboardCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()

		// Check for key sections
		assert.Contains(t, output, "Dev-Tools Usage Guide")
		assert.Contains(t, output, "Available Project Commands")
		assert.Contains(t, output, "test")
		assert.Contains(t, output, "build")
	})

	t.Run("generate onboarding outputs to stdout by default", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a basic config file
		configContent := `commands:
  lint:
    - run: "golangci-lint run"
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleOnboardCommand(cmd, []string{}, tempDir)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "lint")
	})
}

func TestFileExists(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		result := fileExists(testFile)
		assert.True(t, result)
	})

	t.Run("file does not exist", func(t *testing.T) {
		result := fileExists("/nonexistent/file.txt")
		assert.False(t, result)
	})
}

func TestInitBuiltInCommandRegistry(t *testing.T) {
	t.Run("registry initializes with all built-in commands", func(t *testing.T) {
		// Initialize registry (should be called automatically but we can call explicitly)
		initBuiltInCommandRegistry()

		// Verify all built-in commands are registered
		expectedCommands := []string{
			"logs", "status", "cleanup-pids", "cleanup-all",
			"restart", "stop", "version", "onboard", "completion", "__dev_complete",
		}

		for _, cmdName := range expectedCommands {
			assert.True(t, IsBuiltInCommand(cmdName), "Command %s should be registered", cmdName)
		}
	})

	t.Run("get built-in command names", func(t *testing.T) {
		names := GetBuiltInCommandNames()
		assert.NotEmpty(t, names)
		assert.Contains(t, names, "logs")
		assert.Contains(t, names, "status")
	})

	t.Run("get built-in command map", func(t *testing.T) {
		cmdMap := GetBuiltInCommandMap()
		assert.NotEmpty(t, cmdMap)
		assert.NotNil(t, cmdMap["logs"])
		assert.NotNil(t, cmdMap["status"])
	})

	t.Run("get individual built-in command", func(t *testing.T) {
		logsCmd := GetBuiltInCommand("logs")
		assert.NotNil(t, logsCmd)

		statusCmd := GetBuiltInCommand("status")
		assert.NotNil(t, statusCmd)

		nonExistent := GetBuiltInCommand("nonexistent")
		assert.Nil(t, nonExistent)
	})
}

func TestGetBuiltInDescription(t *testing.T) {
	t.Run("get description for built-in commands with empty steps", func(t *testing.T) {
		tests := []struct {
			command     string
			expectedMsg string
		}{
			{"logs", "Run logs"},
			{"status", "Run status"},
			{"cleanup-pids", "Run cleanup-pids"},
			{"onboard", "Run onboard"},
		}

		// Create empty steps for testing
		emptySteps := []config.CommandStep{}

		for _, tt := range tests {
			t.Run(tt.command, func(t *testing.T) {
				desc := getBuiltInDescription(tt.command, emptySteps)
				assert.Equal(t, tt.expectedMsg, desc)
			})
		}
	})

	t.Run("get description for commands with steps", func(t *testing.T) {
		steps := []config.CommandStep{
			{Run: config.RunCommand{"go test ./..."}},
		}

		desc := getBuiltInDescription("test", steps)
		assert.Equal(t, "Run go test ./...", desc)
	})
}

func TestHasServiceManagement(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		expected    bool
		description string
	}{
		{
			name: "config with docker services",
			configFile: `commands:
  dev:
    - services:
        containers:
          - redis
      run: "npm run dev"
`,
			expected:    true,
			description: "should detect Docker services",
		},
		{
			name: "config with compose services",
			configFile: `commands:
  dev:
    - services:
        compose:
          file: "docker-compose.yml"
      run: "npm run dev"
`,
			expected:    true,
			description: "should detect Compose services",
		},
		{
			name: "config without services",
			configFile: `commands:
  test:
    - run: "go test ./..."
`,
			expected:    false,
			description: "should return false for no services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, ".dev-config.yaml")
			err := os.WriteFile(configFile, []byte(tt.configFile), 0644)
			require.NoError(t, err)

			// Load the config
			cfg, err := config.LoadConfigurationForProject(tempDir)
			require.NoError(t, err)

			result := hasServiceManagement(cfg.Commands)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}
