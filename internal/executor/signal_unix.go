//go:build !windows

package executor

import (
	"fmt"
	"os/exec"
	"syscall"
)

// signalProcessGroup sends a signal to a process or its entire process group.
// If the process is a group leader (PGID == PID), the signal is sent to the
// entire process group via negative PID, killing all children.
// Otherwise, falls back to signaling just the single process (backward compat
// with daemons started before process group isolation was added).
func signalProcessGroup(pid int, sig syscall.Signal) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Getpgid can fail on zombie processes (macOS) even though the PID
		// still exists. Fall back to signaling the single PID directly.
		if killErr := syscall.Kill(pid, sig); killErr != nil {
			return fmt.Errorf("failed to signal process %d (getpgid failed: %v): %w", pid, err, killErr)
		}
		return nil
	}

	if pgid == pid {
		// Process is a group leader — signal the entire group
		if err := syscall.Kill(-pid, sig); err != nil {
			return fmt.Errorf("failed to signal process group %d: %w", pid, err)
		}
		return nil
	}

	// Not a group leader — signal the single process
	if err := syscall.Kill(pid, sig); err != nil {
		return fmt.Errorf("failed to signal process %d: %w", pid, err)
	}
	return nil
}

// describeExitSignal returns the signal name if the process was terminated by
// a signal (e.g., "terminated", "killed"), or "" if it exited normally.
func describeExitSignal(exitError *exec.ExitError) string {
	if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return status.Signal().String()
	}
	return ""
}
