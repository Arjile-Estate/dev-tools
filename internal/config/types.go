package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
	Watch            *WatchConfig   `yaml:"watch,omitempty"`              // Watch mode configuration
}

// WatchConfig represents file watching configuration
type WatchConfig struct {
	Patterns []string `yaml:"patterns,omitempty"` // File patterns to watch (e.g., "**/*.go", "src/**/*.ts")
	Debounce string   `yaml:"debounce,omitempty"` // Debounce delay (e.g., "300ms", "1s", default: "300ms")
	Ignore   []string `yaml:"ignore,omitempty"`   // Patterns to ignore (e.g., "**/node_modules/**", "**/.git/**")
}

// ContainerConfig represents configuration for a Docker container
type ContainerConfig struct {
	Name        string            `yaml:"-"`                    // Container name (set from map key)
	Image       string            `yaml:"image"`                // Required: Docker image
	Command     string            `yaml:"command,omitempty"`    // Optional: Command to run
	Environment map[string]string `yaml:"environment,omitempty"` // Environment variables
	Volumes     []string          `yaml:"volumes,omitempty"`    // Volume mounts
	Ports       []string          `yaml:"ports,omitempty"`      // Port mappings
	Networks    []string          `yaml:"networks,omitempty"`   // Network attachments
	Restart     string            `yaml:"restart,omitempty"`    // Restart policy
	Memory      string            `yaml:"memory,omitempty"`     // Memory limit
	CPUs        string            `yaml:"cpus,omitempty"`       // CPU limit
	HealthCheck *HealthCheckConfig `yaml:"healthcheck,omitempty"` // Health check configuration
}

// HealthCheckConfig represents Docker health check configuration
type HealthCheckConfig struct {
	Test     string `yaml:"test,omitempty"`     // Health check command
	Interval string `yaml:"interval,omitempty"` // Check interval
	Timeout  string `yaml:"timeout,omitempty"`  // Command timeout
	Retries  string `yaml:"retries,omitempty"`  // Retry attempts
}

// Validate checks if the container configuration is valid
func (c *ContainerConfig) Validate() error {
	if c.Image == "" {
		return fmt.Errorf("container %s must have an 'image' field", c.Name)
	}
	return nil
}

// ContainerReference represents either a simple string reference or a complex configuration
type ContainerReference struct {
	// Simple is a simple container name (e.g., "redis", "postgres")
	Simple string
	// Complex is a detailed container configuration
	Complex *ContainerConfig
}

// IsSimple returns true if this is a simple string reference
func (c *ContainerReference) IsSimple() bool {
	return c.Simple != "" && c.Complex == nil
}

// GetName returns the container name
func (c *ContainerReference) GetName() string {
	if c.IsSimple() {
		return c.Simple
	}
	if c.Complex != nil {
		return c.Complex.Name
	}
	return ""
}

// UnmarshalYAML implements custom unmarshaling for ContainerReference
// Handles both simple string format ("redis") and complex object format ({name: {image: ...}})
func (c *ContainerReference) UnmarshalYAML(value *yaml.Node) error {
	// Try simple string format first
	var simple string
	if err := value.Decode(&simple); err == nil {
		c.Simple = simple
		return nil
	}

	// Try complex object format: {name: {config}}
	var complexMap map[string]ContainerConfig
	if err := value.Decode(&complexMap); err != nil {
		return fmt.Errorf("container must be a string or object: %w", err)
	}

	// Should have exactly one key (the container name)
	if len(complexMap) != 1 {
		return fmt.Errorf("container object must have exactly one key (the container name)")
	}

	// Extract the container name and config
	for name, config := range complexMap {
		config.Name = name
		c.Complex = &config
		return nil
	}

	return fmt.Errorf("invalid container configuration")
}
