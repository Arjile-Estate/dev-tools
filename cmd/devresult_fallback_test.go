package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"dev-tools/internal/devresult"
	"dev-tools/internal/logger"

	"github.com/stretchr/testify/assert"
)

func TestAdapterFallbackWarningsAndReasons(t *testing.T) {
	tests := []struct {
		name    string
		result  devresult.AdapterResult
		warning string
		reason  string
	}{
		{name: "timeout", result: devresult.AdapterResult{Err: fmt.Errorf("diagnostics adapter exceeded timeout 2s: %w", context.DeadlineExceeded), Reason: devresult.ReasonAdapterTimeout},
			warning: "structured diagnostics are unavailable: adapter timeout", reason: "adapter timeout"},
		{name: "parent deadline", result: devresult.AdapterResult{Err: fmt.Errorf("diagnostics adapter cancelled: %w", context.DeadlineExceeded), Reason: devresult.ReasonAdapterUnavailable},
			warning: "structured diagnostics are unavailable: diagnostics adapter cancelled: context deadline exceeded", reason: "structured diagnostics are unavailable"},
		{name: "version", result: devresult.AdapterResult{Err: errors.New("expected version 1, received version 2"), Reason: "expected version 1, received version 2"},
			warning: "structured diagnostics are unavailable: expected version 1, received version 2", reason: "expected version 1, received version 2"},
		{name: "malformed", result: devresult.AdapterResult{Err: errors.New("decode diagnostics adapter response: unexpected EOF"), Reason: devresult.ReasonMalformedResponse},
			warning: "structured diagnostics are unavailable: malformed adapter response: decode diagnostics adapter response: unexpected EOF", reason: "malformed adapter response"},
		{name: "unsafe source", result: devresult.AdapterResult{Err: fmt.Errorf("invalid diagnostics adapter response: %w", devresult.SourceError{Message: "diagnostic source 0 must use a relative source path"}), Reason: devresult.ReasonUnsafeEvidence},
			warning: "unsafe structured evidence: invalid diagnostics adapter response: diagnostic source 0 must use a relative source path", reason: "unsafe structured evidence"},
		{name: "invalid", result: devresult.AdapterResult{Err: errors.New(`invalid diagnostics adapter response: failure diagnostic "a" has a column without a line`), Reason: devresult.ReasonInvalidResponse},
			warning: `structured diagnostics are unavailable: invalid diagnostics adapter response: failure diagnostic "a" has a column without a line`, reason: "invalid adapter response"},
		{name: "no reason", result: devresult.AdapterResult{Err: errors.New("boom")},
			warning: "structured diagnostics are unavailable: boom", reason: "structured diagnostics are unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected := adapterFallback(collectedDiagnostics{}, tt.result)
			assert.Equal(t, tt.warning, collected.warning)
			assert.Equal(t, tt.reason, collected.fallbackReason)
		})
	}
}

func TestLogDiagnosticsFallbackRedactsTheActivityLog(t *testing.T) {
	t.Setenv("BUILD_API_TOKEN", "secret-demo-12345")
	var output bytes.Buffer
	logger.Init(&output, logger.InfoLevel, false)
	t.Cleanup(func() { logger.Init(io.Discard, logger.InfoLevel, false) })

	logDiagnosticsFallback("build", collectedDiagnostics{warning: "structured diagnostics are unavailable: token secret-demo-12345 rejected", fallbackReason: "adapter failed"})
	logDiagnosticsFallback("lint", collectedDiagnostics{fallbackReason: "reason secret-demo-12345"})

	logged := output.String()
	assert.Contains(t, logged, "DEV-RESULT build diagnostics fell back to the rendered log")
	assert.Contains(t, logged, "DEV-RESULT lint diagnostics fell back to the rendered log")
	assert.NotContains(t, logged, "secret-demo-12345")
}
