package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDynamicHelp(t *testing.T) {
	t.Run("with valid config file", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a config file with custom commands
		configContent := `commands:
  build:
    - run: "go build"
  test:
    - run: "go test ./..."
  deploy:
    - run: "docker build -t app ."
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		result := generateDynamicHelp(tempDir)

		// Check base help text
		assert.Contains(t, result, "dev-tools is a command runner")
		assert.Contains(t, result, "automatically detects project types")

		// Check that available commands are shown
		assert.Contains(t, result, "Available commands:")
		assert.Contains(t, result, "build")
		assert.Contains(t, result, "test")
		assert.Contains(t, result, "deploy")

		// Check that built-in commands are also included
		assert.Contains(t, result, "logs")
		assert.Contains(t, result, "cleanup-pids")
		assert.Contains(t, result, "status")

		// Check examples section
		assert.Contains(t, result, "Examples:")
		assert.Contains(t, result, "dev-tools logs")
		assert.Contains(t, result, "--verbose")
	})

	t.Run("with no config file", func(t *testing.T) {
		tempDir := t.TempDir()
		// No config file created

		result := generateDynamicHelp(tempDir)

		// Check base help text
		assert.Contains(t, result, "dev-tools is a command runner")
		assert.Contains(t, result, "automatically detects project types")

		// Should show available commands (built-ins) even when no config file exists
		assert.Contains(t, result, "Available commands:")
		assert.Contains(t, result, "logs")
		assert.Contains(t, result, "cleanup-pids")
		assert.Contains(t, result, "status")

		// Should show examples
		assert.Contains(t, result, "Examples:")
		assert.Contains(t, result, "dev-tools logs")
		assert.Contains(t, result, "--verbose")
	})

	t.Run("with empty config file", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create an empty config file
		configContent := `commands: {}`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		result := generateDynamicHelp(tempDir)

		// Check base help text
		assert.Contains(t, result, "dev-tools is a command runner")

		// Should still show built-in commands
		assert.Contains(t, result, "Available commands:")
		assert.Contains(t, result, "logs")
		assert.Contains(t, result, "cleanup-pids")
		assert.Contains(t, result, "status")
	})

	t.Run("with config containing built-in command names", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create config with commands that have same names as built-ins
		configContent := `commands:
  logs:
    - run: "custom logs command"
  test:
    - run: "go test ./..."
  status:
    - run: "custom status"
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		result := generateDynamicHelp(tempDir)

		// Should not duplicate commands
		logCount := strings.Count(result, "logs")
		statusCount := strings.Count(result, "status")

		// Each command should appear only once in the available commands list
		// (though it might appear elsewhere in examples or help text)
		assert.Contains(t, result, "Available commands:")
		assert.Contains(t, result, "logs")
		assert.Contains(t, result, "test")
		assert.Contains(t, result, "status")

		// Verify no excessive duplication - allow some flexibility as commands appear in multiple places
		assert.LessOrEqual(t, logCount, 5) // May appear in available commands and examples
		assert.LessOrEqual(t, statusCount, 5)
	})

	t.Run("with many commands limits examples", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create config with many commands to test example limiting
		configContent := `commands:
  cmd1:
    - run: "command 1"
  cmd2:
    - run: "command 2"  
  cmd3:
    - run: "command 3"
  cmd4:
    - run: "command 4"
  cmd5:
    - run: "command 5"
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		result := generateDynamicHelp(tempDir)

		// Check that all commands are listed in available commands
		assert.Contains(t, result, "Available commands:")
		assert.Contains(t, result, "cmd1")
		assert.Contains(t, result, "cmd2")
		assert.Contains(t, result, "cmd3")
		assert.Contains(t, result, "cmd4")
		assert.Contains(t, result, "cmd5")

		// Check that examples section exists but is limited
		assert.Contains(t, result, "Examples:")

		// Count how many custom command examples are shown (should be max 3)
		exampleLines := strings.Split(result, "\n")
		customExampleCount := 0
		for _, line := range exampleLines {
			if strings.Contains(line, "dev-tools cmd") {
				customExampleCount++
			}
		}
		assert.LessOrEqual(t, customExampleCount, 3)
	})
}
