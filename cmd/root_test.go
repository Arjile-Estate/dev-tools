package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"dev-tools/internal/mocks"
)

func TestNewRootCommand(t *testing.T) {
	t.Run("creates command with correct properties", func(t *testing.T) {
		cmd := NewRootCommand()

		assert.Equal(t, "dev-tools [command]", cmd.Use)
		assert.Equal(t, "Dev Tools - A command runner for development workflows", cmd.Short)
		assert.Contains(t, cmd.Long, "dev-tools is a command runner")
		assert.Equal(t, "1.2.0", cmd.Version)
		assert.True(t, cmd.SilenceUsage, "SilenceUsage should be true to prevent printing usage on command execution errors")
		assert.True(t, cmd.SilenceErrors, "SilenceErrors should be true so we handle error display ourselves")
		assert.True(t, cmd.DisableFlagParsing, "DisableFlagParsing should be true for manual flag handling")
	})

	t.Run("help function shows dynamic help", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a config file
		configContent := `commands:
  test:
    - run: "go test ./..."
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		cmd := NewRootCommand()
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		// Pass project-dir as an argument and request help
		cmd.SetArgs([]string{"--project-dir=" + tempDir, "--help"})
		_ = cmd.Execute()

		output := buf.String()
		assert.Contains(t, output, "dev-tools is a command runner")
		assert.Contains(t, output, "Available commands:")
		assert.Contains(t, output, "test")
		assert.Contains(t, output, "Usage:")
		assert.Contains(t, output, "Flags:")
	})
}

func TestRunCommand_BuiltInCommands(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "version command",
			command:     "version",
			expectError: false,
		},
		{
			name:        "cleanup-pids command",
			command:     "cleanup-pids",
			expectError: false,
		},
		{
			name:        "cleanup-all command",
			command:     "cleanup-all",
			expectError: false,
		},
		{
			name:        "status command",
			command:     "status",
			expectError: false,
		},
		{
			name:        "completion command",
			command:     "completion",
			expectError: true, // Requires shell argument
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := NewRootCommand()
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			cmd.SetArgs([]string{"--project-dir=" + tempDir, tt.command})
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunCommand_WithMocks(t *testing.T) {
	// Create mock loader and executor
	mockLoader := new(mocks.ConfigLoader)
	mockExecutor := new(mocks.Executor)

	// Create a new root command
	rootCmd := NewRootCommand()

	// Set the mock loader and executor
	SetConfigLoader(mockLoader)
	SetExecutor(mockExecutor)

	// Set up the mock loader to return a specific config
	expectedConfig := &config.Config{
		Commands: map[string][]config.CommandStep{
			"test": {
				{Run: config.RunCommand{"go test ./..."}},
			},
		},
	}
	mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)

	// Set up the mock executor to return a successful result
	mockExecutor.On("ExecuteCommandWithOptions", mock.Anything).Return(executor.ExecutionResult{Success: true})
	mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)

	// Execute the command
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"test"})
	err := rootCmd.Execute()

	// Assert that the command was successful
	assert.NoError(t, err)

	// Assert that the mocks were called
	mockLoader.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestRunCommand_UserDefinedOverridesBuiltIn(t *testing.T) {
	t.Run("user-defined logs command overrides built-in", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		// Config defines a "logs" command, which is also a built-in
		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"logs": {
					{Run: config.RunCommand{"tail -f /var/log/myapp.log"}},
				},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
		mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)

		// The executor should be called — meaning the user's command runs, not the built-in
		mockExecutor.On("ExecuteCommandWithOptions", mock.MatchedBy(func(opts executor.CommandExecutionOptions) bool {
			return opts.CommandName == "logs"
		})).Return(executor.ExecutionResult{Success: true})

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"logs"})

		err := rootCmd.Execute()
		assert.NoError(t, err)

		// Verify the executor was called (user command ran, not built-in)
		mockExecutor.AssertCalled(t, "ExecuteCommandWithOptions", mock.Anything)
		mockLoader.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("built-in logs runs when no user override", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		// Config has commands but NOT "logs"
		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"test": {
					{Run: config.RunCommand{"go test ./..."}},
				},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"logs"})

		// Should run the built-in logs (which will fail in test env, but NOT call executor)
		_ = rootCmd.Execute()

		// Executor should NOT have been called — built-in handled it
		mockExecutor.AssertNotCalled(t, "ExecuteCommandWithOptions", mock.Anything)
	})

	t.Run("built-in command works when config loading fails", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		// Config loading fails (no .dev-config.yaml)
		mockLoader.On("LoadConfig", ".").Return((*config.Config)(nil), errors.New("no config found"))

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"status"})

		err := rootCmd.Execute()
		// Built-in "status" should still work even without config
		assert.NoError(t, err)

		mockLoader.AssertExpectations(t)
	})
}

func TestRunCommand_ErrorCases(t *testing.T) {
	t.Run("environment loading error with mock executor", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		// Config loads first, must return a config with the command so env loading is attempted
		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"custom-test": {{Run: config.RunCommand{"echo test"}}},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
		mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(errors.New("env load failed"))

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"custom-test"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load environment variables")

		mockLoader.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("environment loading error without mock executor", func(t *testing.T) {
		SetConfigLoader(nil)
		SetExecutor(nil)

		// Create a temp directory with invalid .env file
		tempDir := t.TempDir()

		// Create config file
		configContent := `commands:
  test:
    - run: "echo test"
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)

		rootCmd.SetArgs([]string{"--project-dir=" + tempDir, "test"})
		err = rootCmd.Execute()

		// Should succeed even if .env doesn't exist
		assert.NoError(t, err)
	})

	t.Run("config loading error", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		SetConfigLoader(mockLoader)
		defer SetConfigLoader(nil)

		mockLoader.On("LoadConfig", ".").Return((*config.Config)(nil), errors.New("config load failed"))

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"test"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load configuration")

		mockLoader.AssertExpectations(t)
	})

	t.Run("unknown command", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"build": {{Run: config.RunCommand{"go build"}}},
				"lint":  {{Run: config.RunCommand{"golangci-lint run"}}},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"unknown-command"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown command 'unknown-command'")
		assert.Contains(t, err.Error(), "Available commands:")

		mockLoader.AssertExpectations(t)
	})

	t.Run("command execution failure returns ExitError with FailedCommand", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"test": {{Run: config.RunCommand{"go test ./..."}}},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
		mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)
		mockExecutor.On("ExecuteCommandWithOptions", mock.Anything).Return(
			executor.ExecutionResult{
				Success:       false,
				ReturnCode:    1,
				CommandName:   "test",
				FailedCommand: "go test ./...",
				Stdout:        "test output",
				Stderr:        "test failed",
			})

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"test"})

		err := rootCmd.Execute()
		require.Error(t, err)

		var exitErr *ExitError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 1, exitErr.Code)
		assert.Contains(t, exitErr.Message, "go test ./...")
		assert.Contains(t, exitErr.Message, "failed with error code: 1")

		mockLoader.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("command execution failure falls back to CommandName when FailedCommand is empty", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"test": {{Run: config.RunCommand{"go test ./..."}}},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
		mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)
		mockExecutor.On("ExecuteCommandWithOptions", mock.Anything).Return(
			executor.ExecutionResult{
				Success:     false,
				ReturnCode:  42,
				CommandName: "test",
			})

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"test"})

		err := rootCmd.Execute()
		require.Error(t, err)

		var exitErr *ExitError
		require.True(t, errors.As(err, &exitErr))
		assert.Equal(t, 42, exitErr.Code)
		assert.Equal(t, "Command 'test' failed with error code: 42", exitErr.Message)

		mockLoader.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})

	t.Run("command execution with output", func(t *testing.T) {
		mockLoader := new(mocks.ConfigLoader)
		mockExecutor := new(mocks.Executor)

		SetConfigLoader(mockLoader)
		SetExecutor(mockExecutor)
		defer func() {
			SetConfigLoader(nil)
			SetExecutor(nil)
		}()

		expectedConfig := &config.Config{
			Commands: map[string][]config.CommandStep{
				"test": {{Run: config.RunCommand{"echo hello"}}},
			},
		}
		mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
		mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)
		mockExecutor.On("ExecuteCommandWithOptions", mock.Anything).Return(
			executor.ExecutionResult{
				Success: true,
				Stdout:  "hello world\n",
			})

		rootCmd := NewRootCommand()
		var buf bytes.Buffer
		rootCmd.SetOut(&buf)
		rootCmd.SetArgs([]string{"test"})

		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify output was printed
		assert.Contains(t, buf.String(), "hello world")

		mockLoader.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
	})
}

func TestConfigFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    map[string]string
		expected CommandConfig
	}{
		{
			name:  "default values",
			flags: map[string]string{},
			expected: CommandConfig{
				ProjectDir: ".",
				Format:     "text",
			},
		},
		{
			name: "all flags set",
			flags: map[string]string{
				"verbose":     "true",
				"watch":       "true",
				"no-color":    "true",
				"project-dir": "/tmp/test",
				"format":      "json",
			},
			expected: CommandConfig{
				Verbose:    true,
				Watch:      true,
				NoColor:    true,
				ProjectDir: "/tmp/test",
				Format:     "json",
			},
		},
		{
			name: "partial flags set",
			flags: map[string]string{
				"verbose":     "true",
				"project-dir": "/home/user/project",
			},
			expected: CommandConfig{
				Verbose:    true,
				ProjectDir: "/home/user/project",
				Format:     "text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := configFromFlags(tt.flags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetConfigLoader(t *testing.T) {
	originalLoader := configLoader
	defer func() { configLoader = originalLoader }()

	mockLoader := new(mocks.ConfigLoader)
	SetConfigLoader(mockLoader)

	assert.Equal(t, mockLoader, configLoader)
}

func TestSetExecutor(t *testing.T) {
	originalExec := exec
	defer func() { exec = originalExec }()

	mockExec := new(mocks.Executor)
	SetExecutor(mockExec)

	assert.Equal(t, mockExec, exec)
}

func TestRunCommand_WithPassthroughArgs(t *testing.T) {
	tests := []struct {
		name                string
		args                []string
		expectedPassthrough []string
	}{
		{
			name:                "passthrough args without separator",
			args:                []string{"test", "--option", "toto"},
			expectedPassthrough: []string{"--option", "toto"},
		},
		{
			name:                "passthrough args with separator",
			args:                []string{"test", "--", "--option", "toto"},
			expectedPassthrough: []string{"--option", "toto"},
		},
		{
			name:                "multiple passthrough args without separator",
			args:                []string{"test", "--verbose", "--run", "TestExample"},
			expectedPassthrough: []string{"--verbose", "--run", "TestExample"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLoader := new(mocks.ConfigLoader)
			mockExecutor := new(mocks.Executor)

			SetConfigLoader(mockLoader)
			SetExecutor(mockExecutor)
			defer func() {
				SetConfigLoader(nil)
				SetExecutor(nil)
			}()

			expectedConfig := &config.Config{
				Commands: map[string][]config.CommandStep{
					"test": {
						{Run: config.RunCommand{"echo test"}},
					},
				},
			}
			mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)
			mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)

			// The key assertion: verify that ExecuteCommandWithOptions is called with the correct passthrough args
			mockExecutor.On("ExecuteCommandWithOptions", mock.MatchedBy(func(opts executor.CommandExecutionOptions) bool {
				return opts.CommandName == "test" &&
					len(opts.PassthroughArgs) == len(tt.expectedPassthrough) &&
					(len(tt.expectedPassthrough) == 0 || opts.PassthroughArgs[0] == tt.expectedPassthrough[0])
			})).Return(executor.ExecutionResult{Success: true})

			rootCmd := NewRootCommand()
			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			assert.NoError(t, err)

			mockLoader.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected CommandArgs
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: CommandArgs{CommandName: "", PassthroughArgs: nil},
		},
		{
			name:     "simple command",
			args:     []string{"test"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: nil},
		},
		{
			name:     "command with passthrough args",
			args:     []string{"test", "--", "--verbose", "--timeout=30s"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: []string{"--verbose", "--timeout=30s"}},
		},
		{
			name:     "command with separator but no args",
			args:     []string{"test", "--"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: nil},
		},
		{
			name:     "command with flags before separator",
			args:     []string{"build", "--verbose", "--", "-ldflags=-s -w"},
			expected: CommandArgs{CommandName: "build", PassthroughArgs: []string{"-ldflags=-s -w"}},
		},
		{
			name:     "multiple passthrough args",
			args:     []string{"test", "--", "--verbose", "--run", "TestExample", "--timeout=5m"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: []string{"--verbose", "--run", "TestExample", "--timeout=5m"}},
		},
		{
			name:     "command with args without separator",
			args:     []string{"test", "--option", "toto"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: []string{"--option", "toto"}},
		},
		{
			name:     "command with single arg without separator",
			args:     []string{"build", "--release"},
			expected: CommandArgs{CommandName: "build", PassthroughArgs: []string{"--release"}},
		},
		{
			name:     "command with multiple args without separator",
			args:     []string{"test", "--verbose", "--run", "TestExample", "--timeout=5m"},
			expected: CommandArgs{CommandName: "test", PassthroughArgs: []string{"--verbose", "--run", "TestExample", "--timeout=5m"}},
		},
		{
			name:     "command with mixed flags and values without separator",
			args:     []string{"deploy", "--env", "production", "--region", "us-west-2"},
			expected: CommandArgs{CommandName: "deploy", PassthroughArgs: []string{"--env", "production", "--region", "us-west-2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArgs(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}
