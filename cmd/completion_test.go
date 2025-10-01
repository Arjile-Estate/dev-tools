package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionBashCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		contains []string
	}{
		{
			name:    "bash completion generation",
			args:    []string{"completion", "bash"},
			wantErr: false,
			contains: []string{
				"_dev_tools_completion()",
				"complete -o nospace -F _dev_tools_completion dev-tools",
				"COMPREPLY=",
			},
		},
		{
			name:    "zsh completion generation",
			args:    []string{"completion", "zsh"},
			wantErr: false,
			contains: []string{
				"#compdef dev-tools",
				"_dev_tools()",
			},
		},
		{
			name:    "fish completion generation",
			args:    []string{"completion", "fish"},
			wantErr: false,
			contains: []string{
				"complete -c dev-tools",
			},
		},
		{
			name:    "unsupported shell",
			args:    []string{"completion", "invalid"},
			wantErr: true,
		},
		{
			name:    "no shell specified",
			args:    []string{"completion"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewRootCommand()
			rootCmd.SetArgs(tt.args)

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err := rootCmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("completion command error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				output := buf.String()
				for _, expected := range tt.contains {
					if !strings.Contains(output, expected) {
						t.Errorf("completion output should contain %q, got: %s", expected, output)
					}
				}
			}
		})
	}
}

func TestCompleteCommand(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		configContent string
		projectFiles  map[string]string
		commandLine   string
		expected      []string
		notExpected   []string
	}{
		{
			name: "complete built-in commands only",
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			commandLine: "dev-tools ",
			expected:    []string{"logs", "cleanup-pids", "status", "version", "completion"},
			notExpected: []string{},
		},
		{
			name: "complete with custom commands",
			configContent: `commands:
  custom-test:
    - run: "echo test"
  deploy:
    - run: "echo deploy"
`,
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			commandLine: "dev-tools ",
			expected:    []string{"logs", "custom-test", "deploy", "test", "build"},
		},
		{
			name: "complete partial command",
			configContent: `commands:
  custom-test:
    - run: "echo test"
  custom-build:
    - run: "echo build"
`,
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			commandLine: "dev-tools cus",
			expected:    []string{"custom-test", "custom-build"},
			notExpected: []string{"logs", "test", "build"},
		},
		{
			name: "complete daemon names for restart command",
			configContent: `commands:
  test-daemon:
    - run: "sleep 300"
      daemon: true
  another-daemon:
    - run: "sleep 300"
      daemon: true
`,
			commandLine: "dev-tools restart ",
			expected:    []string{"test-daemon", "another-daemon"},
			notExpected: []string{"logs", "status"},
		},
		{
			name: "complete daemon names for stop command",
			configContent: `commands:
  web-server:
    - run: "python -m http.server"
      daemon: true
`,
			commandLine: "dev-tools stop ",
			expected:    []string{"web-server"},
			notExpected: []string{"logs", "status"},
		},
		{
			name: "complete flags",
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			commandLine: "dev-tools --",
			expected:    []string{"--verbose", "--project-dir", "--no-color", "--version"},
		},
		{
			name: "no completion for unknown command",
			projectFiles: map[string]string{
				"go.mod": "module test",
			},
			commandLine: "dev-tools unknown-command ",
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Create project files
			for filename, content := range tt.projectFiles {
				err := os.WriteFile(filepath.Join(testDir, filename), []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create project file %s: %v", filename, err)
				}
			}

			// Create config file if specified
			if tt.configContent != "" {
				configFile := filepath.Join(testDir, ".dev-config.yaml")
				err := os.WriteFile(configFile, []byte(tt.configContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create config file: %v", err)
				}
			}

			rootCmd := NewRootCommand()
			rootCmd.SetArgs([]string{"--project-dir", testDir, "__dev_complete", tt.commandLine})

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err = rootCmd.Execute()
			if err != nil {
				t.Errorf("__dev_complete command should not error: %v", err)
				return
			}

			output := strings.TrimSpace(buf.String())
			completions := strings.Fields(output)

			// Check expected completions are present
			for _, expected := range tt.expected {
				found := false
				for _, completion := range completions {
					if completion == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completion %q not found in output: %s", expected, output)
				}
			}

			// Check that unexpected completions are not present
			for _, notExpected := range tt.notExpected {
				for _, completion := range completions {
					if completion == notExpected {
						t.Errorf("Unexpected completion %q found in output: %s", notExpected, output)
					}
				}
			}
		})
	}
}

func TestCompleteCommandNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go project without config file
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "__dev_complete", "dev-tools "})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err = rootCmd.Execute()
	if err != nil {
		t.Errorf("__dev_complete command should not error without config: %v", err)
		return
	}

	output := strings.TrimSpace(buf.String())
	completions := strings.Fields(output)

	// Should contain built-in commands
	expectedBuiltins := []string{"logs", "status", "version", "completion"}
	for _, expected := range expectedBuiltins {
		found := false
		for _, completion := range completions {
			if completion == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected built-in completion %q not found in output: %s", expected, output)
		}
	}

	// Should contain default Go commands
	expectedGoDefaults := []string{"test", "build"}
	for _, expected := range expectedGoDefaults {
		found := false
		for _, completion := range completions {
			if completion == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected Go default completion %q not found in output: %s", expected, output)
		}
	}
}

func TestCompleteCommandDifferentProjectTypes(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		projectFile  string
		content      string
		expectedCmds []string
	}{
		{
			name:         "Python project",
			projectFile:  "requirements.txt",
			content:      "django==3.2.0",
			expectedCmds: []string{"test", "lint"},
		},
		{
			name:         "Node.js project",
			projectFile:  "package.json",
			content:      `{"name": "test", "version": "1.0.0"}`,
			expectedCmds: []string{"test", "lint", "build"},
		},
		{
			name:         "Rust project",
			projectFile:  "Cargo.toml",
			content:      `[package]\nname = "test"\nversion = "0.1.0"`,
			expectedCmds: []string{"test", "lint", "dev", "build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Create project file
			projectFilePath := filepath.Join(testDir, tt.projectFile)
			err = os.WriteFile(projectFilePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create project file: %v", err)
			}

			rootCmd := NewRootCommand()
			rootCmd.SetArgs([]string{"--project-dir", testDir, "__dev_complete", "dev-tools "})

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err = rootCmd.Execute()
			if err != nil {
				t.Errorf("__dev_complete command should not error for %s project: %v", tt.name, err)
				return
			}

			output := strings.TrimSpace(buf.String())
			completions := strings.Fields(output)

			// Check that project-specific commands are present
			for _, expected := range tt.expectedCmds {
				found := false
				for _, completion := range completions {
					if completion == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected %s project completion %q not found in output: %s", tt.name, expected, output)
				}
			}
		})
	}
}

func TestCompleteCommandWithRunningDaemons(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("Failed to restore directory: %v", err)
		}
	}()

	// Create enhanced PID files for running daemons
	daemons := []struct {
		name    string
		command string
	}{
		{"web-server", "python -m http.server 8000"},
		{"worker", "celery worker"},
	}

	for _, daemon := range daemons {
		pidContent := `{"pid":` + "12345" + `,"command_name":"` + daemon.name + `","command":"` + daemon.command + `","start_time":"2023-01-01T12:00:00Z","restart_count":0}`
		pidFile := "." + daemon.name + ".pid"
		err := os.WriteFile(pidFile, []byte(pidContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create PID file for %s: %v", daemon.name, err)
		}
		defer func() { _ = os.Remove(pidFile) }()
	}

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"--project-dir", tmpDir, "__dev_complete", "dev-tools restart "})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("__dev_complete command should not error with running daemons: %v", err)
		return
	}

	output := strings.TrimSpace(buf.String())
	completions := strings.Fields(output)

	// Should contain running daemon names
	expectedDaemons := []string{"web-server", "worker"}
	for _, expected := range expectedDaemons {
		found := false
		for _, completion := range completions {
			if completion == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected daemon completion %q not found in output: %s", expected, output)
		}
	}
}

func TestCompleteCommandEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		commandLine string
		wantEmpty   bool
	}{
		{
			name:        "empty command line",
			commandLine: "",
			wantEmpty:   true,
		},
		{
			name:        "just dev-tools",
			commandLine: "dev-tools",
			wantEmpty:   false,
		},
		{
			name:        "command with multiple spaces",
			commandLine: "dev-tools   ",
			wantEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic Go project
			goModPath := filepath.Join(tmpDir, "go.mod")
			err := os.WriteFile(goModPath, []byte("module test"), 0644)
			if err != nil {
				t.Fatalf("Failed to create go.mod: %v", err)
			}

			rootCmd := NewRootCommand()
			rootCmd.SetArgs([]string{"--project-dir", tmpDir, "__dev_complete", tt.commandLine})

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err = rootCmd.Execute()
			if err != nil {
				t.Errorf("__dev_complete command should not error for edge case: %v", err)
				return
			}

			output := strings.TrimSpace(buf.String())
			isEmpty := output == ""

			if tt.wantEmpty && !isEmpty {
				t.Errorf("Expected empty output for %q, got: %s", tt.commandLine, output)
			} else if !tt.wantEmpty && isEmpty {
				t.Errorf("Expected non-empty output for %q, got empty", tt.commandLine)
			}
		})
	}
}
