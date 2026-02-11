package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSchemaCommand(t *testing.T) {
	t.Run("writes schema to default output path", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := HandleSchemaCommand(cmd, []string{}, tmpDir)
		require.NoError(t, err)

		expectedPath := filepath.Join(tmpDir, ".dev-config.schema.json")
		assert.FileExists(t, expectedPath)

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), `"$schema"`)
		assert.Contains(t, string(content), `"Dev-Tools Configuration"`)
		assert.Contains(t, buf.String(), ".dev-config.schema.json")
	})

	t.Run("writes schema to custom output path", func(t *testing.T) {
		tmpDir := t.TempDir()
		customOutput := filepath.Join(tmpDir, "custom-schema.json")

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := HandleSchemaCommand(cmd, []string{"--output", customOutput}, tmpDir)
		require.NoError(t, err)

		assert.FileExists(t, customOutput)

		content, err := os.ReadFile(customOutput)
		require.NoError(t, err)
		assert.Contains(t, string(content), `"$schema"`)
	})

	t.Run("fails when output directory does not exist", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err := HandleSchemaCommand(cmd, []string{"--output", "/nonexistent/dir/schema.json"}, ".")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write schema")
	})

	t.Run("overwrites existing schema file", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaPath := filepath.Join(tmpDir, ".dev-config.schema.json")

		err := os.WriteFile(schemaPath, []byte("old content"), 0644)
		require.NoError(t, err)

		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		err = HandleSchemaCommand(cmd, []string{}, tmpDir)
		require.NoError(t, err)

		content, err := os.ReadFile(schemaPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), `"$schema"`)
		assert.NotContains(t, string(content), "old content")
	})
}
