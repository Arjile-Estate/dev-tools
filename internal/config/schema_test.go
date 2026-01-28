package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with simple command",
			config: &Config{
				Commands: map[string][]CommandStep{
					"test": {
						{
							Run: []string{"go test ./..."},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with services",
			config: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{
							Services: ServicesConfig{
								Compose: &ComposeConfig{
									File: "docker-compose.yml",
								},
								Cleanup:       true,
								WaitForHealth: true,
								Timeout:       30,
							},
							Run: []string{"npm start"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with daemon",
			config: &Config{
				Commands: map[string][]CommandStep{
					"server": {
						{
							Run:        []string{"./server"},
							Background: true,
							Daemon:     true,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with retry",
			config: &Config{
				Commands: map[string][]CommandStep{
					"flaky": {
						{
							Run:              []string{"./test.sh"},
							Retry:            3,
							RetryDelay:       "5s",
							RetryOnExitCodes: []int{1, 2},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with watch",
			config: &Config{
				Commands: map[string][]CommandStep{
					"watch": {
						{
							Run: []string{"npm test"},
							Watch: &WatchConfig{
								Patterns: []string{"**/*.ts"},
								Debounce: "300ms",
								Ignore:   []string{"**/node_modules/**"},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfigFile(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config file",
			configYAML: `commands:
  test:
    - run: go test ./...
  build:
    - run: go build -o app
`,
			expectError: false,
		},
		{
			name: "valid config with services",
			configYAML: `commands:
  dev:
    - services:
        compose:
          file: docker-compose.yml
          services:
            - postgres
            - redis
        cleanup: true
        timeout: 60
      run: npm start
`,
			expectError: false,
		},
		{
			name:        "missing commands key",
			configYAML:  `commands: {}`,
			expectError: true,
			errorMsg:    "at least one command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, ".dev-config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			require.NoError(t, err)

			// Validate
			err = ValidateConfigFile(configPath)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfigFile_NotFound(t *testing.T) {
	err := ValidateConfigFile("/nonexistent/path/.dev-config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestConfigSchema_IsValidJSON(t *testing.T) {
	// Verify the schema itself is valid JSON
	var schema map[string]interface{}
	err := json.Unmarshal([]byte(ConfigSchema), &schema)
	assert.NoError(t, err, "ConfigSchema should be valid JSON")

	// Verify required fields
	assert.Contains(t, schema, "$schema")
	assert.Contains(t, schema, "title")
	assert.Contains(t, schema, "definitions")
}
