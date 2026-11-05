package devresult

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRedactsEveryPersistedStructuredField(t *testing.T) {
	const secret = "secret-demo-12345"
	environment := []string{"BUILD_API_TOKEN=" + secret}
	var diagnostic Diagnostic
	value := reflect.ValueOf(&diagnostic).Elem()
	for index := 0; index < value.NumField(); index++ {
		if value.Field(index).Kind() == reflect.String {
			value.Field(index).SetString("token " + secret)
		}
	}
	diagnostic.Line, diagnostic.Column = 3, 4
	response := DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
		Failures: []Diagnostic{diagnostic}, Warnings: []Diagnostic{diagnostic}}

	result, retained := Build(Input{
		Job: "build-redaction", CommandName: "build", ExitCode: 1,
		NormalizedCommand:         "dev-tools build --note " + secret,
		Scope:                     "--note " + secret,
		Diagnostics:               &response,
		DiagnosticsConfigured:     true,
		DiagnosticsFallbackReason: "reason " + secret,
	}, DefaultBudgets(), environment)

	sidecar, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(sidecar), secret)
	assert.NotContains(t, retained, secret)
	assert.Positive(t, result.Redaction.Matches)
}

func TestBuildCountsAScopeSecretOnce(t *testing.T) {
	result, _ := Build(Input{
		Job: "build-count", CommandName: "build",
		NormalizedCommand: "dev-tools build --note password=hunter2",
		Scope:             "--note password=hunter2",
	}, DefaultBudgets(), nil)
	assert.Equal(t, 1, result.Redaction.Matches)
	assert.NotContains(t, result.Command.Scope, "hunter2")
}

func TestCompactJSONKeepsASoleWarningAtTheDefaultSuccessBudget(t *testing.T) {
	response := DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
		Warnings: []Diagnostic{{Identity: "knip:exports:src/rent.ts:rentLabel", Message: "unused export rentLabel", Package: "web", Task: "web#lint:dead-code", File: "apps/web/src/rent.ts", Line: 12, Column: 14, Rule: "exports"}}}
	result, _ := Build(Input{
		Job: "lint-dead-code-20261105T120000.123456789Z-123456789", CommandName: "lint:dead-code",
		Diagnostics: &response, DiagnosticsConfigured: true,
	}, DefaultBudgets(), nil)

	data, err := MarshalCompactJSON(result, DefaultBudgets())
	require.NoError(t, err)
	var compact Result
	require.NoError(t, json.Unmarshal(data, &compact))
	require.Len(t, compact.Warnings, 1, string(data))
	assert.Contains(t, compact.Warnings[0].Message, "unused export rentLabel")
	assert.LessOrEqual(t, len(data), DefaultBudgets().Success.Bytes)
}

func TestTextAtTheMinimumBudgetDropsTheDiagnosticsLine(t *testing.T) {
	budgets := DefaultBudgets()
	budgets.Success = Budget{Lines: 7, Bytes: 1024}
	result, _ := Build(Input{Job: "build-tight", CommandName: "build", DiagnosticsConfigured: true}, budgets, nil)
	require.NotNil(t, result.Diagnostics)

	text := RenderText(result, budgets)

	assert.NotContains(t, text, "diagnostics:")
	assert.Equal(t, 7, strings.Count(text, "\n"))
	assert.Contains(t, text, "truncation: true")
	assert.NotNil(t, result.Diagnostics, "the sidecar result keeps the marker")

	data, err := MarshalCompactJSON(result, budgets)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"diagnostics":{"mode":"fallback","reason":"no structured evidence was written"}`)
}

func TestCollectEvidenceDemotesTurbo2Echoes(t *testing.T) {
	stdout := "ui:build: src/a.ts(3,5): error TS2322: Type 'string' is not assignable to type 'number'.\n" +
		"ui:build: ERROR: command finished with error: command (/repo/packages/ui) /usr/bin/pnpm run build exited (2)\n" +
		"ERROR: command finished with error: command (/repo/packages/ui) /usr/bin/pnpm run build exited (2)\n"
	failures, _ := collectEvidence(stdout, "", nil, OutcomeFail)
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0].Message, "TS2322")
}

func TestCollectEvidenceKeepsCodeFrames(t *testing.T) {
	assert.False(t, noiseLine("> 12 | const rent = lease.amount * index"))
	assert.False(t, noiseLine("web:build: > 12 | const rent = lease.amount * index"))
	assert.True(t, noiseLine("> vite build"))
}

func TestCollectEvidenceKeepsUnkeywordedContextBesideAnEcho(t *testing.T) {
	stdout := "src/rent.ts(4,9): Type 'string' is not assignable to type 'number'.\n" +
		" ELIFECYCLE  Command failed with exit code 2.\n"
	failures, _ := collectEvidence(stdout, "", nil, OutcomeFail)
	require.Len(t, failures, 2)
	assert.Equal(t, "src/rent.ts(4,9): Type 'string' is not assignable to type 'number'.", failures[0].Message)
	assert.Equal(t, "ELIFECYCLE  Command failed with exit code 2.", failures[1].Message)
}

func TestCompactJSONIgnoresTheTextLineBudget(t *testing.T) {
	budgets := DefaultBudgets()
	budgets.Failure = Budget{Lines: 7, Bytes: 4096}
	response := DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
		Failures: []Diagnostic{{Identity: "tsc:a", Message: "Type 'string' is not assignable to type 'number'.", Task: "ui#build", File: "packages/ui/src/button.ts", Line: 12, Column: 5, Rule: "TS2322"}}}
	result, _ := Build(Input{Job: "build-seven", CommandName: "build", ExitCode: 1, Diagnostics: &response, DiagnosticsConfigured: true}, budgets, nil)

	data, err := MarshalCompactJSON(result, budgets)
	require.NoError(t, err)
	var compact Result
	require.NoError(t, json.Unmarshal(data, &compact))
	require.Len(t, compact.Failures, 1, string(data))
	require.NotNil(t, compact.Diagnostics)
	assert.Equal(t, DiagnosticsStructured, compact.Diagnostics.Mode)
	assert.Equal(t, Budget{Lines: 7, Bytes: 4096}, compact.Truncation.Budget)
	assert.False(t, compact.Truncation.Truncated)

	text := RenderText(result, budgets)
	assert.Contains(t, text, "failures=0/1", "the 7-line text envelope still has no room for the failure")
}
