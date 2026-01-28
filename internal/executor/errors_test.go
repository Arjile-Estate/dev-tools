package executor

import (
	"errors"
	"testing"
)

func TestExecutionError(t *testing.T) {
	baseErr := errors.New("command not found")
	execErr := NewExecutionError("test-command", baseErr)

	if execErr.Error() != "command execution failed: command not found" {
		t.Errorf("Expected proper error message, got: %s", execErr.Error())
	}

	if execErr.Command != "test-command" {
		t.Errorf("Expected command to be 'test-command', got: %s", execErr.Command)
	}

	if errors.Unwrap(execErr) != baseErr {
		t.Error("Expected Unwrap to return base error")
	}
}

func TestServiceError(t *testing.T) {
	baseErr := errors.New("connection refused")
	svcErr := NewServiceError("postgres", "start", baseErr)

	if svcErr.Error() != "service postgres start failed: connection refused" {
		t.Errorf("Expected proper error message, got: %s", svcErr.Error())
	}

	if svcErr.Service != "postgres" {
		t.Errorf("Expected service to be 'postgres', got: %s", svcErr.Service)
	}

	if svcErr.Op != "start" {
		t.Errorf("Expected op to be 'start', got: %s", svcErr.Op)
	}

	if errors.Unwrap(svcErr) != baseErr {
		t.Error("Expected Unwrap to return base error")
	}
}

func TestValidationError(t *testing.T) {
	baseErr := errors.New("path does not exist")
	valErr := NewValidationError("directory", "/invalid/path", baseErr)

	expectedMsg := "validation failed for directory='/invalid/path': path does not exist"
	if valErr.Error() != expectedMsg {
		t.Errorf("Expected proper error message, got: %s", valErr.Error())
	}

	if valErr.Field != "directory" {
		t.Errorf("Expected field to be 'directory', got: %s", valErr.Field)
	}

	if valErr.Value != "/invalid/path" {
		t.Errorf("Expected value to be '/invalid/path', got: %s", valErr.Value)
	}

	if errors.Unwrap(valErr) != baseErr {
		t.Error("Expected Unwrap to return base error")
	}

	// Test without value
	valErrNoValue := NewValidationError("config", "", baseErr)
	expectedMsgNoValue := "validation failed for config: path does not exist"
	if valErrNoValue.Error() != expectedMsgNoValue {
		t.Errorf("Expected proper error message without value, got: %s", valErrNoValue.Error())
	}
}

func TestDaemonError(t *testing.T) {
	baseErr := errors.New("failed to write file")

	// Test with PID
	daemonErr := NewDaemonError(1234, "/tmp/daemon.pid", baseErr)
	if daemonErr.Error() != "daemon error (PID 1234): failed to write file" {
		t.Errorf("Expected proper error message with PID, got: %s", daemonErr.Error())
	}

	if daemonErr.PID != 1234 {
		t.Errorf("Expected PID to be 1234, got: %d", daemonErr.PID)
	}

	if daemonErr.PIDFile != "/tmp/daemon.pid" {
		t.Errorf("Expected PIDFile to be '/tmp/daemon.pid', got: %s", daemonErr.PIDFile)
	}

	if errors.Unwrap(daemonErr) != baseErr {
		t.Error("Expected Unwrap to return base error")
	}

	// Test without PID (file-only error)
	daemonErrNoPID := NewDaemonError(0, "/tmp/daemon.pid", baseErr)
	if daemonErrNoPID.Error() != "daemon error (/tmp/daemon.pid): failed to write file" {
		t.Errorf("Expected proper error message without PID, got: %s", daemonErrNoPID.Error())
	}
}

func TestErrorUnwrapChain(t *testing.T) {
	// Test that errors can be unwrapped multiple levels
	baseErr := errors.New("root cause")
	execErr := NewExecutionError("cmd", baseErr)

	if !errors.Is(execErr, baseErr) {
		t.Error("Expected errors.Is to find base error in chain")
	}

	// Test with wrapped service error
	svcErr := NewServiceError("redis", "stop", execErr)
	if !errors.Is(svcErr, baseErr) {
		t.Error("Expected errors.Is to find base error through multiple wraps")
	}
}
