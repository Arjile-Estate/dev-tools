package logpath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogFilePath(t *testing.T) {
	t.Run("returns custom path from config logs.file field", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: /custom/path/app.log\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, "/custom/path/app.log", result)
	})

	t.Run("expands tilde in logs.file path", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: ~/logs/dev-tools.log\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, filepath.Join(homeDir, "logs", "dev-tools.log"), result)
	})

	t.Run("returns default path when logs.file not configured", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "commands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log"), result)
	})

	t.Run("returns default path when no config file exists", func(t *testing.T) {
		projectDir := t.TempDir()

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log"), result)
	})

	t.Run("returns default path when config has invalid YAML", func(t *testing.T) {
		projectDir := t.TempDir()
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(":\n  invalid: [\n"), 0644)
		require.NoError(t, err)

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log"), result)
	})

	t.Run("returns default path when logs.file is empty string", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: \"\"\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		result := GetLogFilePath(projectDir)
		assert.Equal(t, filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log"), result)
	})
}

func TestGetLogConfig(t *testing.T) {
	t.Run("returns default size 20 when not configured", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "commands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cfg := GetLogConfig(projectDir)
		assert.Equal(t, DefaultLogLines, cfg.Size)
		assert.Equal(t, 20, cfg.Size)
	})

	t.Run("returns custom size from config", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: /tmp/test.log\n  size: 100\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cfg := GetLogConfig(projectDir)
		assert.Equal(t, 100, cfg.Size)
	})

	t.Run("returns file and size together", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: /custom/app.log\n  size: 75\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cfg := GetLogConfig(projectDir)
		assert.Equal(t, "/custom/app.log", cfg.File)
		assert.Equal(t, 75, cfg.Size)
	})

	t.Run("returns default size when size is zero", func(t *testing.T) {
		projectDir := t.TempDir()
		configContent := "logs:\n  file: /tmp/test.log\n  size: 0\ncommands:\n  test:\n    - run: echo hi\n"
		err := os.WriteFile(filepath.Join(projectDir, ".dev-config.yaml"), []byte(configContent), 0644)
		require.NoError(t, err)

		cfg := GetLogConfig(projectDir)
		assert.Equal(t, DefaultLogLines, cfg.Size)
	})

	t.Run("returns default file and size when no config exists", func(t *testing.T) {
		projectDir := t.TempDir()

		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		cfg := GetLogConfig(projectDir)
		assert.Equal(t, filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log"), cfg.File)
		assert.Equal(t, DefaultLogLines, cfg.Size)
	})
}
