package config

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LogsConfig represents the logging configuration block
type LogsConfig struct {
	File string `yaml:"file,omitempty"`
	Size int    `yaml:"size,omitempty"`
}

type ResultBudgetConfig struct {
	Lines int `yaml:"lines,omitempty" json:"lines,omitempty"`
	Bytes int `yaml:"bytes,omitempty" json:"bytes,omitempty"`
}

const (
	MinDevResultLines = 7
	MinDevResultBytes = 1024
	// MinDevResultAdapterLines leaves room for the `diagnostics:` line a command with a
	// diagnostics adapter always renders.
	MinDevResultAdapterLines = MinDevResultLines + 1

	DefaultDiagnosticsAdapterTimeout                   = 2 * time.Second
	DefaultDiagnosticsAdapterStdoutBytes         int64 = 1 << 20
	DefaultDiagnosticsAdapterStderrBytes         int64 = 64 << 10
	DefaultDiagnosticsAdapterMaxDiagnostics            = 1000
	DefaultDiagnosticsAdapterMaxSourceBytes      int64 = 10 << 20
	DefaultDiagnosticsAdapterMaxTotalSourceBytes int64 = 50 << 20
	DefaultDevResultRetentionMaxAge                    = 7 * 24 * time.Hour
	DefaultDevResultRetentionMaxBytes            int64 = 1 << 30
)

func (b *ResultBudgetConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawBudget ResultBudgetConfig
	var decoded rawBudget
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	for index := 0; index+1 < len(value.Content); index += 2 {
		name := value.Content[index].Value
		switch name {
		case "lines":
			if decoded.Lines < MinDevResultLines {
				return fmt.Errorf("DEV-RESULT lines must be greater than or equal to %d", MinDevResultLines)
			}
		case "bytes":
			if decoded.Bytes < MinDevResultBytes {
				return fmt.Errorf("DEV-RESULT bytes must be greater than or equal to %d", MinDevResultBytes)
			}
		}
	}
	*b = ResultBudgetConfig(decoded)
	return nil
}

type DevResultCommandConfig struct {
	Enabled            bool                      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Success            ResultBudgetConfig        `yaml:"success,omitempty" json:"success,omitempty"`
	Failure            ResultBudgetConfig        `yaml:"failure,omitempty" json:"failure,omitempty"`
	Timeout            ResultBudgetConfig        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Background         ResultBudgetConfig        `yaml:"background,omitempty" json:"background,omitempty"`
	DiagnosticsAdapter *DiagnosticsAdapterConfig `yaml:"diagnostics_adapter,omitempty" json:"diagnostics_adapter,omitempty"`
}

// DiagnosticsAdapterConfig defines a directly invoked repository diagnostics adapter.
type DiagnosticsAdapterConfig struct {
	Command             []string      `yaml:"command" json:"command"`
	StartWhen           []string      `yaml:"start_when" json:"start_when"`
	Required            bool          `yaml:"required,omitempty" json:"required,omitempty"`
	Timeout             time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	StdoutBytes         int64         `yaml:"stdout_bytes,omitempty" json:"stdout_bytes,omitempty"`
	StderrBytes         int64         `yaml:"stderr_bytes,omitempty" json:"stderr_bytes,omitempty"`
	MaxDiagnostics      int           `yaml:"max_diagnostics,omitempty" json:"max_diagnostics,omitempty"`
	MaxSourceBytes      int64         `yaml:"max_source_bytes,omitempty" json:"max_source_bytes,omitempty"`
	MaxTotalSourceBytes int64         `yaml:"max_total_source_bytes,omitempty" json:"max_total_source_bytes,omitempty"`
}

// UnmarshalYAML applies the diagnostics adapter resource defaults and rejects
// empty executable arguments, evidence patterns, and non-positive limits.
func (c *DiagnosticsAdapterConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawDiagnosticsAdapter struct {
		Command             []string `yaml:"command"`
		StartWhen           []string `yaml:"start_when"`
		Required            bool     `yaml:"required"`
		Timeout             *string  `yaml:"timeout"`
		StdoutBytes         *int64   `yaml:"stdout_bytes"`
		StderrBytes         *int64   `yaml:"stderr_bytes"`
		MaxDiagnostics      *int     `yaml:"max_diagnostics"`
		MaxSourceBytes      *int64   `yaml:"max_source_bytes"`
		MaxTotalSourceBytes *int64   `yaml:"max_total_source_bytes"`
	}

	var raw rawDiagnosticsAdapter
	if err := value.Decode(&raw); err != nil {
		return err
	}

	decoded := DiagnosticsAdapterConfig{
		Command:             raw.Command,
		StartWhen:           raw.StartWhen,
		Required:            raw.Required,
		Timeout:             DefaultDiagnosticsAdapterTimeout,
		StdoutBytes:         DefaultDiagnosticsAdapterStdoutBytes,
		StderrBytes:         DefaultDiagnosticsAdapterStderrBytes,
		MaxDiagnostics:      DefaultDiagnosticsAdapterMaxDiagnostics,
		MaxSourceBytes:      DefaultDiagnosticsAdapterMaxSourceBytes,
		MaxTotalSourceBytes: DefaultDiagnosticsAdapterMaxTotalSourceBytes,
	}
	if raw.Timeout != nil {
		parsed, err := time.ParseDuration(*raw.Timeout)
		if err != nil {
			return fmt.Errorf("invalid diagnostics_adapter timeout: %w", err)
		}
		if parsed <= 0 {
			return fmt.Errorf("diagnostics_adapter timeout must be greater than zero")
		}
		decoded.Timeout = parsed
	}
	if err := applyPositiveInt64("stdout_bytes", raw.StdoutBytes, &decoded.StdoutBytes); err != nil {
		return err
	}
	if err := applyPositiveInt64("stderr_bytes", raw.StderrBytes, &decoded.StderrBytes); err != nil {
		return err
	}
	if raw.MaxDiagnostics != nil {
		if *raw.MaxDiagnostics <= 0 {
			return fmt.Errorf("diagnostics_adapter max_diagnostics must be greater than zero")
		}
		decoded.MaxDiagnostics = *raw.MaxDiagnostics
	}
	if err := applyPositiveInt64("max_source_bytes", raw.MaxSourceBytes, &decoded.MaxSourceBytes); err != nil {
		return err
	}
	if err := applyPositiveInt64("max_total_source_bytes", raw.MaxTotalSourceBytes, &decoded.MaxTotalSourceBytes); err != nil {
		return err
	}
	if err := decoded.applyDefaultsAndValidate(); err != nil {
		return err
	}

	*c = decoded
	return nil
}

func (c *DiagnosticsAdapterConfig) applyDefaultsAndValidate() error {
	if len(c.Command) == 0 {
		return fmt.Errorf("diagnostics_adapter command must contain at least one argument")
	}
	for _, argument := range c.Command {
		if strings.TrimSpace(argument) == "" {
			return fmt.Errorf("diagnostics_adapter command arguments must not be empty")
		}
	}
	if len(c.StartWhen) == 0 {
		return fmt.Errorf("diagnostics_adapter start_when must contain at least one pattern")
	}
	for _, pattern := range c.StartWhen {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("diagnostics_adapter start_when patterns must not be empty")
		}
	}

	if c.Timeout < 0 {
		return fmt.Errorf("diagnostics_adapter timeout must be greater than zero")
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultDiagnosticsAdapterTimeout
	}
	if c.StdoutBytes < 0 {
		return fmt.Errorf("diagnostics_adapter stdout_bytes must be greater than zero")
	}
	if c.StdoutBytes == 0 {
		c.StdoutBytes = DefaultDiagnosticsAdapterStdoutBytes
	}
	if c.StderrBytes < 0 {
		return fmt.Errorf("diagnostics_adapter stderr_bytes must be greater than zero")
	}
	if c.StderrBytes == 0 {
		c.StderrBytes = DefaultDiagnosticsAdapterStderrBytes
	}
	if c.MaxDiagnostics < 0 {
		return fmt.Errorf("diagnostics_adapter max_diagnostics must be greater than zero")
	}
	if c.MaxDiagnostics == 0 {
		c.MaxDiagnostics = DefaultDiagnosticsAdapterMaxDiagnostics
	}
	if c.MaxSourceBytes < 0 {
		return fmt.Errorf("diagnostics_adapter max_source_bytes must be greater than zero")
	}
	if c.MaxSourceBytes == 0 {
		c.MaxSourceBytes = DefaultDiagnosticsAdapterMaxSourceBytes
	}
	if c.MaxTotalSourceBytes < 0 {
		return fmt.Errorf("diagnostics_adapter max_total_source_bytes must be greater than zero")
	}
	if c.MaxTotalSourceBytes == 0 {
		c.MaxTotalSourceBytes = DefaultDiagnosticsAdapterMaxTotalSourceBytes
	}
	return nil
}

func applyPositiveInt64(name string, value *int64, destination *int64) error {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return fmt.Errorf("diagnostics_adapter %s must be greater than zero", name)
	}
	*destination = *value
	return nil
}

type RetentionConfig struct {
	MaxAge   time.Duration `yaml:"max_age,omitempty" json:"max_age,omitempty"`
	MaxBytes int64         `yaml:"max_bytes,omitempty" json:"max_bytes,omitempty"`
}

func (c *RetentionConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawRetention struct {
		MaxAge   *string `yaml:"max_age"`
		MaxBytes *int64  `yaml:"max_bytes"`
	}
	var raw rawRetention
	if err := value.Decode(&raw); err != nil {
		return err
	}

	decoded := defaultRetentionConfig()
	if raw.MaxAge != nil {
		parsed, err := time.ParseDuration(*raw.MaxAge)
		if err != nil {
			return fmt.Errorf("invalid DEV-RESULT retention max_age: %w", err)
		}
		if parsed < 0 {
			return fmt.Errorf("DEV-RESULT retention max_age must not be negative")
		}
		if parsed > 0 {
			decoded.MaxAge = parsed
		}
	}
	if raw.MaxBytes != nil {
		if *raw.MaxBytes < 0 {
			return fmt.Errorf("DEV-RESULT retention max_bytes must not be negative")
		}
		if *raw.MaxBytes > 0 {
			decoded.MaxBytes = *raw.MaxBytes
		}
	}
	*c = decoded
	return nil
}

func defaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		MaxAge:   DefaultDevResultRetentionMaxAge,
		MaxBytes: DefaultDevResultRetentionMaxBytes,
	}
}

type DevResultConfig struct {
	Commands  map[string]DevResultCommandConfig `yaml:"commands" json:"commands"`
	Retention RetentionConfig                   `yaml:"retention,omitempty" json:"retention,omitempty"`
}

func (c *DevResultConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawDevResultConfig DevResultConfig
	decoded := rawDevResultConfig{Retention: defaultRetentionConfig()}
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	if decoded.Retention.MaxAge == 0 {
		decoded.Retention.MaxAge = DefaultDevResultRetentionMaxAge
	}
	if decoded.Retention.MaxBytes == 0 {
		decoded.Retention.MaxBytes = DefaultDevResultRetentionMaxBytes
	}
	*c = DevResultConfig(decoded)
	return nil
}

// Config represents the complete development configuration
type Config struct {
	Logs      *LogsConfig              `yaml:"logs,omitempty"`
	Commands  map[string][]CommandStep `yaml:"commands"`
	DevResult *DevResultConfig         `yaml:"dev_result,omitempty" json:"dev_result,omitempty"`
}

// CommandStep represents a single step in a command execution
type CommandStep struct {
	Run              RunCommand     `yaml:"run,omitempty"`
	Services         ServicesConfig `yaml:"services,omitempty"`
	Background       bool           `yaml:"background,omitempty"`
	Daemon           bool           `yaml:"daemon,omitempty"`
	Directory        string         `yaml:"directory,omitempty"`
	Retry            int            `yaml:"retry,omitempty"`               // Number of retry attempts (0 = no retry, default)
	RetryDelay       string         `yaml:"retry_delay,omitempty"`         // Delay between retries (e.g., "5s", "1m")
	RetryOnExitCodes []int          `yaml:"retry_on_exit_codes,omitempty"` // Only retry on specific exit codes (empty = retry on any failure)
	Watch            *WatchConfig   `yaml:"watch,omitempty"`               // Watch mode configuration
	ContinueOnError  bool           `yaml:"continue_on_error,omitempty"`   // Continue execution even if this command fails
}

// WatchConfig represents file watching configuration
type WatchConfig struct {
	Patterns []string `yaml:"patterns,omitempty"` // File patterns to watch (e.g., "**/*.go", "src/**/*.ts")
	Debounce string   `yaml:"debounce,omitempty"` // Debounce delay (e.g., "300ms", "1s", default: "300ms")
	Ignore   []string `yaml:"ignore,omitempty"`   // Patterns to ignore (e.g., "**/node_modules/**", "**/.git/**")
}

// ContainerConfig represents configuration for a Docker container
type ContainerConfig struct {
	Name        string             `yaml:"-"`                     // Container name (set from map key)
	Image       string             `yaml:"image"`                 // Required: Docker image
	Command     string             `yaml:"command,omitempty"`     // Optional: Command to run
	Environment map[string]string  `yaml:"environment,omitempty"` // Environment variables
	Volumes     []string           `yaml:"volumes,omitempty"`     // Volume mounts
	Ports       []string           `yaml:"ports,omitempty"`       // Port mappings
	Networks    []string           `yaml:"networks,omitempty"`    // Network attachments
	Restart     string             `yaml:"restart,omitempty"`     // Restart policy
	Memory      string             `yaml:"memory,omitempty"`      // Memory limit
	CPUs        string             `yaml:"cpus,omitempty"`        // CPU limit
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
