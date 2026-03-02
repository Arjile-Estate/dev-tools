package executor

import (
	"dev-tools/internal/logger"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// waitForProcessWithSignalHandling waits for a process to complete while handling signals
// It forwards SIGINT and SIGTERM to the child process for graceful shutdown
func waitForProcessWithSignalHandling(cmd *exec.Cmd) error {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case sig := <-signalChan:
		logger.Infof("Received signal %v, terminating process", sig)
		if cmd.Process != nil {
			sysSig, ok := sig.(syscall.Signal)
			if !ok {
				sysSig = syscall.SIGTERM
			}
			if sigErr := signalProcessGroup(cmd.Process.Pid, sysSig); sigErr != nil {
				logger.Infof("Failed to forward signal to process group: %v", sigErr)
				if killErr := signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL); killErr != nil {
					logger.Infof("Failed to kill process group: %v", killErr)
				}
			}
		}
		return <-done
	}
}
