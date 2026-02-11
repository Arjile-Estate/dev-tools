package executor

import (
	"context"
	"fmt"
	"dev-tools/internal/logger"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
)

// WatchAndExecute watches files and executes command on changes
func WatchAndExecute(ctx context.Context, commandName string, steps []config.CommandStep, workingDir string, passthroughArgs []string) error {
	// Find watch configuration from steps
	var watchConfig *config.WatchConfig
	for _, step := range steps {
		if step.Watch != nil {
			watchConfig = step.Watch
			break
		}
	}

	if watchConfig == nil {
		return fmt.Errorf("no watch configuration found in command steps")
	}

	// Parse debounce delay
	debounceDelay := 300 * time.Millisecond
	if watchConfig.Debounce != "" {
		if parsed, err := time.ParseDuration(watchConfig.Debounce); err == nil {
			debounceDelay = parsed
		} else {
			logger.Warnf("Invalid debounce delay '%s', using default 300ms: %v", watchConfig.Debounce, err)
		}
	}

	// Initialize file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Add directories to watch based on patterns
	watchDirs, err := getWatchDirectories(workingDir, watchConfig.Patterns)
	if err != nil {
		return fmt.Errorf("failed to determine watch directories: %w", err)
	}

	for _, dir := range watchDirs {
		if err := watcher.Add(dir); err != nil {
			logger.Warnf("Failed to watch directory %s: %v", dir, err)
		} else {
			logger.Infof("Watching directory: %s", dir)
		}
	}

	fmt.Printf("%s\n", colors.Info("Watch mode enabled. Watching for changes..."))
	fmt.Printf("%s\n", colors.Info("Press Ctrl+C to stop watching"))

	// Run initial execution
	fmt.Printf("\n%s\n", colors.Success(">>> Initial execution"))
	result := ExecuteCommandWithOptions(CommandExecutionOptions{
		CommandName:     commandName,
		Steps:           steps,
		WorkingDir:      workingDir,
		PassthroughArgs: passthroughArgs,
	})
	if !result.Success {
		fmt.Printf("%s\n", colors.Warning("Initial execution failed"))
	}

	// Debounce timer
	var timer *time.Timer
	var pendingEvent bool

	// Watch loop
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n%s\n", colors.Info("Watch mode stopped"))
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}

			// Check if file matches patterns and is not ignored
			if !shouldProcessEvent(event, watchConfig) {
				continue
			}

			logger.Infof("File changed: %s (%s)", event.Name, event.Op.String())
			pendingEvent = true

			// Reset or create debounce timer
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceDelay, func() {
				if pendingEvent {
					pendingEvent = false
					fmt.Printf("\n%s\n", colors.Success(">>> Change detected, re-running command..."))
					result := ExecuteCommandWithOptions(CommandExecutionOptions{
						CommandName:     commandName,
						Steps:           steps,
						WorkingDir:      workingDir,
						PassthroughArgs: passthroughArgs,
					})
					if result.Success {
						fmt.Printf("%s\n", colors.Success("✓ Command completed successfully"))
					} else {
						fmt.Printf("%s\n", colors.Warning("✗ Command failed"))
					}
					fmt.Printf("\n%s\n", colors.Info("Watching for changes..."))
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher error channel closed")
			}
			logger.Warnf("Watcher error: %v", err)
		}
	}
}

// getWatchDirectories returns directories to watch based on patterns
func getWatchDirectories(workingDir string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		// Default: watch current directory recursively
		return getRecursiveDirectories(workingDir)
	}

	// Collect unique directories from patterns
	dirMap := make(map[string]bool)
	for _, pattern := range patterns {
		// Extract directory part from pattern
		dir := filepath.Dir(pattern)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(workingDir, dir)
		}

		// Add directory and its parents up to working dir
		for {
			dirMap[dir] = true
			if dir == workingDir || dir == filepath.Dir(dir) {
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	// Convert map to slice
	dirs := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		// Check if directory exists
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			dirs = append(dirs, dir)
		}
	}

	return dirs, nil
}

// getRecursiveDirectories returns all directories recursively under the given directory
func getRecursiveDirectories(root string) ([]string, error) {
	var dirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't read
		}

		if info.IsDir() {
			// Skip common ignore patterns
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == ".beads" || base == "vendor" {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return dirs, nil
}

// shouldProcessEvent checks if file event should trigger command execution
func shouldProcessEvent(event fsnotify.Event, watchConfig *config.WatchConfig) bool {
	// Only process write, create, and remove events
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
		return false
	}

	filename := filepath.Base(event.Name)

	// Check ignore patterns
	for _, ignorePattern := range watchConfig.Ignore {
		if matched, _ := filepath.Match(ignorePattern, filename); matched {
			return false
		}
		// Also check full path matching for directory patterns
		if strings.Contains(ignorePattern, "/") || strings.Contains(ignorePattern, "**") {
			if matchesPattern(event.Name, ignorePattern) {
				return false
			}
		}
	}

	// If no patterns specified, accept all (after ignore check)
	if len(watchConfig.Patterns) == 0 {
		return true
	}

	// Check if file matches any pattern
	for _, pattern := range watchConfig.Patterns {
		if matched, _ := filepath.Match(filepath.Base(pattern), filename); matched {
			return true
		}
		// Also check full path matching for directory patterns
		if strings.Contains(pattern, "/") || strings.Contains(pattern, "**") {
			if matchesPattern(event.Name, pattern) {
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if path matches glob pattern (supports ** for recursive matching)
func matchesPattern(path, pattern string) bool {
	// Simple implementation - convert ** to wildcard
	pattern = strings.ReplaceAll(pattern, "**", "*")
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Also try matching just the filename part
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}
