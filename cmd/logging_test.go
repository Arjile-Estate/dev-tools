package cmd

import (
	"os"
	"testing"
)

func TestGetLogFilePath(t *testing.T) {
	tests := []struct {
		name         string
		projectDir   string
		expectedPath string
		isGoRun      bool
	}{
		{
			name:         "go run executable",
			projectDir:   "/project",
			expectedPath: "/project/activity.log",
			isGoRun:      true,
		},
		{
			name:         "compiled binary",
			projectDir:   "/project",
			expectedPath: "/home/user/Library/Logs/dev-tools.log",
			isGoRun:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock home directory
			originalHome := os.Getenv("HOME")
			_ = os.Setenv("HOME", "/home/user")
			defer func() { _ = os.Setenv("HOME", originalHome) }()

			// Mock executable path
			var executable string
			if tt.isGoRun {
				executable = "/tmp/go-build123/b001/exe/main"
			} else {
				executable = "/usr/local/bin/dev-tools"
			}

			originalOsExecutable := osExecutable
			osExecutable = func() (string, error) { return executable, nil }
			defer func() { osExecutable = originalOsExecutable }()

			actualPath, err := getLogFilePath(tt.projectDir)
			if err != nil {
				t.Fatalf("getLogFilePath() error = %v", err)
			}

			if actualPath != tt.expectedPath {
				t.Errorf("getLogFilePath() path = %v, want %v", actualPath, tt.expectedPath)
			}
		})
	}
}
