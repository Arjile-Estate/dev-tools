package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

type commandInfo struct {
	Name        string
	Description string
	IsCustom    bool
}

// HandleOnboardCommand generates AI assistant documentation
func HandleOnboardCommand(cmd *cobra.Command, args []string, projectDir string) error {
	logger.Info("Generating onboarding documentation for AI assistants")

	// Check for --output-file flag
	var outputFile string
	for i, arg := range args {
		if arg == "--output-file" && i+1 < len(args) {
			outputFile = args[i+1]
			break
		} else if strings.HasPrefix(arg, "--output-file=") {
			outputFile = strings.TrimPrefix(arg, "--output-file=")
			break
		}
	}

	// Detect project type
	projectType := config.DetectProjectType(projectDir)
	projectTypeStr := projectTypeToString(projectType)

	// Load configuration - try to load user config, fallback to defaults
	configPath := filepath.Join(projectDir, ".dev-config.yaml")
	cfg, err := config.LoadConfigFromFile(configPath)
	var customCommands map[string][]config.CommandStep
	if err != nil {
		logger.Infof("No custom config found, using defaults only: %v", err)
		customCommands = make(map[string][]config.CommandStep)
	} else {
		customCommands = cfg.Commands
	}

	// Get built-in commands for this project type
	defaults := config.GetDefaultCommandsForProjectType(projectType)
	builtInCommands := defaults.Commands

	// Generate documentation
	doc, err := generateOnboardingDoc(projectTypeStr, projectDir, builtInCommands, customCommands)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to generate documentation: %v", err)))
	}

	// Output to file if --output-file specified, otherwise stdout
	if outputFile != "" {
		// Make path absolute if relative
		if !filepath.IsAbs(outputFile) {
			outputFile = filepath.Join(projectDir, outputFile)
		}

		err = os.WriteFile(outputFile, []byte(doc), 0644)
		if err != nil {
			return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to write to %s: %v", outputFile, err)))
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Generated AI assistant documentation at %s", outputFile)))
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Project type: %s", projectTypeStr)))
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Commands documented: %d built-in, %d custom",
			len(builtInCommands), len(customCommands))))
	} else {
		// Output to stdout
		_, _ = fmt.Fprint(cmd.OutOrStdout(), doc)
	}

	return nil
}

func generateOnboardingDoc(projectType, projectDir string, builtInCmds, customCmds map[string][]config.CommandStep) (string, error) {
	// Collect all commands - prefer custom over built-in
	commandMap := make(map[string]commandInfo)

	// Add built-in commands first
	for name := range builtInCmds {
		commandMap[name] = commandInfo{
			Name:        name,
			Description: getBuiltInDescription(name, builtInCmds[name]),
			IsCustom:    false,
		}
	}

	// Add/override with custom commands
	for name := range customCmds {
		commandMap[name] = commandInfo{
			Name:        name,
			Description: getCustomDescription(customCmds[name]),
			IsCustom:    true,
		}
	}

	// Convert map to slice for sorting
	var commands []commandInfo
	for _, cmd := range commandMap {
		commands = append(commands, cmd)
	}

	// Sort commands alphabetically
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	// Built-in dev-tools commands
	builtInDevToolsCommands := []commandInfo{
		{Name: "logs", Description: "View recent activity logs"},
		{Name: "status", Description: "Show daemon process status"},
		{Name: "cleanup-pids", Description: "Clean up stale PID files"},
		{Name: "cleanup-all", Description: "Clean up all daemon processes"},
		{Name: "restart <name>", Description: "Restart a daemon process"},
		{Name: "stop <name>", Description: "Stop a daemon process"},
		{Name: "version", Description: "Show version information"},
		{Name: "validate", Description: "Validate configuration file"},
		{Name: "completion <shell>", Description: "Generate shell completion script"},
	}

	// Template data
	data := struct {
		ProjectType          string
		ProjectDir           string
		Commands             []commandInfo
		BuiltInDevToolsCmds  []commandInfo
		HasCustomCommands    bool
		HasServiceManagement bool
	}{
		ProjectType:          projectType,
		ProjectDir:           projectDir,
		Commands:             commands,
		BuiltInDevToolsCmds:  builtInDevToolsCommands,
		HasCustomCommands:    len(customCmds) > 0,
		HasServiceManagement: hasServiceManagement(customCmds),
	}

	// Template
	tmpl := template.Must(template.New("onboard").Parse(onboardingTemplate))

	// Execute template
	var buf strings.Builder
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getBuiltInDescription(name string, steps []config.CommandStep) string {
	if len(steps) > 0 && len(steps[0].Run) > 0 {
		cmd := string(steps[0].Run[0])
		if len(cmd) > OnboardBuiltInCmdMaxLen {
			cmd = cmd[:OnboardBuiltInCmdMaxLen-3] + "..."
		}
		return fmt.Sprintf("Run %s", cmd)
	}
	return "Run " + name
}

func getCustomDescription(steps []config.CommandStep) string {
	// Count run commands
	runCount := 0
	for _, step := range steps {
		runCount += len(step.Run)
	}

	if runCount == 1 && len(steps[0].Run) > 0 {
		cmd := string(steps[0].Run[0])
		if len(cmd) > OnboardCustomCmdMaxLen {
			cmd = cmd[:OnboardCustomCmdMaxLen-3] + "..."
		}
		return "Run " + cmd
	} else if runCount > 0 {
		return fmt.Sprintf("Run %d commands", runCount)
	}

	return "Custom command"
}

func hasServiceManagement(customCmds map[string][]config.CommandStep) bool {
	for _, steps := range customCmds {
		for _, step := range steps {
			if step.Services.Compose != nil || len(step.Services.Containers) > 0 {
				return true
			}
		}
	}
	return false
}

const onboardingTemplate = `# Dev-Tools Usage Guide for AI Assistants

## Project Information
- **Project Type**: {{.ProjectType}}
- **Project Directory**: {{.ProjectDir}}
- **Tool**: dev-tools (unified command runner)

## Available Project Commands

These commands are available for this {{.ProjectType}} project:

{{range .Commands}}
### ` + "`dev-tools {{.Name}}`" + `
{{.Description}}{{if .IsCustom}} (custom){{end}}
{{end}}

## Built-in Dev-Tools Commands

These management commands are always available:

{{range .BuiltInDevToolsCmds}}
### ` + "`dev-tools {{.Name}}`" + `
{{.Description}}
{{end}}

## Best Practices for AI Assistants

### Command Execution
- Always run dev-tools commands from the project root directory
- Use passthrough arguments with ` + "`--`" + ` separator: ` + "`dev-tools test -- --verbose`" + `
- Check command output for errors before proceeding
- Use ` + "`dev-tools status`" + ` to check running daemons

### Flags
- ` + "`--verbose` or `-v`" + `: Enable detailed logging
- ` + "`--project-dir <path>` or `-p <path>`" + `: Run commands in a different directory
- ` + "`--watch` or `-w`" + `: Watch mode - re-run command on file changes (requires watch config)
- ` + "`--format <format>`" + `: Output format - text or json (default: text, currently status command only)
- ` + "`--no-color`" + `: Disable colored output (useful for parsing)

### Debugging
- View recent logs: ` + "`dev-tools logs`" + `
- Check daemon status: ` + "`dev-tools status`" + `
- Clean up stale processes: ` + "`dev-tools cleanup-pids`" + `
{{if .HasServiceManagement}}
### Service Management

This project uses Docker services that are automatically managed:
- Services start automatically before commands that need them
- Health checks ensure services are ready before proceeding
- Use ` + "`dev-tools status`" + ` to see running services
- Check ` + "`activity.log`" + ` for service startup details
{{end}}
{{if .HasCustomCommands}}
## Custom Configuration

This project has custom commands defined in ` + "`.dev-config.yaml`" + `. Commands can:
- Start Docker containers or Docker Compose services with health checks
- Run multiple commands in sequence
- Execute commands in specific directories
- Run background daemons
- **Retry on failures** with configurable delays and exit code filters
- **Watch files** and re-run automatically on changes (debounced)
- Define service timeouts and cleanup behaviors

Check ` + "`.dev-config.yaml`" + ` for:
- ` + "`retry`, `retry_delay`, `retry_on_exit_codes`" + ` - Retry configuration
- ` + "`watch.patterns`, `watch.debounce`, `watch.ignore`" + ` - Watch mode configuration
- ` + "`services.wait_for_health`, `services.timeout`, `services.cleanup`" + ` - Service management

Always check the configuration file for detailed command behavior.
{{end}}

## Common Workflows

### Running Tests
` + "```bash" + `
dev-tools test
` + "```" + `

### Starting Development Environment
` + "```bash" + `
dev-tools dev
` + "```" + `

### Watch Mode for TDD
` + "```bash" + `
dev-tools --watch test  # Re-runs on file changes
` + "```" + `

### Checking Running Processes
` + "```bash" + `
dev-tools status                # Human-readable
dev-tools status --format json  # Machine-readable
` + "```" + `

### Viewing Logs
` + "```bash" + `
dev-tools logs
` + "```" + `

### Shell Completion Setup
` + "```bash" + `
# Bash
source <(dev-tools completion bash)

# Zsh
source <(dev-tools completion zsh)

# Fish
dev-tools completion fish | source
` + "```" + `

### Debugging Commands
` + "```bash" + `
dev-tools --verbose test  # See detailed execution
` + "```" + `

## Notes

- This documentation was auto-generated by ` + "`dev-tools onboard`" + `
- Commands and their behavior depend on project configuration
- Always verify command output before making assumptions about success
- Use ` + "`dev-tools --help`" + ` for more information

---

*Generated by dev-tools onboard command*
`
