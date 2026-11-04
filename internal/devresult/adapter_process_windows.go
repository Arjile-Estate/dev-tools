//go:build windows

package devresult

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func configureAdapterProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func killAdapterProcessTree(process *os.Process) error {
	command := exec.Command("taskkill.exe", "/T", "/F", "/PID", strconv.Itoa(process.Pid))
	if err := command.Run(); err == nil {
		return nil
	}
	return process.Kill()
}
