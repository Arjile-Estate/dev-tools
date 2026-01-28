package colors

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// ANSI color codes
const (
	Reset    = "\033[0m"
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	Blue     = "\033[34m"
	DarkGray = "\033[90m"
)

// colorEnabled uses atomic operations for thread-safe access
// 1 = enabled, 0 = disabled
var colorEnabled atomic.Int32

func init() {
	// Default to enabled
	colorEnabled.Store(1)
}

// InitializeColorSupport sets up color support based on environment and flags
// This function is thread-safe and can be called multiple times
func InitializeColorSupport(noColor bool) {
	if noColor {
		colorEnabled.Store(0)
		return
	}

	// Check for NO_COLOR environment variable (following https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		colorEnabled.Store(0)
		return
	}

	// Check if we're in a terminal that supports colors
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		colorEnabled.Store(0)
		return
	}

	// Check if output is being piped
	if !isTerminal() {
		colorEnabled.Store(0)
		return
	}

	// Enable colors if all checks pass
	colorEnabled.Store(1)
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// colorize applies color to text if colors are enabled
func colorize(color, text string) string {
	if colorEnabled.Load() == 0 || text == "" {
		return text
	}
	return color + text + Reset
}

// Success formats text in green for success messages
func Success(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	return colorize(Green, text)
}

// Error formats text in red for error messages
func Error(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	return colorize(Red, text)
}

// Warning formats text in yellow for warning messages
func Warning(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	return colorize(Yellow, text)
}

// Info formats text in dark gray for informational messages
func Info(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	return colorize(DarkGray, text)
}

// Highlight formats text in blue for highlighted messages
func Highlight(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	return colorize(Blue, text)
}

// IsColorEnabled returns whether color output is currently enabled
// This function is thread-safe
func IsColorEnabled() bool {
	return colorEnabled.Load() == 1
}

// StripColors removes ANSI color codes from text
func StripColors(text string) string {
	// Simple regex replacement would be better, but avoiding external dependencies
	replacements := []string{
		Reset, "",
		Red, "",
		Green, "",
		Yellow, "",
		Blue, "",
		DarkGray, "",
	}

	result := text
	for i := 0; i < len(replacements); i += 2 {
		result = strings.ReplaceAll(result, replacements[i], replacements[i+1])
	}

	return result
}
