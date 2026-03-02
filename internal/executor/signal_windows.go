//go:build windows

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// signalProcessGroup sends a signal to a process on Windows.
// Windows does not support Unix process groups, so this always signals
// a single process.
func signalProcessGroup(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("failed to signal process %d: %w", pid, err)
	}
	return nil
}

// describeExitSignal returns "" on Windows since Windows does not use Unix
// signal-based termination.
func describeExitSignal(exitError *exec.ExitError) string {
	_ = exitError // unused on Windows
	return ""
}
