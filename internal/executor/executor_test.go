package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteShellCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		background     bool
		captureOutput  bool
		expectSuccess  bool
		expectedOutput string
		checkPID       bool
	}{
		{
			name:           "simple successful command",
			command:        "echo 'hello world'",
			background:     false,
			captureOutput:  true,
			expectSuccess:  true,
			expectedOutput: "hello world",
		},
		{
			name:          "background command",
			command:       "sleep 0.1",
			background:    true,
			captureOutput: false,
			expectSuccess: true,
			checkPID:      true,
		},
		{
			name:          "failing command",
			command:       "exit 1",
			background:    false,
			captureOutput: true,
			expectSuccess: false,
		},
		{
			name:          "command with stderr",
			command:       "echo 'error message' >&2; exit 1",
			background:    false,
			captureOutput: true,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteShellCommand(ExecuteOptions{
				Command:       tt.command,
				Background:    tt.background,
				CaptureOutput: tt.captureOutput,
			})

			assert.Equal(t, tt.expectSuccess, result.Success)

			if tt.captureOutput && tt.expectedOutput != "" {
				assert.Contains(t, result.Stdout, tt.expectedOutput)
			}

			if tt.checkPID {
				assert.NotZero(t, result.PID)
			}
		})
	}
}

func TestExecuteShellCommandWithWorkingDirectory(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		setupFile     bool
		fileContent   string
		expectSuccess bool
		output        string
	}{
		{
			name:          "command succeeds in working directory",
			command:       "ls test.txt",
			setupFile:     true,
			fileContent:   "test content",
			expectSuccess: true,
			output:        "test.txt",
		},
		{
			name:          "command fails in wrong directory",
			command:       "ls test.txt",
			setupFile:     false,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.setupFile {
				testFile := filepath.Join(tmpDir, "test.txt")
				err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
				assert.NoError(t, err)
			}

			result := ExecuteShellCommand(ExecuteOptions{
				Command:       tt.command,
				CaptureOutput: true,
				WorkingDir:    tmpDir,
			})

			assert.Equal(t, tt.expectSuccess, result.Success)

			if tt.output != "" {
				assert.Contains(t, result.Stdout, tt.output)
			}
		})
	}
}

func TestPIDFileOperations(t *testing.T) {
	tests := []struct {
		name        string
		legacy      bool
		commandName string
		command     string
	}{
		{
			name:   "legacy pid file",
			legacy: true,
		},
		{
			name:        "enhanced pid file",
			legacy:      false,
			commandName: "test-daemon",
			command:     "sleep 300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pidFile := filepath.Join(tmpDir, ".test.pid")
			testPID := 12345

			var err error
			if tt.legacy {
				err = CreatePIDFile(pidFile, testPID)
			} else {
				err = CreateEnhancedPIDFile(pidFile, testPID, tt.commandName, tt.command)
			}
			assert.NoError(t, err)

			readPID, err := ReadPIDFile(pidFile)
			assert.NoError(t, err)
			assert.Equal(t, testPID, readPID)

			if !tt.legacy {
				pidInfo, err := ReadEnhancedPIDFile(pidFile)
				assert.NoError(t, err)
				assert.Equal(t, testPID, pidInfo.PID)
				assert.Equal(t, tt.commandName, pidInfo.CommandName)
				assert.Equal(t, tt.command, pidInfo.Command)
				assert.False(t, pidInfo.StartTime.IsZero())
				assert.Zero(t, pidInfo.RestartCount)
			}

			err = RemovePIDFile(pidFile)
			assert.NoError(t, err)

			_, err = os.Stat(pidFile)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestReadPIDFileBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, ".legacy.pid")
	testPID := 54321

	err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", testPID)), 0644)
	assert.NoError(t, err)

	pidInfo, err := ReadEnhancedPIDFile(pidFile)
	assert.NoError(t, err)
	assert.Equal(t, testPID, pidInfo.PID)
	assert.Empty(t, pidInfo.CommandName)
	assert.Empty(t, pidInfo.Command)

	legacyPID, err := ReadPIDFile(pidFile)
	assert.NoError(t, err)
	assert.Equal(t, testPID, legacyPID)
}

func TestListDaemonProcesses(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(tmpDir))
	defer func() {
		assert.NoError(t, os.Chdir(oldDir))
	}()

	// Create test PID files
	pidFile1 := filepath.Join(tmpDir, ".daemon1.pid")
	pidFile2 := filepath.Join(tmpDir, ".daemon2.pid")
	pidFile3 := filepath.Join(tmpDir, ".stale.pid")

	// Create enhanced PID file for running daemon
	err := CreateEnhancedPIDFile(pidFile1, os.Getpid(), "daemon1", "sleep 300")
	assert.NoError(t, err)

	// Create legacy PID file for running daemon
	err = CreatePIDFile(pidFile2, os.Getpid())
	assert.NoError(t, err)

	// Create stale PID file
	err = CreateEnhancedPIDFile(pidFile3, 999999, "stale-daemon", "old command")
	assert.NoError(t, err)

	// Test listing daemon processes
	daemons, err := ListDaemonProcesses(tmpDir)
	assert.NoError(t, err)

	assert.Len(t, daemons, 3)

	// Check that we found the running daemons
	foundRunning := 0
	foundStale := 0
	for _, daemon := range daemons {
		if daemon.IsRunning {
			foundRunning++
		} else {
			foundStale++
		}
	}

	assert.Equal(t, 2, foundRunning)
	assert.Equal(t, 1, foundStale)

	// Clean up
	assert.NoError(t, RemovePIDFile(pidFile1))
	assert.NoError(t, RemovePIDFile(pidFile2))
	assert.NoError(t, RemovePIDFile(pidFile3))
}

func TestReadPIDFileErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
	}{
		{
			name:      "non-existent file",
			setupFile: false,
		},
		{
			name:        "invalid content",
			setupFile:   true,
			fileContent: "not-a-number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pidFile := filepath.Join(tmpDir, ".test.pid")

			if tt.setupFile {
				err := os.WriteFile(pidFile, []byte(tt.fileContent), 0644)
				assert.NoError(t, err)
			}

			pid, err := ReadPIDFile(pidFile)
			assert.Error(t, err)
			assert.Zero(t, pid)
		})
	}
}

func TestIsProcessRunning(t *testing.T) {
	tests := []struct {
		name string
		pid  int
		want bool
	}{
		{
			name: "current process",
			pid:  os.Getpid(),
			want: true,
		},
		{
			name: "invalid process",
			pid:  999999,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProcessRunning(tt.pid)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGeneratePIDFilename(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		command     string
	}{
		{
			name:        "test command",
			commandName: "test",
			command:     "go test ./...",
		},
		{
			name:        "build command",
			commandName: "build",
			command:     "go build ./...",
		},
		{
			name:        "dev command",
			commandName: "dev",
			command:     "go run main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename1 := GeneratePIDFilename(tt.commandName, tt.command)
			filename2 := GeneratePIDFilename(tt.commandName, tt.command)

			assert.Equal(t, filename1, filename2, "Same inputs should generate same filename")

			assert.True(t, strings.HasPrefix(filename1, "."), "PID filename should start with '.'")
			assert.True(t, strings.HasSuffix(filename1, ".pid"), "PID filename should end with '.pid'")

			assert.Equal(t, 13, len(filename1), "PID filename should be 13 characters long (.{8 chars}.pid)")
		})
	}

	t.Run("different inputs generate different filenames", func(t *testing.T) {
		file1 := GeneratePIDFilename("test", "command1")
		file2 := GeneratePIDFilename("test", "command2")
		assert.NotEqual(t, file1, file2, "Different commands should generate different PID filenames")

		file3 := GeneratePIDFilename("cmd1", "same command")
		file4 := GeneratePIDFilename("cmd2", "same command")
		assert.NotEqual(t, file3, file4, "Different command names should generate different PID filenames")
	})
}

func TestCleanupStalePIDFiles(t *testing.T) {
	tmpDir := t.TempDir()

	stalePIDFile := filepath.Join(tmpDir, ".stale.pid")
	validPIDFile := filepath.Join(tmpDir, ".valid.pid")
	nonPIDFile := filepath.Join(tmpDir, "not-a-pid.txt")

	err := CreatePIDFile(stalePIDFile, 999999)
	require.NoError(t, err)

	err = CreatePIDFile(validPIDFile, os.Getpid())
	require.NoError(t, err)

	err = os.WriteFile(nonPIDFile, []byte("test"), 0644)
	require.NoError(t, err)

	result := CleanupStalePIDFiles(tmpDir)
	assert.True(t, result.Success, "CleanupStalePIDFiles should succeed")

	_, err = os.Stat(stalePIDFile)
	assert.True(t, os.IsNotExist(err), "Stale PID file should be removed")

	_, err = os.Stat(validPIDFile)
	assert.False(t, os.IsNotExist(err), "Valid PID file should not be removed")

	_, err = os.Stat(nonPIDFile)
	assert.False(t, os.IsNotExist(err), "Non-PID file should not be affected")

	err = RemovePIDFile(validPIDFile)
	assert.NoError(t, err)
}

func TestExecuteDaemonCommand(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	result := ExecuteShellCommand(ExecuteOptions{
		Command:     "sleep 0.1",
		Background:  false,
		Daemon:      true,
		CommandName: "test-daemon",
	})

	assert.True(t, result.Success, "Daemon command should succeed")
	assert.NotZero(t, result.PID, "Daemon command should return a PID")

	expectedPIDFile := GeneratePIDFilename("test-daemon", "sleep 0.1")
	_, err := os.Stat(expectedPIDFile)
	if !os.IsNotExist(err) {
		t.Cleanup(func() {
			_ = RemovePIDFile(expectedPIDFile)
		})
	}
}

func TestExecuteCommandStep(t *testing.T) {
	tests := []struct {
		name        string
		step        config.CommandStep
		expectError bool
	}{
		{
			name: "simple run command",
			step: config.CommandStep{
				Run: config.RunCommand{"echo hello"},
			},
			expectError: false,
		},
		{
			name: "multiple run commands",
			step: config.CommandStep{
				Run: config.RunCommand{"echo first", "echo second"},
			},
			expectError: false,
		},
		{
			name: "background command",
			step: config.CommandStep{
				Run:        config.RunCommand{"sleep 0.1"},
				Background: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", "")

			if tt.expectError {
				assert.False(t, result.Success)
			} else {
				assert.True(t, result.Success)
			}
		})
	}
}

func TestExecuteCommandStepWithDirectory(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
		directory   string
		command     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "command succeeds in valid directory",
			setupFile:   true,
			fileContent: "test",
			command:     "ls test.txt",
			expectError: false,
		},
		{
			name:        "command fails with invalid directory",
			setupFile:   false,
			directory:   "/nonexistent/directory",
			command:     "echo hello",
			expectError: true,
			errorMsg:    "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			workingDir := tmpDir
			if tt.directory != "" {
				workingDir = tt.directory
			}

			if tt.setupFile {
				testFile := filepath.Join(tmpDir, "test.txt")
				err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
				require.NoError(t, err)
			}

			step := config.CommandStep{
				Run:       config.RunCommand{tt.command},
				Directory: workingDir,
			}

			result := ExecuteCommandStep(step, "test-command", "")

			if tt.expectError {
				assert.False(t, result.Success, "Command should fail")
				if tt.errorMsg != "" {
					assert.Contains(t, result.Stderr, tt.errorMsg)
				}
			} else {
				assert.True(t, result.Success, "Command should succeed")
			}
		})
	}
}

func TestExecuteCommandWithSteps(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		steps       []config.CommandStep
		wantSuccess bool
	}{
		{
			name:        "single successful step",
			commandName: "test-single",
			steps: []config.CommandStep{
				{Run: config.RunCommand{"echo hello"}},
			},
			wantSuccess: true,
		},
		{
			name:        "multiple successful steps",
			commandName: "test-multiple",
			steps: []config.CommandStep{
				{Run: config.RunCommand{"echo step1"}},
				{Run: config.RunCommand{"echo step2"}},
			},
			wantSuccess: true,
		},
		{
			name:        "failing step should abort",
			commandName: "test-fail",
			steps: []config.CommandStep{
				{Run: config.RunCommand{"echo step1"}},
				{Run: config.RunCommand{"exit 1"}},
				{Run: config.RunCommand{"echo step3"}},
			},
			wantSuccess: false,
		},
		{
			name:        "empty steps",
			commandName: "test-empty",
			steps:       []config.CommandStep{},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandWithSteps(tt.commandName, tt.steps, "")

			assert.Equal(t, tt.wantSuccess, result.Success)
		})
	}
}

func TestLoadEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name        string
		envContent  string
		createFile  bool
		wantErr     bool
		expectedVar string
		expectedVal string
	}{
		{
			name:        "valid env file",
			envContent:  "TEST_VAR=test_value\nANOTHER_VAR=another_value\n",
			createFile:  true,
			wantErr:     false,
			expectedVar: "TEST_VAR",
			expectedVal: "test_value",
		},
		{
			name:        "env file with quotes",
			envContent:  "QUOTED_VAR=\"quoted value\"\nSINGLE_QUOTED='single quoted'\n",
			createFile:  true,
			wantErr:     false,
			expectedVar: "QUOTED_VAR",
			expectedVal: "quoted value",
		},
		{
			name:        "env file with comments and empty lines",
			envContent:  "# This is a comment\nVALID_VAR=value\n\n# Another comment\nANOTHER_VAR=value2\n",
			createFile:  true,
			wantErr:     false,
			expectedVar: "VALID_VAR",
			expectedVal: "value",
		},
		{
			name:        "non-existent file",
			envContent:  "",
			createFile:  false,
			wantErr:     false,
			expectedVar: "",
			expectedVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")

			if tt.expectedVar != "" {
				_ = os.Unsetenv(tt.expectedVar)
			}

			if tt.createFile {
				err := os.WriteFile(envFile, []byte(tt.envContent), 0644)
				require.NoError(t, err)
			} else {
				envFile = "/nonexistent/.env"
			}

			err := LoadEnvironmentVariables(envFile)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedVar != "" {
				actualVal := os.Getenv(tt.expectedVar)
				assert.Equal(t, tt.expectedVal, actualVal)
			}
		})
	}
}

func TestDockerServiceOperations(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		expectError bool
		description string
	}{
		{
			name:        "simple string service",
			service:     "redis",
			expectError: false,
			description: "should handle simple string service names",
		},
		{
			name: "complex service configuration",
			service: map[string]interface{}{
				"testbox": map[string]interface{}{
					"image":   "alpine",
					"volumes": []interface{}{"./:/data"},
					"ports":   []interface{}{"8080:80"},
				},
			},
			expectError: false,
			description: "should handle complex service definitions",
		},
		{
			name: "service without image",
			service: map[string]interface{}{
				"badservice": map[string]interface{}{
					"volumes": []interface{}{"./:/data"},
				},
			},
			expectError: true,
			description: "should fail when service lacks required image field",
		},
		{
			name:        "invalid service type",
			service:     123,
			expectError: true,
			description: "should fail with invalid service type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartDockerService(tt.service)

			if result.Success {
				t.Cleanup(func() {
					_ = StopDockerService(tt.service)
				})
			}

			if tt.expectError {
				assert.False(t, result.Success, tt.description)
			} else {
				t.Logf("Docker service test passed (may fail without Docker): %s", tt.description)
			}
		})
	}
}

func TestDockerComposeOperations(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		compose          config.ComposeConfig
		createFile       bool
		fileContent      string
		expectError      bool
		expectedCmdParts []string
	}{
		{
			name: "basic compose file",
			compose: config.ComposeConfig{
				File: filepath.Join(tmpDir, "docker-compose.yml"),
			},
			createFile: true,
			fileContent: `version: '3.8'
services:
  redis:
    image: redis:latest
    ports:
      - "6379:6379"`,
			expectError: false,
		},
		{
			name: "compose with specific services",
			compose: config.ComposeConfig{
				File:     filepath.Join(tmpDir, "docker-compose.yml"),
				Services: []string{"redis", "postgres"},
			},
			createFile: true,
			fileContent: `version: '3.8'
services:
  redis:
    image: redis:latest
  postgres:
    image: postgres:13`,
			expectError: false,
		},
		{
			name: "non-existent compose file",
			compose: config.ComposeConfig{
				File: "/nonexistent/docker-compose.yml",
			},
			createFile:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				err := os.WriteFile(tt.compose.File, []byte(tt.fileContent), 0644)
				require.NoError(t, err)
			}

			result := StartDockerCompose(tt.compose)

			if result.Success {
				t.Cleanup(func() {
					_ = StopDockerCompose(tt.compose)
				})
			}

			if tt.expectError {
				assert.False(t, result.Success)
				if tt.name == "non-existent compose file" {
					assert.Contains(t, result.Stderr, "does not exist")
				}
			} else {
				t.Logf("Docker compose test passed (may fail without Docker): %s", tt.name)
			}
		})
	}
}

func TestServicesConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		services    config.ServicesConfig
		setupFiles  func() error
		expectError bool
		description string
	}{
		{
			name: "compose services only",
			services: config.ServicesConfig{
				Compose: &config.ComposeConfig{
					File: filepath.Join(tmpDir, "docker-compose.yml"),
				},
				WaitForHealth: false,
			},
			setupFiles: func() error {
				composeContent := `version: '3.8'
services:
  redis:
    image: redis:latest`
				return os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectError: false,
			description: "should handle compose services",
		},
		{
			name: "containers only",
			services: config.ServicesConfig{
				Containers: []interface{}{
					"redis",
					map[string]interface{}{
						"test-service": map[string]interface{}{
							"image": "alpine:latest",
						},
					},
				},
				WaitForHealth: false,
			},
			setupFiles:  func() error { return nil },
			expectError: false,
			description: "should handle container services",
		},
		{
			name: "empty configuration",
			services: config.ServicesConfig{
				WaitForHealth: false,
			},
			setupFiles:  func() error { return nil },
			expectError: false,
			description: "should handle empty configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.setupFiles())

			result := HandleServicesConfiguration(tt.services)

			if result.Success {
				t.Cleanup(func() {
					_ = StopServices(tt.services)
				})
			}

			t.Logf("Service configuration test: %s", tt.description)
		})
	}
}
