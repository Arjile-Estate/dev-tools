package commands

// Display formatting constants

// Status command display widths
const (
	// StatusCommandMaxLen is the maximum display length for command strings in status output
	// Commands longer than this are truncated with "..." to fit in the terminal
	StatusCommandMaxLen = 38

	// StatusImageMaxLen is the maximum display length for Docker image names
	// Images longer than this are truncated to keep the status table readable
	StatusImageMaxLen = 15

	// StatusPortsMaxLen is the maximum display length for port mappings
	// Port strings longer than this are truncated for readability
	StatusPortsMaxLen = 20

	// DockerPSFieldCount is the expected number of fields in docker ps output
	// Format: "name|status|image|ports" = 4 fields
	DockerPSFieldCount = 4
)

// Onboard command description lengths
const (
	// OnboardBuiltInCmdMaxLen is the maximum length for built-in command descriptions
	// Commands longer than this are truncated for readability in documentation
	OnboardBuiltInCmdMaxLen = 50

	// OnboardCustomCmdMaxLen is the maximum length for custom command descriptions
	// Shorter than built-in to leave room for additional context
	OnboardCustomCmdMaxLen = 40
)
