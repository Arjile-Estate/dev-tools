package devresult

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	DiagnosticsRequestSchemaV1  = "dev-tools/diagnostics-request/v1"
	DiagnosticsResponseSchemaV1 = "dev-tools/diagnostics-response/v1"
)

// SourceError reports a returned diagnostic source that cannot be read safely: no kind, or
// a path that could leave the run directory.
type SourceError struct{ Message string }

func (err SourceError) Error() string { return err.Message }

type EvidenceState string

const (
	EvidenceComplete    EvidenceState = "complete"
	EvidencePartial     EvidenceState = "partial"
	EvidenceUnavailable EvidenceState = "unavailable"
)

type DiagnosticsRequest struct {
	Schema     string `json:"schema"`
	RunID      string `json:"run_id"`
	RunDir     string `json:"run_dir"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
	Cancelled  bool   `json:"cancelled"`
}

type Diagnostic struct {
	Identity string `json:"identity"`
	Message  string `json:"message"`
	Package  string `json:"package,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Task     string `json:"task,omitempty"`
	Suite    string `json:"suite,omitempty"`
	Test     string `json:"test,omitempty"`
}

type EvidenceProblem struct {
	Message string `json:"message"`
}

type DiagnosticSource struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type DiagnosticsResponse struct {
	Schema        string             `json:"schema"`
	EvidenceState EvidenceState      `json:"evidence_state"`
	Failures      []Diagnostic       `json:"failures,omitempty"`
	Warnings      []Diagnostic       `json:"warnings,omitempty"`
	Problems      []EvidenceProblem  `json:"problems,omitempty"`
	Sources       []DiagnosticSource `json:"sources,omitempty"`
}

type Limits struct {
	Timeout          time.Duration
	StdoutBytes      int64
	StderrBytes      int64
	MaxDiagnostics   int
	PerSourceBytes   int64
	TotalSourceBytes int64
}

func DefaultAdapterLimits() Limits {
	return Limits{
		Timeout:          2 * time.Second,
		StdoutBytes:      1 << 20,
		StderrBytes:      64 << 10,
		MaxDiagnostics:   1000,
		PerSourceBytes:   10 << 20,
		TotalSourceBytes: 50 << 20,
	}
}

func (limits Limits) withDefaults() Limits {
	defaults := DefaultAdapterLimits()
	if limits.Timeout <= 0 {
		limits.Timeout = defaults.Timeout
	}
	if limits.StdoutBytes <= 0 {
		limits.StdoutBytes = defaults.StdoutBytes
	}
	if limits.StderrBytes <= 0 {
		limits.StderrBytes = defaults.StderrBytes
	}
	if limits.MaxDiagnostics <= 0 {
		limits.MaxDiagnostics = defaults.MaxDiagnostics
	}
	if limits.PerSourceBytes <= 0 {
		limits.PerSourceBytes = defaults.PerSourceBytes
	}
	if limits.TotalSourceBytes <= 0 {
		limits.TotalSourceBytes = defaults.TotalSourceBytes
	}
	return limits
}

func ValidateDiagnosticsResponse(response DiagnosticsResponse, limits Limits) error {
	limits = limits.withDefaults()
	if response.Schema != DiagnosticsResponseSchemaV1 {
		return fmt.Errorf("unsupported diagnostics response schema %q", response.Schema)
	}
	switch response.EvidenceState {
	case EvidenceComplete, EvidencePartial, EvidenceUnavailable:
	default:
		return fmt.Errorf("invalid evidence state %q", response.EvidenceState)
	}
	diagnosticCount := len(response.Failures) + len(response.Warnings)
	count := diagnosticCount + len(response.Problems) + len(response.Sources)
	if count > limits.MaxDiagnostics {
		return fmt.Errorf("diagnostic record limit exceeded: got %d, limit %d", count, limits.MaxDiagnostics)
	}
	if response.EvidenceState == EvidenceUnavailable && diagnosticCount != 0 {
		return fmt.Errorf("unavailable evidence cannot contain diagnostics")
	}
	for group, diagnostics := range map[string][]Diagnostic{"failure": response.Failures, "warning": response.Warnings} {
		for index, diagnostic := range diagnostics {
			if strings.TrimSpace(diagnostic.Identity) == "" {
				return fmt.Errorf("%s diagnostic %d has no stable identity", group, index)
			}
			if strings.TrimSpace(diagnostic.Message) == "" {
				return fmt.Errorf("%s diagnostic %q has no message", group, diagnostic.Identity)
			}
			if diagnostic.Line < 0 {
				return fmt.Errorf("%s diagnostic %q has invalid line", group, diagnostic.Identity)
			}
			if diagnostic.Column < 0 {
				return fmt.Errorf("%s diagnostic %q has invalid column", group, diagnostic.Identity)
			}
			if diagnostic.Column > 0 && diagnostic.Line == 0 {
				return fmt.Errorf("%s diagnostic %q has a column without a line", group, diagnostic.Identity)
			}
		}
	}
	for index, problem := range response.Problems {
		if strings.TrimSpace(problem.Message) == "" {
			return fmt.Errorf("evidence problem %d has no message", index)
		}
	}
	for index, source := range response.Sources {
		if strings.TrimSpace(source.Kind) == "" {
			return SourceError{Message: fmt.Sprintf("diagnostic source %d has no kind", index)}
		}
		path := filepath.FromSlash(source.Path)
		if source.Path == "" || filepath.IsAbs(path) || filepath.Clean(path) == "." || filepath.Clean(path) == ".." || strings.HasPrefix(filepath.Clean(path), ".."+string(filepath.Separator)) {
			return SourceError{Message: fmt.Sprintf("diagnostic source %d must use a relative source path", index)}
		}
	}
	return nil
}
