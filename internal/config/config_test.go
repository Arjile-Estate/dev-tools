package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadConfigFromFile(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
		wantConfig  *Config
	}{
		{
			name: "valid config file",
			fileContent: `commands:
  test:
    - run: "go test ./..."
  lint:
    - run: ["golangci-lint run", "go fmt ./..."]`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"test": {{Run: RunCommand{"go test ./..."}}},
					"lint": {{Run: RunCommand{"golangci-lint run", "go fmt ./..."}}},
				},
			},
		},
		{
			name:        "empty config file",
			fileContent: "",
			wantErr:     false,
			wantConfig:  &Config{Commands: make(map[string][]CommandStep)},
		},
		{
			name:        "invalid yaml",
			fileContent: "invalid: yaml: content: [",
			wantErr:     true,
			wantConfig:  nil,
		},
		{
			name: "config with services and background",
			fileContent: `commands:
  dev:
    - start_services: ["redis", "postgres"]
    - run: "go run main.go"
      background: true
      daemon: true`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{StartServices: StartServices{"redis", "postgres"}},
						{
							Run:        RunCommand{"go run main.go"},
							Background: true,
							Daemon:     true,
						},
					},
				},
			},
		},
		{
			name: "config with multiple run commands in array",
			fileContent: `commands:
  test-multicmd:
    - run:
      - "echo command 1"
      - "echo command 2"
      - "echo command 3"`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"test-multicmd": {
						{Run: RunCommand{"echo command 1", "echo command 2", "echo command 3"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, ".dev-config.yaml")
			err := os.WriteFile(configFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := LoadConfigFromFile(configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !compareConfigs(got, tt.wantConfig) {
				t.Errorf("LoadConfigFromFile() = %+v, want %+v", got, tt.wantConfig)
			}
		})
	}
}

func TestLoadConfigFromFileNotExists(t *testing.T) {
	config, err := LoadConfigFromFile("/nonexistent/path/.dev-config.yaml")
	if err != nil {
		t.Errorf("LoadConfigFromFile() with non-existent file should not error, got: %v", err)
	}
	if config != nil {
		t.Errorf("LoadConfigFromFile() with non-existent file should return nil, got: %+v", config)
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		wantType ProjectType
	}{
		{
			name:     "go project",
			files:    []string{"go.mod"},
			wantType: ProjectTypeGo,
		},
		{
			name:     "python project with pyproject.toml",
			files:    []string{"pyproject.toml"},
			wantType: ProjectTypePython,
		},
		{
			name:     "python project with requirements.txt",
			files:    []string{"requirements.txt"},
			wantType: ProjectTypePython,
		},
		{
			name:     "nodejs project",
			files:    []string{"package.json"},
			wantType: ProjectTypeNodeJS,
		},
		{
			name:     "rust project",
			files:    []string{"Cargo.toml"},
			wantType: ProjectTypeRust,
		},
		{
			name:     "unknown project",
			files:    []string{"README.md"},
			wantType: ProjectTypeUnknown,
		},
		{
			name:     "multiple project types - go takes precedence",
			files:    []string{"go.mod", "package.json", "pyproject.toml"},
			wantType: ProjectTypeGo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create test files
			for _, file := range tt.files {
				filePath := filepath.Join(tmpDir, file)
				err := os.WriteFile(filePath, []byte("test content"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", file, err)
				}
			}

			got := DetectProjectType(tmpDir)
			if got != tt.wantType {
				t.Errorf("DetectProjectType() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

func TestGetDefaultCommandsForProjectType(t *testing.T) {
	tests := []struct {
		name         string
		projectType  ProjectType
		wantCommands []string
	}{
		{
			name:         "go project defaults",
			projectType:  ProjectTypeGo,
			wantCommands: []string{"test", "lint", "build"},
		},
		{
			name:         "python project defaults",
			projectType:  ProjectTypePython,
			wantCommands: []string{"test", "lint"},
		},
		{
			name:         "nodejs project defaults",
			projectType:  ProjectTypeNodeJS,
			wantCommands: []string{"test", "lint", "build"},
		},
		{
			name:         "rust project defaults",
			projectType:  ProjectTypeRust,
			wantCommands: []string{"test", "lint", "dev", "build"},
		},
		{
			name:         "unknown project defaults",
			projectType:  ProjectTypeUnknown,
			wantCommands: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaults := GetDefaultCommandsForProjectType(tt.projectType)

			for _, expectedCmd := range tt.wantCommands {
				if _, exists := defaults.Commands[expectedCmd]; !exists {
					t.Errorf("Expected command '%s' not found in defaults for %s", expectedCmd, tt.projectType)
				}
			}
		})
	}
}

// Helper function to compare Config structs
func compareConfigs(a, b *Config) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if len(a.Commands) != len(b.Commands) {
		return false
	}

	for key, stepsA := range a.Commands {
		stepsB, exists := b.Commands[key]
		if !exists {
			return false
		}

		if len(stepsA) != len(stepsB) {
			return false
		}

		for i, stepA := range stepsA {
			stepB := stepsB[i]
			if !compareCommandSteps(stepA, stepB) {
				return false
			}
		}
	}

	return true
}

func compareCommandSteps(a, b CommandStep) bool {
	// Compare Run slices
	if len(a.Run) != len(b.Run) {
		return false
	}
	for i, runA := range a.Run {
		if runA != b.Run[i] {
			return false
		}
	}

	// Compare StartServices slices
	if len(a.StartServices) != len(b.StartServices) {
		return false
	}
	for i, serviceA := range a.StartServices {
		if serviceA != b.StartServices[i] {
			return false
		}
	}

	// Compare Services configuration
	if !compareServicesConfig(a.Services, b.Services) {
		return false
	}

	// Compare other fields
	return a.Background == b.Background &&
		a.Daemon == b.Daemon &&
		a.Directory == b.Directory
}

func TestServicesConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		want        ServicesConfig
		wantErr     bool
	}{
		{
			name: "compose only configuration",
			yamlContent: `services:
  compose:
    file: "docker-compose.yml"
    services: ["redis", "postgres"]
    profiles: ["dev", "testing"]`,
			want: ServicesConfig{
				Compose: &ComposeConfig{
					File:     "docker-compose.yml",
					Services: []string{"redis", "postgres"},
					Profiles: []string{"dev", "testing"},
				},
				Cleanup:       false,
				WaitForHealth: true,
				Timeout:       30,
			},
			wantErr: false,
		},
		{
			name: "containers only configuration",
			yamlContent: `services:
  containers:
    - "redis"
    - database:
        image: "postgres:15"
        ports: ["5432:5432"]
        environment:
          POSTGRES_PASSWORD: "password"
        volumes: ["./data:/var/lib/postgresql/data"]`,
			want: ServicesConfig{
				Containers: []interface{}{
					"redis",
					map[string]interface{}{
						"database": map[string]interface{}{
							"image": "postgres:15",
							"ports": []interface{}{"5432:5432"},
							"environment": map[string]interface{}{
								"POSTGRES_PASSWORD": "password",
							},
							"volumes": []interface{}{"./data:/var/lib/postgresql/data"},
						},
					},
				},
				Cleanup:       false,
				WaitForHealth: true,
				Timeout:       30,
			},
			wantErr: false,
		},
		{
			name: "full configuration with custom defaults",
			yamlContent: `services:
  compose:
    file: "docker-compose.dev.yml"
    services: ["api", "database"]
  containers:
    - "redis"
  cleanup: true
  wait_for_health: false
  timeout: 60`,
			want: ServicesConfig{
				Compose: &ComposeConfig{
					File:     "docker-compose.dev.yml",
					Services: []string{"api", "database"},
				},
				Containers: []interface{}{
					"redis",
				},
				Cleanup:       true,
				WaitForHealth: false,
				Timeout:       60,
			},
			wantErr: false,
		},
		{
			name:        "empty services configuration",
			yamlContent: `services: {}`,
			want: ServicesConfig{
				Cleanup:       false,
				WaitForHealth: true,
				Timeout:       30,
			},
			wantErr: false,
		},
		{
			name: "invalid yaml structure",
			yamlContent: `services:
  compose:
    file: ["not", "a", "string"]`,
			want:    ServicesConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Services ServicesConfig `yaml:"services"`
			}

			err := yaml.Unmarshal([]byte(tt.yamlContent), &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ServicesConfig.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !compareServicesConfig(result.Services, tt.want) {
				t.Errorf("ServicesConfig.UnmarshalYAML() = %+v, want %+v", result.Services, tt.want)
			}
		})
	}
}

func TestComposeConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		want        ComposeConfig
		wantErr     bool
	}{
		{
			name: "complete compose configuration",
			yamlContent: `file: "docker-compose.yml"
services: ["redis", "postgres", "api"]
profiles: ["dev", "testing"]`,
			want: ComposeConfig{
				File:     "docker-compose.yml",
				Services: []string{"redis", "postgres", "api"},
				Profiles: []string{"dev", "testing"},
			},
			wantErr: false,
		},
		{
			name:        "minimal compose configuration",
			yamlContent: `file: "docker-compose.dev.yml"`,
			want: ComposeConfig{
				File: "docker-compose.dev.yml",
			},
			wantErr: false,
		},
		{
			name: "compose with empty services array",
			yamlContent: `file: "docker-compose.yml"
services: []`,
			want: ComposeConfig{
				File:     "docker-compose.yml",
				Services: []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ComposeConfig
			err := yaml.Unmarshal([]byte(tt.yamlContent), &result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComposeConfig.UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !compareComposeConfig(result, tt.want) {
				t.Errorf("ComposeConfig.UnmarshalYAML() = %+v, want %+v", result, tt.want)
			}
		})
	}
}

func TestBackwardCompatibility_StartServices(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
		wantConfig  *Config
	}{
		{
			name: "backward compatibility with start_services",
			fileContent: `commands:
  dev:
    - start_services: ["redis", "postgres"]
    - run: "go run main.go"`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{StartServices: StartServices{"redis", "postgres"}},
						{Run: RunCommand{"go run main.go"}},
					},
				},
			},
		},
		{
			name: "new services configuration",
			fileContent: `commands:
  dev:
    - services:
        containers: ["redis", "postgres"]
        cleanup: true
    - run: "go run main.go"`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{
							Services: ServicesConfig{
								Containers:    []interface{}{"redis", "postgres"},
								Cleanup:       true,
								WaitForHealth: true,
								Timeout:       30,
							},
						},
						{Run: RunCommand{"go run main.go"}},
					},
				},
			},
		},
		{
			name: "mixed configuration - both start_services and services",
			fileContent: `commands:
  dev:
    - start_services: ["redis"]
    - services:
        containers: ["postgres"]
    - run: "go run main.go"`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{StartServices: StartServices{"redis"}},
						{
							Services: ServicesConfig{
								Containers:    []interface{}{"postgres"},
								Cleanup:       false,
								WaitForHealth: true,
								Timeout:       30,
							},
						},
						{Run: RunCommand{"go run main.go"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, ".dev-config.yaml")
			err := os.WriteFile(configFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := LoadConfigFromFile(configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !compareConfigs(got, tt.wantConfig) {
				t.Errorf("LoadConfigFromFile() = %+v, want %+v", got, tt.wantConfig)
			}
		})
	}
}

// Helper functions for comparing new data structures

func compareServicesConfig(a, b ServicesConfig) bool {
	if !compareComposeConfigPointers(a.Compose, b.Compose) {
		return false
	}

	if len(a.Containers) != len(b.Containers) {
		return false
	}

	for i, containerA := range a.Containers {
		if !compareContainerInterface(containerA, b.Containers[i]) {
			return false
		}
	}

	return a.Cleanup == b.Cleanup &&
		a.WaitForHealth == b.WaitForHealth &&
		a.Timeout == b.Timeout
}

func compareComposeConfigPointers(a, b *ComposeConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return compareComposeConfig(*a, *b)
}

func compareComposeConfig(a, b ComposeConfig) bool {
	if a.File != b.File {
		return false
	}

	if len(a.Services) != len(b.Services) {
		return false
	}

	for i, serviceA := range a.Services {
		if serviceA != b.Services[i] {
			return false
		}
	}

	if len(a.Profiles) != len(b.Profiles) {
		return false
	}

	for i, profileA := range a.Profiles {
		if profileA != b.Profiles[i] {
			return false
		}
	}

	return true
}

func compareContainerInterface(a, b interface{}) bool {
	switch aVal := a.(type) {
	case string:
		if bVal, ok := b.(string); ok {
			return aVal == bVal
		}
		return false
	case map[string]interface{}:
		if bVal, ok := b.(map[string]interface{}); ok {
			return compareMapInterface(aVal, bVal)
		}
		return false
	default:
		return false
	}
}

func compareMapInterface(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		valueB, exists := b[key]
		if !exists {
			return false
		}

		switch valA := valueA.(type) {
		case string:
			if valB, ok := valueB.(string); !ok || valA != valB {
				return false
			}
		case []interface{}:
			if valB, ok := valueB.([]interface{}); !ok || !compareSliceInterface(valA, valB) {
				return false
			}
		case map[string]interface{}:
			if valB, ok := valueB.(map[string]interface{}); !ok || !compareMapInterface(valA, valB) {
				return false
			}
		default:
			if valueA != valueB {
				return false
			}
		}
	}

	return true
}

func compareSliceInterface(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for i, valueA := range a {
		if valueA != b[i] {
			return false
		}
	}

	return true
}
