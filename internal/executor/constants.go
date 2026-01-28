package executor

import "time"

// Daemon process management constants
const (
	// DaemonStopMaxRetries is the maximum number of attempts to wait for a daemon to stop gracefully
	// Each retry waits DaemonStopCheckInterval, giving a total of ~3 seconds for graceful shutdown
	DaemonStopMaxRetries = 30

	// DaemonStopCheckInterval is the time to wait between checking if a daemon process has stopped
	// Combined with DaemonStopMaxRetries, this allows up to 3 seconds for graceful shutdown
	DaemonStopCheckInterval = 100 * time.Millisecond

	// DaemonForceKillWaitTime is the additional time to wait after sending SIGKILL
	// This gives the OS time to forcefully terminate the process
	DaemonForceKillWaitTime = 500 * time.Millisecond

	// DaemonRestartDelay is the delay between stopping and restarting a daemon
	// This allows the old process to fully terminate and release resources
	DaemonRestartDelay = 100 * time.Millisecond
)
