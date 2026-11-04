package devresult

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationRunIsUniqueAndExposesEnvironment(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	const count = 8
	runs := make(chan ValidationRun, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, err := CreateValidationRun(root, "test:focused", now)
			runs <- run
			errs <- err
		}()
	}
	wg.Wait()
	close(runs)
	close(errs)
	seen := map[string]bool{}
	for err := range errs {
		require.NoError(t, err)
	}
	for run := range runs {
		assert.NotEmpty(t, run.ID)
		assert.False(t, seen[run.ID])
		seen[run.ID] = true
		info, err := os.Stat(run.Dir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
		env := run.Environment()
		assert.Equal(t, run.ID, env[RunIDEnvironment])
		assert.Equal(t, run.Dir, env[RunDirEnvironment])
	}
}

func TestValidationRunFreshEvidenceUsesConfiguredPatterns(t *testing.T) {
	run, err := CreateValidationRun(t.TempDir(), "test", time.Now())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(run.Dir, "startup.log"), []byte("failed"), 0o600))
	fresh, err := run.HasFreshEvidence([]string{"junit/**/*.xml", "turbo-summary.json"})
	require.NoError(t, err)
	assert.False(t, fresh)

	require.NoError(t, os.MkdirAll(filepath.Join(run.Dir, "junit", "checkout"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(run.Dir, "junit", "checkout", "result.xml"), []byte("<testsuite/>"), 0o600))
	fresh, err = run.HasFreshEvidence([]string{"junit/**/*.xml", "turbo-summary.json"})
	require.NoError(t, err)
	assert.True(t, fresh)

	_, err = run.HasFreshEvidence([]string{"../*.xml"})
	assert.ErrorContains(t, err, "unsafe evidence pattern")
}

func TestValidationRunValidatesSourceContainmentAndBounds(t *testing.T) {
	run, err := CreateValidationRun(t.TempDir(), "test", time.Now())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(run.Dir, "junit"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(run.Dir, "junit", "checkout.xml"), []byte("safe"), 0o600))

	sources, err := run.ValidateSources([]DiagnosticSource{{Kind: "junit", Path: "junit/checkout.xml"}}, DefaultAdapterLimits())
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "junit/checkout.xml", sources[0].Path)
	assert.EqualValues(t, 4, sources[0].Size)
	assert.Equal(t, []byte("safe"), sources[0].Content)

	_, err = run.ValidateSources([]DiagnosticSource{{Kind: "junit", Path: "../foreign.xml"}}, DefaultAdapterLimits())
	assert.ErrorContains(t, err, "escapes run directory")

	outside := filepath.Join(t.TempDir(), "foreign.xml")
	require.NoError(t, os.WriteFile(outside, []byte("foreign"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(run.Dir, "junit", "linked.xml")))
	_, err = run.ValidateSources([]DiagnosticSource{{Kind: "junit", Path: "junit/linked.xml"}}, DefaultAdapterLimits())
	assert.ErrorContains(t, err, "symlink escapes run directory")

	limits := DefaultAdapterLimits()
	limits.PerSourceBytes = 3
	_, err = run.ValidateSources([]DiagnosticSource{{Kind: "junit", Path: "junit/checkout.xml"}}, limits)
	assert.ErrorContains(t, err, "source size limit")
}
