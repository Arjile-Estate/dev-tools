package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatLogLine(t *testing.T) {
	t.Run("complete JSON entry formats all fields", func(t *testing.T) {
		line := `{"level":"info","time":"2025-06-15T10:30:00+02:00","exec_dir":"/home/user/project","command":"test","message":"all tests passed"}`
		result := formatLogLine(line)
		assert.Equal(t, "2025-06-15 10:30:00+02:00 - /home/user/project - test - INFO - all tests passed", result)
	})

	t.Run("missing exec_dir and command show dashes", func(t *testing.T) {
		line := `{"level":"warn","time":"2025-06-15T10:30:00Z","message":"old entry"}`
		result := formatLogLine(line)
		assert.Equal(t, "2025-06-15 10:30:00+00:00 - - - - - WARN - old entry", result)
	})

	t.Run("malformed JSON returns raw line", func(t *testing.T) {
		line := "this is not json at all"
		result := formatLogLine(line)
		assert.Equal(t, "this is not json at all", result)
	})

	t.Run("level is uppercased", func(t *testing.T) {
		line := `{"level":"debug","time":"2025-01-01T00:00:00Z","exec_dir":"/x","command":"build","message":"msg"}`
		result := formatLogLine(line)
		assert.Contains(t, result, "DEBUG")
	})

	t.Run("missing level shows dash", func(t *testing.T) {
		line := `{"time":"2025-01-01T00:00:00Z","message":"no level"}`
		result := formatLogLine(line)
		assert.Equal(t, "2025-01-01 00:00:00+00:00 - - - - - - - no level", result)
	})

	t.Run("missing message shows dash", func(t *testing.T) {
		line := `{"level":"info","time":"2025-01-01T00:00:00Z","exec_dir":"/x","command":"y"}`
		result := formatLogLine(line)
		assert.Equal(t, "2025-01-01 00:00:00+00:00 - /x - y - INFO - -", result)
	})
}

func TestFormatTimestamp(t *testing.T) {
	t.Run("RFC3339 reformats to expected layout", func(t *testing.T) {
		result := formatTimestamp("2025-06-15T10:30:00+02:00")
		assert.Equal(t, "2025-06-15 10:30:00+02:00", result)
	})

	t.Run("UTC time shows +00:00", func(t *testing.T) {
		result := formatTimestamp("2025-01-01T00:00:00Z")
		assert.Equal(t, "2025-01-01 00:00:00+00:00", result)
	})

	t.Run("unparseable timestamp returns raw value", func(t *testing.T) {
		result := formatTimestamp("not-a-date")
		assert.Equal(t, "not-a-date", result)
	})

	t.Run("empty string returns dash", func(t *testing.T) {
		result := formatTimestamp("")
		assert.Equal(t, "-", result)
	})
}

func TestReadLastNLines(t *testing.T) {
	t.Run("returns last N lines from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := filepath.Join(tmpDir, "test.log")
		err := os.WriteFile(f, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)
		require.NoError(t, err)

		lines, err := readLastNLines(f, 3)
		require.NoError(t, err)
		assert.Equal(t, []string{"line3", "line4", "line5"}, lines)
	})

	t.Run("returns all lines when fewer than N", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := filepath.Join(tmpDir, "test.log")
		err := os.WriteFile(f, []byte("a\nb\n"), 0644)
		require.NoError(t, err)

		lines, err := readLastNLines(f, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, lines)
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := filepath.Join(tmpDir, "test.log")
		err := os.WriteFile(f, []byte(""), 0644)
		require.NoError(t, err)

		lines, err := readLastNLines(f, 5)
		require.NoError(t, err)
		assert.Empty(t, lines)
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := readLastNLines("/nonexistent/file.log", 5)
		require.Error(t, err)
	})
}

func TestHandleLogsCommandUsesConfiguredSize(t *testing.T) {
	t.Run("respects configured size from logs.size", func(t *testing.T) {
		tempDir := t.TempDir()

		// Write 30 log lines
		logFile := filepath.Join(tempDir, "custom.log")
		var content string
		for i := 1; i <= 30; i++ {
			content += fmt.Sprintf("line%d\n", i)
		}
		err := os.WriteFile(logFile, []byte(content), 0644)
		require.NoError(t, err)

		// Configure size: 5
		configContent := "logs:\n  file: " + logFile + "\n  size: 5\ncommands:\n  test:\n    - run: echo hi\n"
		err = os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleLogsCommand(cmd, tempDir)
		require.NoError(t, err)

		output := buf.String()
		assert.NotContains(t, output, "line25")
		assert.Contains(t, output, "line26")
		assert.Contains(t, output, "line30")
	})

	t.Run("defaults to 20 lines when size not configured", func(t *testing.T) {
		tempDir := t.TempDir()

		// Write 30 log lines
		logFile := filepath.Join(tempDir, "custom.log")
		var content string
		for i := 1; i <= 30; i++ {
			content += fmt.Sprintf("line%d\n", i)
		}
		err := os.WriteFile(logFile, []byte(content), 0644)
		require.NoError(t, err)

		// No size configured
		configContent := "logs:\n  file: " + logFile + "\ncommands:\n  test:\n    - run: echo hi\n"
		err = os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleLogsCommand(cmd, tempDir)
		require.NoError(t, err)

		output := buf.String()
		// With 30 lines and default size 20, line10 should be excluded, line11 should be included
		assert.NotContains(t, output, "line10\n")
		assert.Contains(t, output, "line11")
		assert.Contains(t, output, "line30")
	})
}
