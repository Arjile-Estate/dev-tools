package mocks

import (
	"dev-tools/internal/config"
	"dev-tools/internal/executor"

	"github.com/stretchr/testify/mock"
)

// Executor is a mock type for the Executor type
type Executor struct {
	mock.Mock
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
