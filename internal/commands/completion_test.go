package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCompletionCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
		expectError    bool
		errorMsg       string
	}{
		{
			name:        "missing shell argument",
			args:        []string{"completion"},
			expectError: true,
			errorMsg:    "completion command requires a shell type",
		},
		{
			name: "bash completion",
			args: []string{"completion", "bash"},
			expectedOutput: []string{
				"#!/bin/bash",
				"_dev_tools_completion",
				"complete -o nospace -F _dev_tools_completion dev-tools",
			},
			expectError: false,
		},
		{
			name: "zsh completion",
			args: []string{"completion", "zsh"},
			expectedOutput: []string{
				"#compdef dev-tools",
				"_dev_tools",
				"_arguments -C",
			},
			expectError: false,
		},
		{
			name: "fish completion",
			args: []string{"completion", "fish"},
			expectedOutput: []string{
				"# dev-tools fish completion",
				"__dev_tools_complete",
				"complete -c dev-tools",
			},
			expectError: false,
		},
		{
			name:        "unsupported shell",
			args:        []string{"completion", "powershell"},
			expectError: true,
			errorMsg:    "unsupported shell: powershell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleCompletionCommand(cmd, tt.args)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				output := buf.String()
				for _, expected := range tt.expectedOutput {
					assert.Contains(t, output, expected)
				}
			}
		})
	}
}

func TestGenerateBashCompletion(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := generateBashCompletion(cmd)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "#!/bin/bash")
	assert.Contains(t, output, "_dev_tools_completion")
	assert.Contains(t, output, "complete -o nospace -F _dev_tools_completion dev-tools")
	assert.Contains(t, output, "dev-tools __dev_complete")
	// Verify colon handling for commands like "test:coverage"
	assert.Contains(t, output, "_get_comp_words_by_ref -n :")
	assert.Contains(t, output, "__ltrim_colon_completions")
}

func TestGenerateZshCompletion(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := generateZshCompletion(cmd)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "#compdef dev-tools")
	assert.Contains(t, output, "_dev_tools")
	assert.Contains(t, output, "_arguments -C")
	assert.Contains(t, output, "restart|stop")
}

func TestGenerateFishCompletion(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := generateFishCompletion(cmd)

	require.NoError(t, err)
	output := buf.String()

	assert.Contains(t, output, "# dev-tools fish completion")
	assert.Contains(t, output, "__dev_tools_complete")
	assert.Contains(t, output, "complete -c dev-tools")
	assert.Contains(t, output, "-l verbose")
}

func TestHandleCompleteCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments",
			args:        []string{"__dev_complete"},
			expectError: false,
		},
		{
			name:        "with command line",
			args:        []string{"__dev_complete", "dev-tools", "test"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			err := HandleCompleteCommand(cmd, tt.args, tempDir)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseCompletionContext(t *testing.T) {
	tests := []struct {
		name            string
		commandLine     string
		projectDir      string
		expectedContext *CompletionContext
	}{
		{
			name:            "empty command line",
			commandLine:     "",
			projectDir:      "/test",
			expectedContext: nil,
		},
		{
			name:        "dev-tools prefix removed",
			commandLine: "dev-tools test ",
			projectDir:  "/test",
			expectedContext: &CompletionContext{
				Words:       []string{"test"},
				CurrentWord: "",
				WordIndex:   1,
				IsFlag:      false,
				CommandName: "test",
				ProjectDir:  "/test",
			},
		},
		{
			name:        "completing command name",
			commandLine: "dev-tools te",
			projectDir:  "/test",
			expectedContext: &CompletionContext{
				Words:       []string{"te"},
				CurrentWord: "te",
				WordIndex:   0,
				IsFlag:      false,
				CommandName: "te",
				ProjectDir:  "/test",
			},
		},
		{
			name:        "completing flag",
			commandLine: "dev-tools --ver",
			projectDir:  "/test",
			expectedContext: &CompletionContext{
				Words:       []string{"--ver"},
				CurrentWord: "--ver",
				WordIndex:   0,
				IsFlag:      true,
				CommandName: "",
				ProjectDir:  "/test",
			},
		},
		{
			name:        "completing second argument",
			commandLine: "dev-tools restart daemon",
			projectDir:  "/test",
			expectedContext: &CompletionContext{
				Words:       []string{"restart", "daemon"},
				CurrentWord: "daemon",
				WordIndex:   1,
				IsFlag:      false,
				CommandName: "restart",
				ProjectDir:  "/test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCompletionContext(tt.commandLine, tt.projectDir)

			if tt.expectedContext == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedContext.Words, result.Words)
				assert.Equal(t, tt.expectedContext.CurrentWord, result.CurrentWord)
				assert.Equal(t, tt.expectedContext.WordIndex, result.WordIndex)
				assert.Equal(t, tt.expectedContext.IsFlag, result.IsFlag)
				assert.Equal(t, tt.expectedContext.CommandName, result.CommandName)
				assert.Equal(t, tt.expectedContext.ProjectDir, result.ProjectDir)
			}
		})
	}
}

func TestLoadConfigForCompletion(t *testing.T) {
	tempDir := t.TempDir()

	// This will attempt to load config from the temp directory
	// which should return a default config
	config, err := loadConfigForCompletion(tempDir)

	require.NoError(t, err)
	assert.NotNil(t, config)
}

func TestGenerateCompletions(t *testing.T) {
	tests := []struct {
		name                string
		context             *CompletionContext
		config              *config.Config
		expectedCompletions []string
	}{
		{
			name: "flag completion",
			context: &CompletionContext{
				CurrentWord: "--ver",
				IsFlag:      true,
				WordIndex:   0,
			},
			config:              &config.Config{},
			expectedCompletions: []string{"--verbose", "--version"},
		},
		{
			name: "command completion",
			context: &CompletionContext{
				CurrentWord: "te",
				IsFlag:      false,
				WordIndex:   0,
			},
			config: &config.Config{
				Commands: map[string][]config.CommandStep{
					"test": {},
				},
			},
			expectedCompletions: []string{"test"},
		},
		{
			name: "restart daemon completion",
			context: &CompletionContext{
				CurrentWord: "",
				IsFlag:      false,
				WordIndex:   1,
				CommandName: "restart",
				ProjectDir:  "/test",
			},
			config:              &config.Config{},
			expectedCompletions: []string{},
		},
		{
			name: "completion shell type",
			context: &CompletionContext{
				CurrentWord: "ba",
				IsFlag:      false,
				WordIndex:   1,
				CommandName: "completion",
			},
			config:              &config.Config{},
			expectedCompletions: []string{"bash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateCompletions(tt.context, tt.config)

			assert.Equal(t, tt.expectedCompletions, result)
		})
	}
}

func TestGetAllAvailableCommands(t *testing.T) {
	tests := []struct {
		name             string
		config           *config.Config
		expectedCommands []string
	}{
		{
			name: "built-in commands only",
			config: &config.Config{
				Commands: map[string][]config.CommandStep{},
			},
			expectedCommands: []string{
				"cleanup-all", "cleanup-pids", "completion", "logs", "onboard", "restart", "status", "stop", "version",
			},
		},
		{
			name: "with custom commands",
			config: &config.Config{
				Commands: map[string][]config.CommandStep{
					"build":  {},
					"deploy": {},
					"test":   {},
				},
			},
			expectedCommands: []string{
				"build", "cleanup-all", "cleanup-pids", "completion", "deploy", "logs", "onboard", "restart", "status", "stop", "test", "version",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAllAvailableCommands(tt.config)

			assert.Equal(t, tt.expectedCommands, result)
		})
	}
}

func TestGetDaemonNames(t *testing.T) {
	t.Run("no daemon processes", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with no daemon processes (will depend on executor implementation)
		result := getDaemonNames(tempDir)

		// Should return an empty slice or slice with daemon names from config
		assert.NotNil(t, result)
	})

	t.Run("with daemon processes and config", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a PID file for a daemon
		pidFile := filepath.Join(tempDir, ".test-daemon.pid")
		pidInfo := executor.PIDFileInfo{
			PID:          os.Getpid(),
			CommandName:  "test-daemon",
			Command:      "sleep 300",
			StartTime:    time.Now(),
			RestartCount: 0,
		}
		data, err := json.Marshal(pidInfo)
		require.NoError(t, err)
		err = os.WriteFile(pidFile, data, 0644)
		require.NoError(t, err)

		// Create a config file with daemon commands
		configContent := `
commands:
  dev:
    - run: "npm run dev"
      daemon: true
  test:
    - run: "npm test"
`
		configFile := filepath.Join(tempDir, ".dev-config.yaml")
		err = os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		result := getDaemonNames(tempDir)

		// Should include daemon names from both running processes and config
		assert.NotNil(t, result)
		// The result should be a slice of strings (daemon names)
		for _, name := range result {
			assert.IsType(t, "", name)
		}
	})
}

func TestGetCachedCompletions(t *testing.T) {
	// Create a fresh cache for testing
	cache := NewCompletionCache(5 * time.Second)

	ctx := &CompletionContext{
		Words:       []string{"test"},
		WordIndex:   1,
		CurrentWord: "arg",
		ProjectDir:  "/test",
	}

	// Should return nil when no cache exists
	result := cache.Get(ctx)
	assert.Nil(t, result)

	// Cache some completions
	completions := []string{"completion1", "completion2"}
	cache.Set(ctx, completions)

	// Should return cached completions
	result = cache.Get(ctx)
	assert.Equal(t, completions, result)

	// Test cache expiration - create a cache with very short TTL
	shortCache := NewCompletionCache(1 * time.Nanosecond)
	shortCache.Set(ctx, completions)
	time.Sleep(2 * time.Millisecond) // Wait for expiration
	result = shortCache.Get(ctx)
	assert.Nil(t, result)
}

func TestCacheCompletions(t *testing.T) {
	// Create a fresh cache for testing
	cache := NewCompletionCache(5 * time.Second)

	ctx := &CompletionContext{
		Words:       []string{"test"},
		WordIndex:   1,
		CurrentWord: "arg",
		ProjectDir:  "/test",
	}

	completions := []string{"completion1", "completion2"}

	cache.Set(ctx, completions)

	// Verify the cached value can be retrieved
	result := cache.Get(ctx)
	assert.Equal(t, completions, result)

	// Test that changing project directory invalidates cache
	ctx2 := &CompletionContext{
		Words:       []string{"test"},
		WordIndex:   1,
		CurrentWord: "arg",
		ProjectDir:  "/different",
	}
	result = cache.Get(ctx2)
	assert.Nil(t, result, "Cache should be invalid for different project directory")
}

func TestCompletionCacheConcurrency(t *testing.T) {
	cache := NewCompletionCache(5 * time.Second)

	ctx := &CompletionContext{
		Words:       []string{"test"},
		WordIndex:   1,
		CurrentWord: "arg",
		ProjectDir:  "/test",
	}

	completions := []string{"completion1", "completion2", "completion3"}

	// Run concurrent reads and writes
	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // readers and writers

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Set(ctx, append(completions, fmt.Sprintf("writer-%d-%d", id, j)))
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				result := cache.Get(ctx)
				// Just verify we can read without panic
				_ = result
			}
		}(i)
	}

	wg.Wait()

	// Verify cache still works after concurrent operations
	cache.Set(ctx, completions)
	result := cache.Get(ctx)
	assert.Equal(t, completions, result)
}

func TestOutputCompletions(t *testing.T) {
	tests := []struct {
		name           string
		completions    []string
		expectedOutput string
	}{
		{
			name:           "empty completions",
			completions:    []string{},
			expectedOutput: "",
		},
		{
			name:           "single completion",
			completions:    []string{"test"},
			expectedOutput: "test\n",
		},
		{
			name:           "multiple completions",
			completions:    []string{"test", "build", "deploy"},
			expectedOutput: "test build deploy\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			outputCompletions(cmd, tt.completions)

			assert.Equal(t, tt.expectedOutput, buf.String())
		})
	}
}
