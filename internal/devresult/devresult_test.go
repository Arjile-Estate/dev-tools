package devresult

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportedCommand(t *testing.T) {
	for _, name := range []string{"test", "test:focused", "build", "lint", "lint:dead-code", "typecheck"} {
		assert.True(t, SupportedCommand(name), name)
	}
	for _, name := range []string{"test:coverage", "dev", "result", ""} {
		assert.False(t, SupportedCommand(name), name)
	}
}

func TestDefaultBudgets(t *testing.T) {
	assert.Equal(t, Budget{Lines: 10, Bytes: 1024}, DefaultBudgets().Success)
	assert.Equal(t, Budget{Lines: 80, Bytes: 12 * 1024}, DefaultBudgets().Failure)
	assert.Equal(t, Budget{Lines: 30, Bytes: 4 * 1024}, DefaultBudgets().Timeout)
	assert.Equal(t, Budget{Lines: 3, Bytes: 1024}, DefaultBudgets().Background)
}

func TestRedactRemovesCommonAndEnvironmentSecrets(t *testing.T) {
	input := "Authorization: Bearer secret-demo-12345\nAPI_TOKEN=token-demo-67890\ncustom=" + "environment-secret"
	redacted, matches := Redact(input, []string{"CUSTOM_SECRET=environment-secret", "PATH=/usr/bin"})

	assert.NotContains(t, redacted, "secret-demo-12345")
	assert.NotContains(t, redacted, "token-demo-67890")
	assert.NotContains(t, redacted, "environment-secret")
	assert.Contains(t, redacted, RedactionMarker)
	assert.GreaterOrEqual(t, matches, 3)
}

func TestRedactRemovesSpaceSeparatedCredentialFlags(t *testing.T) {
	redacted, matches := Redact("dev-tools test --token secret-value --password=other-secret", nil)
	assert.NotContains(t, redacted, "secret-value")
	assert.NotContains(t, redacted, "other-secret")
	assert.Equal(t, 2, matches)
}

func TestRedactRemovesQuotedJSONAndMultiwordCredentialValues(t *testing.T) {
	redacted, matches := Redact(`{"token":"json-secret","password":"two words"} dev-tools test --token 'alpha beta' --api-key="gamma delta"`, nil)

	for _, secret := range []string{"json-secret", "two words", "alpha beta", "gamma delta"} {
		assert.NotContains(t, redacted, secret)
	}
	assert.Equal(t, 4, matches)
}

func TestNormalizeCommandRedactsCredentialArgumentsBeforeShellQuoting(t *testing.T) {
	normalized := NormalizeCommand("test", []string{
		"--token=alpha beta",
		"--password", "odd ' quoted secret",
	})

	assert.NotContains(t, normalized, "alpha beta")
	assert.NotContains(t, normalized, "odd")
	assert.NotContains(t, normalized, "quoted secret")
	assert.Equal(t, 2, strings.Count(normalized, RedactionMarker))
	result, _ := Build(Input{CommandName: "test", NormalizedCommand: normalized}, DefaultBudgets(), nil)
	assert.True(t, result.Redaction.Applied)
}

func TestBuildAndRenderSelectsDistinctActionableEvidenceDeterministically(t *testing.T) {
	input := Input{
		Job:               "build-fixed-001",
		CommandName:       "build",
		NormalizedCommand: "dev-tools build",
		ExitCode:          42,
		Duration:          3410 * time.Millisecond,
		Attempts:          1,
		Stdout:            strings.Repeat("progress 10%\r", 20),
		Stderr:            "\x1b[31minternal/parser.go:84: error: expected identifier\x1b[0m\ninternal/parser.go:84: error: expected identifier\nwarning: cache unavailable\n",
	}

	result, retained := Build(input, DefaultBudgets(), nil)
	first := RenderText(result, DefaultBudgets())
	second := RenderText(result, DefaultBudgets())

	assert.Equal(t, first, second)
	assert.Contains(t, first, "DEV-RESULT v1 FAIL")
	assert.Equal(t, 1, strings.Count(first, "internal/parser.go:84"))
	assert.Contains(t, first, "warning: cache unavailable")
	assert.NotContains(t, first, "\x1b[")
	assert.Contains(t, retained, "--- stdout ---")
	assert.Contains(t, retained, "--- stderr ---")
	assert.Equal(t, 1, result.Truncation.Total.Failures)
	assert.Equal(t, 1, result.Truncation.Total.Warnings)
	assert.LessOrEqual(t, len([]byte(first)), DefaultBudgets().Failure.Bytes)
	assert.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(first, "\n"), "\n")), DefaultBudgets().Failure.Lines)
}

func TestWriteAndLoadArtifactsArePrivateAndComplete(t *testing.T) {
	root := t.TempDir()
	result, retained := Build(Input{
		Job:               "test-fixed-001",
		CommandName:       "test",
		NormalizedCommand: "dev-tools test",
		ExitCode:          0,
		Duration:          8 * time.Second,
		Attempts:          1,
		Stdout:            "all tests passed\n",
	}, DefaultBudgets(), nil)

	written, err := WriteArtifacts(root, result, retained)
	require.NoError(t, err)
	loaded, logText, err := LoadArtifacts(root, result.Job)
	require.NoError(t, err)

	assert.Equal(t, written.Job, loaded.Job)
	assert.Contains(t, logText, "all tests passed")
	assert.Equal(t, filepath.ToSlash(filepath.Join(".build", "agent", "logs", result.Job+".log")), written.Artifacts.Log)
	assert.Equal(t, filepath.ToSlash(filepath.Join(".build", "agent", "results", result.Job+".json")), written.Artifacts.Result)

	for _, relative := range []string{written.Artifacts.Log, written.Artifacts.Result} {
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(written.Artifacts.Result)))
	require.NoError(t, err)
	var decoded Result
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, SchemaV1, decoded.Schema)
}

func TestWriteArtifactsRejectsUnsafeJobIdentifier(t *testing.T) {
	result, retained := Build(Input{Job: "../escape", CommandName: "test"}, DefaultBudgets(), nil)
	_, err := WriteArtifacts(t.TempDir(), result, retained)
	require.Error(t, err)
}

func TestNewJobIDIsNormalizedAndCollisionSafe(t *testing.T) {
	now := time.Date(2026, 11, 3, 14, 25, 1, 123456789, time.UTC)
	assert.Equal(t, "test-focused-20261103T142501.123456789Z", NewJobID("test:focused", now))
}

func TestTimeoutAndCancellationUseBoundedOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   Input
		outcome Outcome
		header  string
	}{
		{name: "timeout", input: Input{Job: "typecheck-timeout", CommandName: "typecheck", TimedOut: true, ExitCode: -1}, outcome: OutcomeTimeout, header: "DEV-RESULT v1 TIMEOUT"},
		{name: "cancelled", input: Input{Job: "lint-cancelled", CommandName: "lint", Cancelled: true, ExitCode: -1}, outcome: OutcomeCancelled, header: "DEV-RESULT v1 CANCELLED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := Build(tc.input, DefaultBudgets(), nil)
			assert.Equal(t, tc.outcome, result.Outcome)
			assert.Equal(t, DefaultBudgets().Timeout, result.Truncation.Budget)
			assert.Contains(t, RenderText(result, DefaultBudgets()), tc.header)
		})
	}
}

func TestCompactJSONNeverContainsRawStreams(t *testing.T) {
	result, _ := Build(Input{
		Job: "build-json", CommandName: "build", ExitCode: 42,
		Stdout: "raw stdout payload", Stderr: "error: raw stderr payload",
	}, DefaultBudgets(), nil)
	data, err := json.Marshal(Compact(result, DefaultBudgets()))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "raw stdout payload")
	assert.NotContains(t, string(data), `"stdout":`)
	assert.NotContains(t, string(data), `"stderr":`)
}

func TestCompactTextAndJSONRespectHardBudgets(t *testing.T) {
	budgets := DefaultBudgets()
	budgets.Failure = Budget{Lines: 7, Bytes: 1024}
	result, _ := Build(Input{
		Job: "build-budget", CommandName: "build", ExitCode: 42,
		NormalizedCommand: "dev-tools build -- " + strings.Repeat("long-argument-", 300),
		Stderr:            strings.Repeat("error: distinct failure payload\n", 100),
	}, budgets, nil)

	text := RenderText(result, budgets)
	assert.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(text, "\n"), "\n")), budgets.Failure.Lines)
	assert.LessOrEqual(t, len([]byte(text)), budgets.Failure.Bytes)

	jsonData, err := MarshalCompactJSON(result, budgets)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(jsonData), budgets.Failure.Bytes)
	assert.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(string(jsonData), "\n"), "\n")), budgets.Failure.Lines)
	var decoded Result
	require.NoError(t, json.Unmarshal(jsonData, &decoded))
	assert.Equal(t, 1, decoded.Truncation.Shown.Lines)
	assert.Equal(t, len(jsonData), decoded.Truncation.Shown.Bytes)
}

func TestCompactReportsMetadataTruncation(t *testing.T) {
	budgets := DefaultBudgets()
	result, _ := Build(Input{
		Job: "test-metadata", CommandName: "test", ExitCode: 0,
		NormalizedCommand: "dev-tools test -- " + strings.Repeat("very-long-argument-", 100),
	}, budgets, nil)

	compact := Compact(result, budgets)
	assert.True(t, compact.Truncation.Truncated)
	assert.Contains(t, compact.Command.Normalized, "…")
}

func TestWriteArtifactsUsesNewIdentifierWhenJobAlreadyExists(t *testing.T) {
	root := t.TempDir()
	result, retained := Build(Input{Job: "test-fixed", CommandName: "test"}, DefaultBudgets(), nil)
	first, err := WriteArtifacts(root, result, retained)
	require.NoError(t, err)
	second, err := WriteArtifacts(root, result, retained)
	require.NoError(t, err)
	assert.Equal(t, "test-fixed", first.Job)
	assert.Equal(t, "test-fixed-2", second.Job)
	loaded, _, err := LoadArtifacts(root, second.Job)
	require.NoError(t, err)
	expected := Compact(loaded, DefaultBudgets())
	assert.Equal(t, expected.Truncation.Shown, loaded.Truncation.Shown)
	assert.Equal(t, expected.Truncation.Truncated, loaded.Truncation.Truncated)
}

func TestWriteArtifactsReservesCollisionSafeIdentifiersConcurrently(t *testing.T) {
	root := t.TempDir()
	result, retained := Build(Input{Job: "test-concurrent", CommandName: "test"}, DefaultBudgets(), nil)
	const writers = 8
	jobs := make(chan string, writers)
	errs := make(chan error, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			written, err := WriteArtifacts(root, result, retained)
			if err != nil {
				errs <- err
				return
			}
			jobs <- written.Job
		}()
	}
	wait.Wait()
	close(jobs)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	seen := map[string]struct{}{}
	for job := range jobs {
		seen[job] = struct{}{}
		loaded, logText, err := LoadArtifacts(root, job)
		require.NoError(t, err)
		assert.Equal(t, job, loaded.Job)
		assert.Contains(t, logText, "job: "+job)
	}
	assert.Len(t, seen, writers)
}
