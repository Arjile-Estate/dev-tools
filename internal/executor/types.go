package executor

import "time"

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
	Success         bool
	Stdout          string
	Stderr          string
	ReturnCode      int
	PID             int
	CommandName     string
	FailedCommand   string // The actual shell command string that failed (for multi-command steps)
	DurationMs      int64
	ServicesStarted []string
	StartTime       time.Time
	Warnings        []string // Non-fatal issues (e.g., cleanup failures, PID file errors)
}
