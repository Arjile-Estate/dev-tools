package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev-tools/internal/config"
	"dev-tools/internal/devresult"
	"dev-tools/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARJ-3515 acceptance: the diagnostics adapter for build, typecheck, lint and dead-code
// validation. Example titles live in acceptance/arj-3515.yaml.

const arj3515HelperEnvironment = "ARJ3515_ADAPTER_HELPER"

func TestARJ3515AdapterHelperProcess(t *testing.T) {
	if os.Getenv(arj3515HelperEnvironment) != "1" {
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
	var request devresult.DiagnosticsRequest
	_ = json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&request)
	complete := func(failures, warnings []devresult.Diagnostic) {
		_ = json.NewEncoder(os.Stdout).Encode(devresult.DiagnosticsResponse{
			Schema:        devresult.DiagnosticsResponseSchemaV1,
			EvidenceState: devresult.EvidenceComplete,
			Failures:      failures,
			Warnings:      warnings,
		})
	}

	switch arguments[0] {
	case "ui-build":
		complete([]devresult.Diagnostic{{
			Identity: "tsc:TS2322:packages/ui/src/button.ts:12:5",
			Message:  "Type 'string' is not assignable to type 'number'.",
			Package:  "ui",
			Task:     "ui#build",
			File:     "packages/ui/src/button.ts",
			Line:     12,
			Column:   5,
			Rule:     "TS2322",
		}}, nil)
	case "knip-warning":
		complete(nil, []devresult.Diagnostic{{
			Identity: "knip:exports:src/rent.ts:rentLabel",
			Message:  "unused export rentLabel",
			File:     "src/rent.ts",
			Rule:     "exports",
		}})
	case "oxlint":
		complete([]devresult.Diagnostic{{
			Identity: "oxlint:no-debugger:apps/web/src/quittance.ts:8:3",
			Message:  "`debugger` statement is not allowed",
			File:     "apps/web/src/quittance.ts",
			Line:     8,
			Column:   3,
			Rule:     "no-debugger",
		}}, nil)
	case "column-without-line":
		complete([]devresult.Diagnostic{{
			Identity: "tsc:src/lease.ts",
			Message:  "unexpected token",
			File:     "src/lease.ts",
			Column:   5,
		}}, nil)
	case "not-json":
		_, _ = fmt.Fprint(os.Stdout, "not json at all")
	case "counter":
		if len(arguments) < 2 {
			os.Exit(92)
		}
		_ = os.WriteFile(arguments[1], []byte("1"), 0o600)
		complete(nil, nil)
	default:
		os.Exit(94)
	}
	os.Exit(0)
}

func TestARJ3515BuildAndStaticAnalysisDiagnostics(t *testing.T) {
	t.Setenv(claudeCodeEnvVar, "")
	t.Setenv(arj3515HelperEnvironment, "1")
	t.Setenv(arj3539HelperEnvironment, "1")

	t.Run("R1.E1_build_uses_the_repository_adapter", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "turbo-summary.json", "{}", arj3539Failure("build", 1, "ui:build: build failed"))
		arj3539Install(t, arj3515Config("build", arj3515Adapter("ui-build")), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "failures: shown 1/1")
		assert.Contains(t, output, "ui#build: Type 'string' is not assignable to type 'number'. (packages/ui/src/button.ts:12:5) [TS2322]")
	})

	t.Run("R1.E2_dead-code_validation_passes_with_an_adapter_warning", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "knip.json", "{}", executor.ExecutionResult{Success: true, CommandName: "lint:dead-code", Attempts: 1})
		arj3539Install(t, arj3515Config("lint:dead-code", arj3515Adapter("knip-warning", "knip.json")), runner)

		output, err := arj3539RunRoot(projectDir, "lint:dead-code")

		require.NoError(t, err)
		assert.Contains(t, output, "DEV-RESULT v1 PASS")
		assert.Contains(t, output, "warnings: shown 1/1")
		assert.Contains(t, output, "- unused export rentLabel (src/rent.ts) [exports]")
	})

	t.Run("R1.E3_typecheck_without_an_adapter_is_unchanged", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539FailureWithOutput(opts.CommandName, 2, "src/lease.ts(3,7): error TS2304: Cannot find name 'Bail'.\n", "")
		}}
		arj3539Install(t, arj3515Config("typecheck", nil), runner)

		output, err := arj3539RunRoot(projectDir, "typecheck")

		arj3539RequireExitCode(t, err, 2)
		assert.Contains(t, output, "failures: shown 1/1\n- src/lease.ts(3,7): error TS2304: Cannot find name 'Bail'.\n")
		assert.NotContains(t, output, "diagnostics:")
	})

	t.Run("R2.E1_renders_column_rule_and_task", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "turbo-summary.json", "{}", arj3539Failure("build", 1, "build failed"))
		arj3539Install(t, arj3515Config("build", arj3515Adapter("ui-build")), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "(packages/ui/src/button.ts:12:5) [TS2322]")
		failures := arj3515Sidecar(t, projectDir).Failures
		require.Len(t, failures, 1)
		assert.Equal(t, "ui#build", failures[0].Task)
		assert.Equal(t, "ui", failures[0].Package)
		assert.Equal(t, "packages/ui/src/button.ts", failures[0].File)
		assert.Equal(t, 12, failures[0].Line)
		assert.Equal(t, 5, failures[0].Column)
		assert.Equal(t, "TS2322", failures[0].Rule)
	})

	t.Run("R2.E2_renders_an_oxlint_location_and_rule", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "oxlint.json", "{}", arj3539Failure("lint", 1, "Found 1 error."))
		arj3539Install(t, arj3515Config("lint", arj3515Adapter("oxlint", "oxlint.json")), runner)

		output, err := arj3539RunRoot(projectDir, "lint")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "- `debugger` statement is not allowed (apps/web/src/quittance.ts:8:3) [no-debugger]")
	})

	t.Run("R2.E3_keeps_adapters_without_the_new_fields_unchanged", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "checkout.xml", "<testsuite/>", arj3539Failure("test", 42, "two failures"))
		arj3539Install(t, arj3539Config(arj3539Adapter("complete"), false), runner)

		output, err := arj3539RunRoot(projectDir, "test")

		arj3539RequireExitCode(t, err, 42)
		assert.Contains(t, output, "failures: shown 2/2\n- checkout rejects an expired card\n- rent rounds the annual revision\n")
	})

	t.Run("R2.E4_rejects_a_column_without_a_line", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "turbo-summary.json", "{}", arj3539Failure("build", 8, "error Unexpected token"))
		arj3539Install(t, arj3515Config("build", arj3515Adapter("column-without-line")), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 8)
		assert.Contains(t, output, "diagnostics: fallback (invalid adapter response)")
		assert.Contains(t, output, "a column without a line")
		assert.Contains(t, output, "- error Unexpected token")
	})

	t.Run("R3.E1_reports_a_missing_required_source", func(t *testing.T) {
		projectDir := t.TempDir()
		counter := filepath.Join(projectDir, "adapter-counter")
		adapter := arj3515Adapter("counter", counter)
		adapter.Required = true
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539Failure(opts.CommandName, 2, "Error: Cannot find module 'vite'")
		}}
		arj3539Install(t, arj3515Config("build", adapter), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 2)
		assert.NoFileExists(t, counter)
		assert.Contains(t, output, "- structured source turbo-summary.json is missing")
		assert.Contains(t, output, "diagnostics: fallback (structured source turbo-summary.json is missing)")
		assert.Contains(t, output, "- Error: Cannot find module 'vite'")
	})

	t.Run("R3.E2_skips_an_optional_missing_source_quietly", func(t *testing.T) {
		projectDir := t.TempDir()
		counter := filepath.Join(projectDir, "adapter-counter")
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539Failure(opts.CommandName, 2, "Error: Cannot find module 'vite'")
		}}
		arj3539Install(t, arj3515Config("build", arj3515Adapter("counter", counter)), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 2)
		assert.NoFileExists(t, counter)
		assert.NotContains(t, output, "is missing")
		assert.NotContains(t, output, "warnings:")
		assert.Contains(t, output, "- Error: Cannot find module 'vite'")
	})

	t.Run("R3.E3_passes_with_a_missing_required_source_warning", func(t *testing.T) {
		projectDir := t.TempDir()
		adapter := arj3515Adapter("oxlint", "oxlint.json")
		adapter.Required = true
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return executor.ExecutionResult{Success: true, CommandName: opts.CommandName, Attempts: 1}
		}}
		arj3539Install(t, arj3515Config("lint", adapter), runner)

		output, err := arj3539RunRoot(projectDir, "lint")

		require.NoError(t, err)
		assert.Contains(t, output, "DEV-RESULT v1 PASS")
		assert.Contains(t, output, "- structured source oxlint.json is missing")
	})

	t.Run("R4.E1_marks_structured_diagnostics", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "turbo-summary.json", "{}", arj3539Failure("build", 1, "build failed"))
		arj3539Install(t, arj3515Config("build", arj3515Adapter("ui-build")), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "\ndiagnostics: structured\n")
		sidecar := arj3515Sidecar(t, projectDir)
		require.NotNil(t, sidecar.Diagnostics)
		assert.Equal(t, devresult.DiagnosticsStructured, sidecar.Diagnostics.Mode)
		assert.Empty(t, sidecar.Diagnostics.Reason)

		jsonDir := t.TempDir()
		jsonOutput, jsonErr := arj3539RunRoot(jsonDir, "--format", "json", "build")
		arj3539RequireExitCode(t, jsonErr, 1)
		assert.Contains(t, jsonOutput, `"diagnostics":{"mode":"structured"}`)
	})

	t.Run("R4.E2_marks_a_malformed_adapter_fallback_deterministically", func(t *testing.T) {
		var lines []string
		var sidecars []*devresult.DiagnosticsSummary
		for range 2 {
			projectDir := t.TempDir()
			runner := arj3539EvidenceExecutor(t, "knip.json", "{}", arj3539Failure("lint:dead-code", 1, "Unused files (1)"))
			arj3539Install(t, arj3515Config("lint:dead-code", arj3515Adapter("not-json", "knip.json")), runner)

			output, err := arj3539RunRoot(projectDir, "lint:dead-code")

			arj3539RequireExitCode(t, err, 1)
			lines = append(lines, arj3515Line(t, output, "diagnostics: "))
			sidecars = append(sidecars, arj3515Sidecar(t, projectDir).Diagnostics)
		}
		assert.Equal(t, "diagnostics: fallback (malformed adapter response)", lines[0])
		assert.Equal(t, lines[0], lines[1])
		require.NotNil(t, sidecars[0])
		assert.Equal(t, devresult.DiagnosticsSummary{Mode: devresult.DiagnosticsFallback, Reason: "malformed adapter response"}, *sidecars[0])
		assert.Equal(t, sidecars[0], sidecars[1])
	})

	t.Run("R4.E3_reports_the_requested_scope_apart_from_the_failing_task", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := arj3539EvidenceExecutor(t, "turbo-summary.json", "{}", arj3539Failure("build", 1, "build failed"))
		arj3539Install(t, arj3515Config("build", arj3515Adapter("ui-build")), runner)

		output, err := arj3539RunRoot(projectDir, "build", "--", "--filter", "web")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "command: dev-tools build --filter web | scope: --filter web\n")
		assert.Contains(t, output, "- ui#build: ")
		sidecar := arj3515Sidecar(t, projectDir)
		assert.Equal(t, "--filter web", sidecar.Command.Scope)
		require.Len(t, sidecar.Failures, 1)
		assert.Equal(t, "ui#build", sidecar.Failures[0].Task)
	})

	t.Run("R5.E1_drops_package-manager_and_Turbo_banners", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539FailureWithOutput(opts.CommandName, 1, arj3515NoisyBuildLog, "")
		}}
		arj3539Install(t, arj3515Config("build", nil), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "failures: shown 1/1\n- ui:build: Error: Cannot find module './rent'\n")
		for _, banner := range []string{"Packages in scope", "Tasks:", "Cached:", "Time:", "> ui@0.0.0"} {
			assert.NotContains(t, output, banner)
		}
	})

	t.Run("R5.E2_does_not_repeat_package-manager_echoes", func(t *testing.T) {
		projectDir := t.TempDir()
		log := arj3515NoisyBuildLog +
			"ui:build:  ELIFECYCLE  Command failed with exit code 1.\n" +
			"ui#build: command (/repo/packages/ui) /usr/bin/pnpm run build exited (1)\n"
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539FailureWithOutput(opts.CommandName, 1, log, "")
		}}
		arj3539Install(t, arj3515Config("build", nil), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "failures: shown 1/1\n- ui:build: Error: Cannot find module './rent'\n")
		assert.NotContains(t, output, "ELIFECYCLE")
		assert.NotContains(t, output, "exited (1)")
	})

	t.Run("R5.E3_shows_an_echo_when_it_is_the_only_failure", func(t *testing.T) {
		projectDir := t.TempDir()
		runner := &arj3539Executor{run: func(opts executor.CommandExecutionOptions) executor.ExecutionResult {
			return arj3539FailureWithOutput(opts.CommandName, 1, " ELIFECYCLE  Command failed with exit code 1.\n", "")
		}}
		arj3539Install(t, arj3515Config("build", nil), runner)

		output, err := arj3539RunRoot(projectDir, "build")

		arj3539RequireExitCode(t, err, 1)
		assert.Contains(t, output, "failures: shown 1/1\n- ELIFECYCLE  Command failed with exit code 1.\n")
	})
}

const arj3515NoisyBuildLog = "\x1b[1mturbo 2.5.4\x1b[0m\n" +
	"• Packages in scope: ui, web\n" +
	"• Running build in 2 packages\n" +
	"• Remote caching disabled\n" +
	"ui:build: > ui@0.0.0 build /repo/packages/ui\n" +
	"ui:build: > vite build\n" +
	"ui:build: Error: Cannot find module './rent'\n" +
	" Tasks:    0 successful, 2 total\n" +
	"Cached:    0 cached, 2 total\n" +
	"  Time:    1.204s\n"

func arj3515Adapter(mode string, arguments ...string) *config.DiagnosticsAdapterConfig {
	startWhen := "turbo-summary.json"
	if len(arguments) > 0 && strings.HasSuffix(arguments[0], ".json") {
		startWhen = arguments[0]
		arguments = arguments[1:]
	}
	command := []string{os.Args[0], "-test.run=^TestARJ3515AdapterHelperProcess$", "--", mode}
	return &config.DiagnosticsAdapterConfig{Command: append(command, arguments...), StartWhen: []string{startWhen}}
}

func arj3515Config(command string, adapter *config.DiagnosticsAdapterConfig) *config.Config {
	return &config.Config{
		Commands: map[string][]config.CommandStep{command: {{Run: config.RunCommand{"fixture " + command}}}},
		DevResult: &config.DevResultConfig{Commands: map[string]config.DevResultCommandConfig{
			command: {Enabled: true, DiagnosticsAdapter: adapter},
		}},
	}
}

func arj3515Sidecar(t *testing.T, projectDir string) devresult.Result {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, ".build", "agent", "results", arj3539OnlyJob(t, projectDir)+".json"))
	require.NoError(t, err)
	var result devresult.Result
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func arj3515Line(t *testing.T, output, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("no line starting with %q in:\n%s", prefix, output)
	return ""
}
