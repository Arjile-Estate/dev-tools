package executor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// validateAndResolveDirectory validates and resolves a directory path
// It handles relative paths, checks existence, and verifies accessibility
func validateAndResolveDirectory(stepDir, workingDir string) (string, error) {
	if stepDir == "" {
		return workingDir, nil
	}

	// Make path absolute if relative
	if !filepath.IsAbs(stepDir) && workingDir != "" {
		stepDir = filepath.Join(workingDir, stepDir)
	}

	// Validate directory exists and is accessible
	info, err := os.Stat(stepDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory '%s' does not exist", stepDir)
		}
		return "", fmt.Errorf("directory '%s' is not accessible: %w", stepDir, err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path '%s' is not a directory", stepDir)
	}

	// Test directory accessibility
	if _, err := os.ReadDir(stepDir); err != nil {
		return "", fmt.Errorf("directory '%s' is not accessible: %w", stepDir, err)
	}

	log.Printf("Using directory: %s", stepDir)
	return stepDir, nil
}
