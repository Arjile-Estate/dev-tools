package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectType represents the detected project type
type ProjectType string

const (
	ProjectTypeGo      ProjectType = "go"
	ProjectTypePython  ProjectType = "python"
	ProjectTypeNodeJS  ProjectType = "nodejs"
	ProjectTypeRust    ProjectType = "rust"
	ProjectTypeUnknown ProjectType = "unknown"
)

// RunCommand represents a command that can be either a string or array of strings
type RunCommand []string

// UnmarshalYAML implements custom unmarshaling for RunCommand
func (r *RunCommand) UnmarshalYAML(value *yaml.Node) error {
	var single string
	var multiple []string

	// Try to unmarshal as a single string first
	if err := value.Decode(&single); err == nil {
		*r = []string{single}
		return nil
	}

	// If that fails, try to unmarshal as array of strings
	if err := value.Decode(&multiple); err == nil {
		*r = multiple
		return nil
	}

	return fmt.Errorf("run must be a string or array of strings")
}

// StartServices represents services that can be strings or complex objects
type StartServices []interface{}

// UnmarshalYAML implements custom unmarshaling for StartServices
func (s *StartServices) UnmarshalYAML(value *yaml.Node) error {
	var services []interface{}

	// Try to unmarshal as array of interfaces
	if err := value.Decode(&services); err == nil {
		*s = services
		return nil
	}

	return fmt.Errorf("start_services must be an array")
}

// CommandStep represents a single step in a command execution
type CommandStep struct {
	Run           RunCommand    `yaml:"run,omitempty"`
	StartServices StartServices `yaml:"start_services,omitempty"`
	Background    bool          `yaml:"background,omitempty"`
	Daemon        bool          `yaml:"daemon,omitempty"`
	Directory     string        `yaml:"directory,omitempty"`
}

// DevConfig represents the complete development configuration
type DevConfig struct {
	Commands map[string][]CommandStep `yaml:"commands"`
}

// LoadDevConfigFromFile loads configuration from a .dev-config.yaml file
func LoadDevConfigFromFile(configPath string) (*DevConfig, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // File doesn't exist, return nil without error
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config DevConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config %s: %w", configPath, err)
	}

	// Initialize commands map if it's nil
	if config.Commands == nil {
		config.Commands = make(map[string][]CommandStep)
	}

	return &config, nil
}

// DetectProjectType detects the project type based on the presence of specific files
func DetectProjectType(projectDir string) ProjectType {
	detectionPatterns := []struct {
		projectType ProjectType
		patterns    []string
	}{
		{ProjectTypeGo, []string{"go.mod"}},
		{ProjectTypePython, []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"}},
		{ProjectTypeNodeJS, []string{"package.json"}},
		{ProjectTypeRust, []string{"Cargo.toml"}},
	}

	for _, detection := range detectionPatterns {
		for _, pattern := range detection.patterns {
			if _, err := os.Stat(filepath.Join(projectDir, pattern)); err == nil {
				return detection.projectType
			}
		}
	}

	return ProjectTypeUnknown
}

// GetDefaultCommandsForProjectType returns default commands based on project type
func GetDefaultCommandsForProjectType(projectType ProjectType) *DevConfig {
	defaults := map[ProjectType]*DevConfig{
		ProjectTypeGo: {
			Commands: map[string][]CommandStep{
				"test":  {{Run: RunCommand{"go test ./..."}}},
				"lint":  {{Run: RunCommand{"golangci-lint run"}}},
				"build": {{Run: RunCommand{"go build ./..."}}},
			},
		},
		ProjectTypePython: {
			Commands: map[string][]CommandStep{
				"test": {{Run: RunCommand{"uv run pytest tests/"}}},
				"lint": {{Run: RunCommand{"uv run ruff check .", "uv run black ."}}},
			},
		},
		ProjectTypeNodeJS: {
			Commands: map[string][]CommandStep{
				"test":  {{Run: RunCommand{"npm test"}}},
				"lint":  {{Run: RunCommand{"npm run lint"}}},
				"build": {{Run: RunCommand{"npm run build"}}},
			},
		},
		ProjectTypeRust: {
			Commands: map[string][]CommandStep{
				"test":  {{Run: RunCommand{"cargo test"}}},
				"lint":  {{Run: RunCommand{"cargo clippy"}}},
				"dev":   {{Run: RunCommand{"cargo run"}}},
				"build": {{Run: RunCommand{"cargo build"}}},
			},
		},
		ProjectTypeUnknown: {
			Commands: map[string][]CommandStep{
			},
		},
	}

	if config, exists := defaults[projectType]; exists {
		return config
	}
	return defaults[ProjectTypeUnknown]
}

// MergeConfigWithDefaults merges user configuration with defaults
func MergeConfigWithDefaults(userConfig, defaults *DevConfig) *DevConfig {
	if userConfig == nil {
		return defaults
	}
	if defaults == nil {
		return userConfig
	}

	merged := &DevConfig{
		Commands: make(map[string][]CommandStep),
	}

	// Start with defaults
	for cmd, steps := range defaults.Commands {
		merged.Commands[cmd] = steps
	}

	// Override with user config
	for cmd, steps := range userConfig.Commands {
		merged.Commands[cmd] = steps
	}

	return merged
}

// LoadConfigurationForProject loads complete configuration for a project
func LoadConfigurationForProject(projectDir string) (*DevConfig, error) {
	configPath := filepath.Join(projectDir, ".dev-config.yaml")
	userConfig, err := LoadDevConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load user config: %w", err)
	}

	projectType := DetectProjectType(projectDir)
	defaults := GetDefaultCommandsForProjectType(projectType)

	finalConfig := MergeConfigWithDefaults(userConfig, defaults)
	return finalConfig, nil
}
