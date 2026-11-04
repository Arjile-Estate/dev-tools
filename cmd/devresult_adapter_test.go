package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"dev-tools/internal/config"
	"dev-tools/internal/devresult"
	"dev-tools/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	arj3539HelperEnvironment = "ARJ3539_ADAPTER_HELPER"
	arj3539RunIDEnvironment  = "DEV_TOOLS_RUN_ID"
	arj3539RunDirEnvironment = "DEV_TOOLS_RUN_DIR"
)

type arj3539ConfigLoader struct{ cfg *config.Config }

func (l arj3539ConfigLoader) LoadConfig(string) (*config.Config, error) { return l.cfg, nil }

type arj3539Executor struct {
	mu   sync.Mutex
	envs []map[string]string
	run  func(executor.CommandExecutionOptions) executor.ExecutionResult
}

func (e *arj3539Executor) ExecuteCommandWithOptions(opts executor.CommandExecutionOptions) executor.ExecutionResult {
	environment := make(map[string]string, len(opts.Environment))
	for key, value := range opts.Environment {
		environment[key] = value
	}
	e.mu.Lock()
	e.envs = append(e.envs, environment)
	e.mu.Unlock()
	if e.run != nil {
		return e.run(opts)
	}
	return executor.ExecutionResult{Success: true, CommandName: opts.CommandName, Attempts: 1}
}

func (*arj3539Executor) LoadEnvironmentVariables(string) error { return nil }

func (*arj3539Executor) WatchAndExecute(context.Context, string, []config.CommandStep, string, []string) error {
	return nil
}

func (e *arj3539Executor) environments() []map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]map[string]string, len(e.envs))
	copy(result, e.envs)
	return result
}

func TestARJ3539AdapterHelperProcess(t *testing.T) {
	if os.Getenv(arj3539HelperEnvironment) != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(91)
	}
	arguments := os.Args[separator+1:]
	mode := arguments[0]
	var request devresult.DiagnosticsRequest
	_ = json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&request)
	encode := func(response devresult.DiagnosticsResponse) {
		_ = json.NewEncoder(os.Stdout).Encode(response)
	}
	response := func(state devresult.EvidenceState) devresult.DiagnosticsResponse {
		return devresult.DiagnosticsResponse{Schema: devresult.DiagnosticsResponseSchemaV1, EvidenceState: state}
	}

	switch mode {
	case "sources":
		var failures []devresult.Diagnostic
		_ = filepath.WalkDir(request.RunDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".xml" {
				return nil
			}
			name := filepath.Base(path)
			failures = append(failures, devresult.Diagnostic{Identity: "fixture/" + name, Message: name})
			return nil
		})
		sort.Slice(failures, func(i, j int) bool { return failures[i].Identity < failures[j].Identity })
		value := response(devresult.EvidenceComplete)
		value.Failures = failures
		encode(value)
	case "capture":
		if len(arguments) < 2 {
			os.Exit(92)
		}
		data, _ := json.Marshal(request)
		_ = os.WriteFile(arguments[1], data, 0o600)
		encode(response(devresult.EvidenceComplete))
	case "literal":
		if len(arguments) < 3 {
			os.Exit(93)
		}
		_ = os.WriteFile(arguments[2], []byte(arguments[1]), 0o600)
		encode(response(devresult.EvidenceComplete))
	case "sleep":
		time.Sleep(3 * time.Second)
	case "oversized":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("adapter-stderr-", 6*1024))
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("adapter-stdout-", 140*1024))
	case "complete":
		value := response(devresult.EvidenceComplete)
		value.Failures = []devresult.Diagnostic{
			{Identity: "checkout/rejects-expired-card", Message: "checkout rejects an expired card"},
			{Identity: "rent/rounds-annual-revision", Message: "rent rounds the annual revision"},
		}
		encode(value)
	case "partial":
		value := response(devresult.EvidencePartial)
		value.Failures = []devresult.Diagnostic{{Identity: "statement/export", Message: "exports the annual statement failed", File: "statement.go", Line: 73}}
		value.Problems = []devresult.EvidenceProblem{{Message: "billing.xml is missing"}}
		encode(value)
	case "malformed":
		_, _ = fmt.Fprint(os.Stdout, "{broken-json")
	case "v2":
		value := response(devresult.EvidenceComplete)
		value.Schema = "dev-tools/diagnostics-response/v2"
		encode(value)
	case "secret":
		value := response(devresult.EvidenceComplete)
		value.Failures = []devresult.Diagnostic{{Identity: "checkout/token", Message: "token secret-demo-12345 rejected"}}
		value.Sources = []devresult.DiagnosticSource{{Kind: "junit", Path: "checkout.xml"}}
		encode(value)
	case "traversal":
		value := response(devresult.EvidenceComplete)
		value.Sources = []devresult.DiagnosticSource{{Kind: "junit", Path: "../test-010/checkout.xml"}}
		encode(value)
	case "symlink":
		value := response(devresult.EvidenceComplete)
		value.Sources = []devresult.DiagnosticSource{{Kind: "junit", Path: "current.xml"}}
		encode(value)
	case "retained":
		value := response(devresult.EvidenceComplete)
		value.Failures = []devresult.Diagnostic{{Identity: "checkout/retained", Message: "checkout retained failure"}}
		value.Sources = []devresult.DiagnosticSource{{Kind: "junit", Path: "checkout.xml"}}
		encode(value)
	case "portable":
		value := response(devresult.EvidencePartial)
		value.Failures = []devresult.Diagnostic{{Identity: "checkout/portable", Message: "checkout failure"}}
		value.Problems = []devresult.EvidenceProblem{{Message: "missing-suite warning"}}
		encode(value)
	case "performance":
		value := response(devresult.EvidenceComplete)
		value.Failures = []devresult.Diagnostic{{Identity: "checkout/fixture", Message: "checkout fixture has 12 tests"}}
		value.Sources = []devresult.DiagnosticSource{{Kind: "junit", Path: "checkout.xml"}}
		encode(value)
	default:
		os.Exit(94)
	}
	os.Exit(0)
}

func TestRunCommandDiagnostics(t *testing.T) {
	t.Setenv(claudeCodeEnvVar, "")
	t.Setenv(arj3539HelperEnvironment, "1")

	t.Run("R1.E1_isolates_concurrent_run_evidence", func(t *testing.T) {
		projectDir := t.TempDir()
		adapter := arj3539Adapter("sources")
		cfg := arj3539Config(adapter, true)
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			name := "checkout.xml"
			if opts.CommandName == "test:focused" {
				name = "rent.xml"
			}
			if err := arj3539WriteRunFile(opts, name, "<testsuite/>"); err != nil {
				return arj3539Failure(opts.CommandName, 1, err.Error())
			}
			return arj3539Failure(opts.CommandName, 1, "assertion failed")
		}}
		arj3539Install(t, cfg, runner)

		type commandResult struct {
			name, output string
			err          error
		}
		results := make(chan commandResult, 2)
		for _, name := range []string{"test", "test:focused"} {
			go func(commandName string) {
				output, err := arj3539RunRoot(projectDir, commandName)
				results <- commandResult{name: commandName, output: output, err: err}
			}(name)
		}
		seen := map[string]commandResult{}
		for range 2 {
			result := <-results
			seen[result.name] = result
		}
		require.Error(t, seen["test"].err)
		assert.Contains(t, seen["test"].output, "checkout.xml")
		assert.NotContains(t, seen["test"].output, "rent.xml")
		require.Error(t, seen["test:focused"].err)
		assert.Contains(t, seen["test:focused"].output, "rent.xml")
		assert.NotContains(t, seen["test:focused"].output, "checkout.xml")
		environments := runner.environments()
		require.Len(t, environments, 2)
		assert.NotEqual(t, environments[0][arj3539RunIDEnvironment], environments[1][arj3539RunIDEnvironment])
		assert.NotEqual(t, environments[0][arj3539RunDirEnvironment], environments[1][arj3539RunDirEnvironment])
	})

	t.Run("R1.E2_skips_adapter_without_matching_evidence", func(t *testing.T) {
		projectDir := t.TempDir()
		counter := filepath.Join(projectDir, "adapter-counter")
		adapter := arj3539Adapter("capture", counter)
		adapter.StartWhen = []string{"junit/**/*.xml", "turbo-summary.json"}
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			require.NoError(t, arj3539WriteRunFile(opts, "startup.log", "startup failed"))
			return arj3539Failure(opts.CommandName, 2, "first rendered startup error")
		}}
		arj3539Install(t, arj3539Config(adapter, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 2)
		assert.Contains(t, output, "first rendered startup error")
		_, statErr := os.Stat(counter)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})

	t.Run("R1.E3_preserves_opted-in_command_without_adapter", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return executor.ExecutionResult{Success: true, CommandName: opts.CommandName, ReturnCode: 0, Attempts: 1,
				Stdout: "184 tests selected\ncoverage: 87.5%\ncache hits: 184\n"}
		}}
		arj3539Install(t, arj3539Config(nil, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		require.NoError(t, err)
		assert.Contains(t, output, "DEV-RESULT v1 PASS")
		retained := arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent"))
		assert.Contains(t, retained, "184 tests selected")
		assert.Contains(t, retained, "coverage: 87.5%")
		assert.Contains(t, retained, "cache hits: 184")
		require.Len(t, runner.environments(), 1)
		assert.NotEmpty(t, runner.environments()[0][arj3539RunDirEnvironment])
	})

	t.Run("R1.E4_preserves_non-opted-in_command", func(t *testing.T) {
		projectDir := t.TempDir()
		lines := make([]string, 37)
		for index := range lines {
			lines[index] = fmt.Sprintf("existing line %02d", index+1)
		}
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539FailureWithOutput(opts.CommandName, 6, strings.Join(lines, "\n")+"\n", "")
		}}
		arj3539Install(t, arj3539Config(nil, false, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 6)
		assert.Equal(t, strings.Join(lines, "\n")+"\n", output)
		require.Len(t, runner.environments(), 1)
		assert.Empty(t, runner.environments()[0][arj3539RunDirEnvironment])
		assert.NoDirExists(t, filepath.Join(projectDir, ".build", "agent", "runs"))
		assert.NotContains(t, arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent")), "diagnostics-response")
	})

	t.Run("R2.E1_exchanges_version_1_JSON", func(t *testing.T) {
		projectDir := t.TempDir()
		capture := filepath.Join(projectDir, "request.json")
		adapter := arj3539Adapter("capture", capture)
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 4, "fallback error"))
		arj3539Install(t, arj3539Config(adapter, false), runner)
		_, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 4)
		data, readErr := os.ReadFile(capture)
		require.NoError(t, readErr)
		var request devresult.DiagnosticsRequest
		require.NoError(t, json.Unmarshal(data, &request))
		assert.Equal(t, devresult.DiagnosticsRequestSchemaV1, request.Schema)
		assert.Equal(t, "test", request.Command)
		assert.NotEmpty(t, request.RunID)
		assert.NotEmpty(t, request.RunDir)
		assert.Contains(t, arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent")), devresult.DiagnosticsResponseSchemaV1)
	})

	t.Run("R2.E2_invokes_adapter_without_a_shell", func(t *testing.T) {
		projectDir := t.TempDir()
		argumentCapture := filepath.Join(projectDir, "argument")
		literal := "report; touch marker.txt"
		adapter := arj3539Adapter("literal", literal, argumentCapture)
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 1, "fallback error"))
		arj3539Install(t, arj3539Config(adapter, false), runner)
		_, _ = arj3539RunRoot(projectDir, "test")
		captured, err := os.ReadFile(argumentCapture)
		require.NoError(t, err)
		assert.Equal(t, literal, string(captured))
		assert.NoFileExists(t, filepath.Join(projectDir, "marker.txt"))
	})

	t.Run("R2.E3_bounds_adapter_runtime", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 8, "first assertion error"))
		arj3539Install(t, arj3539Config(arj3539Adapter("sleep"), false), runner)
		started := time.Now()
		output, err := arj3539RunRoot(projectDir, "test")
		elapsed := time.Since(started)
		arj3539RequireExitCode(t, err, 8)
		assert.Less(t, elapsed, 2900*time.Millisecond)
		assert.Contains(t, strings.ToLower(output), "adapter timeout")
		assert.Contains(t, output, "first assertion error")
	})

	t.Run("R2.E4_enforces_adapter_resource_limits", func(t *testing.T) {
		projectDir := t.TempDir()
		adapter := arj3539Adapter("oversized")
		runner := arj3539EvidenceExecutor(t, "oversized.xml", strings.Repeat("source-data", 1_100_000), arj3539Failure("test", 7, "resource fallback error"))
		arj3539Install(t, arj3539Config(adapter, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 7)
		assert.Contains(t, strings.ToLower(output), "standard output")
		assert.Contains(t, output, "resource fallback error")
		retained := arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent"))
		assert.LessOrEqual(t, strings.Count(retained, "adapter-stderr-"), (64<<10)/len("adapter-stderr-")+1)
		assert.NotContains(t, retained, strings.Repeat("source-data", 1_100_000))
	})

	t.Run("R3.E1_uses_complete_structured_evidence", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 42, "rendered fallback must not win"))
		arj3539Install(t, arj3539Config(arj3539Adapter("complete"), false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 42)
		assert.Equal(t, 1, strings.Count(output, "checkout rejects an expired card"))
		assert.Equal(t, 1, strings.Count(output, "rent rounds the annual revision"))
		assert.NotContains(t, output, "rendered fallback must not win")
	})

	t.Run("R3.E2_uses_partial_structured_evidence", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "statement.xml", "<testsuite/>", arj3539Failure("test", 17, "fallback"))
		arj3539Install(t, arj3539Config(arj3539Adapter("partial"), false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 17)
		assert.Contains(t, output, "exports the annual statement failed")
		assert.Contains(t, output, "statement.go:73")
		assert.Contains(t, output, "billing.xml is missing")
	})

	t.Run("R3.E3_falls_back_when_adapter_cannot_start", func(t *testing.T) {
		projectDir := t.TempDir()
		adapter := &config.DiagnosticsAdapterConfig{Command: []string{filepath.Join(projectDir, "missing-node")}, StartWhen: []string{"*.xml"}}
		runner := arj3539EvidenceExecutor(t, "compile.xml", "<testsuite/>", arj3539Failure("test", 9, "rendered compilation error"))
		arj3539Install(t, arj3539Config(adapter, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 9)
		assert.Contains(t, strings.ToLower(output), "structured diagnostics are unavailable")
		assert.Contains(t, output, "rendered compilation error")
	})

	t.Run("R3.E4_keeps_primary_success_on_malformed_response", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite tests=\"184\"/>", executor.ExecutionResult{Success: true, CommandName: "test", Attempts: 1, Stdout: "184 tests passed"})
		arj3539Install(t, arj3539Config(arj3539Adapter("malformed"), false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		require.NoError(t, err)
		assert.Contains(t, output, "DEV-RESULT v1 PASS")
		assert.Contains(t, strings.ToLower(output), "malformed")
		resultData, readErr := os.ReadFile(filepath.Join(projectDir, ".build", "agent", "results", arj3539OnlyJob(t, projectDir)+".json"))
		require.NoError(t, readErr)
		var stored devresult.Result
		require.NoError(t, json.Unmarshal(resultData, &stored))
		assert.Empty(t, stored.Failures)
	})

	t.Run("R4.E1_redacts_every_retained_artifact", func(t *testing.T) {
		t.Setenv("ARJ3539_TOKEN", "secret-demo-12345")
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<failure token=\"secret-demo-12345\"/>", arj3539Failure("test", 10, "token secret-demo-12345 failed"))
		arj3539Install(t, arj3539Config(arj3539Adapter("secret"), false), runner)
		_, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 10)
		retained := arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent"))
		assert.NotContains(t, retained, "secret-demo-12345")
		assert.Contains(t, retained, devresult.RedactionMarker)
	})

	t.Run("R4.E2_rejects_parent_traversal_source", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			runDir := opts.Environment[arj3539RunDirEnvironment]
			if runDir == "" {
				return arj3539Failure("test", 11, "DEV_TOOLS_RUN_DIR was not injected")
			}
			foreign := filepath.Join(filepath.Dir(runDir), "test-010", "checkout.xml")
			require.NoError(t, os.MkdirAll(filepath.Dir(foreign), 0o700))
			require.NoError(t, os.WriteFile(foreign, []byte("foreign-test-010-evidence"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(runDir, "current.xml"), []byte("current"), 0o600))
			return arj3539Failure("test", 11, "safe fallback for traversal")
		}}
		adapter := arj3539Adapter("traversal")
		adapter.StartWhen = []string{"current.xml"}
		arj3539Install(t, arj3539Config(adapter, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 11)
		assert.Contains(t, strings.ToLower(output), "unsafe structured evidence")
		assert.Contains(t, output, "safe fallback for traversal")
		assert.NotContains(t, arj3539ReadRetainedResults(t, projectDir), "foreign-test-010-evidence")
	})

	t.Run("R4.E3_rejects_symlink_source", func(t *testing.T) {
		projectDir := t.TempDir()
		foreign := filepath.Join(t.TempDir(), "foreign.xml")
		require.NoError(t, os.WriteFile(foreign, []byte("foreign-symlink-evidence"), 0o600))
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			runDir := opts.Environment[arj3539RunDirEnvironment]
			if runDir == "" {
				return arj3539Failure("test", 12, "DEV_TOOLS_RUN_DIR was not injected")
			}
			require.NoError(t, os.Symlink(foreign, filepath.Join(runDir, "current.xml")))
			return arj3539Failure("test", 12, "safe fallback for symlink")
		}}
		adapter := arj3539Adapter("symlink")
		adapter.StartWhen = []string{"current.xml"}
		arj3539Install(t, arj3539Config(adapter, false), runner)
		output, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 12)
		assert.Contains(t, strings.ToLower(output), "unsafe structured evidence")
		assert.NotContains(t, arj3539ReadRetainedResults(t, projectDir), "foreign-symlink-evidence")
	})

	t.Run("R4.E4_drill-down_does_not_rerun_validation", func(t *testing.T) {
		projectDir := t.TempDir()
		validationCount := 0
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			validationCount++
			require.NoError(t, arj3539WriteRunFile(opts, "checkout.xml", "<testsuite name=\"checkout\"/>"))
			stdout := make([]string, 200)
			stderr := make([]string, 40)
			for index := range stdout {
				stdout[index] = fmt.Sprintf("stdout-%03d", index+1)
			}
			for index := range stderr {
				stderr[index] = fmt.Sprintf("stderr-%03d", index+1)
			}
			return arj3539FailureWithOutput("test", 13, strings.Join(stdout, "\n"), strings.Join(stderr, "\n"))
		}}
		arj3539Install(t, arj3539Config(arj3539Adapter("retained"), false), runner)
		_, err := arj3539RunRoot(projectDir, "test")
		arj3539RequireExitCode(t, err, 13)
		job := arj3539OnlyJob(t, projectDir)
		full, retrieveErr := arj3539RunRoot(projectDir, "result", job, "--full")
		require.NoError(t, retrieveErr)
		assert.Contains(t, full, "stdout-200")
		assert.Contains(t, full, "stderr-040")
		assert.Contains(t, full, devresult.DiagnosticsResponseSchemaV1)
		assert.Contains(t, full, "testsuite name=\"checkout\"")
		assert.Equal(t, 1, validationCount)
	})

	t.Run("R5.E1_rejects_version_mismatch_deterministically", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 14, "stable rendered fallback"))
		arj3539Install(t, arj3539Config(arj3539Adapter("v2"), false), runner)
		for range 2 {
			output, err := arj3539RunRoot(projectDir, "test")
			arj3539RequireExitCode(t, err, 14)
			assert.Contains(t, output, "expected version 1")
			assert.Contains(t, output, "received version 2")
			assert.Equal(t, 1, strings.Count(output, "stable rendered fallback"))
		}
	})

	t.Run("R5.E2_conformance_fixture_is_portable", func(t *testing.T) {
		for _, repository := range []string{"dev-tools fixture", "Atelier Fictif copy"} {
			t.Run(repository, func(t *testing.T) {
				projectDir := t.TempDir()
				runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 15, "fallback"))
				arj3539Install(t, arj3539Config(arj3539Adapter("portable"), false), runner)
				output, err := arj3539RunRoot(projectDir, "test")
				arj3539RequireExitCode(t, err, 15)
				assert.Equal(t, 1, strings.Count(output, "checkout failure"))
				assert.Equal(t, 1, strings.Count(output, "missing-suite warning"))
			})
		}
	})

	t.Run("R5.E3_meets_reference_p95_budget", func(t *testing.T) {
		projectDir := t.TempDir()
		run, err := devresult.CreateValidationRun(projectDir, "test", time.Now())
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(run.Dir, "checkout.xml"), []byte("<testsuite tests=\"12\"/>"), 0o600))
		durations := make([]time.Duration, 0, 100)
		for range 100 {
			started := time.Now()
			result := devresult.ExecuteAdapter(context.Background(), devresult.AdapterOptions{
				Argv:    arj3539AdapterCommand("performance"),
				Request: devresult.DiagnosticsRequest{Schema: devresult.DiagnosticsRequestSchemaV1, RunID: run.ID, RunDir: run.Dir, Command: "test"},
				Limits:  devresult.DefaultAdapterLimits(),
			})
			require.NoError(t, result.Err)
			require.NotNil(t, result.Response)
			assert.Contains(t, result.Response.Failures[0].Message, "12 tests")
			durations = append(durations, time.Since(started))
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		assert.Less(t, durations[94], 100*time.Millisecond, "p95 was %s", durations[94])
	})

	t.Run("R5.E4_honors_retention_leases", func(t *testing.T) {
		projectDir := t.TempDir()
		now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		arj3539WriteRetentionJob(t, projectDir, "test-015", now.Add(-8*24*time.Hour))
		arj3539WriteRetentionJob(t, projectDir, "test-016", now.Add(-12*24*time.Hour))
		leaseDir := filepath.Join(projectDir, ".build", "agent", "leases")
		require.NoError(t, os.MkdirAll(leaseDir, 0o700))
		lease := fmt.Sprintf(`{"schema":"dev-tools/retention-lease/v1","owner":"build-42","run_ids":["test-016"],"expires_at":%q}`, now.Add(time.Hour).Format(time.RFC3339Nano))
		require.NoError(t, os.WriteFile(filepath.Join(leaseDir, "build-42.json"), []byte(lease), 0o600))
		report, err := devresult.ApplyRetention(projectDir, "", now, devresult.RetentionPolicy{MaxAge: 7 * 24 * time.Hour, MaxBytes: 1 << 30})
		require.NoError(t, err)
		assert.Contains(t, report.Removed, "test-015")
		assert.NoFileExists(t, filepath.Join(projectDir, ".build", "agent", "results", "test-015.json"))
		assert.FileExists(t, filepath.Join(projectDir, ".build", "agent", "results", "test-016.json"))
	})
}

func arj3539Adapter(mode string, arguments ...string) *config.DiagnosticsAdapterConfig {
	return &config.DiagnosticsAdapterConfig{
		Command:   arj3539AdapterCommand(mode, arguments...),
		StartWhen: []string{"*.xml"},
	}
}

func arj3539AdapterCommand(mode string, arguments ...string) []string {
	command := []string{os.Args[0], "-test.run=^TestARJ3539AdapterHelperProcess$", "--", mode}
	return append(command, arguments...)
}

func arj3539Config(adapter *config.DiagnosticsAdapterConfig, focused bool, enabled ...bool) *config.Config {
	devResultEnabled := true
	if len(enabled) > 0 {
		devResultEnabled = enabled[0]
	}
	commands := map[string][]config.CommandStep{"test": {{Run: config.RunCommand{"fixture test"}}}}
	resultCommands := map[string]config.DevResultCommandConfig{}
	if devResultEnabled {
		resultCommands["test"] = config.DevResultCommandConfig{Enabled: true, DiagnosticsAdapter: adapter}
	}
	if focused {
		commands["test:focused"] = []config.CommandStep{{Run: config.RunCommand{"fixture focused test"}}}
		resultCommands["test:focused"] = config.DevResultCommandConfig{Enabled: true, DiagnosticsAdapter: adapter}
	}
	return &config.Config{Commands: commands, DevResult: &config.DevResultConfig{Commands: resultCommands}}
}

func arj3539Install(t *testing.T, cfg *config.Config, runner executor.Executor) {
	t.Helper()
	SetConfigLoader(arj3539ConfigLoader{cfg: cfg})
	SetExecutor(runner)
	t.Cleanup(func() {
		SetConfigLoader(nil)
		SetExecutor(nil)
	})
}

func arj3539RunRoot(projectDir string, arguments ...string) (string, error) {
	var output bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&output)
	root.SetArgs(append([]string{"--project-dir", projectDir}, arguments...))
	err := root.Execute()
	return output.String(), err
}

func arj3539Failure(command string, code int, stderr string) executor.ExecutionResult {
	return arj3539FailureWithOutput(command, code, "", stderr)
}

func arj3539FailureWithOutput(command string, code int, stdout, stderr string) executor.ExecutionResult {
	return executor.ExecutionResult{Success: false, CommandName: command, FailedCommand: command, ReturnCode: code, Stdout: stdout, Stderr: stderr, Attempts: 1}
}

func arj3539RequireExitCode(t *testing.T, err error, expected int) {
	t.Helper()
	require.Error(t, err)
	var exitError *ExitError
	require.True(t, errors.As(err, &exitError), "expected ExitError, got %T: %v", err, err)
	assert.Equal(t, expected, exitError.Code)
}

func arj3539WriteRunFile(opts executor.CommandExecutionOptions, relative, contents string) error {
	runDir := opts.Environment[arj3539RunDirEnvironment]
	if runDir == "" {
		return errors.New("DEV_TOOLS_RUN_DIR was not injected")
	}
	path := filepath.Join(runDir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), 0o600)
}

func arj3539EvidenceExecutor(t *testing.T, relative, contents string, result executor.ExecutionResult) *arj3539Executor {
	t.Helper()
	return &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
		require.NoError(t, arj3539WriteRunFile(opts, relative, contents))
		return result
	}}
}

func arj3539ReadTree(t *testing.T, root string) string {
	t.Helper()
	var combined strings.Builder
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			combined.WriteString(filepath.ToSlash(path))
			combined.WriteByte('\n')
			combined.Write(data)
			combined.WriteByte('\n')
		}
		return nil
	})
	return combined.String()
}

func arj3539ReadRetainedResults(t *testing.T, projectDir string) string {
	t.Helper()
	var combined strings.Builder
	for _, directory := range []string{"results", "logs", "evidence"} {
		combined.WriteString(arj3539ReadTree(t, filepath.Join(projectDir, ".build", "agent", directory)))
	}
	return combined.String()
}

func arj3539OnlyJob(t *testing.T, projectDir string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(projectDir, ".build", "agent", "results", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	return strings.TrimSuffix(filepath.Base(files[0]), ".json")
}

func arj3539WriteRetentionJob(t *testing.T, root, job string, modified time.Time) {
	t.Helper()
	for _, relative := range []string{filepath.Join(".build", "agent", "results", job+".json"), filepath.Join(".build", "agent", "logs", job+".log")} {
		path := filepath.Join(root, relative)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(job), 0o600))
		require.NoError(t, os.Chtimes(path, modified, modified))
	}
}

var _ config.ConfigLoader = arj3539ConfigLoader{}
var _ executor.Executor = (*arj3539Executor)(nil)
