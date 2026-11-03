package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"dev-tools/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunCommandDevResultOptInIsPerCommand(t *testing.T) {
	t.Setenv(claudeCodeEnvVar, "")
	projectDir := t.TempDir()
	mockLoader := new(mocks.ConfigLoader)
	mockExecutor := new(mocks.Executor)
	SetConfigLoader(mockLoader)
	SetExecutor(mockExecutor)
	t.Cleanup(func() {
		SetConfigLoader(nil)
		SetExecutor(nil)
	})

	cfg := &config.Config{
		Commands: map[string][]config.CommandStep{
			"test":  {{Run: config.RunCommand{"go test ./..."}}},
			"build": {{Run: config.RunCommand{"go build ./..."}}},
		},
		DevResult: &config.DevResultConfig{Commands: map[string]config.DevResultCommandConfig{
			"test": {Enabled: true},
		}},
	}
	mockLoader.On("LoadConfig", projectDir).Return(cfg, nil).Twice()
	mockExecutor.On("LoadEnvironmentVariables", filepath.Join(projectDir, ".env")).Return(nil).Twice()
	mockExecutor.On("ExecuteCommandWithOptions", mock.MatchedBy(func(opts executor.CommandExecutionOptions) bool {
		return opts.CommandName == "test" && opts.CaptureOutput
	})).Return(executor.ExecutionResult{
		Success: true, CommandName: "test", Stdout: "184 tests passed\n", Attempts: 1,
	}).Once()
	mockExecutor.On("ExecuteCommandWithOptions", mock.MatchedBy(func(opts executor.CommandExecutionOptions) bool {
		return opts.CommandName == "build" && !opts.CaptureOutput
	})).Return(executor.ExecutionResult{
		Success: true, CommandName: "build", Stdout: "legacy build output\n",
	}).Once()

	var testOutput bytes.Buffer
	testCmd := NewRootCommand()
	testCmd.SetOut(&testOutput)
	testCmd.SetArgs([]string{"--project-dir", projectDir, "test"})
	require.NoError(t, testCmd.Execute())
	assert.Contains(t, testOutput.String(), "DEV-RESULT v1 PASS")
	resultFiles, err := filepath.Glob(filepath.Join(projectDir, ".build", "agent", "results", "test-*.json"))
	require.NoError(t, err)
	require.Len(t, resultFiles, 1)

	var buildOutput bytes.Buffer
	buildCmd := NewRootCommand()
	buildCmd.SetOut(&buildOutput)
	buildCmd.SetArgs([]string{"--project-dir", projectDir, "build"})
	require.NoError(t, buildCmd.Execute())
	assert.Equal(t, "legacy build output\n", buildOutput.String())
	buildFiles, err := filepath.Glob(filepath.Join(projectDir, ".build", "agent", "results", "build-*.json"))
	require.NoError(t, err)
	assert.Empty(t, buildFiles)

	mockLoader.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
}

func TestRunCommandDevResultPreservesFailureExitCodeAndRedactsArtifacts(t *testing.T) {
	t.Setenv(claudeCodeEnvVar, "")
	t.Setenv("BUILD_API_TOKEN", "secret-demo-12345")
	projectDir := t.TempDir()
	mockLoader := new(mocks.ConfigLoader)
	mockExecutor := new(mocks.Executor)
	SetConfigLoader(mockLoader)
	SetExecutor(mockExecutor)
	t.Cleanup(func() {
		SetConfigLoader(nil)
		SetExecutor(nil)
	})

	cfg := &config.Config{
		Commands: map[string][]config.CommandStep{"build": {{Run: config.RunCommand{"go build ./..."}}}},
		DevResult: &config.DevResultConfig{Commands: map[string]config.DevResultCommandConfig{
			"build": {Enabled: true},
		}},
	}
	mockLoader.On("LoadConfig", projectDir).Return(cfg, nil)
	mockExecutor.On("LoadEnvironmentVariables", filepath.Join(projectDir, ".env")).Return(nil)
	mockExecutor.On("ExecuteCommandWithOptions", mock.Anything).Return(executor.ExecutionResult{
		Success: false, CommandName: "build", FailedCommand: "go build ./...", ReturnCode: 42,
		Stderr: "Authorization: Bearer secret-demo-12345\nparser.go:84: error: expected identifier\n", Attempts: 1,
	})

	var output bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetOut(&output)
	rootCmd.SetArgs([]string{"--project-dir", projectDir, "build"})
	err := rootCmd.Execute()
	require.Error(t, err)
	var exitErr *ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 42, exitErr.Code)
	assert.Contains(t, output.String(), "DEV-RESULT v1 FAIL")
	assert.NotContains(t, output.String(), "secret-demo-12345")

	resultFiles, globErr := filepath.Glob(filepath.Join(projectDir, ".build", "agent", "results", "build-*.json"))
	require.NoError(t, globErr)
	require.Len(t, resultFiles, 1)
	resultData, readErr := os.ReadFile(resultFiles[0])
	require.NoError(t, readErr)
	assert.NotContains(t, string(resultData), "secret-demo-12345")
	logFiles, globErr := filepath.Glob(filepath.Join(projectDir, ".build", "agent", "logs", "build-*.log"))
	require.NoError(t, globErr)
	require.Len(t, logFiles, 1)
	logData, readErr := os.ReadFile(logFiles[0])
	require.NoError(t, readErr)
	assert.NotContains(t, string(logData), "secret-demo-12345")
}
