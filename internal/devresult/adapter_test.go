package devresult

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapterHelperProcess(t *testing.T) {
	if os.Getenv("DEV_RESULT_ADAPTER_HELPER") != "1" {
		return
	}
	args := os.Args
	mode := args[len(args)-1]
	var request DiagnosticsRequest
	_ = json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&request)
	switch mode {
	case "complete":
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{
			Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
			Failures: []Diagnostic{{Identity: "checkout/failure", Message: "checkout failed"}},
		})
	case "echo-argument":
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{
			Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
			Warnings: []Diagnostic{{Identity: "argument", Message: args[len(args)-2]}},
		})
	case "working-directory":
		workingDirectory, _ := os.Getwd()
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{
			Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete,
			Warnings: []Diagnostic{{Identity: "working-directory", Message: workingDirectory}},
		})
	case "partial":
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{
			Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidencePartial,
			Failures: []Diagnostic{{Identity: "statement/failure", Message: "statement failed"}},
			Problems: []EvidenceProblem{{Message: "billing.xml is missing"}},
		})
	case "stderr":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("x", 256))
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{Schema: DiagnosticsResponseSchemaV1, EvidenceState: EvidenceComplete})
	case "oversized":
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("x", 4096))
	case "malformed":
		_, _ = fmt.Fprint(os.Stdout, "not-json")
	case "version":
		_ = json.NewEncoder(os.Stdout).Encode(DiagnosticsResponse{Schema: "dev-tools/diagnostics-response/v2", EvidenceState: EvidenceComplete})
	case "sleep":
		time.Sleep(3 * time.Second)
	}
	os.Exit(0)
}

func helperAdapter(mode string, extra ...string) []string {
	args := []string{os.Args[0], "-test.run=TestAdapterHelperProcess", "--"}
	args = append(args, extra...)
	args = append(args, mode)
	return args
}

func testRequest() DiagnosticsRequest {
	return DiagnosticsRequest{Schema: DiagnosticsRequestSchemaV1, RunID: "test-004", RunDir: "/tmp/test-004", Command: "test", ExitCode: 5}
}

func TestExecuteAdapterCompleteAndDirectArguments(t *testing.T) {
	t.Setenv("DEV_RESULT_ADAPTER_HELPER", "1")
	result := ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("complete"), Request: testRequest(), Limits: DefaultAdapterLimits()})
	require.NoError(t, result.Err)
	require.NotNil(t, result.Response)
	assert.Equal(t, EvidenceComplete, result.Response.EvidenceState)
	assert.Equal(t, "checkout/failure", result.Response.Failures[0].Identity)

	literal := "report; touch marker.txt"
	result = ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("echo-argument", literal), Request: testRequest(), Limits: DefaultAdapterLimits()})
	require.NoError(t, result.Err)
	assert.Equal(t, literal, result.Response.Warnings[0].Message)
}

func TestExecuteAdapterUsesConfiguredWorkingDirectory(t *testing.T) {
	t.Setenv("DEV_RESULT_ADAPTER_HELPER", "1")
	workingDirectory := t.TempDir()
	result := ExecuteAdapter(context.Background(), AdapterOptions{
		Argv:       helperAdapter("working-directory"),
		WorkingDir: workingDirectory,
		Request:    testRequest(),
		Limits:     DefaultAdapterLimits(),
	})
	require.NoError(t, result.Err)
	assert.Equal(t, workingDirectory, result.Response.Warnings[0].Message)
}

func TestExecuteAdapterBoundsTimeoutCancellationAndInvalidResponses(t *testing.T) {
	t.Setenv("DEV_RESULT_ADAPTER_HELPER", "1")
	limits := DefaultAdapterLimits()
	limits.Timeout = 50 * time.Millisecond
	result := ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("sleep"), Request: testRequest(), Limits: limits})
	assert.Error(t, result.Err)
	assert.True(t, result.TimedOut)

	limits = DefaultAdapterLimits()
	limits.StdoutBytes = 64
	result = ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("oversized"), Request: testRequest(), Limits: limits})
	assert.ErrorContains(t, result.Err, "standard output limit")
	assert.True(t, result.StdoutTruncated)

	limits = DefaultAdapterLimits()
	limits.StderrBytes = 32
	result = ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("stderr"), Request: testRequest(), Limits: limits})
	require.NoError(t, result.Err)
	assert.True(t, result.StderrTruncated)
	assert.Len(t, result.Stderr, 32)

	for _, mode := range []string{"malformed", "version"} {
		result = ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter(mode), Request: testRequest(), Limits: DefaultAdapterLimits()})
		assert.Error(t, result.Err, mode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = ExecuteAdapter(ctx, AdapterOptions{Argv: helperAdapter("complete"), Request: testRequest(), Limits: DefaultAdapterLimits()})
	assert.Error(t, result.Err)
	assert.True(t, result.Cancelled)

	result = ExecuteAdapter(context.Background(), AdapterOptions{Argv: []string{"definitely-missing-dev-result-runtime"}, Request: testRequest(), Limits: DefaultAdapterLimits()})
	assert.Error(t, result.Err)
}

func TestExecuteAdapterPreservesPartialEvidence(t *testing.T) {
	t.Setenv("DEV_RESULT_ADAPTER_HELPER", "1")
	result := ExecuteAdapter(context.Background(), AdapterOptions{Argv: helperAdapter("partial"), Request: testRequest(), Limits: DefaultAdapterLimits()})
	require.NoError(t, result.Err)
	assert.Equal(t, EvidencePartial, result.Response.EvidenceState)
	assert.Equal(t, "statement/failure", result.Response.Failures[0].Identity)
	assert.Equal(t, "billing.xml is missing", result.Response.Problems[0].Message)
}
