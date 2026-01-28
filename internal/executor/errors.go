package executor

import "fmt"

// ExecutionError represents errors that occur during command execution
type ExecutionError struct {
	Command string
	Err     error
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("command execution failed: %v", e.Err)
}

func (e *ExecutionError) Unwrap() error {
	return e.Err
}

// NewExecutionError creates a new ExecutionError
func NewExecutionError(command string, err error) *ExecutionError {
	return &ExecutionError{
		Command: command,
		Err:     err,
	}
}

// ServiceError represents errors related to Docker service management
type ServiceError struct {
	Service string
	Op      string // Operation: "start", "stop", "health_check"
	Err     error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("service %s %s failed: %v", e.Service, e.Op, e.Err)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a new ServiceError
func NewServiceError(service, op string, err error) *ServiceError {
	return &ServiceError{
		Service: service,
		Op:      op,
		Err:     err,
	}
}

// ValidationError represents validation errors (directories, configuration, etc.)
type ValidationError struct {
	Field string
	Value string
	Err   error
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation failed for %s='%s': %v", e.Field, e.Value, e.Err)
	}
	return fmt.Sprintf("validation failed for %s: %v", e.Field, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, value string, err error) *ValidationError {
	return &ValidationError{
		Field: field,
		Value: value,
		Err:   err,
	}
}

// DaemonError represents errors related to daemon process management
type DaemonError struct {
	PID     int
	PIDFile string
	Err     error
}

func (e *DaemonError) Error() string {
	if e.PID > 0 {
		return fmt.Sprintf("daemon error (PID %d): %v", e.PID, e.Err)
	}
	return fmt.Sprintf("daemon error (%s): %v", e.PIDFile, e.Err)
}

func (e *DaemonError) Unwrap() error {
	return e.Err
}

// NewDaemonError creates a new DaemonError
func NewDaemonError(pid int, pidFile string, err error) *DaemonError {
	return &DaemonError{
		PID:     pid,
		PIDFile: pidFile,
		Err:     err,
	}
}
