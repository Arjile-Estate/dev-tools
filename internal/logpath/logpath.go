package logpath

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultLogLines is the default number of log lines to display.
const DefaultLogLines = 20

// LogConfig holds the resolved log file path and display size.
type LogConfig struct {
	File string
	Size int
}

// lightweightConfig is a minimal struct for extracting just the logs block
// from .dev-config.yaml without importing the full config package.
type lightweightConfig struct {
	Logs struct {
		File string `yaml:"file"`
		Size int    `yaml:"size"`
	} `yaml:"logs"`
}

// GetLogConfig reads the logs configuration from .dev-config.yaml.
// It returns a LogConfig with the resolved file path and display size.
// Never errors — always returns a usable config with sensible defaults.
func GetLogConfig(projectDir string) LogConfig {
	result := LogConfig{Size: DefaultLogLines}

	configPath := filepath.Join(projectDir, ".dev-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		result.File = defaultLogPath()
		return result
	}

	var cfg lightweightConfig
	if parseErr := yaml.Unmarshal(data, &cfg); parseErr == nil && cfg.Logs.File != "" {
		result.File = expandTilde(cfg.Logs.File)
	} else {
		result.File = defaultLogPath()
	}

	if cfg.Logs.Size > 0 {
		result.Size = cfg.Logs.Size
	}

	return result
}

// GetLogFilePath determines the log file path by checking .dev-config.yaml
// for a "logs.file" field. If not configured, returns the default path
// ~/.local/state/dev-tools/activity.log. Never errors — always returns a usable path.
func GetLogFilePath(projectDir string) string {
	return GetLogConfig(projectDir).File
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(homeDir, path[1:])
}

// defaultLogPath returns the default log file path: ~/.local/state/dev-tools/activity.log
func defaultLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "activity.log"
	}

	return filepath.Join(homeDir, ".local", "state", "dev-tools", "activity.log")
}
