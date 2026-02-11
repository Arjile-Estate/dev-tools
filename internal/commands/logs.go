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

// HandleLogsCommand displays recent activity logs in human-readable format
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

	out := cmd.OutOrStdout()
	for _, line := range lines {
		fmt.Fprintln(out, formatLogLine(line))
	}

	return nil
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
