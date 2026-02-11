package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"dev-tools/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupLogging(t *testing.T) {
	t.Run("verbose mode writes to stdout", func(t *testing.T) {
		originalStdout := os.Stdout
		defer func() { os.Stdout = originalStdout }()

		r, w, _ := os.Pipe()
		os.Stdout = w

		cleanup := setupLogging(true, "/project")
		defer cleanup()

		logger.Info("test message")

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("verbose mode cleanup is no-op", func(t *testing.T) {
		originalStdout := os.Stdout
		defer func() { os.Stdout = originalStdout }()

		r, w, _ := os.Pipe()
		os.Stdout = w

		cleanup := setupLogging(true, "/project")

		// Cleanup should not panic
		cleanup()

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
	})

	t.Run("non-verbose mode creates log file at logpath location", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a config with a custom logs.file pointing to temp dir
		logFile := filepath.Join(tempDir, "test.log")
		configContent := "logs:\n  file: " + logFile + "\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cleanup := setupLogging(false, tempDir)
		defer cleanup()

		assert.FileExists(t, logFile)
	})

	t.Run("cleanup closes the log file", func(t *testing.T) {
		tempDir := t.TempDir()

		logFile := filepath.Join(tempDir, "test.log")
		configContent := "logs:\n  file: " + logFile + "\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cleanup := setupLogging(false, tempDir)

		// Write something to verify file is open
		logger.Info("before cleanup")

		// Call cleanup - should close the file handle
		cleanup()

		// File should still exist after close
		assert.FileExists(t, logFile)
	})

	t.Run("log directory creation error falls back to stdout", func(t *testing.T) {
		// Use a project dir with no config, so default path is used.
		// Set HOME to a read-only location to cause mkdir to fail.
		originalHome := os.Getenv("HOME")
		defer func() { _ = os.Setenv("HOME", originalHome) }()
		_ = os.Setenv("HOME", "/")

		originalStdout := os.Stdout
		originalStderr := os.Stderr
		defer func() {
			os.Stdout = originalStdout
			os.Stderr = originalStderr
		}()

		r, w, _ := os.Pipe()
		os.Stdout = w
		os.Stderr = w

		cleanup := setupLogging(false, "/nonexistent-project-dir")
		defer cleanup()

		logger.Info("test message")

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("log file open error falls back to stdout", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a directory where the log file should be, causing open to fail
		logFile := filepath.Join(tempDir, "test.log")
		err := os.MkdirAll(logFile, 0755) // Create as directory instead of file
		require.NoError(t, err)

		configContent := "logs:\n  file: " + logFile + "\ncommands:\n  test:\n    - run: echo hi\n"
		err = os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		originalStdout := os.Stdout
		originalStderr := os.Stderr
		defer func() {
			os.Stdout = originalStdout
			os.Stderr = originalStderr
		}()

		r, w, _ := os.Pipe()
		os.Stdout = w
		os.Stderr = w

		cleanup := setupLogging(false, tempDir)
		defer cleanup()

		logger.Info("test message")

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		assert.Contains(t, buf.String(), "test message")
	})
}
