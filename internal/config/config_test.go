package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDevConfigFromFile(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
		wantConfig  *DevConfig
	}{
		{
			name: "valid config file",
			fileContent: `commands:
  test:
    - run: "go test ./..."
  lint:
    - run: ["golangci-lint run", "go fmt ./..."]`,
			wantErr: false,
			wantConfig: &DevConfig{
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
			wantConfig:  &DevConfig{Commands: make(map[string][]CommandStep)},
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
			wantConfig: &DevConfig{
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

			got, err := LoadDevConfigFromFile(configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadDevConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if !compareDevConfigs(got, tt.wantConfig) {
				t.Errorf("LoadDevConfigFromFile() = %+v, want %+v", got, tt.wantConfig)
			}
		})
	}
}

func TestLoadDevConfigFromFileNotExists(t *testing.T) {
	config, err := LoadDevConfigFromFile("/nonexistent/path/.dev-config.yaml")
	if err != nil {
		t.Errorf("LoadDevConfigFromFile() with non-existent file should not error, got: %v", err)
	}
	if config != nil {
		t.Errorf("LoadDevConfigFromFile() with non-existent file should return nil, got: %+v", config)
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		wantType  ProjectType
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
		name        string
		projectType ProjectType
		wantCommands []string
	}{
		{
			name:        "go project defaults",
			projectType: ProjectTypeGo,
			wantCommands: []string{"test", "lint", "build", "logs"},
		},
		{
			name:        "python project defaults",
			projectType: ProjectTypePython,
			wantCommands: []string{"test", "lint", "logs"},
		},
		{
			name:        "nodejs project defaults",
			projectType: ProjectTypeNodeJS,
			wantCommands: []string{"test", "lint", "build", "logs"},
		},
		{
			name:        "rust project defaults",
			projectType: ProjectTypeRust,
			wantCommands: []string{"test", "lint", "dev", "build", "logs"},
		},
		{
			name:        "unknown project defaults",
			projectType: ProjectTypeUnknown,
			wantCommands: []string{"logs"},
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

func TestMergeConfigWithDefaults(t *testing.T) {
	defaults := &DevConfig{
		Commands: map[string][]CommandStep{
			"test": {{Run: RunCommand{"go test ./..."}}},
			"lint": {{Run: RunCommand{"golangci-lint run"}}},
		},
	}

	userConfig := &DevConfig{
		Commands: map[string][]CommandStep{
			"test": {{Run: RunCommand{"go test -v ./..."}}}, // Override
			"build": {{Run: RunCommand{"go build ./..."}}}, // New command
		},
	}

	merged := MergeConfigWithDefaults(userConfig, defaults)

	// Check that user config overrides defaults
	if len(merged.Commands["test"]) != 1 || merged.Commands["test"][0].Run[0] != "go test -v ./..." {
		t.Errorf("User config should override defaults for 'test' command")
	}

	// Check that default commands are preserved
	if len(merged.Commands["lint"]) != 1 || merged.Commands["lint"][0].Run[0] != "golangci-lint run" {
		t.Errorf("Default 'lint' command should be preserved")
	}

	// Check that new user commands are added
	if len(merged.Commands["build"]) != 1 || merged.Commands["build"][0].Run[0] != "go build ./..." {
		t.Errorf("New user 'build' command should be added")
	}
}

func TestLoadConfigurationForProject(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create go.mod to make it a Go project
	goMod := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goMod, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create custom config
	configContent := `commands:
  test:
    - run: "go test -race ./..."
  custom:
    - run: "echo custom command"`
	
	configFile := filepath.Join(tmpDir, ".dev-config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := LoadConfigurationForProject(tmpDir)
	if err != nil {
		t.Errorf("LoadConfigurationForProject() error = %v", err)
		return
	}

	// Should have both default Go commands and custom commands
	expectedCommands := []string{"test", "lint", "build", "logs", "custom"}
	for _, cmd := range expectedCommands {
		if _, exists := config.Commands[cmd]; !exists {
			t.Errorf("Expected command '%s' not found in final config", cmd)
		}
	}

	// Custom test command should override default
	if config.Commands["test"][0].Run[0] != "go test -race ./..." {
		t.Errorf("Custom test command should override default")
	}
}

// Helper function to compare DevConfig structs
func compareDevConfigs(a, b *DevConfig) bool {
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
	
	// Compare other fields
	return a.Background == b.Background &&
		a.Daemon == b.Daemon &&
		a.Directory == b.Directory
}