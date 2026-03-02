//go:build !windows

package executor

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalProcessGroup_GroupLeader(t *testing.T) {
	// Start a process as its own group leader with a child
	cmd := exec.Command("sh", "-c", "sleep 60 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())

	pid := cmd.Process.Pid

	// Give the child time to start
	time.Sleep(50 * time.Millisecond)

	// Verify the process is its own group leader
	pgid, err := syscall.Getpgid(pid)
	require.NoError(t, err)
	assert.Equal(t, pid, pgid, "process should be its own group leader")

	// Signal the process group — should kill leader and children
	err = signalProcessGroup(pid, syscall.SIGTERM)
	assert.NoError(t, err)

	// Wait for the process to exit
	_ = cmd.Wait()

	// Verify the process is no longer running
	time.Sleep(50 * time.Millisecond)
	assert.False(t, IsProcessRunning(pid), "process should be terminated")
}

func TestSignalProcessGroup_NonGroupLeader(t *testing.T) {
	// Start a process that is NOT a group leader (no Setpgid)
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())

	pid := cmd.Process.Pid

	// This process shares the test's process group, so PGID != PID
	// signalProcessGroup should fall back to single-process signal
	err := signalProcessGroup(pid, syscall.SIGTERM)
	assert.NoError(t, err)

	// Wait for the process to exit
	_ = cmd.Wait()

	time.Sleep(50 * time.Millisecond)
	assert.False(t, IsProcessRunning(pid), "process should be terminated")
}

func TestSignalProcessGroup_NonExistentPID(t *testing.T) {
	// Use a PID that almost certainly doesn't exist
	err := signalProcessGroup(99999999, syscall.SIGTERM)
	assert.Error(t, err, "signaling non-existent PID should return error")
}

func TestSignalProcessGroup_KillsAllChildren(t *testing.T) {
	// Start a process group leader that spawns multiple children
	cmd := exec.Command("sh", "-c", "sleep 60 & sleep 60 & sleep 60 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, cmd.Start())

	pid := cmd.Process.Pid

	// Give children time to start
	time.Sleep(100 * time.Millisecond)

	// Find children in the process group
	pgid, err := syscall.Getpgid(pid)
	require.NoError(t, err)
	assert.Equal(t, pid, pgid)

	// Signal the entire group
	err = signalProcessGroup(pid, syscall.SIGKILL)
	assert.NoError(t, err)

	// Wait for the process to fully exit
	_ = cmd.Wait()
	time.Sleep(100 * time.Millisecond)

	// Verify no processes remain in the group by checking the leader
	assert.False(t, IsProcessRunning(pid), "group leader should be terminated")

	// Verify children are gone by trying to signal the group
	err = syscall.Kill(-pid, 0)
	assert.Error(t, err, "no processes should remain in the group")
	if err != nil {
		assert.True(t, os.IsPermission(err) || err == syscall.ESRCH,
			"error should be ESRCH (no such process), got: %v", err)
	}
}
