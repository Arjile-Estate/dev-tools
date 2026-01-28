package executor

import (
	"context"
	"dev-tools/internal/logger"
	"time"

	"dev-tools/internal/config"
)

// executeWithRetry executes a command with retry logic based on step configuration
// Supports configurable retry attempts, delays, and exit code filtering
// Context allows for cancellation and timeouts
func executeWithRetry(ctx context.Context, step config.CommandStep, command, executionDir, commandName string) ExecutionResult {
	maxAttempts := step.Retry + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Parse retry delay
	retryDelay := time.Second
	if step.RetryDelay != "" {
		if parsed, err := time.ParseDuration(step.RetryDelay); err == nil {
			retryDelay = parsed
		} else {
			logger.Infof("Invalid retry_delay '%s', using 1s: %v", step.RetryDelay, err)
		}
	}

	var result ExecutionResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Infof("Retry attempt %d/%d after %v delay", attempt, maxAttempts, retryDelay)
			time.Sleep(retryDelay)
		}

		result = ExecuteShellCommand(ctx, ExecuteOptions{
			Command:       command,
			Background:    step.Background,
			CaptureOutput: step.Background,
			WorkingDir:    executionDir,
			Daemon:        step.Daemon,
			CommandName:   commandName,
		})

		if result.Success {
			break
		}

		// Check if we should retry based on exit code
		shouldRetry := len(step.RetryOnExitCodes) == 0
		if !shouldRetry {
			for _, code := range step.RetryOnExitCodes {
				if result.ReturnCode == code {
					shouldRetry = true
					break
				}
			}
		}

		if attempt >= maxAttempts || !shouldRetry {
			if !shouldRetry {
				logger.Infof("Exit code %d not in retry list, not retrying", result.ReturnCode)
			}
			break
		}
	}

	return result
}
