//go:build !windows

package executor

import (
	"os/exec"
	"syscall"
)

// setProcessGroupAttr configures the command to run in its own process group.
// On Unix systems, this sets Setpgid so the background process doesn't receive
// signals sent to the parent's process group.
func setProcessGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
