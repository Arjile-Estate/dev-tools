package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeConfigWithDefaults_Refactored(t *testing.T) {
	tests := []struct {
		name       string
		userConfig *Config
		defaults   *Config
		want       *Config
	}{
		{
			name: "user config overrides defaults",
			userConfig: &Config{
				Commands: map[string][]CommandStep{
					"test":  {{Run: RunCommand{"go test -v ./..."}}},
					"build": {{Run: RunCommand{"go build ./..."}}},
				},
			},
			defaults: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
					"lint": {{Run: RunCommand{"golangci-lint run"}}},
				},
			},
			want: &Config{
				Commands: map[string][]CommandStep{
					"test":  {{Run: RunCommand{"go test -v ./..."}}},
					"lint":  {{Run: RunCommand{"golangci-lint run"}}},
					"build": {{Run: RunCommand{"go build ./..."}}},
				},
			},
		},
		{
			name:       "nil user config",
			userConfig: nil,
			defaults: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
				},
			},
			want: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
				},
			},
		},
		{
			name: "nil defaults config",
			userConfig: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
				},
			},
			defaults: nil,
			want: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeConfigWithDefaults(tt.userConfig, tt.defaults)
			assert.True(t, compareConfigs(got, tt.want), "Expected merged config to be %+v, but got %+v", tt.want, got)
		})
	}
}

func TestLoadConfigurationForProject_Refactored(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		configContent    string
		wantErr          bool
		expectedCommands []string
		overrides        map[string]string
	}{
		{
			name:  "go project with custom config",
			files: map[string]string{"go.mod": "module test"},
			configContent: `commands:
  test:
    - run: "go test -race ./..."
  custom:
    - run: "echo custom command"`,
			wantErr:          false,
			expectedCommands: []string{"test", "lint", "build", "custom"},
			overrides:        map[string]string{"test": "go test -race ./..."},
		},
		{
			name:             "python project with no custom config",
			files:            map[string]string{"pyproject.toml": ""},
			configContent:    "",
			wantErr:          false,
			expectedCommands: []string{"test", "lint"},
			overrides:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for filename, content := range tt.files {
				filePath := filepath.Join(tmpDir, filename)
				err := os.WriteFile(filePath, []byte(content), 0644)
				assert.NoError(t, err)
			}

			if tt.configContent != "" {
				configFile := filepath.Join(tmpDir, ".dev-config.yaml")
				err := os.WriteFile(configFile, []byte(tt.configContent), 0644)
				assert.NoError(t, err)
			}

			config, err := LoadConfigurationForProject(tmpDir)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			for _, cmd := range tt.expectedCommands {
				assert.Contains(t, config.Commands, cmd)
			}

			if tt.overrides != nil {
				for cmd, expectedRun := range tt.overrides {
					assert.Equal(t, expectedRun, config.Commands[cmd][0].Run[0])
				}
			}
		})
	}
}
