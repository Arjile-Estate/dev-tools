package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		verbose  bool
		logFunc  func()
		expected string
	}{
		{
			name:    "debug level logs debug messages",
			level:   DebugLevel,
			verbose: false,
			logFunc: func() { Debug("debug message") },
		},
		{
			name:    "info level logs info messages",
			level:   InfoLevel,
			verbose: false,
			logFunc: func() { Info("info message") },
		},
		{
			name:    "warn level logs warning messages",
			level:   WarnLevel,
			verbose: false,
			logFunc: func() { Warn("warning message") },
		},
		{
			name:    "error level logs error messages",
			level:   ErrorLevel,
			verbose: false,
			logFunc: func() { Error("error message") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Init(&buf, tt.level, tt.verbose)
			tt.logFunc()

			output := buf.String()
			assert.NotEmpty(t, output, "Expected log output")

			if !tt.verbose {
				// JSON output - verify it's valid JSON
				var logEntry map[string]interface{}
				err := json.Unmarshal([]byte(output), &logEntry)
				assert.NoError(t, err, "Expected valid JSON output")
				assert.Contains(t, logEntry, "time", "Expected timestamp in log")
				assert.Contains(t, logEntry, "level", "Expected level in log")
				assert.Contains(t, logEntry, "message", "Expected message in log")
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	// Set level to Warn - should filter out Debug and Info
	var buf bytes.Buffer
	Init(&buf, WarnLevel, false)

	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	output := buf.String()

	// Debug and Info should not appear
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")

	// Warn and Error should appear
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestFormattedLogging(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, DebugLevel, false)

	Debugf("debug %s %d", "test", 123)
	Infof("info %s %d", "test", 456)
	Warnf("warn %s %d", "test", 789)
	Errorf("error %s %d", "test", 999)

	output := buf.String()

	assert.Contains(t, output, "debug test 123")
	assert.Contains(t, output, "info test 456")
	assert.Contains(t, output, "warn test 789")
	assert.Contains(t, output, "error test 999")
}

func TestVerboseModeConsoleOutput(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, InfoLevel, true)

	Info("test message")

	output := buf.String()

	// Console output should be human-readable, not JSON
	assert.NotContains(t, output, "{\"level\"")
	assert.Contains(t, output, "test message")
}

func TestWithError(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, ErrorLevel, false)

	testErr := assert.AnError
	WithError(testErr).Msg("operation failed")

	output := buf.String()

	// Check for error field in JSON output
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	assert.NoError(t, err)
	assert.Contains(t, logEntry, "error")
	assert.Contains(t, output, "operation failed")
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, InfoLevel, false)

	logger := With().Str("component", "test").Logger()
	logger.Info().Msg("test message")

	output := buf.String()

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	assert.NoError(t, err)
	assert.Equal(t, "test", logEntry["component"])
}

func TestGetGlobalLevel(t *testing.T) {
	Init(&bytes.Buffer{}, InfoLevel, false)
	level := GetGlobalLevel()
	assert.Equal(t, zerolog.InfoLevel, level)

	Init(&bytes.Buffer{}, DebugLevel, false)
	level = GetGlobalLevel()
	assert.Equal(t, zerolog.DebugLevel, level)
}

func TestLevelConversion(t *testing.T) {
	tests := []struct {
		appLevel  Level
		zeroLevel zerolog.Level
	}{
		{DebugLevel, zerolog.DebugLevel},
		{InfoLevel, zerolog.InfoLevel},
		{WarnLevel, zerolog.WarnLevel},
		{ErrorLevel, zerolog.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.zeroLevel.String(), func(t *testing.T) {
			var buf bytes.Buffer
			Init(&buf, tt.appLevel, false)
			assert.Equal(t, tt.zeroLevel, GetGlobalLevel())
		})
	}
}

func TestMultipleLogEntries(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, InfoLevel, false)

	Info("first message")
	Info("second message")
	Warn("third message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	assert.Len(t, lines, 3, "Expected 3 log lines")

	for _, line := range lines {
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(line), &logEntry)
		assert.NoError(t, err, "Each line should be valid JSON")
	}
}
