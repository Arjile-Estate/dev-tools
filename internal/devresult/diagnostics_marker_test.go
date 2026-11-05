package devresult

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDiagnosticsResponseChecksColumns(t *testing.T) {
	response := DiagnosticsResponse{
		Schema:        DiagnosticsResponseSchemaV1,
		EvidenceState: EvidenceComplete,
		Failures:      []Diagnostic{{Identity: "tsc:a", Message: "broken", File: "a.ts", Line: 3, Column: 7, Rule: "TS2322", Task: "ui#build"}},
	}
	require.NoError(t, ValidateDiagnosticsResponse(response, DefaultAdapterLimits()))

	response.Failures[0].Line = 0
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, DefaultAdapterLimits()), "a column without a line")
	response.Failures[0].Line = 3
	response.Failures[0].Column = -1
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, DefaultAdapterLimits()), "invalid column")
}

func TestRedactDiagnosticsResponseCoversEveryStringField(t *testing.T) {
	const secret = "secret-demo-12345"
	environment := []string{"BUILD_API_TOKEN=" + secret}
	var diagnostic Diagnostic
	value := reflect.ValueOf(&diagnostic).Elem()
	for index := 0; index < value.NumField(); index++ {
		if value.Field(index).Kind() == reflect.String {
			value.Field(index).SetString("token " + secret)
		}
	}
	response := DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
		Failures: []Diagnostic{diagnostic}, Warnings: []Diagnostic{diagnostic}}

	redacted, matches := RedactDiagnosticsResponse(response, environment)

	for _, group := range [][]Diagnostic{redacted.Failures, redacted.Warnings} {
		redactedValue := reflect.ValueOf(group[0])
		for index := 0; index < redactedValue.NumField(); index++ {
			field := redactedValue.Field(index)
			if field.Kind() == reflect.String {
				assert.NotContains(t, field.String(), secret, "Diagnostic.%s", redactedValue.Type().Field(index).Name)
			}
		}
	}
	assert.Positive(t, matches)
}

func TestFormatDiagnosticOmitsAbsentParts(t *testing.T) {
	tests := []struct {
		name       string
		diagnostic Diagnostic
		want       string
	}{
		{name: "message only", diagnostic: Diagnostic{Message: "checkout failed"}, want: "checkout failed"},
		{name: "file and line", diagnostic: Diagnostic{Message: "checkout failed", File: "a.go", Line: 4}, want: "checkout failed (a.go:4)"},
		{name: "column without file is dropped", diagnostic: Diagnostic{Message: "m", Line: 4, Column: 2}, want: "m"},
		{name: "everything", diagnostic: Diagnostic{Message: "m", Task: "ui#build", File: "a.ts", Line: 4, Column: 2, Rule: "TS1"}, want: "ui#build: m (a.ts:4:2) [TS1]"},
		{name: "rule without location", diagnostic: Diagnostic{Message: "unused file", Rule: "files"}, want: "unused file [files]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDiagnostic(tt.diagnostic))
		})
	}
}

func TestBuildDiagnosticsMarker(t *testing.T) {
	complete := &DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
		Failures: []Diagnostic{{Identity: "a", Message: "a failed"}}}
	empty := &DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete}
	unavailable := &DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceUnavailable}
	tests := []struct {
		name  string
		input Input
		want  *DiagnosticsSummary
	}{
		{name: "no adapter", input: Input{ExitCode: 1, Diagnostics: complete}, want: nil},
		{name: "structured", input: Input{ExitCode: 1, Diagnostics: complete, DiagnosticsConfigured: true}, want: &DiagnosticsSummary{Mode: DiagnosticsStructured}},
		{name: "passing run with empty evidence", input: Input{Diagnostics: empty, DiagnosticsConfigured: true}, want: &DiagnosticsSummary{Mode: DiagnosticsStructured}},
		{name: "failure without structured failure", input: Input{ExitCode: 1, Diagnostics: empty, DiagnosticsConfigured: true}, want: &DiagnosticsSummary{Mode: DiagnosticsFallback, Reason: "no actionable structured failure"}},
		{name: "unavailable evidence", input: Input{Diagnostics: unavailable, DiagnosticsConfigured: true}, want: &DiagnosticsSummary{Mode: DiagnosticsFallback, Reason: "structured evidence unavailable"}},
		{name: "adapter never ran", input: Input{ExitCode: 2, DiagnosticsConfigured: true}, want: &DiagnosticsSummary{Mode: DiagnosticsFallback, Reason: "no structured evidence was written"}},
		{name: "adapter failed", input: Input{ExitCode: 2, DiagnosticsConfigured: true, DiagnosticsFallbackReason: "adapter timeout"}, want: &DiagnosticsSummary{Mode: DiagnosticsFallback, Reason: "adapter timeout"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.Job = "build-marker"
			tt.input.CommandName = "build"
			result, _ := Build(tt.input, DefaultBudgets(), nil)
			assert.Equal(t, tt.want, result.Diagnostics)
		})
	}
}

func TestRenderShowsScopeAndDiagnosticsLines(t *testing.T) {
	result, _ := Build(Input{
		Job: "build-scope", CommandName: "build", ExitCode: 1,
		NormalizedCommand:     NormalizeCommand("build", []string{"--filter", "web", "--token", "secret-demo-12345"}),
		Scope:                 NormalizeScope([]string{"--filter", "web", "--token", "secret-demo-12345"}),
		Stderr:                "error: broken",
		DiagnosticsConfigured: true, DiagnosticsFallbackReason: "adapter timeout",
	}, DefaultBudgets(), nil)
	text := RenderText(result, DefaultBudgets())
	assert.Contains(t, text, "command: dev-tools build --filter web --token '[REDACTED]' | scope: --filter web --token '[REDACTED]'\n")
	assert.Contains(t, text, "\ndiagnostics: fallback (adapter timeout)\n")
	assert.NotContains(t, text, "secret-demo-12345")
}

func TestCollectEvidenceDropsBannersAndDemotesEchoes(t *testing.T) {
	stdout := "turbo 2.5.4\n• Packages in scope: ui\nweb:build: > web@0.0.0 build /repo/apps/web\n" +
		"Tasks:    0 successful, 1 total\nCached:    0 cached, 1 total\n  Time:    1s\n" +
		" ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL  web@0.0.0 build: `vite build`\n" +
		" ELIFECYCLE  Command failed with exit code 1.\n" +
		"web#build: command (/repo/apps/web) /usr/bin/pnpm run build exited (1)\n" +
		" ERROR  run failed: command  exited (1)\n" +
		"Failed:    web#build\n"
	failures, warnings := collectEvidence(stdout, "web:build: error during build: missing export\n", nil, OutcomeFail)
	require.Len(t, failures, 1)
	assert.Equal(t, "web:build: error during build: missing export", failures[0].Message)
	assert.Empty(t, warnings)

	failures, _ = collectEvidence(stdout, "", nil, OutcomeFail)
	require.Len(t, failures, 1)
	assert.Equal(t, "ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL  web@0.0.0 build: `vite build`", failures[0].Message)
}

func TestCollectEvidenceKeepsPnpmRootCauses(t *testing.T) {
	stderr := " ERR_PNPM_OUTDATED_LOCKFILE  Cannot install with \"frozen-lockfile\" because pnpm-lock.yaml is not up to date\n" +
		"Failure reason:\n"
	failures, _ := collectEvidence("", stderr, nil, OutcomeFail)
	require.NotEmpty(t, failures)
	assert.Equal(t, "ERR_PNPM_OUTDATED_LOCKFILE  Cannot install with \"frozen-lockfile\" because pnpm-lock.yaml is not up to date", failures[0].Message)
}

func TestCollectEvidenceKeepsActionableLinesThatResembleBanners(t *testing.T) {
	stderr := "> Compilation failed; see the compiler error output\n" +
		"src/a.ts:3: > unexpected token\n" +
		"Failed: the migration could not lock the table\n"
	failures, _ := collectEvidence("", stderr, nil, OutcomeFail)
	messages := make([]string, 0, len(failures))
	for _, failure := range failures {
		messages = append(messages, failure.Message)
	}
	assert.Contains(t, messages, "> Compilation failed; see the compiler error output")
	assert.Contains(t, messages, "Failed: the migration could not lock the table")
	assert.True(t, noiseLine("web:build: > web@0.0.0 build /repo/apps/web"))
	assert.False(t, noiseLine("src/a.ts:3: > unexpected token"))
}
