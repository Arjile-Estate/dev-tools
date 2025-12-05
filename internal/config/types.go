package config

// Config represents the complete development configuration
type Config struct {
	Commands map[string][]CommandStep `yaml:"commands"`
}

// CommandStep represents a single step in a command execution
type CommandStep struct {
	Run              RunCommand     `yaml:"run,omitempty"`
	Services         ServicesConfig `yaml:"services,omitempty"`
	Background       bool           `yaml:"background,omitempty"`
	Daemon           bool           `yaml:"daemon,omitempty"`
	Directory        string         `yaml:"directory,omitempty"`
	Retry            int            `yaml:"retry,omitempty"`              // Number of retry attempts (0 = no retry, default)
	RetryDelay       string         `yaml:"retry_delay,omitempty"`        // Delay between retries (e.g., "5s", "1m")
	RetryOnExitCodes []int          `yaml:"retry_on_exit_codes,omitempty"` // Only retry on specific exit codes (empty = retry on any failure)
}
