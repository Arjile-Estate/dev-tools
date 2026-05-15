package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"dev-tools/internal/logger"
	"dev-tools/internal/logpath"

	"github.com/spf13/cobra"
)

// HandleLogsCommand displays recent activity logs. In "text" format
// (default) each entry is rendered as a single human-readable line; in
// "json" format the entries are emitted as a JSON object with a "logs"
// array, preserving every field of the original log records.
func HandleLogsCommand(cmd *cobra.Command, projectDir string) error {
	logger.Info("Displaying recent activity logs")

	logConfig := logpath.GetLogConfig(projectDir)

	if _, err := os.Stat(logConfig.File); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s", logConfig.File)
	}

	lines, err := readLastNLines(logConfig.File, logConfig.Size)
	if err != nil {
		return fmt.Errorf("failed to read logs: %w", err)
	}

	if FormatFromContext(cmd) == "json" {
		entries := make([]map[string]any, 0, len(lines))
		for _, line := range lines {
			entries = append(entries, parseLogLineForJSON(line))
		}
		return EmitJSON(cmd, map[string]any{"logs": entries})
	}

	out := cmd.OutOrStdout()
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, formatLogLine(line)); err != nil {
			return fmt.Errorf("failed to write log line: %w", err)
		}
	}

	return nil
}

// parseLogLineForJSON decodes a single log line as a generic JSON object so
// every field (including unexpected ones) survives a round-trip through the
// JSON output. Lines that aren't valid JSON are wrapped as {"raw": <line>}
// so consumers can still recover their content.
func parseLogLineForJSON(rawLine string) map[string]any {
	var entry map[string]any
	if err := json.Unmarshal([]byte(rawLine), &entry); err != nil || entry == nil {
		return map[string]any{"raw": rawLine}
	}
	return entry
}

// readLastNLines reads a file and returns the last n non-empty lines.
func readLastNLines(filePath string, n int) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	raw := strings.Split(string(data), "\n")
	// Filter out empty lines (e.g. trailing newline)
	var lines []string
	for _, l := range raw {
		if l != "" {
			lines = append(lines, l)
		}
	}

	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

// formatLogLine parses a JSON log line and formats it as:
//
//	YYYY-MM-DD HH:MM:SS+TZ - exec_dir - command - LEVEL - message
//
// Falls back to returning the raw line if JSON parsing fails.
func formatLogLine(rawLine string) string {
	var entry map[string]string
	if err := json.Unmarshal([]byte(rawLine), &entry); err != nil {
		return rawLine
	}

	ts := fieldOrDash(entry, "time")
	if ts != "-" {
		ts = formatTimestamp(ts)
	}
	execDir := fieldOrDash(entry, "exec_dir")
	command := fieldOrDash(entry, "command")
	level := fieldOrDash(entry, "level")
	if level != "-" {
		level = strings.ToUpper(level)
	}
	message := fieldOrDash(entry, "message")

	return fmt.Sprintf("%s - %s - %s - %s - %s", ts, execDir, command, level, message)
}

// formatTimestamp parses an RFC3339 timestamp and reformats it as
// "2006-01-02 15:04:05-07:00". Returns the raw string on parse error.
func formatTimestamp(raw string) string {
	if raw == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02 15:04:05-07:00")
}

func fieldOrDash(entry map[string]string, key string) string {
	v, ok := entry[key]
	if !ok || v == "" {
		return "-"
	}
	return v
}
