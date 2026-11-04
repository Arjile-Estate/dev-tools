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
func executeWithRetry(ctx context.Context, step config.CommandStep, command, executionDir, commandName string, captureOutput bool, environment map[string]string) ExecutionResult {
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
			logger.Warnf("Invalid retry_delay '%s', using 1s: %v", step.RetryDelay, err)
		}
	}

	var result ExecutionResult
	var stdout, stderr string
	var attempted []string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Infof("Retry attempt %d/%d after %v delay", attempt, maxAttempts, retryDelay)
			if delayErr := waitForRetryDelay(ctx, retryDelay); delayErr != nil {
				result = ExecutionResult{Success: false, ReturnCode: -1}
				if delayErr == context.DeadlineExceeded {
					result.TimedOut = true
					result.Stderr = "command timeout exceeded during retry delay"
				} else {
					result.Cancelled = true
					result.Stderr = "command cancelled during retry delay"
				}
				if captureOutput {
					stderr += result.Stderr
				}
				break
			}
		}

		result = ExecuteShellCommand(ctx, ExecuteOptions{
			Command:       command,
			Background:    step.Background,
			CaptureOutput: step.Background || captureOutput,
			WorkingDir:    executionDir,
			Daemon:        step.Daemon,
			CommandName:   commandName,
			Environment:   environment,
		})
		attempted = append(attempted, command)
		if captureOutput {
			stdout += result.Stdout
			stderr += result.Stderr
		}

		if result.Success {
			break
		}
		if result.Cancelled || result.TimedOut {
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
				logger.Warnf("Exit code %d not in retry list, not retrying", result.ReturnCode)
			}
			break
		}
	}
	if captureOutput {
		result.Stdout = stdout
		result.Stderr = stderr
	}
	result.AttemptedCommands = attempted
	result.Attempts = len(attempted)

	return result
}
