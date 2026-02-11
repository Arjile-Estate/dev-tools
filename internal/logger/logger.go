package logger

import (
	"io"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog for structured logging throughout the application
var Logger zerolog.Logger

// Level represents log levels
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// Init initializes the global logger with the specified configuration
func Init(writer io.Writer, level Level, verbose bool) {
	// Set global log level
	var zeroLevel zerolog.Level
	switch level {
	case DebugLevel:
		zeroLevel = zerolog.DebugLevel
	case InfoLevel:
		zeroLevel = zerolog.InfoLevel
	case WarnLevel:
		zeroLevel = zerolog.WarnLevel
	case ErrorLevel:
		zeroLevel = zerolog.ErrorLevel
	default:
		zeroLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(zeroLevel)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339

	// Create logger with appropriate output format
	if verbose {
		// Console writer for human-readable output in verbose mode
		Logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        writer,
			TimeFormat: "15:04:05",
		}).With().Timestamp().Logger()
	} else {
		// JSON output for machine-readable logs
		Logger = zerolog.New(writer).With().Timestamp().Logger()
	}

	// Set as global logger
	log.Logger = Logger
}

// SetContext enriches the global logger with execution directory and command fields.
// These fields will be included in every subsequent log entry.
func SetContext(execDir, command string) {
	Logger = Logger.With().Str("exec_dir", execDir).Str("command", command).Logger()
	log.Logger = Logger
}

// Debug logs a debug message
func Debug(msg string) {
	Logger.Debug().Msg(msg)
}

// Debugf logs a formatted debug message
func Debugf(format string, v ...interface{}) {
	Logger.Debug().Msgf(format, v...)
}

// Info logs an info message
func Info(msg string) {
	Logger.Info().Msg(msg)
}

// Infof logs a formatted info message
func Infof(format string, v ...interface{}) {
	Logger.Info().Msgf(format, v...)
}

// Warn logs a warning message
func Warn(msg string) {
	Logger.Warn().Msg(msg)
}

// Warnf logs a formatted warning message
func Warnf(format string, v ...interface{}) {
	Logger.Warn().Msgf(format, v...)
}

// Error logs an error message
func Error(msg string) {
	Logger.Error().Msg(msg)
}

// Errorf logs a formatted error message
func Errorf(format string, v ...interface{}) {
	Logger.Error().Msgf(format, v...)
}

// With returns a new logger with the specified field added
func With() zerolog.Context {
	return Logger.With()
}

// WithError returns a new logger with the error field added
func WithError(err error) *zerolog.Event {
	return Logger.Error().Err(err)
}

// GetGlobalLevel returns the current global log level
func GetGlobalLevel() zerolog.Level {
	return zerolog.GlobalLevel()
}
