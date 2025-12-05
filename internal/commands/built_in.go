package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/executor"
	"github.com/spf13/cobra"
)

func HandleLogsCommand(cmd *cobra.Command, projectDir string) error {
	log.Print("Displaying recent activity logs")

	logFile := filepath.Join(projectDir, "activity.log")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s", logFile)
	}

	result := executor.ExecuteShellCommand(executor.ExecuteOptions{
		Command:       fmt.Sprintf("tail -n 50 %s", logFile),
		CaptureOutput: true,
	})

	if !result.Success {
		return fmt.Errorf("failed to read logs: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func HandleCleanupPidsCommand(cmd *cobra.Command, projectDir string) error {
	result := executor.CleanupStalePIDFiles(projectDir)
	if !result.Success {
		return fmt.Errorf("cleanup failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func HandleCleanupAllCommand(cmd *cobra.Command, projectDir string) error {
	log.Print("Cleaning up all daemon processes and PID files")

	result := executor.CleanupStalePIDFilesWithTermination(projectDir, true)
	if !result.Success {
		return fmt.Errorf("cleanup-all failed: %s", result.Stderr)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return nil
}

func HandleStatusCommand(cmd *cobra.Command, projectDir string) error {
	log.Print("Displaying daemon process status")

	daemons, err := executor.ListDaemonProcesses(projectDir)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to list daemon processes: %v", err)))
	}

	if len(daemons) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("No daemon processes found in %s", projectDir)))
		return nil
	}

	// Display header
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Highlight("DAEMON STATUS"))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")

	// Display table header
	header := fmt.Sprintf("%-20s %-10s %-8s %-12s %s",
		"COMMAND NAME", "STATUS", "PID", "UPTIME", "COMMAND")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(header))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))

	// Display each daemon
	for _, daemon := range daemons {
		var status, statusColor string
		if daemon.IsRunning {
			status = "Running"
			statusColor = colors.Success(status)
		} else {
			status = "Stopped"
			statusColor = colors.Warning(status)
		}

		commandName := daemon.CommandName
		if commandName == "" {
			commandName = "(legacy)"
		}

		uptime := daemon.Uptime
		if uptime == "" {
			uptime = "N/A"
		}

		command := daemon.Command
		if command == "" {
			command = "(unknown)"
		}
		if len(command) > 40 {
			command = command[:37] + "..."
		}

		row := fmt.Sprintf("%-20s %-10s %-8d %-12s %s",
			commandName,
			statusColor,
			daemon.PID,
			uptime,
			command)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), row)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "")
	_, _ = fmt.Fprint(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Total: %d daemon process(es)\n", len(daemons))))

	return nil
}

func HandleRestartCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("restart command requires a daemon name"))
	}

	daemonName := args[1]
	log.Printf("Restarting daemon: %s", daemonName)

	daemon, err := executor.FindDaemonByCommandName(projectDir, daemonName)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("daemon '%s' not found: %v", daemonName, err)))
	}

	err = executor.RestartDaemonProcess(projectDir, daemon)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to restart daemon '%s': %v", daemonName, err)))
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Restarted daemon '%s'", daemonName)))
	return nil
}

func HandleStopCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", colors.Error("stop command requires a daemon name"))
	}

	daemonName := args[1]
	log.Printf("Stopping daemon: %s", daemonName)

	daemon, err := executor.FindDaemonByCommandName(projectDir, daemonName)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("daemon '%s' not found: %v", daemonName, err)))
	}

	err = executor.StopDaemonProcess(projectDir, daemon)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to stop daemon '%s': %v", daemonName, err)))
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Stopped daemon '%s'", daemonName)))
	return nil
}

func HandleOnboardCommand(cmd *cobra.Command, args []string, projectDir string) error {
	log.Print("Generating onboarding documentation for AI assistants")

	// Determine output file
	outputFile := "CLAUDE.md"
	if len(args) >= 2 {
		outputFile = args[1]
	}

	// Make path absolute if relative
	if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(projectDir, outputFile)
	}

	// Detect project type
	projectType := config.DetectProjectType(projectDir)
	projectTypeStr := projectTypeToString(projectType)

	// Load configuration - try to load user config, fallback to defaults
	configPath := filepath.Join(projectDir, ".dev-config.yaml")
	cfg, err := config.LoadConfigFromFile(configPath)
	var customCommands map[string][]config.CommandStep
	if err != nil {
		log.Printf("No custom config found, using defaults only: %v", err)
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

	// Write to file
	err = os.WriteFile(outputFile, []byte(doc), 0644)
	if err != nil {
		return fmt.Errorf("%s", colors.Error(fmt.Sprintf("failed to write to %s: %v", outputFile, err)))
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success(fmt.Sprintf("Generated AI assistant documentation at %s", outputFile)))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Project type: %s", projectTypeStr)))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Info(fmt.Sprintf("Commands documented: %d built-in, %d custom",
		len(builtInCommands), len(customCommands))))

	return nil
}

func projectTypeToString(pt config.ProjectType) string {
	switch pt {
	case config.ProjectTypeGo:
		return "Go"
	case config.ProjectTypePython:
		return "Python"
	case config.ProjectTypeNodeJS:
		return "Node.js"
	case config.ProjectTypeRust:
		return "Rust"
	case config.ProjectTypeMaven:
		return "Java/Maven"
	case config.ProjectTypeDotNet:
		return ".NET"
	case config.ProjectTypePHP:
		return "PHP"
	case config.ProjectTypeRuby:
		return "Ruby"
	default:
		return "Unknown"
	}
}

type commandInfo struct {
	Name        string
	Description string
	IsCustom    bool
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
		{Name: "completion <shell>", Description: "Generate shell completion script"},
	}

	// Template data
	data := struct {
		ProjectType            string
		ProjectDir             string
		Commands               []commandInfo
		BuiltInDevToolsCmds   []commandInfo
		HasCustomCommands      bool
		HasServiceManagement   bool
	}{
		ProjectType:            projectType,
		ProjectDir:             projectDir,
		Commands:               commands,
		BuiltInDevToolsCmds:   builtInDevToolsCommands,
		HasCustomCommands:      len(customCmds) > 0,
		HasServiceManagement:   hasServiceManagement(customCmds),
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
		if len(cmd) > 50 {
			cmd = cmd[:47] + "..."
		}
		return fmt.Sprintf("Run %s", cmd)
	}
	return "Run " + name
}

func getCustomDescription(steps []config.CommandStep) string {
	var parts []string

	// Check for services
	hasServices := false
	for _, step := range steps {
		if step.Services.Compose != nil || len(step.Services.Containers) > 0 {
			hasServices = true
			break
		}
	}

	if hasServices {
		parts = append(parts, "with services")
	}

	// Count run commands
	runCount := 0
	for _, step := range steps {
		runCount += len(step.Run)
	}

	if runCount == 1 && len(steps[0].Run) > 0 {
		cmd := string(steps[0].Run[0])
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
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

This project has custom commands defined in ` + "`.dev-config.yaml`" + `. These commands may:
- Start Docker containers or Docker Compose services
- Run multiple commands in sequence
- Execute commands in specific directories
- Run background daemons

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

### Checking Running Processes
` + "```bash" + `
dev-tools status
` + "```" + `

### Viewing Logs
` + "```bash" + `
dev-tools logs
` + "```" + `

## Notes

- This documentation was auto-generated by ` + "`dev-tools onboard`" + `
- Commands and their behavior depend on project configuration
- Always verify command output before making assumptions about success
- Use ` + "`dev-tools --help`" + ` for more information

---

*Generated by dev-tools onboard command*
`
