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
			wantErr:     true,
			wantConfig:  nil,
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
    - services:
        containers: ["redis", "postgres"]
    - run: "go run main.go"
      background: true
      daemon: true`,
			wantErr: false,
			wantConfig: &Config{
				Commands: map[string][]CommandStep{
					"dev": {
						{
							Services: ServicesConfig{
								Containers: []ContainerReference{
									{Simple: "redis"},
									{Simple: "postgres"},
								},
								Cleanup:       false,
								WaitForHealth: true,
								Timeout:       30,
							},
						},
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
				Containers: []ContainerReference{
					{Simple: "redis"},
					{
						Complex: &ContainerConfig{
							Name:  "database",
							Image: "postgres:15",
							Ports: []string{"5432:5432"},
							Environment: map[string]string{
								"POSTGRES_PASSWORD": "password",
							},
							Volumes: []string{"./data:/var/lib/postgresql/data"},
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
			name: "compose with project_name configuration",
			yamlContent: `services:
  compose:
    file: "docker-compose.yml"
    project_name: "shared-project"
    services: ["postgres"]
  wait_for_health: true
  timeout: 60`,
			want: ServicesConfig{
				Compose: &ComposeConfig{
					File:        "docker-compose.yml",
					ProjectName: "shared-project",
					Services:    []string{"postgres"},
				},
				Cleanup:       false,
				WaitForHealth: true,
				Timeout:       60,
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
				Containers: []ContainerReference{
					{Simple: "redis"},
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
		{
			name: "compose with project_name",
			yamlContent: `file: "docker-compose.yml"
project_name: "my-project"
services: ["redis"]`,
			want: ComposeConfig{
				File:        "docker-compose.yml",
				ProjectName: "my-project",
				Services:    []string{"redis"},
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

	return a.ProjectName == b.ProjectName
}

func compareContainerInterface(a, b ContainerReference) bool {
	// Compare simple references
	if a.IsSimple() && b.IsSimple() {
		return a.Simple == b.Simple
	}

	// Compare complex references
	if a.Complex != nil && b.Complex != nil {
		return compareContainerConfig(a.Complex, b.Complex)
	}

	// One is simple, other is complex - not equal
	return false
}

func compareContainerConfig(a, b *ContainerConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare all fields
	if a.Name != b.Name || a.Image != b.Image || a.Command != b.Command {
		return false
	}
	if a.Restart != b.Restart || a.Memory != b.Memory || a.CPUs != b.CPUs {
		return false
	}

	// Compare maps
	if !compareStringMap(a.Environment, b.Environment) {
		return false
	}

	// Compare slices
	if !compareStringSlice(a.Volumes, b.Volumes) {
		return false
	}
	if !compareStringSlice(a.Ports, b.Ports) {
		return false
	}
	if !compareStringSlice(a.Networks, b.Networks) {
		return false
	}

	// Compare health check
	return compareHealthCheck(a.HealthCheck, b.HealthCheck)
}

func compareStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func compareStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareHealthCheck(a, b *HealthCheckConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Test == b.Test &&
		a.Interval == b.Interval &&
		a.Timeout == b.Timeout &&
		a.Retries == b.Retries
}
