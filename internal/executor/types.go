package executor

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
	Success    bool
	Stdout     string
	Stderr     string
	ReturnCode int
	PID        int
}
