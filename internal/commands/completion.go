package commands

import (
	"dev-tools/internal/logger"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"dev-tools/internal/config"
	"dev-tools/internal/executor"
)

// Completion cache configuration
const (
	// DefaultCompletionCacheTTL is the default time-to-live for completion cache entries
	DefaultCompletionCacheTTL = 5 * time.Second
)

// CompletionCache provides thread-safe caching for rapid completions
type CompletionCache struct {
	mu         sync.RWMutex
	cache      map[string][]string
	projectDir string
	timestamp  time.Time
	ttl        time.Duration
}

// NewCompletionCache creates a new thread-safe completion cache
func NewCompletionCache(ttl time.Duration) *CompletionCache {
	return &CompletionCache{
		cache: make(map[string][]string),
		ttl:   ttl,
	}
}

// Get retrieves cached completions if still valid
func (c *CompletionCache) Get(ctx *CompletionContext) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if cache is invalid
	if c.cache == nil ||
		c.projectDir != ctx.ProjectDir ||
		time.Since(c.timestamp) > c.ttl {
		return nil
	}

	key := fmt.Sprintf("%s:%d:%s", strings.Join(ctx.Words, " "), ctx.WordIndex, ctx.CurrentWord)
	return c.cache[key]
}

// Set caches completions for rapid access
func (c *CompletionCache) Set(ctx *CompletionContext, completions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset cache if project directory changed
	if c.cache == nil || c.projectDir != ctx.ProjectDir {
		c.cache = make(map[string][]string)
	}

	c.projectDir = ctx.ProjectDir
	c.timestamp = time.Now()

	key := fmt.Sprintf("%s:%d:%s", strings.Join(ctx.Words, " "), ctx.WordIndex, ctx.CurrentWord)
	c.cache[key] = completions
}

// Global completion cache instance (thread-safe)
var globalCompletionCache = NewCompletionCache(DefaultCompletionCacheTTL)

// HandleCompletionCommand generates shell completion scripts
func HandleCompletionCommand(cmd *cobra.Command, args []string) error {
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
    COMPREPLY=()

    # Try to use bash-completion helpers if available
    if declare -F _get_comp_words_by_ref >/dev/null; then
        _get_comp_words_by_ref -n : cur prev words cword
    else
        # Fallback: handle word splitting manually
        # Bash splits on COMP_WORDBREAKS which includes ':'
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"

        # Reconstruct the current word if it was split by ':'
        if [[ "$prev" == ":" ]]; then
            # The word was split, reconstruct it
            local i=$((COMP_CWORD - 2))
            while [[ $i -ge 0 ]]; do
                if [[ "${COMP_WORDS[$i]}" == ":" ]]; then
                    i=$((i - 1))
                    continue
                fi
                cur="${COMP_WORDS[$i]}:$cur"
                if [[ $i -eq 0 ]] || [[ "${COMP_WORDS[$i-1]}" != ":" ]]; then
                    break
                fi
                i=$((i - 1))
            done
        elif [[ "$cur" == ":" ]]; then
            # Cursor is on the colon itself
            cur="${COMP_WORDS[COMP_CWORD-1]}:"
        fi
    fi

    # Get completions from dev-tools using the full COMP_LINE
    local completions=$(dev-tools __dev_complete "$COMP_LINE" 2>/dev/null)
    if [[ $? -eq 0 && -n "$completions" ]]; then
        # Generate completions - compgen expects space-separated words
        COMPREPLY=($(compgen -W "$completions" -- "$cur"))

        # Handle colon trimming if bash-completion helpers exist
        if declare -F __ltrim_colon_completions >/dev/null; then
            __ltrim_colon_completions "$cur"
        fi
    fi
}

complete -o nospace -F _dev_tools_completion dev-tools

# Auto-register completion for any aliases pointing to dev-tools
_dev_tools_register_aliases() {
    local alias_name alias_value
    while IFS='=' read -r alias_name alias_value; do
        # Strip "alias " prefix and quotes from value
        alias_name="${alias_name#alias }"
        alias_value="${alias_value%\'}"
        alias_value="${alias_value#\'}"
        alias_value="${alias_value%\"}"
        alias_value="${alias_value#\"}"

        # Check if alias points to dev-tools (exact match or with args)
        if [[ "$alias_value" == "dev-tools" || "$alias_value" == "dev-tools "* ]]; then
            complete -o nospace -F _dev_tools_completion "$alias_name"
        fi
    done < <(alias 2>/dev/null)
}
_dev_tools_register_aliases
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

compdef _dev_tools dev-tools

# Auto-register completion for aliases
() {
    local name value
    for name value in ${(kv)aliases}; do
        if [[ "$value" == "dev-tools" || "$value" == "dev-tools "* ]]; then
            compdef _dev_tools "$name"
        fi
    done
}
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

# Auto-register completion for abbreviations and aliases
for abbr_line in (abbr --show 2>/dev/null)
    set -l parts (string split " -- " $abbr_line)
    if test (count $parts) -ge 2
        set -l name (string replace "abbr -a " "" $parts[1])
        set -l value (string trim -c "'" $parts[2])
        if string match -q "dev-tools*" "$value"
            complete -c $name -w dev-tools
        end
    end
end
`

	_, _ = fmt.Fprint(cmd.OutOrStdout(), script)
	return nil
}

// HandleCompleteCommand provides dynamic completions for shell completion
func HandleCompleteCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 2 {
		return nil // No command line to complete
	}

	commandLine := strings.Join(args[1:], " ")
	logger.Infof("Handling completion for: %q", commandLine)
	logger.Infof("Args received: %v", args)

	// Parse completion context
	ctx := parseCompletionContext(commandLine, projectDir)
	if ctx == nil {
		logger.Infof("No completion context generated")
		return nil // Invalid context
	}
	logger.Infof("Completion context: WordIndex=%d, CurrentWord=%q, CommandName=%q, IsFlag=%v",
		ctx.WordIndex, ctx.CurrentWord, ctx.CommandName, ctx.IsFlag)

	// Check cache first using thread-safe cache
	if completions := globalCompletionCache.Get(ctx); completions != nil {
		outputCompletions(cmd, completions)
		return nil
	}

	// Load configuration
	config, err := loadConfigForCompletion(projectDir)
	if err != nil {
		logger.Infof("Failed to load config for completion: %v", err)
		return nil // Don't error on completion failures
	}

	// Generate completions
	completions := generateCompletions(ctx, config)
	logger.Infof("Generated %d completions: %v", len(completions), completions)

	// Cache the results using thread-safe cache
	globalCompletionCache.Set(ctx, completions)

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
func parseCompletionContext(commandLine string, projectDir string) *CompletionContext {
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
func loadConfigForCompletion(projectDir string) (*config.Config, error) {
	return config.LoadConfigurationForProject(projectDir)
}

// generateCompletions generates appropriate completions based on context
func generateCompletions(ctx *CompletionContext, config *config.Config) []string {
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
func getAllAvailableCommands(config *config.Config) []string {
	commandSet := make(map[string]bool)

	// Add built-in commands - initialize registry first, then iterate
	initBuiltInCommandRegistry()
	for _, cmd := range builtInCommandRegistry {
		// Skip internal commands (like __dev_complete) - they shouldn't be visible to users
		if strings.HasPrefix(cmd.Name, "__") {
			continue
		}
		commandSet[cmd.Name] = true
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
		logger.Infof("Failed to list daemon processes: %v", err)
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

// outputCompletions outputs completions in space-separated format
func outputCompletions(cmd *cobra.Command, completions []string) {
	if len(completions) > 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(completions, " "))
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
