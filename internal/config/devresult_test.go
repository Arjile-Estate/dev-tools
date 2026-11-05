package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestDevResultConfigurationSurvivesDefaultCommandMerge(t *testing.T) {
	user := &Config{
		Commands: map[string][]CommandStep{"test": {{Run: RunCommand{"go test ./..."}}}},
		DevResult: &DevResultConfig{Commands: map[string]DevResultCommandConfig{
			"test": {Enabled: true},
		}},
	}
	merged := MergeConfigWithDefaults(user, GetDefaultCommandsForProjectType(ProjectTypeGo))

	require.NotNil(t, merged.DevResult)
	assert.True(t, merged.DevResult.Commands["test"].Enabled)
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

func TestDevResultDiagnosticsAdapterDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  test:
    - run: "go test ./..."
dev_result:
  commands:
    test:
      enabled: true
      diagnostics_adapter:
        command: [node, tools/dev-tools-diagnostics.mjs]
        start_when: [junit/**/*.xml, turbo-summary.json]
`), 0o600))

	cfg, err := LoadConfigFromFile(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg.DevResult)
	require.NotNil(t, cfg.DevResult.Commands["test"].DiagnosticsAdapter)
	adapter := cfg.DevResult.Commands["test"].DiagnosticsAdapter
	assert.Equal(t, []string{"node", "tools/dev-tools-diagnostics.mjs"}, adapter.Command)
	assert.Equal(t, []string{"junit/**/*.xml", "turbo-summary.json"}, adapter.StartWhen)
	assert.Equal(t, 2*time.Second, adapter.Timeout)
	assert.Equal(t, int64(1<<20), adapter.StdoutBytes)
	assert.Equal(t, int64(64<<10), adapter.StderrBytes)
	assert.Equal(t, 1000, adapter.MaxDiagnostics)
	assert.Equal(t, int64(10<<20), adapter.MaxSourceBytes)
	assert.Equal(t, int64(50<<20), adapter.MaxTotalSourceBytes)
	assert.Equal(t, 7*24*time.Hour, cfg.DevResult.Retention.MaxAge)
	assert.Equal(t, int64(1<<30), cfg.DevResult.Retention.MaxBytes)
}

func TestDevResultDiagnosticsAdapterAndRetentionOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  test:focused:
    - run: "go test ./..."
dev_result:
  retention:
    max_age: 24h
    max_bytes: 2097152
  commands:
    test:focused:
      enabled: true
      diagnostics_adapter:
        command: [adapter, "literal; argument"]
        start_when: [results.json]
        timeout: 750ms
        stdout_bytes: 2048
        stderr_bytes: 1024
        max_diagnostics: 25
        max_source_bytes: 4096
        max_total_source_bytes: 8192
`), 0o600))

	cfg, err := LoadConfigFromFile(configPath)
	require.NoError(t, err)
	adapter := cfg.DevResult.Commands["test:focused"].DiagnosticsAdapter
	require.NotNil(t, adapter)
	assert.Equal(t, []string{"adapter", "literal; argument"}, adapter.Command)
	assert.Equal(t, 750*time.Millisecond, adapter.Timeout)
	assert.Equal(t, int64(2048), adapter.StdoutBytes)
	assert.Equal(t, int64(1024), adapter.StderrBytes)
	assert.Equal(t, 25, adapter.MaxDiagnostics)
	assert.Equal(t, int64(4096), adapter.MaxSourceBytes)
	assert.Equal(t, int64(8192), adapter.MaxTotalSourceBytes)
	assert.Equal(t, 24*time.Hour, cfg.DevResult.Retention.MaxAge)
	assert.Equal(t, int64(2097152), cfg.DevResult.Retention.MaxBytes)
}

func TestDevResultDiagnosticsAdapterRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		adapterYML string
		wantError  string
	}{
		{name: "empty command", command: "test", adapterYML: "command: []\n        start_when: [results.json]", wantError: "command"},
		{name: "empty command argument", command: "test", adapterYML: "command: [adapter, \"\"]\n        start_when: [results.json]", wantError: "command"},
		{name: "empty start patterns", command: "test", adapterYML: "command: [adapter]\n        start_when: []", wantError: "start_when"},
		{name: "empty start pattern", command: "test", adapterYML: "command: [adapter]\n        start_when: [\"\"]", wantError: "start_when"},
		{name: "zero timeout", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        timeout: 0s", wantError: "timeout"},
		{name: "zero stdout limit", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        stdout_bytes: 0", wantError: "stdout_bytes"},
		{name: "zero stderr limit", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        stderr_bytes: 0", wantError: "stderr_bytes"},
		{name: "zero diagnostic limit", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        max_diagnostics: 0", wantError: "max_diagnostics"},
		{name: "zero source limit", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        max_source_bytes: 0", wantError: "max_source_bytes"},
		{name: "zero total source limit", command: "test", adapterYML: "command: [adapter]\n        start_when: [results.json]\n        max_total_source_bytes: 0", wantError: "max_total_source_bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
			contents := "commands:\n  " + tt.command + ":\n    - run: echo ok\ndev_result:\n  commands:\n    " + tt.command + ":\n      enabled: true\n      diagnostics_adapter:\n        " + tt.adapterYML + "\n"
			require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o600))

			_, err := LoadConfigFromFile(configPath)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestDevResultDiagnosticsAdapterIsAllowedForBuildAndStaticAnalysis(t *testing.T) {
	for _, command := range []string{"build", "typecheck", "lint", "lint:dead-code"} {
		t.Run(command, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
			contents := "commands:\n  " + command + ":\n    - run: echo ok\ndev_result:\n  commands:\n    " + command + ":\n      enabled: true\n      diagnostics_adapter:\n        command: [node, adapter.mjs]\n        start_when: [report.json]\n        required: true\n"
			require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o600))

			cfg, err := LoadConfigFromFile(configPath)
			require.NoError(t, err)
			adapter := cfg.DevResult.Commands[command].DiagnosticsAdapter
			require.NotNil(t, adapter)
			assert.True(t, adapter.Required)
		})
	}
}

func TestDevResultDiagnosticsAdapterRequiresRoomForTheDiagnosticsLine(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".dev-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`commands:
  build:
    - run: echo ok
dev_result:
  commands:
    build:
      enabled: true
      success:
        lines: 7
      diagnostics_adapter:
        command: [node, adapter.mjs]
        start_when: [report.json]
`), 0o600))

	_, err := LoadConfigFromFile(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greater than or equal to 8")
}
