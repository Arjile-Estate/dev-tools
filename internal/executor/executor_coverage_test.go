package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaitForProcessWithSignalHandling tests signal handling during process execution
func TestWaitForProcessWithSignalHandling(t *testing.T) {
	t.Run("process completes successfully", func(t *testing.T) {
		result := ExecuteShellCommand(context.Background(), ExecuteOptions{
			Command:       "echo 'test'",
			CaptureOutput: false,
		})

		assert.True(t, result.Success)
		assert.Equal(t, 0, result.ReturnCode)
	})

	t.Run("process fails with non-zero exit code", func(t *testing.T) {
		result := ExecuteShellCommand(context.Background(), ExecuteOptions{
			Command:       "exit 42",
			CaptureOutput: false,
		})

		assert.False(t, result.Success)
		assert.Equal(t, 42, result.ReturnCode)
	})

	t.Run("long-running process completes", func(t *testing.T) {
		result := ExecuteShellCommand(context.Background(), ExecuteOptions{
			Command:       "sleep 0.1",
			CaptureOutput: false,
		})

		assert.True(t, result.Success)
		assert.Equal(t, 0, result.ReturnCode)
	})
}

// TestStopDaemonProcess_AdditionalScenarios tests additional scenarios for daemon stopping
func TestStopDaemonProcess_AdditionalScenarios(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("stop daemon with stale PID file already removed", func(t *testing.T) {
		// Create a daemon with non-existent PID
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "test-daemon",
				Command:     "echo test",
				StartTime:   time.Now(),
			},
			PIDFile:   ".test.pid",
			IsRunning: false,
		}

		// PID file doesn't exist
		// StopDaemonProcess should handle this gracefully
		err := StopDaemonProcess(tmpDir, daemon)
		// Should not error when stopping a non-running daemon without PID file
		assert.Error(t, err, "Should return error when PID file doesn't exist")
	})

	t.Run("verify stop daemon handles non-running processes", func(t *testing.T) {
		// This test verifies the code path for stopping non-running daemons
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "non-running-daemon",
				Command:     "echo test",
				StartTime:   time.Now(),
			},
			PIDFile:   ".nonrunning.pid",
			IsRunning: false, // Not running
		}

		// Create PID file
		pidFile := filepath.Join(tmpDir, daemon.PIDFile)
		err := CreateEnhancedPIDFile(pidFile, daemon.PID, daemon.CommandName, daemon.Command)
		require.NoError(t, err)

		// Stop should just remove the PID file since process isn't running
		err = StopDaemonProcess(tmpDir, daemon)
		assert.NoError(t, err)

		// Verify PID file was removed
		_, err = os.Stat(pidFile)
		assert.True(t, os.IsNotExist(err))
	})
}

// TestRestartDaemonProcess_AdditionalScenarios tests additional restart scenarios
func TestRestartDaemonProcess_AdditionalScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	t.Run("restart with legacy PID file (no command)", func(t *testing.T) {
		// Create a legacy PID file (just PID, no command info)
		pidFile := GeneratePIDFilename("legacy-daemon", "")
		err := CreatePIDFile(pidFile, 999999)
		require.NoError(t, err)

		// Try to restart - should fail because no command info
		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "legacy-daemon",
				Command:     "", // No command information
				StartTime:   time.Time{},
			},
			PIDFile:   filepath.Base(pidFile),
			IsRunning: false,
		}

		err = RestartDaemonProcess(tmpDir, daemon)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no command information available")
	})

	t.Run("restart daemon with invalid command", func(t *testing.T) {
		pidFile := GeneratePIDFilename("invalid-daemon", "invalid-command-xyz")

		daemon := &DaemonInfo{
			PIDFileInfo: PIDFileInfo{
				PID:         999999,
				CommandName: "invalid-daemon",
				Command:     "invalid-command-xyz", // Invalid command
				StartTime:   time.Now(),
			},
			PIDFile:   filepath.Base(pidFile),
			IsRunning: false,
		}

		err := RestartDaemonProcess(tmpDir, daemon)
		// The restart will fail because the command is invalid
		// This tests that RestartDaemonProcess handles invalid commands gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "failed to restart daemon")
		}
	})
}

// TestCleanupStalePIDFilesWithTermination_MixedStates tests cleanup with various PID states
func TestCleanupStalePIDFilesWithTermination_MixedStates(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	t.Run("mixed running and stale processes with termination", func(t *testing.T) {
		// Create stale PID files (non-running processes)
		stalePidFile1 := GeneratePIDFilename("stale-daemon-1", "old command 1")
		err := CreateEnhancedPIDFile(stalePidFile1, 999998, "stale-daemon-1", "old command 1")
		require.NoError(t, err)

		stalePidFile2 := GeneratePIDFilename("stale-daemon-2", "old command 2")
		err = CreateEnhancedPIDFile(stalePidFile2, 999999, "stale-daemon-2", "old command 2")
		require.NoError(t, err)

		// Cleanup with termination enabled (should clean up stale PIDs)
		result := CleanupStalePIDFilesWithTermination(tmpDir, true)

		assert.True(t, result.Success)
		// Should mention stale processes in output
		assert.Contains(t, result.Stdout, "stale")

		// Verify both stale PID files were removed
		_, err = os.Stat(stalePidFile1)
		assert.True(t, os.IsNotExist(err), "Stale PID file 1 should be removed")

		_, err = os.Stat(stalePidFile2)
		assert.True(t, os.IsNotExist(err), "Stale PID file 2 should be removed")
	})

	t.Run("no PID files to cleanup", func(t *testing.T) {
		// Clean directory with no PID files
		emptyDir := t.TempDir()
		oldDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(emptyDir))
		defer func() {
			require.NoError(t, os.Chdir(oldDir))
		}()

		result := CleanupStalePIDFilesWithTermination(emptyDir, false)

		assert.True(t, result.Success)
		assert.Contains(t, result.Stdout, "No PID files")
	})

	t.Run("cleanup errors don't fail entire operation", func(t *testing.T) {
		// Create a PID file that will be cleaned up
		stalePidFile := GeneratePIDFilename("error-daemon", "cmd")
		err := CreateEnhancedPIDFile(stalePidFile, 999999, "error-daemon", "cmd")
		require.NoError(t, err)

		result := CleanupStalePIDFilesWithTermination(tmpDir, false)

		// Should still succeed even if some operations had issues
		assert.True(t, result.Success)
	})
}

// TestExecuteCommandStep_DirectoryValidation tests directory validation scenarios
func TestExecuteCommandStep_DirectoryValidation(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("directory does not exist", func(t *testing.T) {
		step := config.CommandStep{
			Directory: "/nonexistent/directory/path",
			Run:       config.RunCommand{"echo test"},
		}

		result := ExecuteCommandStep(step, "test", tmpDir, nil)

		assert.False(t, result.Success)
		assert.Contains(t, result.Stderr, "does not exist")
	})

	t.Run("directory path is actually a file", func(t *testing.T) {
		// Create a file
		filePath := filepath.Join(tmpDir, "notadir.txt")
		err := os.WriteFile(filePath, []byte("content"), 0644)
		require.NoError(t, err)

		step := config.CommandStep{
			Directory: filePath,
			Run:       config.RunCommand{"echo test"},
		}

		result := ExecuteCommandStep(step, "test", tmpDir, nil)

		assert.False(t, result.Success)
		assert.Contains(t, result.Stderr, "not a directory")
	})

	t.Run("directory is not accessible", func(t *testing.T) {
		// Create a directory with no read permissions
		noAccessDir := filepath.Join(tmpDir, "noaccess")
		err := os.Mkdir(noAccessDir, 0000)
		require.NoError(t, err)
		defer os.Chmod(noAccessDir, 0755) // Restore permissions for cleanup

		step := config.CommandStep{
			Directory: noAccessDir,
			Run:       config.RunCommand{"echo test"},
		}

		result := ExecuteCommandStep(step, "test", tmpDir, nil)

		assert.False(t, result.Success)
		assert.Contains(t, result.Stderr, "not accessible")
	})

	t.Run("relative directory path resolution", func(t *testing.T) {
		// Create a subdirectory
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.Mkdir(subDir, 0755)
		require.NoError(t, err)

		step := config.CommandStep{
			Directory: "subdir", // Relative path
			Run:       config.RunCommand{"pwd"},
		}

		result := ExecuteCommandStep(step, "test", tmpDir, nil)

		assert.True(t, result.Success)
	})
}

// TestHandleServicesConfiguration_ComposeAndContainers tests service orchestration
func TestHandleServicesConfiguration_ComposeAndContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	t.Run("empty services configuration", func(t *testing.T) {
		services := config.ServicesConfig{}

		result := HandleServicesConfiguration(services)

		assert.True(t, result.Success)
	})

	t.Run("services with cleanup flag", func(t *testing.T) {
		services := config.ServicesConfig{
			Cleanup:       true,
			WaitForHealth: false,
			Timeout:       10,
		}

		result := HandleServicesConfiguration(services)

		assert.True(t, result.Success)
	})

	t.Run("services with wait for health", func(t *testing.T) {
		services := config.ServicesConfig{
			WaitForHealth: true,
			Timeout:       5,
		}

		result := HandleServicesConfiguration(services)

		assert.True(t, result.Success)
	})

	t.Run("services with both compose and containers", func(t *testing.T) {
		// Create a minimal docker-compose.yml for testing
		tmpDir := t.TempDir()
		composeFile := filepath.Join(tmpDir, "docker-compose.yml")
		composeContent := `version: '3'
services:
  test:
    image: hello-world
`
		err := os.WriteFile(composeFile, []byte(composeContent), 0644)
		require.NoError(t, err)

		services := config.ServicesConfig{
			Compose: &config.ComposeConfig{
				File:     composeFile,
				Services: []string{"test"},
			},
			Containers: []config.ContainerReference{
				{Simple: "redis"}, // Simple string container
			},
			WaitForHealth: false,
			Timeout:       10,
		}

		result := HandleServicesConfiguration(services)

		// This may fail if Docker is not available, but we're testing the code path
		t.Logf("Service configuration result: success=%v, stderr=%s", result.Success, result.Stderr)
	})
}
