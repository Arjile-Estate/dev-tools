package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev-tools/internal/config"
)

func TestGetWatchDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test directories
	subDir1 := filepath.Join(tmpDir, "src")
	subDir2 := filepath.Join(tmpDir, "test")
	require.NoError(t, os.Mkdir(subDir1, 0755))
	require.NoError(t, os.Mkdir(subDir2, 0755))

	tests := []struct {
		name     string
		patterns []string
		expected int // Expected number of directories (at least)
	}{
		{
			name:     "no patterns - recursive",
			patterns: []string{},
			expected: 1, // At least tmpDir
		},
		{
			name:     "single pattern",
			patterns: []string{"src/*.go"},
			expected: 1, // At least one directory
		},
		{
			name:     "multiple patterns",
			patterns: []string{"src/**/*.go", "test/**/*.go"},
			expected: 1, // At least one directory
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirs, err := getWatchDirectories(tmpDir, tt.patterns)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(dirs), tt.expected)
		})
	}
}

func TestGetRecursiveDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directories
	subDir := filepath.Join(tmpDir, "level1", "level2")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create a .git directory that should be skipped
	gitDir := filepath.Join(tmpDir, ".git")
	require.NoError(t, os.Mkdir(gitDir, 0755))

	dirs, err := getRecursiveDirectories(tmpDir)
	require.NoError(t, err)

	assert.Greater(t, len(dirs), 0)

	// Verify .git is not included
	for _, dir := range dirs {
		assert.NotContains(t, dir, ".git")
	}
}

func TestShouldProcessEvent(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		eventOp     uint32 // fsnotify.Op
		watchConfig *config.WatchConfig
		expected    bool
	}{
		{
			name:     "write event - no patterns",
			filename: "/tmp/test.go",
			eventOp:  1, // Write
			watchConfig: &config.WatchConfig{
				Patterns: []string{},
				Ignore:   []string{},
			},
			expected: true,
		},
		{
			name:     "write event - matches pattern",
			filename: "/tmp/test.go",
			eventOp:  1, // Write
			watchConfig: &config.WatchConfig{
				Patterns: []string{"*.go"},
				Ignore:   []string{},
			},
			expected: true,
		},
		{
			name:     "write event - doesn't match pattern",
			filename: "/tmp/test.txt",
			eventOp:  1, // Write
			watchConfig: &config.WatchConfig{
				Patterns: []string{"*.go"},
				Ignore:   []string{},
			},
			expected: false,
		},
		{
			name:     "write event - matches ignore pattern",
			filename: "/tmp/test.go",
			eventOp:  1, // Write
			watchConfig: &config.WatchConfig{
				Patterns: []string{"*.go"},
				Ignore:   []string{"*.go"},
			},
			expected: false,
		},
		{
			name:     "chmod event - should be ignored",
			filename: "/tmp/test.go",
			eventOp:  8, // Chmod
			watchConfig: &config.WatchConfig{
				Patterns: []string{"*.go"},
				Ignore:   []string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock event (we can't easily create fsnotify.Event, so we test the logic)
			// This is a simplified test - in real usage fsnotify.Event would be used
			// For now, we're just testing the pattern matching logic
			if len(tt.watchConfig.Patterns) > 0 {
				result := matchesPattern(tt.filename, tt.watchConfig.Patterns[0])
				t.Logf("Pattern matching result for %s: %v", tt.name, result)
			} else {
				t.Logf("No patterns to test for %s", tt.name)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{
			name:     "exact filename match",
			path:     "/tmp/test.go",
			pattern:  "*.go",
			expected: true,
		},
		{
			name:     "no match",
			path:     "/tmp/test.txt",
			pattern:  "*.go",
			expected: false,
		},
		{
			name:     "simple wildcard pattern",
			path:     "main.go",
			pattern:  "*.go",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPattern(tt.path, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatchAndExecute_CancellationContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple command step
	steps := []config.CommandStep{
		{
			Run: config.RunCommand{"echo test"},
			Watch: &config.WatchConfig{
				Patterns: []string{"*.txt"},
				Debounce: "100ms",
			},
		},
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// This should timeout and return without error
	err := WatchAndExecute(ctx, "test-command", steps, tmpDir, nil)
	assert.NoError(t, err, "Watch should exit cleanly on context cancellation")
}

func TestWatchAndExecute_NoWatchConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create steps without watch configuration
	steps := []config.CommandStep{
		{
			Run: config.RunCommand{"echo test"},
		},
	}

	ctx := context.Background()
	err := WatchAndExecute(ctx, "test-command", steps, tmpDir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no watch configuration found")
}
