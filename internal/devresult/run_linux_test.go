//go:build linux

package devresult

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationRunRejectsFIFOWithoutBlocking(t *testing.T) {
	run, err := CreateValidationRun(t.TempDir(), "test", time.Now())
	require.NoError(t, err)
	path := filepath.Join(run.Dir, "report.xml")
	require.NoError(t, syscall.Mkfifo(path, 0o600))
	done := make(chan error, 1)
	go func() {
		_, validateErr := run.ValidateSources([]DiagnosticSource{{Kind: "junit", Path: "report.xml"}}, DefaultAdapterLimits())
		done <- validateErr
	}()
	select {
	case validateErr := <-done:
		assert.ErrorContains(t, validateErr, "not a regular file")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("source validation blocked while opening a FIFO")
	}
}
