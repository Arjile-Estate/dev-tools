package mocks

import (
	"context"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"

	"github.com/stretchr/testify/mock"
)

// Executor is a mock type for the Executor type
type Executor struct {
	mock.Mock
}

// ExecuteCommandWithOptions provides a mock function with given fields: opts
func (_m *Executor) ExecuteCommandWithOptions(opts executor.CommandExecutionOptions) executor.ExecutionResult {
	ret := _m.Called(opts)

	var r0 executor.ExecutionResult
	if rf, ok := ret.Get(0).(func(executor.CommandExecutionOptions) executor.ExecutionResult); ok {
		r0 = rf(opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(executor.ExecutionResult)
		}
	}

	return r0
}

// ExecuteCommandWithSteps provides a mock function with given fields: commandName, steps, projectDir, passthroughArgs
func (_m *Executor) ExecuteCommandWithSteps(commandName string, steps []config.CommandStep, projectDir string, passthroughArgs []string) executor.ExecutionResult {
	ret := _m.Called(commandName, steps, projectDir, passthroughArgs)

	var r0 executor.ExecutionResult
	if rf, ok := ret.Get(0).(func(string, []config.CommandStep, string, []string) executor.ExecutionResult); ok {
		r0 = rf(commandName, steps, projectDir, passthroughArgs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(executor.ExecutionResult)
		}
	}

	return r0
}

// LoadEnvironmentVariables provides a mock function with given fields: envFile
func (_m *Executor) LoadEnvironmentVariables(envFile string) error {
	ret := _m.Called(envFile)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(envFile)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WatchAndExecute provides a mock function with given fields: ctx, commandName, steps, workingDir, passthroughArgs
func (_m *Executor) WatchAndExecute(ctx context.Context, commandName string, steps []config.CommandStep, workingDir string, passthroughArgs []string) error {
	ret := _m.Called(ctx, commandName, steps, workingDir, passthroughArgs)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []config.CommandStep, string, []string) error); ok {
		r0 = rf(ctx, commandName, steps, workingDir, passthroughArgs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
