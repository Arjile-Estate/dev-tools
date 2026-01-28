package executor

import (
	"log"
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
		log.Printf("Received signal %v, terminating process", sig)
		if cmd.Process != nil {
			if sigErr := cmd.Process.Signal(sig); sigErr != nil {
				log.Printf("Failed to forward signal to child process: %v", sigErr)
				if killErr := cmd.Process.Kill(); killErr != nil {
					log.Printf("Failed to kill child process: %v", killErr)
				}
			}
		}
		return <-done
	}
}
