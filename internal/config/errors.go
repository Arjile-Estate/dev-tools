package config

import "fmt"

// ConfigError represents errors related to configuration loading and parsing
type ConfigError struct {
	File string
	Err  error
}

func (e *ConfigError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("config error in %s: %v", e.File, e.Err)
	}
	return fmt.Sprintf("config error: %v", e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError
func NewConfigError(file string, err error) *ConfigError {
	return &ConfigError{
		File: file,
		Err:  err,
	}
}

// ProjectTypeError represents errors related to project type detection
type ProjectTypeError struct {
	Path string
	Err  error
}

func (e *ProjectTypeError) Error() string {
	return fmt.Sprintf("project type detection failed for %s: %v", e.Path, e.Err)
}

func (e *ProjectTypeError) Unwrap() error {
	return e.Err
}

// NewProjectTypeError creates a new ProjectTypeError
func NewProjectTypeError(path string, err error) *ProjectTypeError {
	return &ProjectTypeError{
		Path: path,
		Err:  err,
	}
}

// CommandNotFoundError represents when a requested command doesn't exist
type CommandNotFoundError struct {
	Command string
}

func (e *CommandNotFoundError) Error() string {
	return fmt.Sprintf("command '%s' not found in configuration", e.Command)
}

// NewCommandNotFoundError creates a new CommandNotFoundError
func NewCommandNotFoundError(command string) *CommandNotFoundError {
	return &CommandNotFoundError{
		Command: command,
	}
}
