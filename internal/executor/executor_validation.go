package executor

import (
	"fmt"
	"dev-tools/internal/logger"
	"os"
	"path/filepath"
)

// validateAndResolveDirectory validates and resolves a directory path
// It handles relative paths, checks existence, and verifies accessibility
// Returns ValidationError for consistency in error handling
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
			return "", NewValidationError("directory", stepDir, fmt.Errorf("does not exist"))
		}
		return "", NewValidationError("directory", stepDir, fmt.Errorf("not accessible: %w", err))
	}

	if !info.IsDir() {
		return "", NewValidationError("directory", stepDir, fmt.Errorf("path is not a directory"))
	}

	// Test directory accessibility
	if _, err := os.ReadDir(stepDir); err != nil {
		return "", NewValidationError("directory", stepDir, fmt.Errorf("not accessible: %w", err))
	}

	logger.Infof("Using directory: %s", stepDir)
	return stepDir, nil
}
