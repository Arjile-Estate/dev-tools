package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleValidateCommand(t *testing.T) {
	t.Run("valid config in text mode reports success", func(t *testing.T) {
		tempDir := t.TempDir()
		configContent := "commands:\n  test:\n    - run: echo hi\n"
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644))

		cmd := &cobra.Command{}
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		require.NoError(t, HandleValidateCommand(cmd, []string{}, tempDir))
		assert.Contains(t, stdout.String(), "Configuration is valid")
	})

	t.Run("valid config in JSON mode emits structured success", func(t *testing.T) {
		tempDir := t.TempDir()
		configContent := "commands:\n  test:\n    - run: echo hi\n"
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte(configContent), 0644))

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetContext(context.WithValue(context.Background(), FormatCtxKey, "json"))

		require.NoError(t, HandleValidateCommand(cmd, []string{}, tempDir))

		var payload map[string]any
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
		assert.Equal(t, true, payload["valid"])
		assert.Contains(t, payload["config_file"], ".dev-config.yaml")
		assert.Equal(t, []any{}, payload["errors"])
	})

	t.Run("invalid config in JSON mode emits structured failure", func(t *testing.T) {
		tempDir := t.TempDir()
		// Use an obviously broken YAML so config.ValidateConfigFile reports an error.
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".dev-config.yaml"), []byte("this is: not: valid: yaml: ["), 0644))

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetContext(context.WithValue(context.Background(), FormatCtxKey, "json"))

		err := HandleValidateCommand(cmd, []string{}, tempDir)
		require.Error(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &payload))
		assert.Equal(t, false, payload["valid"])
		assert.Contains(t, payload["config_file"], ".dev-config.yaml")
		errs, _ := payload["errors"].([]any)
		assert.NotEmpty(t, errs)
	})
}
