package config

// Config represents the complete development configuration
type Config struct {
	Commands map[string][]CommandStep `yaml:"commands"`
}

// CommandStep represents a single step in a command execution
type CommandStep struct {
	Run        RunCommand     `yaml:"run,omitempty"`
	Services   ServicesConfig `yaml:"services,omitempty"`
	Background bool           `yaml:"background,omitempty"`
	Daemon     bool           `yaml:"daemon,omitempty"`
	Directory  string         `yaml:"directory,omitempty"`
}
