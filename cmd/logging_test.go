package cmd

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRunningViaGoRun(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		expected   bool
	}{
		{
			name:       "go run on linux",
			executable: "/tmp/go-build123456789/b001/exe/main",
			expected:   true,
		},
		{
			name:       "go run on macOS",
			executable: "/var/folders/xy/abcdef/T/go-build987654321/b001/exe/main",
			expected:   true,
		},
		{
			name:       "compiled binary",
			executable: "/usr/local/bin/dev-tools",
			expected:   false,
		},
		{
			name:       "binary with go-build but not in tmp",
			executable: "/home/user/go-build123/myapp",
			expected:   false,
		},
		{
			name:       "binary in tmp but no go-build",
			executable: "/tmp/myapp",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRunningViaGoRun(tt.executable)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLogFilePath(t *testing.T) {
	t.Run("go run executable", func(t *testing.T) {
		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		osExecutable = func() (string, error) {
			return "/tmp/go-build123/b001/exe/main", nil
		}

		result, err := getLogFilePath("/project")
		require.NoError(t, err)
		assert.Equal(t, "/project/activity.log", result)
	})

	t.Run("compiled binary", func(t *testing.T) {
		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		originalHome := os.Getenv("HOME")
		defer func() { _ = os.Setenv("HOME", originalHome) }()
		_ = os.Setenv("HOME", "/home/user")

		osExecutable = func() (string, error) {
			return "/usr/local/bin/dev-tools", nil
		}

		result, err := getLogFilePath("/project")
		require.NoError(t, err)
		assert.Equal(t, "/home/user/Library/Logs/dev-tools.log", result)
	})

	t.Run("executable error", func(t *testing.T) {
		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		osExecutable = func() (string, error) {
			return "", errors.New("failed to get executable")
		}

		_, err := getLogFilePath("/project")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get executable path")
	})

	t.Run("home directory error", func(t *testing.T) {
		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		originalHome := os.Getenv("HOME")
		defer func() { _ = os.Setenv("HOME", originalHome) }()
		_ = os.Unsetenv("HOME")

		osExecutable = func() (string, error) {
			return "/usr/local/bin/dev-tools", nil
		}

		_, err := getLogFilePath("/project")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get user home directory")
	})
}

func TestSetupLogging(t *testing.T) {
	t.Run("verbose mode", func(t *testing.T) {
		// Save original stdout
		originalStdout := os.Stdout
		defer func() { os.Stdout = originalStdout }()

		// Create a pipe to capture stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		setupLogging(true, "/project")

		// Write a test log message
		log.Print("test message")

		// Close write end and read from read end
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		// Should write to stdout
		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("non-verbose mode with go run", func(t *testing.T) {
		tempDir := t.TempDir()

		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		osExecutable = func() (string, error) {
			return "/tmp/go-build123/b001/exe/main", nil
		}

		setupLogging(false, tempDir)

		// Verify log file was created
		logFile := filepath.Join(tempDir, "activity.log")
		assert.FileExists(t, logFile)
	})

	t.Run("non-verbose mode with compiled binary", func(t *testing.T) {
		tempDir := t.TempDir()
		homeDir := filepath.Join(tempDir, "home")
		logDir := filepath.Join(homeDir, "Library", "Logs")
		err := os.MkdirAll(logDir, 0755)
		require.NoError(t, err)

		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()
		originalHome := os.Getenv("HOME")
		defer func() { _ = os.Setenv("HOME", originalHome) }()

		osExecutable = func() (string, error) {
			return "/usr/local/bin/dev-tools", nil
		}
		_ = os.Setenv("HOME", homeDir)

		setupLogging(false, "/project")

		// Verify log file was created in home directory
		logFile := filepath.Join(logDir, "dev-tools.log")
		assert.FileExists(t, logFile)
	})

	t.Run("log file path error fallback", func(t *testing.T) {
		tempDir := t.TempDir()

		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		osExecutable = func() (string, error) {
			return "", errors.New("failed to get executable")
		}

		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(os.Stderr)

		setupLogging(false, tempDir)

		// Should fallback to project directory and create log file there
		logFile := filepath.Join(tempDir, "activity.log")
		assert.FileExists(t, logFile)
	})

	t.Run("log directory creation error fallback", func(t *testing.T) {
		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()
		originalHome := os.Getenv("HOME")
		defer func() { _ = os.Setenv("HOME", originalHome) }()

		osExecutable = func() (string, error) {
			return "/usr/local/bin/dev-tools", nil
		}
		// Set HOME to a read-only directory to cause mkdir to fail
		_ = os.Setenv("HOME", "/")

		// Save original stdout
		originalStdout := os.Stdout
		defer func() { os.Stdout = originalStdout }()

		// Create a pipe to capture stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		setupLogging(false, "/project")

		// Write a test log message
		log.Print("test message")

		// Close write end and read from read end
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		// Should fallback to stdout
		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("log file open error fallback", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a directory where the log file should be, causing open to fail
		logFile := filepath.Join(tempDir, "activity.log")
		err := os.MkdirAll(logFile, 0755) // Create as directory instead of file
		require.NoError(t, err)

		originalOsExecutable := osExecutable
		defer func() { osExecutable = originalOsExecutable }()

		osExecutable = func() (string, error) {
			return "/tmp/go-build123/b001/exe/main", nil
		}

		// Save original stdout
		originalStdout := os.Stdout
		defer func() { os.Stdout = originalStdout }()

		// Create a pipe to capture stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		setupLogging(false, tempDir)

		// Write a test log message
		log.Print("test message")

		// Close write end and read from read end
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		// Should fallback to stdout
		assert.Contains(t, buf.String(), "test message")
	})
}
