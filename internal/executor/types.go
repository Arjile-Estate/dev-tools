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
	DurationMs      int64
	ServicesStarted []string
	StartTime       time.Time
}
