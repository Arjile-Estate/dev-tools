package devresult

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createRetentionJob(t *testing.T, root, job string, age time.Duration, bytes int) {
	t.Helper()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	resultData, err := json.Marshal(Result{Schema: SchemaV1, Job: job})
	require.NoError(t, err)
	artifacts := map[string][]byte{
		filepath.Join(".build", "agent", "results", job+".json"): resultData,
		filepath.Join(".build", "agent", "logs", job+".log"):     make([]byte, bytes/2),
	}
	for rel, data := range artifacts {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, data, 0o600))
		require.NoError(t, os.Chtimes(path, now.Add(-age), now.Add(-age)))
	}
	evidencePath := filepath.Join(root, ".build", "agent", "evidence", job, "junit.xml")
	require.NoError(t, os.MkdirAll(filepath.Dir(evidencePath), 0o700))
	require.NoError(t, os.WriteFile(evidencePath, []byte("evidence"), 0o600))
	require.NoError(t, os.Chtimes(evidencePath, now.Add(-age), now.Add(-age)))
	require.NoError(t, os.Chtimes(filepath.Dir(evidencePath), now.Add(-age), now.Add(-age)))
}

func TestApplyRetentionIgnoresUnrelatedAgentLogs(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(root, ".build", "agent", "logs", "arj-3539-implementation-test.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, make([]byte, 1024), 0o600))
	require.NoError(t, os.Chtimes(path, now.Add(-30*24*time.Hour), now.Add(-30*24*time.Hour)))

	report, err := ApplyRetention(root, "", now, RetentionPolicy{MaxAge: 7 * 24 * time.Hour, MaxBytes: 1})
	require.NoError(t, err)
	assert.Empty(t, report.Removed)
	assert.FileExists(t, path)
}

func TestApplyRetentionProtectsOtherActiveValidationRuns(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	run, err := CreateValidationRun(root, "test", now)
	require.NoError(t, err)
	createRetentionJob(t, root, run.ID, 8*24*time.Hour, 100)
	require.NoError(t, os.Chtimes(filepath.Join(run.Dir, activeRunMarker), now.Add(-30*24*time.Hour), now.Add(-30*24*time.Hour)))

	report, err := ApplyRetention(root, "", now, RetentionPolicy{MaxAge: time.Nanosecond, MaxBytes: 1})
	require.NoError(t, err)
	assert.NotContains(t, report.Removed, run.ID)
	assert.FileExists(t, filepath.Join(root, ".build", "agent", "results", run.ID+".json"))
}

func TestApplyRetentionRemovesDeadOrphanScratchWithoutTouchingSameNamedLog(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	runID := "test-orphan"
	runDir := filepath.Join(root, ".build", "agent", "runs", runID)
	require.NoError(t, os.MkdirAll(runDir, 0o700))
	marker := []byte(`{"schema":"dev-tools/active-run/v1","pid":2147483647,"created_at":"2026-10-01T12:00:00Z"}`)
	for path, data := range map[string][]byte{
		filepath.Join(runDir, activeRunMarker): marker,
		filepath.Join(runDir, "secret.xml"):    []byte("secret-bearing scratch"),
	} {
		require.NoError(t, os.WriteFile(path, data, 0o600))
		require.NoError(t, os.Chtimes(path, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour)))
	}
	require.NoError(t, os.Chtimes(runDir, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour)))
	unrelatedLog := filepath.Join(root, ".build", "agent", "logs", runID+".log")
	require.NoError(t, os.MkdirAll(filepath.Dir(unrelatedLog), 0o700))
	require.NoError(t, os.WriteFile(unrelatedLog, []byte("build-agent log"), 0o600))

	report, err := ApplyRetention(root, "", now, RetentionPolicy{MaxAge: 7 * 24 * time.Hour, MaxBytes: 1 << 30})
	require.NoError(t, err)
	assert.Contains(t, report.Removed, runID)
	assert.NoDirExists(t, runDir)
	assert.FileExists(t, unrelatedLog)
}

func writeLease(t *testing.T, root, id string, lease Lease) {
	t.Helper()
	path := filepath.Join(root, ".build", "agent", "leases", id+".json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	data, err := json.Marshal(lease)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func TestApplyRetentionRemovesOldUnleasedAndPreservesCurrentAndLiveLease(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	createRetentionJob(t, root, "old", 8*24*time.Hour, 100)
	createRetentionJob(t, root, "current", 8*24*time.Hour, 100)
	createRetentionJob(t, root, "leased", 12*24*time.Hour, 100)
	writeLease(t, root, "build-42", Lease{Schema: LeaseSchemaV1, Owner: "build-42", RunIDs: []string{"leased"}, ExpiresAt: now.Add(time.Hour)})

	report, err := ApplyRetention(root, "current", now, RetentionPolicy{MaxAge: 7 * 24 * time.Hour, MaxBytes: 1 << 30})
	require.NoError(t, err)
	assert.Equal(t, []string{"old"}, report.Removed)
	_, err = os.Stat(filepath.Join(root, ".build", "agent", "evidence", "old"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	for _, job := range []string{"current", "leased"} {
		_, err = os.Stat(filepath.Join(root, ".build", "agent", "results", job+".json"))
		assert.NoError(t, err)
	}
}

func TestApplyRetentionEnforcesSizeOldestFirst(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	createRetentionJob(t, root, "oldest", 3*time.Hour, 120)
	createRetentionJob(t, root, "newest", time.Hour, 120)
	jobs, err := scanRetentionJobs(root)
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	maxSingleJob := jobs[0].size
	if jobs[1].size > maxSingleJob {
		maxSingleJob = jobs[1].size
	}
	report, err := ApplyRetention(root, "", now, RetentionPolicy{MaxAge: 30 * 24 * time.Hour, MaxBytes: maxSingleJob})
	require.NoError(t, err)
	assert.Equal(t, []string{"oldest"}, report.Removed)
}

func TestApplyRetentionFailsClosedOnMalformedLease(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	createRetentionJob(t, root, "old", 8*24*time.Hour, 100)
	leasePath := filepath.Join(root, ".build", "agent", "leases", "broken.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(leasePath), 0o700))
	require.NoError(t, os.WriteFile(leasePath, []byte("{"), 0o600))

	report, err := ApplyRetention(root, "", now, DefaultRetentionPolicy())
	require.NoError(t, err)
	assert.True(t, report.Skipped)
	assert.Contains(t, report.Warning, "malformed lease")
	assert.Empty(t, report.Removed)
	_, err = os.Stat(filepath.Join(root, ".build", "agent", "results", "old.json"))
	assert.NoError(t, err)
}
