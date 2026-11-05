package devresult

import (
	"regexp"
	"strings"
)

var (
	secretNamePattern    = regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|private[_-]?key|api[_-]?key)`)
	secretValuePattern   = `("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s,}]+)`
	authorizationPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+)?)` + secretValuePattern)
	assignmentPattern    = regexp.MustCompile(`(?i)(["']?(?:api[_-]?key|token|secret|password|passwd|credential|private[_-]?key)["']?\s*[:=]\s*)` +
		secretValuePattern)
	credentialFlagPattern = regexp.MustCompile(`(?i)(--(?:api[_-]?key|token|secret|password|passwd|credential|private[_-]?key)(?:\s+|=))` + secretValuePattern)
	privateKeyPattern     = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
)

func Redact(input string, environment []string) (string, int) {
	redacted := input
	matches := 0
	for _, pattern := range []*regexp.Regexp{credentialFlagPattern, authorizationPattern, assignmentPattern} {
		var count int
		redacted, count = redactCapturedSecret(redacted, pattern)
		matches += count
	}
	redacted = privateKeyPattern.ReplaceAllStringFunc(redacted, func(string) string {
		matches++
		return RedactionMarker
	})
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if !ok || !secretNamePattern.MatchString(name) || len(strings.TrimSpace(value)) < 4 {
			continue
		}
		count := strings.Count(redacted, value)
		if count > 0 {
			redacted = strings.ReplaceAll(redacted, value, RedactionMarker)
			matches += count
		}
	}
	return redacted, matches
}

func RedactDiagnosticsResponse(response DiagnosticsResponse, environment []string) (DiagnosticsResponse, int) {
	redacted := response
	redacted.Failures = append([]Diagnostic(nil), response.Failures...)
	redacted.Warnings = append([]Diagnostic(nil), response.Warnings...)
	redacted.Problems = append([]EvidenceProblem(nil), response.Problems...)
	redacted.Sources = append([]DiagnosticSource(nil), response.Sources...)
	matches := 0
	redactString := func(value *string) {
		var count int
		*value, count = Redact(*value, environment)
		matches += count
	}
	for _, diagnostics := range [][]Diagnostic{redacted.Failures, redacted.Warnings} {
		for index := range diagnostics {
			redactString(&diagnostics[index].Identity)
			redactString(&diagnostics[index].Message)
			redactString(&diagnostics[index].Package)
			redactString(&diagnostics[index].File)
			redactString(&diagnostics[index].Rule)
			redactString(&diagnostics[index].Task)
			redactString(&diagnostics[index].Suite)
			redactString(&diagnostics[index].Test)
		}
	}
	for index := range redacted.Problems {
		redactString(&redacted.Problems[index].Message)
	}
	for index := range redacted.Sources {
		redactString(&redacted.Sources[index].Kind)
		redactString(&redacted.Sources[index].Path)
	}
	return redacted, matches
}

func redactCapturedSecret(input string, pattern *regexp.Regexp) (string, int) {
	matches := 0
	redacted := pattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		if len(parts) != 3 || strings.Contains(parts[2], RedactionMarker) {
			return match
		}
		matches++
		value := parts[2]
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			quote := value[:1]
			return parts[1] + quote + RedactionMarker + quote
		}
		return parts[1] + RedactionMarker
	})
	return redacted, matches
}
