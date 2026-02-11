//go:build windows

package executor

import "os/exec"

// setProcessGroupAttr is a no-op on Windows.
// Windows does not support Unix process groups via Setpgid.
func setProcessGroupAttr(cmd *exec.Cmd) {
	// No-op: Windows uses a different process model
}
