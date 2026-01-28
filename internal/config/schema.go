package config

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// ConfigSchema defines the JSON schema for .dev-config.yaml
const ConfigSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Dev-Tools Configuration",
  "description": "Configuration schema for dev-tools command runner",
  "type": "object",
  "properties": {
    "commands": {
      "type": "object",
      "description": "Command definitions",
      "additionalProperties": {
        "type": "array",
        "items": {
          "$ref": "#/definitions/commandStep"
        }
      }
    }
  },
  "definitions": {
    "commandStep": {
      "type": "object",
      "properties": {
        "run": {
          "description": "Command(s) to execute",
          "oneOf": [
            {"type": "string"},
            {"type": "array", "items": {"type": "string"}}
          ]
        },
        "services": {
          "type": "object",
          "description": "Docker services configuration",
          "properties": {
            "compose": {
              "type": "object",
              "properties": {
                "file": {"type": "string"},
                "services": {"type": "array", "items": {"type": "string"}},
                "profiles": {"type": "array", "items": {"type": "string"}}
              },
              "required": ["file"]
            },
            "containers": {
              "type": "array",
              "items": {
                "oneOf": [
                  {"type": "string"},
                  {"type": "object"}
                ]
              }
            },
            "cleanup": {"type": "boolean"},
            "wait_for_health": {"type": "boolean"},
            "timeout": {"type": "integer", "minimum": 1}
          }
        },
        "background": {
          "type": "boolean",
          "description": "Run command in background"
        },
        "daemon": {
          "type": "boolean",
          "description": "Run as daemon process"
        },
        "directory": {
          "type": "string",
          "description": "Working directory for command"
        },
        "retry": {
          "type": "integer",
          "minimum": 0,
          "description": "Number of retry attempts"
        },
        "retry_delay": {
          "type": "string",
          "pattern": "^[0-9]+(ms|s|m|h)$",
          "description": "Delay between retries (e.g., '5s', '1m')"
        },
        "retry_on_exit_codes": {
          "type": "array",
          "items": {"type": "integer"},
          "description": "Only retry on specific exit codes"
        },
        "watch": {
          "type": "object",
          "properties": {
            "patterns": {
              "type": "array",
              "items": {"type": "string"},
              "description": "File patterns to watch"
            },
            "debounce": {
              "type": "string",
              "pattern": "^[0-9]+(ms|s|m)$",
              "description": "Debounce delay"
            },
            "ignore": {
              "type": "array",
              "items": {"type": "string"},
              "description": "Patterns to ignore"
            }
          }
        }
      }
    }
  }
}`

// ValidateConfig validates a config object against the schema
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate that commands is not empty
	if cfg.Commands == nil || len(cfg.Commands) == 0 {
		return fmt.Errorf("configuration must have at least one command defined")
	}

	// Convert config to JSON for validation
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	// Load schema
	schemaLoader := gojsonschema.NewStringLoader(ConfigSchema)
	documentLoader := gojsonschema.NewBytesLoader(configJSON)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		// Build detailed error message
		var errorMsg string
		for i, err := range result.Errors() {
			if i > 0 {
				errorMsg += "\n"
			}
			errorMsg += fmt.Sprintf("- %s: %s", err.Field(), err.Description())
		}
		return fmt.Errorf("configuration validation failed:\n%s", errorMsg)
	}

	return nil
}

// ValidateConfigFile validates a config file at the given path
func ValidateConfigFile(configPath string) error {
	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg == nil {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	return ValidateConfig(cfg)
}
