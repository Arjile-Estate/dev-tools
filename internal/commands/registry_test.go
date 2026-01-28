package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBuiltInCommandNames(t *testing.T) {
	names := GetBuiltInCommandNames()

	// Should have all built-in commands except internal ones
	expectedCommands := []string{"logs", "status", "cleanup-pids", "cleanup-all", "restart", "stop", "version", "onboard", "validate", "completion"}

	assert.Len(t, names, len(expectedCommands), "Should have correct number of built-in commands")

	// Check each expected command exists
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, expected := range expectedCommands {
		assert.True(t, nameSet[expected], "Built-in command '%s' should be in the list", expected)
	}

	// Internal commands should not be in the list
	for _, name := range names {
		assert.NotEqual(t, "__dev_complete", name, "Internal commands should not be in public list")
	}
}

func TestGetBuiltInCommandMap(t *testing.T) {
	commandMap := GetBuiltInCommandMap()

	// Should include all commands including internal ones
	expectedCommands := []string{"logs", "status", "cleanup-pids", "cleanup-all", "restart", "stop", "version", "onboard", "validate", "completion", "__dev_complete"}

	assert.Len(t, commandMap, len(expectedCommands), "Should have all commands including internal")

	for _, cmdName := range expectedCommands {
		handler, exists := commandMap[cmdName]
		assert.True(t, exists, "Command '%s' should exist in map", cmdName)
		assert.NotNil(t, handler, "Handler for '%s' should not be nil", cmdName)
	}
}

func TestGetBuiltInCommand(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		shouldExist bool
	}{
		{
			name:        "existing command - logs",
			commandName: "logs",
			shouldExist: true,
		},
		{
			name:        "existing command - status",
			commandName: "status",
			shouldExist: true,
		},
		{
			name:        "internal command",
			commandName: "__dev_complete",
			shouldExist: true,
		},
		{
			name:        "non-existent command",
			commandName: "nonexistent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := GetBuiltInCommand(tt.commandName)

			if tt.shouldExist {
				assert.NotNil(t, cmd, "Command '%s' should exist", tt.commandName)
				assert.Equal(t, tt.commandName, cmd.Name, "Command name should match")
				assert.NotEmpty(t, cmd.Description, "Description should not be empty")
				assert.NotNil(t, cmd.Handler, "Handler should not be nil")
			} else {
				assert.Nil(t, cmd, "Command '%s' should not exist", tt.commandName)
			}
		})
	}
}

func TestIsBuiltInCommand(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		expected    bool
	}{
		{"logs is built-in", "logs", true},
		{"status is built-in", "status", true},
		{"cleanup-pids is built-in", "cleanup-pids", true},
		{"cleanup-all is built-in", "cleanup-all", true},
		{"restart is built-in", "restart", true},
		{"stop is built-in", "stop", true},
		{"version is built-in", "version", true},
		{"onboard is built-in", "onboard", true},
		{"validate is built-in", "validate", true},
		{"completion is built-in", "completion", true},
		{"__dev_complete is built-in", "__dev_complete", true},
		{"test is not built-in", "test", false},
		{"build is not built-in", "build", false},
		{"random is not built-in", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBuiltInCommand(tt.commandName)
			assert.Equal(t, tt.expected, result, "IsBuiltInCommand('%s') should return %v", tt.commandName, tt.expected)
		})
	}
}

func TestBuiltInCommandRegistry_NoEmptyDescriptions(t *testing.T) {
	for _, cmd := range builtInCommandRegistry {
		// Internal commands can have empty descriptions
		if cmd.Name == "__dev_complete" {
			continue
		}
		assert.NotEmpty(t, cmd.Description, "Command '%s' should have a description", cmd.Name)
	}
}

func TestBuiltInCommandRegistry_UniqueNames(t *testing.T) {
	nameSet := make(map[string]bool)
	for _, cmd := range builtInCommandRegistry {
		assert.False(t, nameSet[cmd.Name], "Command name '%s' should be unique", cmd.Name)
		nameSet[cmd.Name] = true
	}
}
