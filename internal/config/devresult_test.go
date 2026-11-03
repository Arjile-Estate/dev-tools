package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevResultConfigurationIsOptInPerCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  test:
    - run: "go test ./..."
  build:
    - run: "go build ./..."
dev_result:
  commands:
    test:
      enabled: true
      failure:
        lines: 120
        bytes: 16384
    build:
      enabled: false
`), 0o600))

	cfg, err := LoadConfigFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg.DevResult)
	assert.True(t, cfg.DevResult.Commands["test"].Enabled)
	assert.Equal(t, 120, cfg.DevResult.Commands["test"].Failure.Lines)
	assert.Equal(t, 16384, cfg.DevResult.Commands["test"].Failure.Bytes)
	assert.False(t, cfg.DevResult.Commands["build"].Enabled)
	_, listed := cfg.DevResult.Commands["lint"]
	assert.False(t, listed)
}

func TestDevResultConfigurationRejectsInvalidBudgets(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  test:
    - run: "go test ./..."
dev_result:
  commands:
    test:
      enabled: true
      success:
        lines: 0
        bytes: 1024
`), 0o600))

	_, err := LoadConfigFromFile(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEV-RESULT lines")
}

func TestDevResultConfigurationRejectsBudgetsTooSmallForContract(t *testing.T) {
	for _, budget := range []string{
		"lines: 6\n        bytes: 1024",
		"lines: 7\n        bytes: 1023",
	} {
		configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
		contents := `commands:
  test:
    - run: "go test ./..."
dev_result:
  commands:
    test:
      enabled: true
      success:
        ` + budget + "\n"
		require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o600))
		_, err := LoadConfigFromFile(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DEV-RESULT")
	}
}

func TestDevResultConfigurationRejectsUnsupportedCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  test:coverage:
    - run: "go test -cover ./..."
dev_result:
  commands:
    test:coverage:
      enabled: true
`), 0o600))

	_, err := LoadConfigFromFile(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test:coverage")
}
