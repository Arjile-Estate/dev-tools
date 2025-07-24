package executor

import "dev-tools/internal/config"

//go:generate mockery --name Executor --output ../mocks --outpkg mocks
type Executor interface {
	ExecuteCommandWithSteps(commandName string, steps []config.CommandStep, projectDir string) ExecutionResult
	LoadEnvironmentVariables(envFile string) error
}
