package executor

import (
	"context"

	"dev-tools/internal/config"
)

// Executor defines the interface for command execution operations.
// This interface enables dependency injection and mocking in tests.
// It includes the core operations needed by the CLI layer.
//
//go:generate mockery --name Executor --output ../mocks --outpkg mocks
type Executor interface {
	// ExecuteCommandWithOptions executes a command using the options struct pattern
	ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult

	// LoadEnvironmentVariables loads environment variables from a .env file
	LoadEnvironmentVariables(envFile string) error

	// WatchAndExecute watches files and re-executes command on changes
	WatchAndExecute(ctx context.Context, commandName string, steps []config.CommandStep, workingDir string, passthroughArgs []string) error
}
