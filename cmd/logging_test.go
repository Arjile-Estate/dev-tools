package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetLogFilePath(t *testing.T) {
	tests := []struct {
		name           string
		executable     string
		homeDir        string
		projectDir     string
		expectedPath   string
		expectedIsGoRun bool
	}{
		{
			name:           "go run executable",
			executable:     "/tmp/go-build123456789/b001/exe/main",
			homeDir:        "/home/user",
			projectDir:     "/project",
			expectedPath:   "/project/activity.log",
			expectedIsGoRun: true,
		},
		{
			name:           "compiled binary",
			executable:     "/usr/local/bin/dev-tools",
			homeDir:        "/home/user",
			projectDir:     "/project",
			expectedPath:   "/home/user/Library/Logs/dev-tools.log",
			expectedIsGoRun: false,
		},
		{
			name:           "local compiled binary",
			executable:     "./dev-tools",
			homeDir:        "/home/user",
			projectDir:     "/project",
			expectedPath:   "/home/user/Library/Logs/dev-tools.log",
			expectedIsGoRun: false,
		},
		{
			name:           "go run with different temp path",
			executable:     "/var/folders/xy/abcdef/T/go-build987654321/b001/exe/dev-tools",
			homeDir:        "/Users/john",
			projectDir:     "/workspace",
			expectedPath:   "/workspace/activity.log",
			expectedIsGoRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPath, actualIsGoRun := getLogFilePath(tt.executable, tt.homeDir, tt.projectDir)
			
			if actualPath != tt.expectedPath {
				t.Errorf("getLogFilePath() path = %v, want %v", actualPath, tt.expectedPath)
			}
			
			if actualIsGoRun != tt.expectedIsGoRun {
				t.Errorf("getLogFilePath() isGoRun = %v, want %v", actualIsGoRun, tt.expectedIsGoRun)
			}
		})
	}
}

func TestIsRunningViaGoRun(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		expected   bool
	}{
		{
			name:       "typical go run path on Unix",
			executable: "/tmp/go-build123456789/b001/exe/main",
			expected:   true,
		},
		{
			name:       "typical go run path on macOS",
			executable: "/var/folders/xy/abcdef/T/go-build987654321/b001/exe/dev-tools",
			expected:   true,
		},
		{
			name:       "compiled binary in PATH",
			executable: "/usr/local/bin/dev-tools",
			expected:   false,
		},
		{
			name:       "local compiled binary",
			executable: "./dev-tools",
			expected:   false,
		},
		{
			name:       "absolute path to compiled binary",
			executable: "/home/user/projects/dev-tools/dev-tools",
			expected:   false,
		},
		{
			name:       "go run pattern in filename but not temp dir",
			executable: "/usr/local/bin/go-build-tool",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := isRunningViaGoRun(tt.executable)
			if actual != tt.expected {
				t.Errorf("isRunningViaGoRun(%q) = %v, want %v", tt.executable, actual, tt.expected)
			}
		})
	}
}

func TestEnsureLogDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "Library", "Logs")
	logFile := filepath.Join(logDir, "dev-tools.log")

	// Test that the directory is created if it doesn't exist
	err := ensureLogDirectory(logFile)
	if err != nil {
		t.Fatalf("ensureLogDirectory() error = %v", err)
	}

	// Check that the directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("ensureLogDirectory() did not create directory %s", logDir)
	}

	// Test that it doesn't fail if directory already exists
	err = ensureLogDirectory(logFile)
	if err != nil {
		t.Errorf("ensureLogDirectory() error on existing directory = %v", err)
	}
}

func TestGetHomeDirectory(t *testing.T) {
	// Test with environment variable set
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	testHome := "/test/home"
	_ = os.Setenv("HOME", testHome)

	home, err := getHomeDirectory()
	if err != nil {
		t.Fatalf("getHomeDirectory() error = %v", err)
	}

	if home != testHome {
		t.Errorf("getHomeDirectory() = %v, want %v", home, testHome)
	}

	// Test with no HOME environment variable
	_ = os.Unsetenv("HOME")
	_, err = getHomeDirectory()
	if err == nil {
		t.Error("getHomeDirectory() should return error when HOME is not set")
	}
}