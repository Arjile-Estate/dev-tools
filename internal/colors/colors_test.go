package colors

import (
	"os"
	"testing"
)

func TestColorizeWhenEnabled(t *testing.T) {
	// Enable colors for testing
	colorEnabled.Store(1)

	tests := []struct {
		name     string
		function func(string, ...interface{}) string
		input    string
		expected string
	}{
		{
			name:     "Success colors text green",
			function: Success,
			input:    "test message",
			expected: Green + "test message" + Reset,
		},
		{
			name:     "Error colors text red",
			function: Error,
			input:    "error message",
			expected: Red + "error message" + Reset,
		},
		{
			name:     "Warning colors text yellow",
			function: Warning,
			input:    "warning message",
			expected: Yellow + "warning message" + Reset,
		},
		{
			name:     "Info colors text dark gray",
			function: Info,
			input:    "info message",
			expected: DarkGray + "info message" + Reset,
		},
		{
			name:     "Highlight colors text blue",
			function: Highlight,
			input:    "highlight message",
			expected: Blue + "highlight message" + Reset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestColorizeWhenDisabled(t *testing.T) {
	// Disable colors for testing
	colorEnabled.Store(0)

	tests := []struct {
		name     string
		function func(string, ...interface{}) string
		input    string
		expected string
	}{
		{
			name:     "Success returns plain text when disabled",
			function: Success,
			input:    "test message",
			expected: "test message",
		},
		{
			name:     "Error returns plain text when disabled",
			function: Error,
			input:    "error message",
			expected: "error message",
		},
		{
			name:     "Warning returns plain text when disabled",
			function: Warning,
			input:    "warning message",
			expected: "warning message",
		},
		{
			name:     "Info returns plain text when disabled",
			function: Info,
			input:    "info message",
			expected: "info message",
		},
		{
			name:     "Highlight returns plain text when disabled",
			function: Highlight,
			input:    "highlight message",
			expected: "highlight message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestColorFormattingWithArguments(t *testing.T) {
	colorEnabled.Store(1)

	result := Success("Process %s completed with code %d", "test", 0)
	expected := Green + "Process test completed with code 0" + Reset

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestInitializeColorSupportWithNoColorFlag(t *testing.T) {
	// Reset state
	colorEnabled.Store(1)

	InitializeColorSupport(true)

	if colorEnabled.Load() == 1 {
		t.Error("Expected colors to be disabled when noColor=true")
	}
}

func TestInitializeColorSupportWithNOCOLOREnv(t *testing.T) {
	// Save original env
	originalNoColor := os.Getenv("NO_COLOR")
	defer func() {
		if originalNoColor == "" {
			_ = os.Unsetenv("NO_COLOR")
		} else {
			_ = os.Setenv("NO_COLOR", originalNoColor)
		}
	}()

	// Reset state
	colorEnabled.Store(1)

	// Set NO_COLOR environment variable
	_ = os.Setenv("NO_COLOR", "1")

	InitializeColorSupport(false)

	if colorEnabled.Load() == 1 {
		t.Error("Expected colors to be disabled when NO_COLOR is set")
	}
}

func TestInitializeColorSupportWithDumbTerm(t *testing.T) {
	// Save original env
	originalTerm := os.Getenv("TERM")
	defer func() {
		if originalTerm == "" {
			_ = os.Unsetenv("TERM")
		} else {
			_ = os.Setenv("TERM", originalTerm)
		}
	}()

	// Reset state
	colorEnabled.Store(1)

	// Set TERM to dumb
	_ = os.Setenv("TERM", "dumb")

	InitializeColorSupport(false)

	if colorEnabled.Load() == 1 {
		t.Error("Expected colors to be disabled when TERM=dumb")
	}
}

func TestIsColorEnabled(t *testing.T) {
	colorEnabled.Store(1)
	if !IsColorEnabled() {
		t.Error("Expected IsColorEnabled to return true when colors are enabled")
	}

	colorEnabled.Store(0)
	if IsColorEnabled() {
		t.Error("Expected IsColorEnabled to return false when colors are disabled")
	}
}

func TestStripColors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Strips red color codes",
			input:    Red + "error message" + Reset,
			expected: "error message",
		},
		{
			name:     "Strips green color codes",
			input:    Green + "success message" + Reset,
			expected: "success message",
		},
		{
			name:     "Strips yellow color codes",
			input:    Yellow + "warning message" + Reset,
			expected: "warning message",
		},
		{
			name:     "Strips dark gray color codes",
			input:    DarkGray + "info message" + Reset,
			expected: "info message",
		},
		{
			name:     "Strips blue color codes",
			input:    Blue + "highlight message" + Reset,
			expected: "highlight message",
		},
		{
			name:     "Handles text without colors",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "Handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Strips multiple color codes",
			input:    Red + "error" + Reset + " and " + Green + "success" + Reset,
			expected: "error and success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripColors(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestColorizeWithEmptyText(t *testing.T) {
	colorEnabled.Store(1)

	result := Success("")
	expected := ""

	if result != expected {
		t.Errorf("Expected empty string for empty input, got %q", result)
	}
}

func TestColorConstants(t *testing.T) {
	expectedColors := map[string]string{
		"Reset":    "\033[0m",
		"Red":      "\033[31m",
		"Green":    "\033[32m",
		"Yellow":   "\033[33m",
		"Blue":     "\033[34m",
		"DarkGray": "\033[90m",
	}

	actualColors := map[string]string{
		"Reset":    Reset,
		"Red":      Red,
		"Green":    Green,
		"Yellow":   Yellow,
		"Blue":     Blue,
		"DarkGray": DarkGray,
	}

	for name, expected := range expectedColors {
		if actual := actualColors[name]; actual != expected {
			t.Errorf("Expected %s to be %q, got %q", name, expected, actual)
		}
	}
}
