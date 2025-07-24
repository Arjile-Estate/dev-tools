package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"dev-tools/internal/mocks"
)

func TestRunCommand_WithMocks(t *testing.T) {
	// Create mock loader and executor
	mockLoader := new(mocks.ConfigLoader)
	mockExecutor := new(mocks.Executor)

	// Create a new root command
	rootCmd := NewRootCommand()

	// Set the mock loader and executor
	SetConfigLoader(mockLoader)
	SetExecutor(mockExecutor)

	// Set up the mock loader to return a specific config
	expectedConfig := &config.Config{
		Commands: map[string][]config.CommandStep{
			"test": {
				{Run: config.RunCommand{"go test ./..."}},
			},
		},
	}
	mockLoader.On("LoadConfig", ".").Return(expectedConfig, nil)

	// Set up the mock executor to return a successful result
	mockExecutor.On("ExecuteCommandWithSteps", "test", mock.Anything, ".").Return(executor.ExecutionResult{Success: true})
	mockExecutor.On("LoadEnvironmentVariables", mock.Anything).Return(nil)

	// Execute the command
	b := new(bytes.Buffer)
	rootCmd.SetOut(b)
	rootCmd.SetArgs([]string{"test"})
	err := rootCmd.Execute()

	// Assert that the command was successful
	assert.NoError(t, err)

	// Assert that the mocks were called
	mockLoader.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}
