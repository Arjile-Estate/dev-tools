package devresult

import (
	"regexp"
	"strings"
)

var (
	secretNamePattern     = regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|private[_-]?key|api[_-]?key)`)
	authorizationPattern  = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+)?)([^\s]+)`)
	assignmentPattern     = regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password|passwd|credential|private[_-]?key)\s*[:=]\s*)([^\s]+)`)
	credentialFlagPattern = regexp.MustCompile(`(?i)(--(?:api[_-]?key|token|secret|password|passwd|credential|private[_-]?key)(?:\s+|=))([^\s]+)`)
	privateKeyPattern     = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
)

func Redact(input string, environment []string) (string, int) {
	redacted := input
	matches := 0
	redacted = credentialFlagPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		if strings.Contains(match, RedactionMarker) {
			return match
		}
		matches++
		location := strings.IndexAny(match, "= \t")
		if location < 0 {
			return match
		}
		return match[:location+1] + RedactionMarker
	})
	for _, pattern := range []*regexp.Regexp{authorizationPattern, assignmentPattern} {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(match string) string {
			if strings.Contains(match, RedactionMarker) {
				return match
			}
			matches++
			parts := regexp.MustCompile(`[:=]`).Split(match, 2)
			separator := ": "
			if strings.Contains(match, "=") && !strings.Contains(match, ":") {
				separator = "="
			}
			prefix := parts[0]
			if strings.Contains(strings.ToLower(match), "bearer") {
				return prefix + separator + "Bearer " + RedactionMarker
			}
			return prefix + separator + RedactionMarker
		})
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
