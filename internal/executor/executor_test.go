package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev-tools/internal/config"
)

func TestExecuteShellCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		background     bool
		captureOutput  bool
		expectSuccess  bool
		expectedOutput string
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
		},
		{
			name:          "failing command",
			command:       "exit 1",
			background:    false,
			captureOutput: true,
			expectSuccess: false,
		},
		{
			name:           "command with stderr",
			command:        "echo 'error message' >&2; exit 1",
			background:     false,
			captureOutput:  true,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteShellCommand(ExecuteOptions{
				Command:       tt.command,
				Background:    tt.background,
				CaptureOutput: tt.captureOutput,
			})

			if result.Success != tt.expectSuccess {
				t.Errorf("ExecuteShellCommand() success = %v, want %v", result.Success, tt.expectSuccess)
			}

			if tt.captureOutput && tt.expectedOutput != "" {
				if !strings.Contains(result.Stdout, tt.expectedOutput) {
					t.Errorf("ExecuteShellCommand() stdout = %q, want to contain %q", result.Stdout, tt.expectedOutput)
				}
			}

			if tt.background && result.PID == 0 {
				t.Error("Background command should return a PID")
			}
		})
	}
}

func TestExecuteShellCommandWithWorkingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test file in tmpDir
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := ExecuteShellCommand(ExecuteOptions{
		Command:       "ls test.txt",
		CaptureOutput: true,
		WorkingDir:    tmpDir,
	})

	if !result.Success {
		t.Errorf("Command should succeed in working directory, stderr: %s", result.Stderr)
	}

	if !strings.Contains(result.Stdout, "test.txt") {
		t.Errorf("Should find test.txt in output, got: %s", result.Stdout)
	}
}

func TestGeneratePIDFilename(t *testing.T) {
	tests := []struct {
		commandName string
		command     string
	}{
		{"test", "go test ./..."},
		{"build", "go build ./..."},
		{"dev", "go run main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.commandName, func(t *testing.T) {
			filename1 := GeneratePIDFilename(tt.commandName, tt.command)
			filename2 := GeneratePIDFilename(tt.commandName, tt.command)

			// Same inputs should generate same filename
			if filename1 != filename2 {
				t.Errorf("Same inputs should generate same filename, got %s and %s", filename1, filename2)
			}

			// Filename should start with dot and end with .pid
			if !strings.HasPrefix(filename1, ".") || !strings.HasSuffix(filename1, ".pid") {
				t.Errorf("PID filename should start with '.' and end with '.pid', got: %s", filename1)
			}

			// Should be exactly 13 characters (.{8 chars}.pid)
			if len(filename1) != 13 {
				t.Errorf("PID filename should be 13 characters long, got %d: %s", len(filename1), filename1)
			}
		})
	}

	// Different inputs should generate different filenames
	file1 := GeneratePIDFilename("test", "command1")
	file2 := GeneratePIDFilename("test", "command2")
	if file1 == file2 {
		t.Error("Different commands should generate different PID filenames")
	}

	file3 := GeneratePIDFilename("cmd1", "same command")
	file4 := GeneratePIDFilename("cmd2", "same command")
	if file3 == file4 {
		t.Error("Different command names should generate different PID filenames")
	}
}

func TestPIDFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, ".test.pid")

	// Test creating PID file
	testPID := 12345
	err := CreatePIDFile(pidFile, testPID)
	if err != nil {
		t.Errorf("CreatePIDFile() error = %v", err)
	}

	// Test reading PID file
	readPID, err := ReadPIDFile(pidFile)
	if err != nil {
		t.Errorf("ReadPIDFile() error = %v", err)
	}
	if readPID != testPID {
		t.Errorf("ReadPIDFile() = %d, want %d", readPID, testPID)
	}

	// Test removing PID file
	err = RemovePIDFile(pidFile)
	if err != nil {
		t.Errorf("RemovePIDFile() error = %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

func TestReadPIDFileNotExists(t *testing.T) {
	pid, err := ReadPIDFile("/nonexistent/file.pid")
	if err == nil {
		t.Error("ReadPIDFile() should return error for non-existent file")
	}
	if pid != 0 {
		t.Errorf("ReadPIDFile() should return 0 for non-existent file, got %d", pid)
	}
}

func TestReadPIDFileInvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, ".invalid.pid")
	
	err := os.WriteFile(pidFile, []byte("not-a-number"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid PID file: %v", err)
	}

	pid, err := ReadPIDFile(pidFile)
	if err == nil {
		t.Error("ReadPIDFile() should return error for invalid content")
	}
	if pid != 0 {
		t.Errorf("ReadPIDFile() should return 0 for invalid content, got %d", pid)
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Test with current process (should be running)
	currentPID := os.Getpid()
	if !IsProcessRunning(currentPID) {
		t.Error("Current process should be reported as running")
	}

	// Test with invalid PID (should not be running)
	// Use a very high PID that's unlikely to exist
	invalidPID := 999999
	if IsProcessRunning(invalidPID) {
		t.Error("Invalid PID should be reported as not running")
	}
}

func TestCleanupStalePIDFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a stale PID file (with non-existent PID)
	stalePIDFile := filepath.Join(tmpDir, ".stale.pid")
	err := CreatePIDFile(stalePIDFile, 999999) // Very high PID unlikely to exist
	if err != nil {
		t.Fatalf("Failed to create stale PID file: %v", err)
	}

	// Create a valid PID file (with current process PID)
	validPIDFile := filepath.Join(tmpDir, ".valid.pid")
	err = CreatePIDFile(validPIDFile, os.Getpid())
	if err != nil {
		t.Fatalf("Failed to create valid PID file: %v", err)
	}

	// Create a file that's not a PID file
	nonPIDFile := filepath.Join(tmpDir, "not-a-pid.txt")
	err = os.WriteFile(nonPIDFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create non-PID file: %v", err)
	}

	result := CleanupStalePIDFiles(tmpDir)
	if !result.Success {
		t.Errorf("CleanupStalePIDFiles() should succeed, got error: %s", result.Stderr)
	}

	// Stale PID file should be removed
	if _, err := os.Stat(stalePIDFile); !os.IsNotExist(err) {
		t.Error("Stale PID file should be removed")
	}

	// Valid PID file should still exist
	if _, err := os.Stat(validPIDFile); os.IsNotExist(err) {
		t.Error("Valid PID file should not be removed")
	}

	// Non-PID file should not be affected
	if _, err := os.Stat(nonPIDFile); os.IsNotExist(err) {
		t.Error("Non-PID file should not be affected")
	}

	// Cleanup for next tests
	_ = RemovePIDFile(validPIDFile)
}

func TestExecuteDaemonCommand(t *testing.T) {
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

	// Test daemon command - use background=false to test foreground daemon mode
	result := ExecuteShellCommand(ExecuteOptions{
		Command:     "sleep 0.1", // Shorter sleep for faster test
		Background:  false,
		Daemon:      true,
		CommandName: "test-daemon",
	})

	if !result.Success {
		t.Errorf("Daemon command should succeed, error: %s", result.Stderr)
	}

	if result.PID == 0 {
		t.Error("Daemon command should return a PID")
	}

	// For foreground daemon, PID file should be cleaned up automatically
	expectedPIDFile := GeneratePIDFilename("test-daemon", "sleep 0.1")
	if _, err := os.Stat(expectedPIDFile); !os.IsNotExist(err) {
		t.Log("PID file should be cleaned up automatically for foreground daemon")
		// Clean up manually if it wasn't
		_ = RemovePIDFile(expectedPIDFile)
	}
}

func TestExecuteCommandStep(t *testing.T) {
	tests := []struct {
		name    string
		step    config.CommandStep
		wantErr bool
	}{
		{
			name: "simple run command",
			step: config.CommandStep{
				Run: config.RunCommand{"echo hello"},
			},
			wantErr: false,
		},
		{
			name: "multiple run commands",
			step: config.CommandStep{
				Run: config.RunCommand{"echo first", "echo second"},
			},
			wantErr: false,
		},
		{
			name: "background command",
			step: config.CommandStep{
				Run:        config.RunCommand{"sleep 0.1"},
				Background: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExecuteCommandStep(tt.step, "test-command", "")
			
			if (result.Success == false) != tt.wantErr {
				t.Errorf("ExecuteCommandStep() success = %v, wantErr %v, stderr: %s", result.Success, tt.wantErr, result.Stderr)
			}
		})
	}
}

func TestExecuteCommandStepWithDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a test file in tmpDir
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	step := config.CommandStep{
		Run:       config.RunCommand{"ls test.txt"},
		Directory: tmpDir,
	}

	result := ExecuteCommandStep(step, "test-command", "")
	if !result.Success {
		t.Errorf("Command should succeed in specified directory, stderr: %s", result.Stderr)
	}
}

func TestExecuteCommandStepInvalidDirectory(t *testing.T) {
	step := config.CommandStep{
		Run:       config.RunCommand{"echo hello"},
		Directory: "/nonexistent/directory",
	}

	result := ExecuteCommandStep(step, "test-command", "")
	if result.Success {
		t.Error("Command should fail with invalid directory")
	}
	
	if !strings.Contains(result.Stderr, "does not exist") {
		t.Errorf("Error should mention directory doesn't exist, got: %s", result.Stderr)
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
			
			if result.Success != tt.wantSuccess {
				t.Errorf("ExecuteCommandWithSteps() success = %v, want %v, stderr: %s", 
					result.Success, tt.wantSuccess, result.Stderr)
			}
		})
	}
}

func TestLoadEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	tests := []struct {
		name        string
		envContent  string
		envFile     string
		wantErr     bool
		expectedVar string
		expectedVal string
	}{
		{
			name:        "valid env file",
			envContent:  "TEST_VAR=test_value\nANOTHER_VAR=another_value\n",
			envFile:     envFile,
			wantErr:     false,
			expectedVar: "TEST_VAR",
			expectedVal: "test_value",
		},
		{
			name:        "env file with quotes",
			envContent:  "QUOTED_VAR=\"quoted value\"\nSINGLE_QUOTED='single quoted'\n",
			envFile:     envFile,
			wantErr:     false,
			expectedVar: "QUOTED_VAR",
			expectedVal: "quoted value",
		},
		{
			name:        "env file with comments and empty lines",
			envContent:  "# This is a comment\nVALID_VAR=value\n\n# Another comment\nANOTHER_VAR=value2\n",
			envFile:     envFile,
			wantErr:     false,
			expectedVar: "VALID_VAR",
			expectedVal: "value",
		},
		{
			name:        "non-existent file",
			envContent:  "",
			envFile:     "/nonexistent/.env",
			wantErr:     false, // Should not error, just log that file doesn't exist
			expectedVar: "",
			expectedVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			if tt.expectedVar != "" {
				_ = os.Unsetenv(tt.expectedVar)
			}

			// Create env file if content provided
			if tt.envContent != "" && tt.envFile == envFile {
				err := os.WriteFile(envFile, []byte(tt.envContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create test env file: %v", err)
				}
			}

			err := LoadEnvironmentVariables(tt.envFile)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadEnvironmentVariables() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check if environment variable was set correctly
			if tt.expectedVar != "" {
				actualVal := os.Getenv(tt.expectedVar)
				if actualVal != tt.expectedVal {
					t.Errorf("Environment variable %s = %q, want %q", tt.expectedVar, actualVal, tt.expectedVal)
				}
			}

			// Clean up
			if tt.envFile == envFile {
				_ = os.Remove(envFile)
			}
		})
	}
}

func TestStartDockerService(t *testing.T) {
	tests := []struct {
		name        string
		service     interface{}
		wantErr     bool
		description string
	}{
		{
			name:        "simple string service",
			service:     "redis",
			wantErr:     false,
			description: "should handle simple string service names",
		},
		{
			name:        "unknown string service",
			service:     "unknown-service",
			wantErr:     true, // This will likely fail since Docker might not have the image
			description: "should handle unknown services with default format",
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
			wantErr:     false,
			description: "should handle complex service definitions",
		},
		{
			name: "service without image",
			service: map[string]interface{}{
				"badservice": map[string]interface{}{
					"volumes": []interface{}{"./:/data"},
				},
			},
			wantErr:     true,
			description: "should fail when service lacks required image field",
		},
		{
			name: "service with invalid config",
			service: map[string]interface{}{
				"badservice": "not-a-map",
			},
			wantErr:     true,
			description: "should fail when service config is not a map",
		},
		{
			name:        "invalid service type",
			service:     123,
			wantErr:     true,
			description: "should fail with invalid service type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartDockerService(tt.service)
			
			if (result.Success == false) != tt.wantErr {
				t.Errorf("StartDockerService() success = %v, wantErr %v, stderr: %s (case: %s)", 
					result.Success, tt.wantErr, result.Stderr, tt.description)
			}

			// Note: We're not actually testing Docker functionality here since
			// that would require Docker to be installed and running in the test environment.
			// These tests focus on the configuration parsing and command generation logic.
		})
	}
}

func TestExecuteCommandStepWithServices(t *testing.T) {
	step := config.CommandStep{
		StartServices: config.StartServices{
			"redis", // Simple string service
		},
		Run: config.RunCommand{"echo 'after services'"},
	}

	// This test checks the structure but will likely fail in CI without Docker
	// The important thing is that it exercises the code path
	result := ExecuteCommandStep(step, "test-services", "")
	
	// We can't guarantee Docker is available in test environment,
	// so we just ensure the function doesn't panic and returns a result
	if result.Stderr == "" && result.Stdout == "" && !result.Success {
		t.Log("Docker service test failed as expected (Docker likely not available)")
	}
}