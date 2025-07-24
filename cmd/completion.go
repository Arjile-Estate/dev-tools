package cmd

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
)

// Caching for rapid completions
var (
	lastCompletionCache     map[string][]string
	lastCompletionCacheDir  string
	lastCompletionCacheTime time.Time
	completionCacheTTL      = 5 * time.Second
)

// handleCompletionCommand generates shell completion scripts
func handleCompletionCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("completion command requires a shell type (bash, zsh, fish)")
	}

	shell := args[1]
	switch shell {
	case "bash":
		return generateBashCompletion(cmd)
	case "zsh":
		return generateZshCompletion(cmd)
	case "fish":
		return generateFishCompletion(cmd)
	default:
		return fmt.Errorf("unsupported shell: %s. Supported shells: bash, zsh, fish", shell)
	}
}

// generateBashCompletion outputs a bash completion script
func generateBashCompletion(cmd *cobra.Command) error {
	script := `#!/bin/bash

_dev_tools_completion() {
    local cur prev words cword
    _init_completion || return

    # Get completions from dev-tools
    local completions=$(dev-tools __dev_complete "$COMP_LINE" 2>/dev/null)
    if [[ $? -eq 0 ]]; then
        COMPREPLY=($(compgen -W "$completions" -- "$cur"))
    fi
}

complete -F _dev_tools_completion dev-tools
`

	_, _ = fmt.Fprint(cmd.OutOrStdout(), script)
	return nil
}

// generateZshCompletion outputs a zsh completion script
func generateZshCompletion(cmd *cobra.Command) error {
	script := `#compdef dev-tools

_dev_tools() {
    local line state

    _arguments -C \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose logging]' \
        '(-p --project-dir)'{-p,--project-dir}'[Project directory]:directory:_directories' \
        '--no-color[Disable colored output]' \
        '--version[Show version]' \
        '1: :_dev_tools_commands' \
        '*::arg:->args'

    case $state in
        args)
            case $words[1] in
                restart|stop)
                    _dev_tools_daemon_names
                    ;;
            esac
            ;;
    esac
}

_dev_tools_commands() {
    local completions
    completions=(${(f)"$(dev-tools __dev_complete "dev-tools " 2>/dev/null)"})
    _describe 'commands' completions
}

_dev_tools_daemon_names() {
    local completions
    completions=(${(f)"$(dev-tools __dev_complete "dev-tools ${words[1]} " 2>/dev/null)"})
    _describe 'daemon names' completions
}

_dev_tools "$@"

`

	_, _ = fmt.Fprint(cmd.OutOrStdout(), script)
	return nil
}

// generateFishCompletion outputs a fish completion script
func generateFishCompletion(cmd *cobra.Command) error {
	script := `# dev-tools fish completion

function __dev_tools_complete
    set -l cmdline (commandline -cp)
    dev-tools __dev_complete "$cmdline " 2>/dev/null
end

# Complete dev-tools commands
complete -c dev-tools -f -a '(__dev_tools_complete)'

# Complete flags
complete -c dev-tools -s v -l verbose -d 'Enable verbose logging'
complete -c dev-tools -s p -l project-dir -d 'Project directory' -r
complete -c dev-tools -l no-color -d 'Disable colored output'
complete -c dev-tools -l version -d 'Show version'

`

	_, _ = fmt.Fprint(cmd.OutOrStdout(), script)
	return nil
}

// handleCompleteCommand provides dynamic completions for shell completion
func handleCompleteCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return nil // No command line to complete
	}

	commandLine := strings.Join(args[1:], " ")
	log.Printf("Handling completion for: %q", commandLine)
	log.Printf("Args received: %v", args)

	// Parse completion context
	ctx := parseCompletionContext(commandLine)
	if ctx == nil {
		log.Printf("No completion context generated")
		return nil // Invalid context
	}
	log.Printf("Completion context: WordIndex=%d, CurrentWord=%q, CommandName=%q, IsFlag=%v",
		ctx.WordIndex, ctx.CurrentWord, ctx.CommandName, ctx.IsFlag)

	// Check cache first
	if completions := getCachedCompletions(ctx); completions != nil {
		outputCompletions(cmd, completions)
		return nil
	}

	// Load configuration
	config, err := loadConfigForCompletion(projectDir)
	if err != nil {
		log.Printf("Failed to load config for completion: %v", err)
		return nil // Don't error on completion failures
	}

	// Generate completions
	completions := generateCompletions(ctx, config)
	log.Printf("Generated %d completions: %v", len(completions), completions)

	// Cache the results
	cacheCompletions(ctx, completions)

	// Output completions
	outputCompletions(cmd, completions)
	return nil
}

// CompletionContext represents the current completion state
type CompletionContext struct {
	Words       []string
	CurrentWord string
	WordIndex   int
	IsFlag      bool
	CommandName string
	ProjectDir  string
}

// parseCompletionContext parses the command line to understand completion context
func parseCompletionContext(commandLine string) *CompletionContext {
	// Remove trailing space and split
	line := strings.TrimRight(commandLine, " ")
	words := strings.Fields(line)

	if len(words) == 0 {
		return nil
	}

	// Skip "dev-tools" if present
	if len(words) > 0 && words[0] == "dev-tools" {
		words = words[1:]
	}

	ctx := &CompletionContext{
		Words:      words,
		WordIndex:  len(words),
		ProjectDir: projectDir,
	}

	// Determine current word and context
	if strings.HasSuffix(commandLine, " ") {
		// Completing a new word
		ctx.CurrentWord = ""
	} else if len(words) > 0 {
		// Completing the last word
		ctx.CurrentWord = words[len(words)-1]
		ctx.WordIndex = len(words) - 1
	}

	// Check if completing a flag
	if ctx.CurrentWord != "" && strings.HasPrefix(ctx.CurrentWord, "-") {
		ctx.IsFlag = true
	}

	// Determine command name
	if len(words) > 0 && !strings.HasPrefix(words[0], "-") {
		ctx.CommandName = words[0]
	}

	return ctx
}

// loadConfigForCompletion loads configuration for completion
func loadConfigForCompletion(projectDir string) (*config.DevConfig, error) {
	return config.LoadConfigurationForProject(projectDir)
}

// generateCompletions generates appropriate completions based on context
func generateCompletions(ctx *CompletionContext, config *config.DevConfig) []string {
	var completions []string

	if ctx.IsFlag {
		// Complete flags
		completions = []string{"--verbose", "--project-dir", "--no-color", "--version", "--help"}
	} else if ctx.WordIndex == 0 {
		// Complete first argument (commands)
		completions = getAllAvailableCommands(config)
	} else if ctx.WordIndex == 1 {
		// Complete second argument based on first command
		switch ctx.CommandName {
		case "restart", "stop":
			completions = getDaemonNames(ctx.ProjectDir)
		case "completion":
			completions = []string{"bash", "zsh", "fish"}
		}
	}

	// Filter completions based on current word
	if ctx.CurrentWord != "" {
		filtered := make([]string, 0)
		for _, completion := range completions {
			if strings.HasPrefix(completion, ctx.CurrentWord) {
				filtered = append(filtered, completion)
			}
		}
		completions = filtered
	}

	// Sort completions
	sort.Strings(completions)
	return completions
}

// getAllAvailableCommands returns all available commands (built-in + config + defaults)
func getAllAvailableCommands(config *config.DevConfig) []string {
	commandSet := make(map[string]bool)

	// Add built-in commands (hardcoded list to avoid circular dependency)
	builtinCommands := []string{
		"logs", "cleanup-pids", "cleanup-all", "status",
		"restart", "stop", "version", "completion",
	}
	for _, cmd := range builtinCommands {
		commandSet[cmd] = true
	}

	// Add commands from config
	for cmd := range config.Commands {
		commandSet[cmd] = true
	}

	// Convert to sorted slice
	commands := make([]string, 0, len(commandSet))
	for cmd := range commandSet {
		commands = append(commands, cmd)
	}

	sort.Strings(commands)
	return commands
}

// getDaemonNames returns names of daemon processes that can be restarted/stopped
func getDaemonNames(projectDir string) []string {
	nameSet := make(map[string]bool)

	// Get names from running daemon processes
	daemons, err := executor.ListDaemonProcesses(projectDir)
	if err != nil {
		log.Printf("Failed to list daemon processes: %v", err)
	} else {
		for _, daemon := range daemons {
			if daemon.CommandName != "" {
				nameSet[daemon.CommandName] = true
			}
		}
	}

	// Also get daemon command names from config file
	config, err := loadConfigForCompletion(projectDir)
	if err == nil && config != nil {
		for cmdName, steps := range config.Commands {
			for _, step := range steps {
				if step.Daemon {
					nameSet[cmdName] = true
					break
				}
			}
		}
	}

	// Convert to sorted slice
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// getCachedCompletions returns cached completions if still valid
func getCachedCompletions(ctx *CompletionContext) []string {
	if lastCompletionCache == nil ||
		lastCompletionCacheDir != ctx.ProjectDir ||
		time.Since(lastCompletionCacheTime) > completionCacheTTL {
		return nil
	}

	key := fmt.Sprintf("%s:%d:%s", strings.Join(ctx.Words, " "), ctx.WordIndex, ctx.CurrentWord)
	return lastCompletionCache[key]
}

// cacheCompletions caches completions for rapid access
func cacheCompletions(ctx *CompletionContext, completions []string) {
	if lastCompletionCache == nil || lastCompletionCacheDir != ctx.ProjectDir {
		lastCompletionCache = make(map[string][]string)
	}

	lastCompletionCacheDir = ctx.ProjectDir
	lastCompletionCacheTime = time.Now()

	key := fmt.Sprintf("%s:%d:%s", strings.Join(ctx.Words, " "), ctx.WordIndex, ctx.CurrentWord)
	lastCompletionCache[key] = completions
}

// outputCompletions outputs completions in space-separated format
func outputCompletions(cmd *cobra.Command, completions []string) {
	if len(completions) > 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(completions, " "))
	}
}
