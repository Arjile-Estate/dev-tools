package devresult

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	SchemaV1        = "dev-result/v1"
	RedactionMarker = "[REDACTED]"
)

type Outcome string

const (
	OutcomePass      Outcome = "pass"
	OutcomeFail      Outcome = "fail"
	OutcomeTimeout   Outcome = "timeout"
	OutcomeCancelled Outcome = "cancelled"
)

type Budget struct {
	Lines int `json:"lines"`
	Bytes int `json:"bytes"`
}

type Budgets struct {
	Success    Budget `json:"success"`
	Failure    Budget `json:"failure"`
	Timeout    Budget `json:"timeout"`
	Background Budget `json:"background"`
}

func DefaultBudgets() Budgets {
	return Budgets{
		Success:    Budget{Lines: 10, Bytes: 1024},
		Failure:    Budget{Lines: 80, Bytes: 12 * 1024},
		Timeout:    Budget{Lines: 30, Bytes: 4 * 1024},
		Background: Budget{Lines: 3, Bytes: 1024},
	}
}

var supportedCommands = map[string]struct{}{
	"test": {}, "test:focused": {}, "build": {}, "lint": {}, "lint:dead-code": {}, "typecheck": {},
}

func SupportedCommand(name string) bool {
	_, ok := supportedCommands[name]
	return ok
}

type Command struct {
	Name       string `json:"name"`
	Normalized string `json:"normalized"`
	Scope      string `json:"scope,omitempty"`
}

type AttemptSummary struct {
	Shown int `json:"shown"`
	Total int `json:"total"`
}

type Evidence struct {
	Identity    string `json:"identity,omitempty"`
	Message     string `json:"message"`
	Source      string `json:"source"`
	Package     string `json:"package,omitempty"`
	Task        string `json:"task,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
	Rule        string `json:"rule,omitempty"`
	identityKey string
}

type DiagnosticsMode string

const (
	DiagnosticsStructured DiagnosticsMode = "structured"
	DiagnosticsFallback   DiagnosticsMode = "fallback"
)

// DiagnosticsSummary records whether a command with a diagnostics adapter used its
// structured evidence or fell back to the rendered log, and why.
type DiagnosticsSummary struct {
	Mode   DiagnosticsMode `json:"mode"`
	Reason string          `json:"reason,omitempty"`
}

type Artifacts struct {
	Log             string   `json:"log"`
	Result          string   `json:"result"`
	AdapterResponse string   `json:"adapter_response,omitempty"`
	AdapterStderr   string   `json:"adapter_stderr,omitempty"`
	Sources         []string `json:"sources,omitempty"`
}

type DrillDown struct {
	Full     string `json:"full"`
	Failures string `json:"failures,omitempty"`
	Warnings string `json:"warnings,omitempty"`
}

type Counts struct {
	Lines    int `json:"lines,omitempty"`
	Bytes    int `json:"bytes,omitempty"`
	Failures int `json:"failures,omitempty"`
	Warnings int `json:"warnings,omitempty"`
}

type Truncation struct {
	Truncated bool   `json:"truncated"`
	Budget    Budget `json:"budget"`
	Shown     Counts `json:"shown"`
	Total     Counts `json:"total"`
}

type Redaction struct {
	Applied bool `json:"applied"`
	Matches int  `json:"matches"`
}

type Result struct {
	Schema       string              `json:"schema"`
	Job          string              `json:"job"`
	Outcome      Outcome             `json:"outcome"`
	Command      Command             `json:"command"`
	ExitCode     *int                `json:"exit_code"`
	DurationMS   int64               `json:"duration_ms"`
	Attempts     AttemptSummary      `json:"attempts"`
	Summary      string              `json:"summary"`
	Diagnostics  *DiagnosticsSummary `json:"diagnostics,omitempty"`
	Failures     []Evidence          `json:"failures,omitempty"`
	Warnings     []Evidence          `json:"warnings,omitempty"`
	Artifacts    Artifacts           `json:"artifacts"`
	DrillDown    DrillDown           `json:"drill_down"`
	Truncation   Truncation          `json:"truncation"`
	Redaction    Redaction           `json:"redaction"`
	RawLineCount int                 `json:"-"`
}

type Input struct {
	Job                        string
	CommandName                string
	NormalizedCommand          string
	Scope                      string
	ExitCode                   int
	Duration                   time.Duration
	Attempts                   int
	Stdout                     string
	Stderr                     string
	Warnings                   []string
	TimedOut                   bool
	Cancelled                  bool
	Diagnostics                *DiagnosticsResponse
	DiagnosticsWarning         string
	DiagnosticsConfigured      bool
	DiagnosticsFallbackReason  string
	AdditionalRedactionMatches int
}

var (
	ansiPattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	// bannerPattern matches pnpm script headers and Turbo banners or totals, optionally behind a
	// Turbo `pkg:task: ` prefix (a task name, never a line number); noiseLine keeps any that
	// still reads as a failure.
	bannerPattern = regexp.MustCompile(`^(?:[^\s:]+:[A-Za-z][\w.-]*: )?(?:> |• |turbo \d+\.\d+|>>> FULL TURBO|(?:Tasks|Cached|Time):\s)`)
	// echoPattern matches package-manager and Turbo lines that only repeat that a task failed.
	// codeFramePattern matches a compiler code frame such as `> 12 | const rent = …`, which
	// shares the `> ` prefix of pnpm script headers but points at the failing source line.
	codeFramePattern = regexp.MustCompile(`^(?:[^\s:]+:[A-Za-z][\w.-]*: )?>\s*\d+\s*\|`)
	echoPattern      = regexp.MustCompile(`(?i)(?:\bELIFECYCLE\b|\bERR_PNPM_RECURSIVE_(?:RUN|EXEC)_FIRST_FAIL\b|^(?:\S+:\S+: )?(?:ERROR: command finished with error: )?(?:\S+#\S+: )?command \(.*\) .*exited \(\d+\)|^(?:x\s+)?ERROR\s+run failed:|^Failed:\s+\S+#\S+)`)
	failurePattern   = regexp.MustCompile(`(?i)\b(error|fail(?:ed|ure)?|fatal|panic|undefined|cannot|denied|expected|timeout)\b`)
	warningPattern   = regexp.MustCompile(`(?i)\bwarn(?:ing)?\b`)
	progressPattern  = regexp.MustCompile(`(?i)^(?:progress\s+)?\d{1,3}%$`)
)

func Build(input Input, budgets Budgets, environment []string) (Result, string) {
	stdout, stdoutMatches := Redact(input.Stdout, environment)
	stderr, stderrMatches := Redact(input.Stderr, environment)
	normalizedCommand, commandMatches := Redact(input.NormalizedCommand, environment)
	commandMatches += strings.Count(input.NormalizedCommand, RedactionMarker)
	// The scope repeats the normalized command's arguments, whose matches are already counted.
	scope, _ := Redact(input.Scope, environment)
	redactedWarnings := make([]string, 0, len(input.Warnings))
	warningMatches := 0
	for _, warning := range input.Warnings {
		redacted, count := Redact(warning, environment)
		redactedWarnings = append(redactedWarnings, redacted)
		warningMatches += count
	}

	outcome := OutcomePass
	switch {
	case input.Cancelled:
		outcome = OutcomeCancelled
	case input.TimedOut:
		outcome = OutcomeTimeout
	case input.ExitCode != 0:
		outcome = OutcomeFail
	}

	if input.Job == "" {
		input.Job = NewJobID(input.CommandName, time.Now().UTC())
	}
	if input.NormalizedCommand == "" {
		normalizedCommand = "dev-tools " + input.CommandName
	}
	if input.Attempts < 1 {
		input.Attempts = 1
	}

	failures, warnings := collectEvidence(stdout, stderr, redactedWarnings, outcome)
	structuredMatches := input.AdditionalRedactionMatches
	var diagnostics *DiagnosticsSummary
	if input.DiagnosticsConfigured {
		reason, matches := Redact(input.DiagnosticsFallbackReason, environment)
		structuredMatches += matches
		if reason == "" {
			reason = "no structured evidence was written"
		}
		diagnostics = &DiagnosticsSummary{Mode: DiagnosticsFallback, Reason: reason}
	}
	if input.Diagnostics != nil {
		structuredFailures, structuredWarnings, matches := collectStructuredEvidence(*input.Diagnostics, environment)
		structuredMatches += matches
		structuredUsed := len(structuredFailures) > 0 || outcome == OutcomePass
		if structuredUsed {
			failures = structuredFailures
		}
		warnings = append(structuredWarnings, warnings...)
		if diagnostics != nil {
			switch {
			case input.Diagnostics.EvidenceState == EvidenceUnavailable:
				diagnostics.Reason = "structured evidence unavailable"
			case structuredUsed:
				diagnostics = &DiagnosticsSummary{Mode: DiagnosticsStructured}
			default:
				diagnostics.Reason = "no actionable structured failure"
			}
		}
	}
	if strings.TrimSpace(input.DiagnosticsWarning) != "" {
		warning, matches := Redact(input.DiagnosticsWarning, environment)
		structuredMatches += matches
		warnings = append(warnings, Evidence{Message: warning, Source: "diagnostics-adapter"})
	}
	failures = deduplicateEvidence(failures)
	warnings = deduplicateEvidence(warnings)
	exitCode := input.ExitCode
	result := Result{
		Schema:      SchemaV1,
		Job:         input.Job,
		Outcome:     outcome,
		Command:     Command{Name: input.CommandName, Normalized: normalizedCommand, Scope: scope},
		ExitCode:    &exitCode,
		DurationMS:  input.Duration.Milliseconds(),
		Attempts:    AttemptSummary{Shown: input.Attempts, Total: input.Attempts},
		Diagnostics: diagnostics,
		Failures:    failures,
		Warnings:    warnings,
		Redaction: Redaction{
			Applied: stdoutMatches+stderrMatches+commandMatches+warningMatches+structuredMatches > 0,
			Matches: stdoutMatches + stderrMatches + commandMatches + warningMatches + structuredMatches,
		},
	}
	result.Summary = summary(result)
	result.Artifacts = artifactPaths(result.Job)
	result.DrillDown = drillDown(result)
	result.Truncation = Truncation{
		Budget: budgetFor(outcome, budgets),
		Shown:  Counts{Failures: len(failures), Warnings: len(warnings)},
		Total:  Counts{Failures: len(failures), Warnings: len(warnings)},
	}

	retained := fmt.Sprintf("DEV-RESULT-LOG v1\njob: %s\ncommand: %s\n\n--- stdout ---\n%s\n--- stderr ---\n%s", result.Job, result.Command.Normalized, stdout, stderr)
	result.RawLineCount = strings.Count(retained, "\n") + 1
	return result, retained
}

func collectStructuredEvidence(response DiagnosticsResponse, environment []string) ([]Evidence, []Evidence, int) {
	failures := make([]Evidence, 0, len(response.Failures))
	warnings := make([]Evidence, 0, len(response.Warnings)+len(response.Problems))
	matches := 0
	for _, diagnostic := range response.Failures {
		evidence, count := structuredEvidence(diagnostic, environment)
		matches += count
		failures = append(failures, evidence)
	}
	for _, diagnostic := range response.Warnings {
		evidence, count := structuredEvidence(diagnostic, environment)
		matches += count
		warnings = append(warnings, evidence)
	}
	for _, problem := range response.Problems {
		message, count := Redact(problem.Message, environment)
		matches += count
		warnings = append(warnings, Evidence{Message: message, Source: "diagnostics-adapter"})
	}
	if response.EvidenceState == EvidencePartial && len(response.Problems) == 0 {
		warnings = append(warnings, Evidence{Message: "Structured diagnostics evidence is partial.", Source: "diagnostics-adapter"})
	}
	return failures, warnings, matches
}

func structuredEvidence(diagnostic Diagnostic, environment []string) (Evidence, int) {
	matches := 0
	redact := func(value string) string {
		redacted, count := Redact(value, environment)
		matches += count
		return redacted
	}
	evidence := Evidence{
		Identity:    redact(diagnostic.Identity),
		Message:     redact(formatDiagnostic(diagnostic)),
		Source:      "diagnostics-adapter",
		Package:     redact(diagnostic.Package),
		Task:        redact(diagnostic.Task),
		File:        redact(diagnostic.File),
		Line:        diagnostic.Line,
		Column:      diagnostic.Column,
		Rule:        redact(diagnostic.Rule),
		identityKey: diagnostic.Identity,
	}
	return evidence, matches
}

// formatDiagnostic renders `<task>: <message> (<file>:<line>:<column>) [<rule>]`, omitting
// every part the adapter did not send.
func formatDiagnostic(diagnostic Diagnostic) string {
	message := diagnostic.Message
	if diagnostic.Task != "" {
		message = diagnostic.Task + ": " + message
	}
	location := diagnostic.File
	if location != "" && diagnostic.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, diagnostic.Line)
		if diagnostic.Column > 0 {
			location = fmt.Sprintf("%s:%d", location, diagnostic.Column)
		}
	}
	if location != "" {
		message = fmt.Sprintf("%s (%s)", message, location)
	}
	if diagnostic.Rule != "" {
		message = fmt.Sprintf("%s [%s]", message, diagnostic.Rule)
	}
	return message
}

func deduplicateEvidence(items []Evidence) []Evidence {
	result := make([]Evidence, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Message))
		if identity := strings.TrimSpace(item.identityKey); identity != "" {
			key = "identity:" + strings.ToLower(identity)
		} else if identity := strings.TrimSpace(item.Identity); identity != "" {
			key = "identity:" + strings.ToLower(identity)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func summary(result Result) string {
	switch result.Outcome {
	case OutcomePass:
		return fmt.Sprintf("Validation passed with %d warning(s).", len(result.Warnings))
	case OutcomeTimeout:
		return fmt.Sprintf("Validation timed out with %d distinct failure(s) and %d warning(s).", len(result.Failures), len(result.Warnings))
	case OutcomeCancelled:
		return fmt.Sprintf("Validation was cancelled with %d distinct failure(s) and %d warning(s).", len(result.Failures), len(result.Warnings))
	default:
		return fmt.Sprintf("Validation failed with %d distinct failure(s) and %d warning(s).", len(result.Failures), len(result.Warnings))
	}
}

func WithArtifactWarning(result Result, err error) Result {
	result.Artifacts = Artifacts{Log: "unavailable", Result: "unavailable"}
	result.DrillDown = DrillDown{Full: "unavailable"}
	result.Warnings = append([]Evidence{{Message: "DEV-RESULT artifacts unavailable: " + err.Error(), Source: "dev-tools"}}, result.Warnings...)
	result.Summary = summary(result)
	return result
}

func collectEvidence(stdout, stderr string, configuredWarnings []string, outcome Outcome) ([]Evidence, []Evidence) {
	type sourcedText struct {
		text, source string
		forceWarning bool
	}
	var lines []sourcedText
	for _, stream := range []sourcedText{{text: stdout, source: "stdout"}, {text: stderr, source: "stderr"}} {
		normalized := strings.ReplaceAll(stream.text, "\r", "\n")
		for _, line := range strings.Split(normalized, "\n") {
			line = strings.TrimSpace(ansiPattern.ReplaceAllString(line, ""))
			if line == "" || noiseLine(line) {
				continue
			}
			lines = append(lines, sourcedText{text: line, source: stream.source})
		}
	}
	for _, warning := range configuredWarnings {
		lines = append(lines, sourcedText{text: strings.TrimSpace(ansiPattern.ReplaceAllString(warning, "")), source: "dev-tools", forceWarning: true})
	}

	var failures, warnings []Evidence
	seenFailures := map[string]struct{}{}
	seenWarnings := map[string]struct{}{}
	var fallback, echoes []Evidence
	for _, line := range lines {
		if !line.forceWarning && echoPattern.MatchString(line.text) {
			echoes = append(echoes, Evidence{Message: line.text, Source: line.source})
			continue
		}
		key := strings.ToLower(line.text)
		if line.forceWarning || warningPattern.MatchString(line.text) {
			if _, exists := seenWarnings[key]; !exists {
				seenWarnings[key] = struct{}{}
				warnings = append(warnings, Evidence{Message: line.text, Source: line.source})
			}
			continue
		}
		evidence := Evidence{Message: line.text, Source: line.source}
		fallback = append(fallback, evidence)
		if failurePattern.MatchString(line.text) {
			if _, exists := seenFailures[key]; !exists {
				seenFailures[key] = struct{}{}
				failures = append(failures, evidence)
			}
		}
	}
	if outcome != OutcomePass && len(failures) == 0 {
		// No line names a failure: keep the first remaining line, which may be the cause the
		// keywords missed, then the first echo, which at least says that the task failed.
		if len(fallback) > 0 {
			failures = append(failures, fallback[0])
		}
		if len(echoes) > 0 {
			failures = append(failures, echoes[0])
		}
	}
	return failures, warnings
}

func noiseLine(line string) bool {
	if progressPattern.MatchString(line) || (bannerPattern.MatchString(line) && !failurePattern.MatchString(line) && !codeFramePattern.MatchString(line)) {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "progress ") && strings.HasSuffix(lower, "%")
}

func budgetFor(outcome Outcome, budgets Budgets) Budget {
	switch outcome {
	case OutcomePass:
		return budgets.Success
	case OutcomeTimeout, OutcomeCancelled:
		return budgets.Timeout
	default:
		return budgets.Failure
	}
}

func artifactPaths(job string) Artifacts {
	return Artifacts{
		Log:    ".build/agent/logs/" + job + ".log",
		Result: ".build/agent/results/" + job + ".json",
	}
}

func drillDown(result Result) DrillDown {
	base := "dev-tools result " + result.Job
	return DrillDown{
		Full:     base + " --full",
		Failures: base + " --failures",
		Warnings: base + " --warnings",
	}
}

func RenderText(result Result, budgets Budgets) string {
	compact := Compact(result, budgets)
	return render(compact)
}

func RenderStoredText(result Result) string {
	budgets := DefaultBudgets()
	switch result.Outcome {
	case OutcomePass:
		budgets.Success = result.Truncation.Budget
	case OutcomeTimeout, OutcomeCancelled:
		budgets.Timeout = result.Truncation.Budget
	default:
		budgets.Failure = result.Truncation.Budget
	}
	return RenderText(result, budgets)
}

func Compact(result Result, budgets Budgets) Result {
	originalCommand := result.Command.Normalized
	originalScope := result.Command.Scope
	originalSummary := result.Summary
	previouslyTruncated := result.Truncation.Truncated
	result.Truncation.Budget = budgetFor(result.Outcome, budgets)
	result.Truncation.Total.Failures = len(result.Failures)
	result.Truncation.Total.Warnings = len(result.Warnings)
	result.Command.Normalized = truncateText(result.Command.Normalized, 256)
	result.Command.Scope = truncateText(result.Command.Scope, 256)
	result.Summary = truncateText(result.Summary, 160)
	diagnosticsTruncated := false
	if result.Diagnostics != nil {
		diagnostics := *result.Diagnostics
		diagnostics.Reason = truncateText(diagnostics.Reason, 160)
		diagnosticsTruncated = diagnostics.Reason != result.Diagnostics.Reason
		result.Diagnostics = &diagnostics
	}
	metadataTruncated := previouslyTruncated || diagnosticsTruncated || result.Command.Normalized != originalCommand || result.Command.Scope != originalScope || result.Summary != originalSummary
	failureCount := len(result.Failures)
	warningCount := len(result.Warnings)

	for {
		candidate := result
		candidate.Failures = append([]Evidence(nil), result.Failures[:failureCount]...)
		candidate.Warnings = append([]Evidence(nil), result.Warnings[:warningCount]...)
		candidate.Truncation.Shown.Failures = failureCount
		candidate.Truncation.Shown.Warnings = warningCount
		candidate.Truncation.Truncated = metadataTruncated || failureCount < len(result.Failures) || warningCount < len(result.Warnings)
		measureText(&candidate)
		if candidate.Truncation.Shown.Lines <= candidate.Truncation.Budget.Lines && candidate.Truncation.Shown.Bytes <= candidate.Truncation.Budget.Bytes {
			return candidate
		}
		if warningCount > 0 {
			warningCount--
		} else if failureCount > 0 {
			failureCount--
		} else {
			shrinkMetadata(&candidate)
			measureText(&candidate)
			return candidate
		}
	}
}

func MarshalCompactJSON(result Result, budgets Budgets) ([]byte, error) {
	compact := Compact(result, budgets)
	// Like the text envelope, name a drill-down only for evidence that exists.
	if compact.Truncation.Total.Failures == 0 {
		compact.DrillDown.Failures = ""
	}
	if compact.Truncation.Total.Warnings == 0 {
		compact.DrillDown.Warnings = ""
	}
	for {
		data, err := marshalMeasuredJSON(&compact)
		if err != nil {
			return nil, err
		}
		if compact.Truncation.Shown.Lines <= compact.Truncation.Budget.Lines && len(data) <= compact.Truncation.Budget.Bytes {
			return data, nil
		}
		compact.Truncation.Truncated = true
		switch {
		case len(compact.Artifacts.Sources) > 0:
			compact.Artifacts.Sources = nil
		case compact.Artifacts.AdapterStderr != "":
			compact.Artifacts.AdapterStderr = ""
		case compact.Artifacts.AdapterResponse != "":
			compact.Artifacts.AdapterResponse = ""
		case hasEvidenceDetail(compact.Failures) || hasEvidenceDetail(compact.Warnings):
			// The message already renders task, location and rule, and the identity only serves
			// de-duplication; the sidecar keeps every field.
			stripEvidenceDetail(compact.Failures)
			stripEvidenceDetail(compact.Warnings)
		case shortenEvidence(compact.Failures, compact.Warnings):
			// Shorten messages before dropping evidence; the sidecar keeps them whole.
		case len(compact.Warnings) > 0:
			compact.Warnings = compact.Warnings[:len(compact.Warnings)-1]
			compact.Truncation.Shown.Warnings = len(compact.Warnings)
		case len(compact.Failures) > 0:
			compact.Failures = compact.Failures[:len(compact.Failures)-1]
			compact.Truncation.Shown.Failures = len(compact.Failures)
		case len(compact.Command.Scope) > 32:
			compact.Command.Scope = truncateText(compact.Command.Scope, len(compact.Command.Scope)-16)
		case len(compact.Command.Normalized) > 32:
			compact.Command.Normalized = truncateText(compact.Command.Normalized, len(compact.Command.Normalized)-16)
		case len(compact.Summary) > 32:
			compact.Summary = truncateText(compact.Summary, len(compact.Summary)-16)
		case compact.Diagnostics != nil:
			compact.Diagnostics = nil
		default:
			return nil, fmt.Errorf("DEV-RESULT JSON metadata exceeds %d-byte budget", compact.Truncation.Budget.Bytes)
		}
	}
}

func marshalMeasuredJSON(result *Result) ([]byte, error) {
	for iteration := 0; iteration < 8; iteration++ {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		lines := len(strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
		bytes := len(data)
		if result.Truncation.Shown.Lines == lines && result.Truncation.Shown.Bytes == bytes {
			return data, nil
		}
		result.Truncation.Shown.Lines = lines
		result.Truncation.Shown.Bytes = bytes
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func measureText(result *Result) {
	for iteration := 0; iteration < 4; iteration++ {
		text := render(*result)
		lines := len(strings.Split(strings.TrimSuffix(text, "\n"), "\n"))
		bytes := len([]byte(text))
		if result.Truncation.Shown.Lines == lines && result.Truncation.Shown.Bytes == bytes {
			return
		}
		result.Truncation.Shown.Lines = lines
		result.Truncation.Shown.Bytes = bytes
	}
}

func hasEvidenceDetail(items []Evidence) bool {
	for _, item := range items {
		if item.Identity != "" || item.Package != "" || item.Task != "" || item.File != "" || item.Line != 0 || item.Column != 0 || item.Rule != "" {
			return true
		}
	}
	return false
}

func stripEvidenceDetail(items []Evidence) {
	for index := range items {
		items[index].Identity, items[index].Package, items[index].Task, items[index].File, items[index].Rule = "", "", "", "", ""
		items[index].Line, items[index].Column = 0, 0
	}
}

// compactEvidenceBytes are the successive message lengths compact JSON tries before it drops
// an evidence item.
var compactEvidenceBytes = []int{120, 80, 48}

// shortenEvidence cuts every message longer than the next limit down to it, and reports
// whether anything changed.
func shortenEvidence(groups ...[]Evidence) bool {
	for _, limit := range compactEvidenceBytes {
		shortened := false
		for _, items := range groups {
			for index := range items {
				if len(items[index].Message) > limit {
					items[index].Message = truncateText(items[index].Message, limit)
					shortened = true
				}
			}
		}
		if shortened {
			return true
		}
	}
	return false
}

func shrinkMetadata(result *Result) {
	for {
		measureText(result)
		if result.Truncation.Shown.Lines <= result.Truncation.Budget.Lines && result.Truncation.Shown.Bytes <= result.Truncation.Budget.Bytes {
			return
		}
		switch {
		case result.Diagnostics != nil && result.Truncation.Shown.Lines > result.Truncation.Budget.Lines:
			// A budget at the 7-line minimum has no room for the diagnostics line; the sidecar keeps it.
			result.Diagnostics = nil
		case len(result.Command.Scope) > 32:
			result.Command.Scope = truncateText(result.Command.Scope, len(result.Command.Scope)-16)
		case len(result.Command.Normalized) > 32:
			result.Command.Normalized = truncateText(result.Command.Normalized, len(result.Command.Normalized)-16)
		case len(result.Summary) > 32:
			result.Summary = truncateText(result.Summary, len(result.Summary)-16)
		case result.Diagnostics != nil:
			result.Diagnostics = nil
		default:
			return
		}
		result.Truncation.Truncated = true
	}
}

func truncateText(value string, maximumBytes int) string {
	if maximumBytes < 1 {
		return ""
	}
	if len([]byte(value)) <= maximumBytes {
		return value
	}
	const suffix = "…"
	limit := maximumBytes - len([]byte(suffix))
	if limit < 1 {
		return ""
	}
	var builder strings.Builder
	for _, character := range value {
		if builder.Len()+len(string(character)) > limit {
			break
		}
		builder.WriteRune(character)
	}
	return builder.String() + suffix
}

func render(result Result) string {
	exit := "null"
	if result.ExitCode != nil {
		exit = fmt.Sprintf("%d", *result.ExitCode)
	}
	command := "command: " + result.Command.Normalized
	if result.Command.Scope != "" {
		command += " | scope: " + result.Command.Scope
	}
	lines := []string{
		"DEV-RESULT v1 " + strings.ToUpper(string(result.Outcome)),
		command,
		fmt.Sprintf("exit: %s | duration: %s | attempts: %d/%d", exit, formatDuration(result.DurationMS), result.Attempts.Shown, result.Attempts.Total),
		"summary: " + result.Summary,
	}
	if result.Diagnostics != nil {
		diagnostics := "diagnostics: " + string(result.Diagnostics.Mode)
		if result.Diagnostics.Reason != "" {
			diagnostics += " (" + result.Diagnostics.Reason + ")"
		}
		lines = append(lines, diagnostics)
	}
	if len(result.Failures) > 0 {
		lines = append(lines, fmt.Sprintf("failures: shown %d/%d", len(result.Failures), result.Truncation.Total.Failures))
		for _, item := range result.Failures {
			lines = append(lines, "- "+item.Message)
		}
	}
	if len(result.Warnings) > 0 {
		lines = append(lines, fmt.Sprintf("warnings: shown %d/%d", len(result.Warnings), result.Truncation.Total.Warnings))
		for _, item := range result.Warnings {
			lines = append(lines, "- "+item.Message)
		}
	}
	lines = append(lines,
		fmt.Sprintf("artifacts: log=%s | result=%s", result.Artifacts.Log, result.Artifacts.Result),
		formatDrillDown(result),
		fmt.Sprintf("truncation: %t | lines=%d/%d | bytes=%d/%d | failures=%d/%d | warnings=%d/%d",
			result.Truncation.Truncated,
			result.Truncation.Shown.Lines, result.Truncation.Budget.Lines,
			result.Truncation.Shown.Bytes, result.Truncation.Budget.Bytes,
			result.Truncation.Shown.Failures, result.Truncation.Total.Failures,
			result.Truncation.Shown.Warnings, result.Truncation.Total.Warnings),
	)
	return strings.Join(lines, "\n") + "\n"
}

func formatDrillDown(result Result) string {
	items := []string{"full=" + result.DrillDown.Full}
	if result.Truncation.Total.Failures > 0 {
		items = append(items, "failures="+result.DrillDown.Failures)
	}
	if result.Truncation.Total.Warnings > 0 {
		items = append(items, "warnings="+result.DrillDown.Warnings)
	}
	return "drill-down: " + strings.Join(items, " | ")
}

func formatDuration(milliseconds int64) string {
	return (time.Duration(milliseconds) * time.Millisecond).String()
}

func NewJobID(command string, now time.Time) string {
	command = strings.ToLower(command)
	command = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(command, "-")
	command = strings.Trim(command, "-")
	if command == "" {
		command = "job"
	}
	return command + "-" + now.UTC().Format("20060102T150405.000000000Z")
}

func NormalizeCommand(command string, args []string) string {
	parts := []string{"dev-tools", command}
	if scope := NormalizeScope(args); scope != "" {
		parts = append(parts, scope)
	}
	return strings.Join(parts, " ")
}

// NormalizeScope renders the requested passthrough arguments, credential-redacted and
// shell-quoted, separately from the command name.
func NormalizeScope(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range redactCredentialArguments(args) {
		if safeArgumentPattern.MatchString(arg) {
			parts = append(parts, arg)
		} else {
			parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
		}
	}
	return strings.Join(parts, " ")
}

var safeArgumentPattern = regexp.MustCompile(`^[A-Za-z0-9_./:=+-]+$`)

var credentialArgumentPattern = regexp.MustCompile(`(?i)^(--(?:api[_-]?key|token|secret|password|passwd|credential|private[_-]?key))(?:=(.*))?$`)

func redactCredentialArguments(args []string) []string {
	redacted := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		parts := credentialArgumentPattern.FindStringSubmatch(arg)
		if len(parts) != 3 {
			redacted = append(redacted, arg)
			continue
		}
		if strings.Contains(arg, "=") {
			redacted = append(redacted, parts[1]+"="+RedactionMarker)
			continue
		}
		redacted = append(redacted, parts[1])
		if index+1 < len(args) {
			redacted = append(redacted, RedactionMarker)
			index++
		}
	}
	return redacted
}

func SortedSupportedCommands() []string {
	commands := make([]string, 0, len(supportedCommands))
	for command := range supportedCommands {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}
