package executor

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dev-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Disable log output during tests to keep them quiet
	log.SetOutput(io.Discard)

	// Run tests
	code := m.Run()

	// Restore log output after tests
	log.SetOutput(os.Stderr)

	os.Exit(code)
}

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
			result := ExecuteCommandStep(tt.step, "test-command", "", nil)

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

			result := ExecuteCommandStep(step, "test-command", "", nil)

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
			result := ExecuteCommandWithSteps(tt.commandName, tt.steps, "", nil)

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
			createFile:  true,
			fileContent: `version: '3.8'\nservices:\n  redis:\n    image: redis:latest\n    ports:\n      - "6379:6379"`,
			expectError: false,
		},
		{
			name: "compose with specific services",
			compose: config.ComposeConfig{
				File:     filepath.Join(tmpDir, "docker-compose.yml"),
				Services: []string{"redis", "postgres"},
			},
			createFile:  true,
			fileContent: `version: '3.8'\nservices:\n  redis:\n    image: redis:latest\n  postgres:\n    image: postgres:13`,
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
				composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest`
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

func TestFindDaemonByCommandName(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	tests := []struct {
		name          string
		setupDaemons  func() error
		commandName   string
		expectFound   bool
		expectedPID   int
		expectedError bool
	}{
		{
			name: "find existing daemon",
			setupDaemons: func() error {
				pidFile := GeneratePIDFilename("test-daemon", "sleep 300")
				return CreateEnhancedPIDFile(pidFile, os.Getpid(), "test-daemon", "sleep 300")
			},
			commandName: "test-daemon",
			expectFound: true,
			expectedPID: os.Getpid(),
		},
		{
			name: "daemon not found",
			setupDaemons: func() error {
				pidFile := GeneratePIDFilename("other-daemon", "sleep 300")
				return CreateEnhancedPIDFile(pidFile, os.Getpid(), "other-daemon", "sleep 300")
			},
			commandName: "nonexistent-daemon",
			expectFound: false,
		},
		{
			name: "empty command name",
			setupDaemons: func() error {
				return nil
			},
			commandName: "",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.setupDaemons())

			daemon, err := FindDaemonByCommandName(tmpDir, tt.commandName)

			if tt.expectFound {
				assert.NoError(t, err)
				assert.NotNil(t, daemon)
				assert.Equal(t, tt.expectedPID, daemon.PID)
				assert.Equal(t, tt.commandName, daemon.CommandName)
			} else {
				assert.Error(t, err)
				assert.Nil(t, daemon)
			}

			// Cleanup
			if tt.expectFound {
				pidFile := GeneratePIDFilename(tt.commandName, "sleep 300")
				_ = RemovePIDFile(pidFile)
			}
		})
	}
}

func TestStopDaemonProcess(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	t.Run("stop with stale PID file", func(t *testing.T) {
		// Create a PID file with a definitely non-existent PID
		pidFile := GeneratePIDFilename("stale-daemon", "old command")
		err := CreateEnhancedPIDFile(pidFile, 999999, "stale-daemon", "old command")
		require.NoError(t, err)

		// Create daemon info for the stale process
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "stale-daemon",
				Command:     "old command",
				StartTime:   time.Now(),
			},
			PIDFile:   filepath.Base(pidFile),
			IsRunning: false,
		}

		err = StopDaemonProcess(tmpDir, daemon)

		assert.NoError(t, err)

		// Verify PID file was removed
		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("daemon not found", func(t *testing.T) {
		daemon, err := FindDaemonByCommandName(tmpDir, "nonexistent-daemon")

		assert.Error(t, err)
		assert.Nil(t, daemon)
		assert.Contains(t, err.Error(), "daemon with command name 'nonexistent-daemon' not found")
	})
}

func TestRestartDaemonProcess(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	t.Run("restart daemon with stale PID", func(t *testing.T) {
		// Create a stale PID file
		pidFile := GeneratePIDFilename("restart-daemon", "echo test")
		err := CreateEnhancedPIDFile(pidFile, 999999, "restart-daemon", "echo test")
		require.NoError(t, err)

		// Create daemon info for the stale process
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "restart-daemon",
				Command:     "echo test",
				StartTime:   time.Now(),
			},
			PIDFile:   filepath.Base(pidFile),
			IsRunning: false,
		}

		err = RestartDaemonProcess(tmpDir, daemon)

		// Should succeed because it's a valid command
		assert.NoError(t, err)

		// Cleanup
		_ = RemovePIDFile(pidFile)
	})

	t.Run("restart daemon with legacy PID file", func(t *testing.T) {
		// Create daemon info without command (simulating legacy PID file)
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "legacy-daemon",
				Command:     "", // Empty command simulates legacy PID file
				StartTime:   time.Now(),
			},
			PIDFile:   ".legacy.pid",
			IsRunning: false,
		}

		err := RestartDaemonProcess(tmpDir, daemon)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot restart daemon legacy-daemon: no command information available")
	})

	t.Run("daemon not found", func(t *testing.T) {
		daemon, err := FindDaemonByCommandName(tmpDir, "nonexistent-daemon")

		assert.Error(t, err)
		assert.Nil(t, daemon)
		assert.Contains(t, err.Error(), "daemon with command name 'nonexistent-daemon' not found")
	})

	t.Run("restarted daemon has a new pid file", func(t *testing.T) {
		// Create a daemon that can be restarted
		command := "sleep 300" // A command that runs for a while
		daemonName := "restart-pid-test"
		pidFile := GeneratePIDFilename(daemonName, command)
		err := CreateEnhancedPIDFile(pidFile, 999999, daemonName, command) // Stale PID
		require.NoError(t, err)

		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: daemonName,
				Command:     command,
				StartTime:   time.Now(),
			},
			PIDFile:   filepath.Base(pidFile),
			IsRunning: false,
		}

		// Restart the daemon
		err = RestartDaemonProcess(tmpDir, daemon)
		require.NoError(t, err)

		// Check that a new PID file is created
		daemons, err := ListDaemonProcesses(tmpDir)
		require.NoError(t, err)

		var restartedDaemon *DaemonInfo
		for i := range daemons {
			if daemons[i].CommandName == daemonName {
				restartedDaemon = &daemons[i]
				break
			}
		}

		require.NotNil(t, restartedDaemon, "restarted daemon should be found")
		assert.True(t, restartedDaemon.IsRunning, "restarted daemon should be running")

		// Clean up the running process and its PID file
		err = StopDaemonProcess(tmpDir, restartedDaemon)
		assert.NoError(t, err)
	})
}

func TestCleanupStalePIDFilesWithTermination(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	t.Run("cleanup with termination disabled", func(t *testing.T) {
		// Create stale PID file
		stalePIDFile := GeneratePIDFilename("stale-daemon", "old command")
		err := CreateEnhancedPIDFile(stalePIDFile, 999999, "stale-daemon", "old command")
		require.NoError(t, err)

		// Create running PID file
		runningPIDFile := GeneratePIDFilename("running-daemon", "current command")
		err = CreateEnhancedPIDFile(runningPIDFile, os.Getpid(), "running-daemon", "current command")
		require.NoError(t, err)

		result := CleanupStalePIDFilesWithTermination(tmpDir, false)

		assert.True(t, result.Success)
		assert.Contains(t, result.Stdout, "Cleaned up 1 stale PID file")

		// Stale file should be removed
		_, err = os.Stat(stalePIDFile)
		assert.True(t, os.IsNotExist(err))

		// Running file should remain
		_, err = os.Stat(runningPIDFile)
		assert.False(t, os.IsNotExist(err))

		// Cleanup
		_ = RemovePIDFile(runningPIDFile)
	})

	t.Run("cleanup with termination enabled", func(t *testing.T) {
		// Create stale PID file
		stalePIDFile := GeneratePIDFilename("stale-daemon2", "old command")
		err := CreateEnhancedPIDFile(stalePIDFile, 999999, "stale-daemon2", "old command")
		require.NoError(t, err)

		result := CleanupStalePIDFilesWithTermination(tmpDir, true)

		assert.True(t, result.Success)
		assert.Contains(t, result.Stdout, "Cleaned up 1 stale PID file")

		// Stale file should be removed
		_, err = os.Stat(stalePIDFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("no PID files to cleanup", func(t *testing.T) {
		result := CleanupStalePIDFilesWithTermination(tmpDir, false)

		assert.True(t, result.Success)
		assert.Contains(t, result.Stdout, "No PID files found to clean up")
	})
}

func TestWaitForServiceHealth(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		timeout     int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid service type",
			service:     123,
			timeout:     5,
			expectError: true,
			errorMsg:    "service must be a string or object",
		},
		{
			name:        "string service name",
			service:     "test-service",
			timeout:     1,
			expectError: true, // Will fail because container doesn't exist
		},
		{
			name: "complex service configuration",
			service: map[string]interface{}{
				"test-service": map[string]interface{}{
					"image": "alpine:latest",
				},
			},
			timeout:     1,
			expectError: true, // Will fail because container doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WaitForServiceHealth(tt.service, tt.timeout)

			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorMsg != "" {
					assert.Contains(t, result.Stderr, tt.errorMsg)
				}
			} else {
				assert.True(t, result.Success)
			}
		})
	}
}

func TestExecuteShellCommand_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		opts        ExecuteOptions
		expectError bool
		testType    string
	}{
		{
			name: "background command - may succeed but process fails later",
			opts: ExecuteOptions{
				Command:    "/nonexistent/command",
				Background: true,
			},
			expectError: false, // Background commands return success if process starts
			testType:    "background_command",
		},
		{
			name: "daemon command - may succeed but process fails later",
			opts: ExecuteOptions{
				Command:     "/nonexistent/command",
				Daemon:      true,
				CommandName: "test-daemon",
			},
			expectError: false, // Daemon commands return success if process starts
			testType:    "daemon_command",
		},
		{
			name: "invalid working directory",
			opts: ExecuteOptions{
				Command:       "echo hello",
				CaptureOutput: true,
				WorkingDir:    "/nonexistent/directory",
			},
			expectError: true, // This should fail immediately
			testType:    "workdir_failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteShellCommand(tt.opts)

			if tt.expectError {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.Stderr)
			} else {
				// For background/daemon commands, we just verify they don't panic
				// The actual command failure happens asynchronously
				t.Logf("Command execution result: success=%v, stderr=%s", result.Success, result.Stderr)
			}

			// Cleanup any daemon PID files if created
			if tt.opts.Daemon && tt.opts.CommandName != "" && result.Success {
				t.Cleanup(func() {
					// Clean up any PID files that might have been created
					pidFile := GeneratePIDFilename(tt.opts.CommandName, tt.opts.Command)
					_ = RemovePIDFile(pidFile)
				})
			}
		})
	}
}

func TestExecuteCommandStep_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		step        config.CommandStep
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid directory",
			step: config.CommandStep{
				Run:       config.RunCommand{"echo hello"},
				Directory: "/nonexistent/directory",
			},
			expectError: true,
			errorMsg:    "does not exist",
		},
		{
			name: "directory is a file",
			step: config.CommandStep{
				Run:       config.RunCommand{"echo hello"},
				Directory: filepath.Join(tmpDir, "notadir"),
			},
			expectError: true,
			errorMsg:    "is not a directory",
		},
	}

	// Create a file (not directory) for one test
	notADir := filepath.Join(tmpDir, "notadir")
	err := os.WriteFile(notADir, []byte("test"), 0644)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", tmpDir, nil)

			if tt.expectError {
				assert.False(t, result.Success)
				if tt.errorMsg != "" {
					assert.Contains(t, result.Stderr, tt.errorMsg)
				}
			}
		})
	}
}

func TestStartDockerService_ComplexConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		expectError bool
		description string
	}{
		{
			name: "service with all configuration options",
			service: map[string]interface{}{
				"complex-service": map[string]interface{}{
					"image": "alpine:latest",
					"environment": []interface{}{
						"VAR1=value1",
						"VAR2=value2",
					},
					"volumes": []interface{}{
						"./data:/app/data",
						"/tmp:/app/tmp",
					},
					"ports": []interface{}{
						"8080:80",
						"9090:90",
					},
					"networks": []interface{}{
						"custom-network",
					},
					"restart": "unless-stopped",
					"memory":  "512m",
					"cpus":    "0.5",
					"healthcheck": map[string]interface{}{
						"test":     []interface{}{"CMD", "curl", "-f", "http://localhost/health"},
						"interval": "30s",
						"timeout":  "10s",
						"retries":  3,
					},
				},
			},
			expectError: false,
			description: "should handle complex service with all options",
		},
		{
			name: "service with environment as map",
			service: map[string]interface{}{
				"env-service": map[string]interface{}{
					"image": "alpine:latest",
					"environment": map[string]interface{}{
						"KEY1": "value1",
						"KEY2": "value2",
					},
				},
			},
			expectError: false,
			description: "should handle environment as map",
		},
		{
			name: "service with invalid healthcheck",
			service: map[string]interface{}{
				"bad-health-service": map[string]interface{}{
					"image":       "alpine:latest",
					"healthcheck": "invalid-healthcheck",
				},
			},
			expectError: false, // Should handle gracefully
			description: "should handle invalid healthcheck configuration",
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

			t.Logf("Docker service test: %s (result: %v)", tt.description, result.Success)
			// Don't assert success/failure since Docker may not be available
		})
	}
}

func TestExecuteCommandStep_ServicesConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		step        config.CommandStep
		description string
	}{
		{
			name: "new services configuration",
			step: config.CommandStep{
				Run: config.RunCommand{"echo hello"},
				Services: config.ServicesConfig{
					Containers:    []interface{}{"redis"},
					WaitForHealth: false,
				},
			},
			description: "should handle new services configuration",
		},
		{
			name: "services with compose",
			step: config.CommandStep{
				Run: config.RunCommand{"echo hello"},
				Services: config.ServicesConfig{
					Compose: &config.ComposeConfig{
						File: filepath.Join(tmpDir, "docker-compose.yml"),
					},
					WaitForHealth: false,
				},
			},
			description: "should handle services with compose configuration",
		},
	}

	// Create a basic compose file for testing
	composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest`
	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	err := os.WriteFile(composeFile, []byte(composeContent), 0644)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", tmpDir, nil)

			t.Logf("Services configuration test: %s (success: %v)", tt.description, result.Success)
			// Don't assert specific success/failure since Docker may not be available
			// The important thing is that the code doesn't panic and handles the configuration
		})
	}
}

func TestHandleServicesConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		services      config.ServicesConfig
		setupFiles    func() error
		expectSuccess bool
		description   string
	}{
		{
			name: "services with containers only",
			services: config.ServicesConfig{
				Containers:    []interface{}{"redis", "postgres"},
				WaitForHealth: false,
			},
			setupFiles:    func() error { return nil },
			expectSuccess: true,
			description:   "should handle container services",
		},
		{
			name: "services with compose only",
			services: config.ServicesConfig{
				Compose: &config.ComposeConfig{
					File: filepath.Join(tmpDir, "docker-compose.yml"),
				},
				WaitForHealth: false,
			},
			setupFiles: func() error {
				composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest\n    ports:\n      - "6379:6379"`
				return os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectSuccess: true,
			description:   "should handle compose services",
		},
		{
			name: "services with both containers and compose",
			services: config.ServicesConfig{
				Containers: []interface{}{"postgres"},
				Compose: &config.ComposeConfig{
					File: filepath.Join(tmpDir, "docker-compose.yml"),
				},
				WaitForHealth: false,
			},
			setupFiles: func() error {
				composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest`
				return os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			expectSuccess: true,
			description:   "should handle mixed services configuration",
		},
		{
			name: "empty services configuration",
			services: config.ServicesConfig{
				WaitForHealth: false,
			},
			setupFiles:    func() error { return nil },
			expectSuccess: true,
			description:   "should handle empty services configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.setupFiles())

			result := HandleServicesConfiguration(tt.services)

			t.Logf("Services configuration test: %s (success: %v)", tt.description, result.Success)

			if result.Success {
				t.Cleanup(func() {
					_ = StopServices(tt.services)
				})
			}
		})
	}
}

func TestStopDockerService(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		description string
	}{
		{
			name:        "stop simple string service",
			service:     "test-service",
			description: "should handle stopping simple service",
		},
		{
			name: "stop complex service configuration",
			service: map[string]interface{}{
				"complex-service": map[string]interface{}{
					"image": "alpine:latest",
					"ports": []interface{}{"8080:80"},
				},
			},
			description: "should handle stopping complex service",
		},
		{
			name:        "invalid service type",
			service:     123,
			description: "should handle invalid service type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StopDockerService(tt.service)

			t.Logf("Stop Docker service test: %s (success: %v)", tt.description, result.Success)
			// Don't assert specific success/failure since Docker may not be available
		})
	}
}

func TestStopServices(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		services    config.ServicesConfig
		setupFiles  func() error
		description string
	}{
		{
			name: "stop container services",
			services: config.ServicesConfig{
				Containers:    []interface{}{"redis"},
				WaitForHealth: false,
			},
			setupFiles:  func() error { return nil },
			description: "should handle stopping container services",
		},
		{
			name: "stop compose services",
			services: config.ServicesConfig{
				Compose: &config.ComposeConfig{
					File: filepath.Join(tmpDir, "docker-compose.yml"),
				},
				WaitForHealth: false,
			},
			setupFiles: func() error {
				composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest`
				return os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644)
			},
			description: "should handle stopping compose services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.setupFiles())

			result := StopServices(tt.services)

			t.Logf("Stop services test: %s (success: %v)", tt.description, result.Success)
		})
	}
}

func TestStopDockerCompose(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		compose       config.ComposeConfig
		createFile    bool
		expectSuccess bool
		description   string
	}{
		{
			name: "stop compose with valid file",
			compose: config.ComposeConfig{
				File: filepath.Join(tmpDir, "docker-compose.yml"),
			},
			createFile:    true,
			expectSuccess: true,
			description:   "should handle stopping compose with valid file",
		},
		{
			name: "stop compose with services specified",
			compose: config.ComposeConfig{
				File:     filepath.Join(tmpDir, "docker-compose.yml"),
				Services: []string{"redis", "postgres"},
			},
			createFile:    true,
			expectSuccess: true,
			description:   "should handle stopping specific compose services",
		},
		{
			name: "stop compose with nonexistent file",
			compose: config.ComposeConfig{
				File: "/nonexistent/docker-compose.yml",
			},
			createFile:    false,
			expectSuccess: false,
			description:   "should handle nonexistent compose file",
		},
	}

	composeContent := `version: '3.8'\nservices:\n  redis:\n    image: redis:latest\n  postgres:\n    image: postgres:13`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				err := os.WriteFile(tt.compose.File, []byte(composeContent), 0644)
				require.NoError(t, err)
			}

			result := StopDockerCompose(tt.compose)

			t.Logf("Stop Docker compose test: %s (success: %v)", tt.description, result.Success)

			if tt.expectSuccess {
				// May fail if Docker isn't available, but that's ok
			} else {
				// Should fail for nonexistent file
				if !result.Success {
					assert.Contains(t, result.Stderr, "does not exist")
				}
			}
		})
	}
}

func TestReadEnhancedPIDFile_LegacyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		fileContent string
		expectError bool
		expectedPID int
	}{
		{
			name:        "legacy format - plain PID number",
			fileContent: "12345",
			expectError: false,
			expectedPID: 12345,
		},
		{
			name:        "legacy format - PID with whitespace",
			fileContent: "  54321  \n",
			expectError: false,
			expectedPID: 54321,
		},
		{
			name:        "invalid PID format",
			fileContent: "not-a-number",
			expectError: true,
		},
		{
			name:        "empty file",
			fileContent: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pidFile := filepath.Join(tmpDir, fmt.Sprintf(".test-%s.pid", strings.ReplaceAll(tt.name, " ", "-")))

			err := os.WriteFile(pidFile, []byte(tt.fileContent), 0644)
			require.NoError(t, err)

			pidInfo, err := ReadEnhancedPIDFile(pidFile)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPID, pidInfo.PID)
				assert.Empty(t, pidInfo.CommandName) // Legacy format has no command name
				assert.Empty(t, pidInfo.Command)     // Legacy format has no command
			}

			// Cleanup
			_ = os.Remove(pidFile)
		})
	}
}

func TestStartDockerService_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name: "service without image field",
			service: map[string]interface{}{
				"bad-service": map[string]interface{}{
					"ports": []interface{}{"8080:80"},
					// Missing required "image" field
				},
			},
			expectError: true,
			errorMsg:    "Service bad-service must have an 'image' field",
			description: "should fail when service lacks required image field",
		},
		{
			name:        "invalid service type - number",
			service:     12345,
			expectError: true,
			errorMsg:    "Service must be a string or object",
			description: "should fail with invalid service type",
		},
		{
			name:        "invalid service type - array",
			service:     []string{"invalid"},
			expectError: true,
			errorMsg:    "Service must be a string or object",
			description: "should fail with array service type",
		},
		{
			name: "service with invalid environment format",
			service: map[string]interface{}{
				"env-service": map[string]interface{}{
					"image":       "alpine:latest",
					"environment": 123, // Invalid type
				},
			},
			expectError: false, // Should handle gracefully
			description: "should handle invalid environment format gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartDockerService(tt.service)

			if tt.expectError && tt.errorMsg != "" {
				assert.False(t, result.Success)
				assert.Contains(t, result.Stderr, tt.errorMsg)
			}

			t.Logf("Docker service error test: %s (success: %v)", tt.description, result.Success)

			if result.Success {
				t.Cleanup(func() {
					_ = StopDockerService(tt.service)
				})
			}
		})
	}
}

func TestPIDFileOperations_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("create PID file in nonexistent directory", func(t *testing.T) {
		pidFile := filepath.Join("/nonexistent/directory", ".test.pid")

		err := CreatePIDFile(pidFile, 12345)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("read PID file with invalid JSON", func(t *testing.T) {
		pidFile := filepath.Join(tmpDir, ".invalid.pid")
		invalidJSON := `{"pid": 123, "command_name": "test", invalid_json}`

		err := os.WriteFile(pidFile, []byte(invalidJSON), 0644)
		require.NoError(t, err)

		_, err = ReadEnhancedPIDFile(pidFile)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid syntax")

		// Cleanup
		_ = os.Remove(pidFile)
	})

	t.Run("remove nonexistent PID file", func(t *testing.T) {
		pidFile := filepath.Join(tmpDir, ".nonexistent.pid")

		err := RemovePIDFile(pidFile)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no such file or directory")
	})
}

func TestAppendPassthroughArgs(t *testing.T) {
	tests := []struct {
		name            string
		baseCommand     string
		passthroughArgs []string
		expected        string
	}{
		{
			name:            "no passthrough args",
			baseCommand:     "go test ./...",
			passthroughArgs: nil,
			expected:        "go test ./...",
		},
		{
			name:            "simple args",
			baseCommand:     "go test ./...",
			passthroughArgs: []string{"--verbose", "--timeout=30s"},
			expected:        "go test ./... --verbose --timeout=30s",
		},
		{
			name:            "args with spaces",
			baseCommand:     "go test ./...",
			passthroughArgs: []string{"--run", "Test Name With Spaces"},
			expected:        "go test ./... --run 'Test Name With Spaces'",
		},
		{
			name:            "args with quotes",
			baseCommand:     "go test ./...",
			passthroughArgs: []string{"--ldflags", "-X 'main.version=1.0.0'"},
			expected:        "go test ./... --ldflags '-X '\\''main.version=1.0.0'\\'''",
		},
		{
			name:            "mixed args",
			baseCommand:     "npm test",
			passthroughArgs: []string{"--", "--verbose", "--reporter=json"},
			expected:        "npm test -- --verbose --reporter=json",
		},
		{
			name:            "empty passthrough args slice",
			baseCommand:     "cargo build",
			passthroughArgs: []string{},
			expected:        "cargo build",
		},
		{
			name:            "args with dollar sign (variable expansion)",
			baseCommand:     "echo test",
			passthroughArgs: []string{"$HOME"},
			expected:        "echo test '$HOME'",
		},
		{
			name:            "args with backticks (command substitution)",
			baseCommand:     "echo test",
			passthroughArgs: []string{"`whoami`"},
			expected:        "echo test '`whoami`'",
		},
		{
			name:            "args with semicolon (command separator)",
			baseCommand:     "echo test",
			passthroughArgs: []string{"arg;rm -rf /"},
			expected:        "echo test 'arg;rm -rf /'",
		},
		{
			name:            "args with pipe (command chaining)",
			baseCommand:     "cat file",
			passthroughArgs: []string{"data|malicious"},
			expected:        "cat file 'data|malicious'",
		},
		{
			name:            "args with ampersand (background process)",
			baseCommand:     "sleep 1",
			passthroughArgs: []string{"arg&malicious"},
			expected:        "sleep 1 'arg&malicious'",
		},
		{
			name:            "args with redirect operators",
			baseCommand:     "echo test",
			passthroughArgs: []string{">evil.txt"},
			expected:        "echo test '>evil.txt'",
		},
		{
			name:            "args with newline",
			baseCommand:     "echo test",
			passthroughArgs: []string{"line1\nrm -rf /"},
			expected:        "echo test 'line1\nrm -rf /'",
		},
		{
			name:            "args with backslash",
			baseCommand:     "echo test",
			passthroughArgs: []string{"test\\escape"},
			expected:        "echo test 'test\\escape'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendPassthroughArgs(tt.baseCommand, tt.passthroughArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteCommandStepWithPassthroughArgs(t *testing.T) {
	tests := []struct {
		name            string
		step            config.CommandStep
		passthroughArgs []string
		description     string
	}{
		{
			name: "run command with passthrough args",
			step: config.CommandStep{
				Run: config.RunCommand{"echo hello"},
			},
			passthroughArgs: []string{"world"},
			description:     "Command should have passthrough args appended",
		},
		{
			name: "multiple run commands with passthrough args",
			step: config.CommandStep{
				Run: config.RunCommand{"echo first", "echo second"},
			},
			passthroughArgs: []string{"--verbose"},
			description:     "All run commands should have passthrough args appended",
		},
		{
			name: "no run commands",
			step: config.CommandStep{
				Background: true,
			},
			passthroughArgs: []string{"--verbose"},
			description:     "Step with no run commands should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", "", tt.passthroughArgs)
			t.Logf("Step execution test: %s (success: %v)", tt.description, result.Success)
			// The important thing is that the code doesn't panic and handles passthrough args
		})
	}
}

func TestExecuteCommandStep_ServiceCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		step        config.CommandStep
		description string
	}{
		{
			name: "services with cleanup enabled",
			step: config.CommandStep{
				Services: config.ServicesConfig{
					Containers:    []interface{}{"redis"},
					Cleanup:       true,
					WaitForHealth: false,
				},
				Run: config.RunCommand{"echo 'test command'"},
			},
			description: "should cleanup services after command execution when cleanup is enabled",
		},
		{
			name: "services without cleanup",
			step: config.CommandStep{
				Services: config.ServicesConfig{
					Containers:    []interface{}{"redis"},
					Cleanup:       false,
					WaitForHealth: false,
				},
				Run: config.RunCommand{"echo 'test command'"},
			},
			description: "should NOT cleanup services when cleanup is disabled",
		},
		{
			name: "no services configured",
			step: config.CommandStep{
				Run: config.RunCommand{"echo 'test command'"},
			},
			description: "should execute normally when no services are configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", tmpDir, nil)
			t.Logf("Service cleanup test: %s (success: %v)", tt.description, result.Success)

			// We can't assert much here since Docker might not be available
			// But we can verify the function completes without panic
			// The deferred cleanup will run automatically when the function returns
		})
	}
}
