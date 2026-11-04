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
	for _, rel := range []string{filepath.Join(".build", "agent", "results", job+".json"), filepath.Join(".build", "agent", "logs", job+".log")} {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, make([]byte, bytes/2), 0o600))
		require.NoError(t, os.Chtimes(path, now.Add(-age), now.Add(-age)))
	}
	evidencePath := filepath.Join(root, ".build", "agent", "evidence", job, "junit.xml")
	require.NoError(t, os.MkdirAll(filepath.Dir(evidencePath), 0o700))
	require.NoError(t, os.WriteFile(evidencePath, []byte("evidence"), 0o600))
	require.NoError(t, os.Chtimes(evidencePath, now.Add(-age), now.Add(-age)))
	require.NoError(t, os.Chtimes(filepath.Dir(evidencePath), now.Add(-age), now.Add(-age)))
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
	report, err := ApplyRetention(root, "", now, RetentionPolicy{MaxAge: 30 * 24 * time.Hour, MaxBytes: 150})
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
