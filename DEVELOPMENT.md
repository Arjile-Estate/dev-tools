# Development Guide for Dev-Tools

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Directory Structure](#directory-structure)
- [Core Components](#core-components)
- [Development Workflow](#development-workflow)
- [Testing Guidelines](#testing-guidelines)
- [Contributing New Features](#contributing-new-features)
- [Code Organization Principles](#code-organization-principles)
- [Common Patterns](#common-patterns)
- [AI Assistant Guidelines](#ai-assistant-guidelines)

## Project Overview

Dev-Tools is a unified command runner for development workflows written in Go. It provides:

- **Smart project detection** (Go, Python, Node.js, Rust, Java, .NET, PHP, Ruby)
- **Consistent CLI interface** across all project types
- **Advanced service management** (Docker, Docker Compose)
- **Daemon process support** with SHA1-based PID tracking
- **Watch mode** for TDD workflows
- **Retry logic** for flaky tests
- **JSON output** for automation
- **Shell completion** (Bash, Zsh, Fish)

**Language**: Go 1.21+
**CLI Framework**: Cobra
**Logging**: zerolog
**Testing**: Standard Go testing + testify/assert

## Architecture

### High-Level Design

```
┌─────────────────────────────────────────────────────┐
│                    main.go                          │
│              (Entry point - minimal)                │
└───────────────────────┬─────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────┐
│                  cmd/root.go                        │
│         (Cobra root command, flag parsing)          │
│       - Flag handling (--watch, --format, etc)      │
│       - Command routing                             │
│       - Exit code handling                          │
└───────────────────────┬─────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
┌───────▼─────────────┐      ┌──────────▼───────────────┐
│ internal/commands/  │      │  internal/executor/      │
│   (Command handlers)│      │  (Execution engine)      │
│ - onboard.go        │      │ - executor_command.go    │
│ - status.go         │      │ - executor_daemon.go     │
│ - cleanup.go        │      │ - executor_docker.go     │
│ - completion.go     │      │ - executor_services.go   │
│ - registry.go       │      │ - executor_watch.go      │
│ - daemon.go         │      │ - executor_retry.go      │
└─────────┬───────────┘      └──────────┬───────────────┘
          │                             │
          │                  ┌──────────▼───────────────┐
          │                  │  internal/config/        │
          └──────────────────►  (Configuration layer)   │
                             │ - config.go              │
                             │ - types.go               │
                             │ - schema.go              │
                             │ - interfaces.go          │
                             └──────────────────────────┘
```

### Layer Responsibilities

1. **Entry Point** (`main.go`): Minimal - just creates root command and handles exit codes
2. **CLI Layer** (`cmd/`): Cobra command setup, flag parsing, command routing
3. **Command Layer** (`internal/commands/`): Business logic for built-in commands
4. **Execution Layer** (`internal/executor/`): Core execution engine (commands, daemons, services)
5. **Configuration Layer** (`internal/config/`): YAML parsing, validation, project detection
6. **Utilities** (`internal/logger/`, `internal/colors/`): Cross-cutting concerns

## Directory Structure

```
dev-tools/
├── main.go                      # Entry point (minimal)
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── .dev-config.yaml             # Project's own dev-tools config
├── README.md                    # User-facing documentation
├── CLAUDE.md                    # AI assistant guide (auto-generated)
├── DEVELOPMENT.md               # This file
│
├── cmd/                         # Cobra CLI layer
│   ├── root.go                  # Root command, flag parsing, command routing
│   ├── root_test.go             # Root command tests
│   ├── logging.go               # Log configuration
│   ├── logging_test.go          # Log tests
│   ├── helpers.go               # CLI utility functions
│   ├── helpers_test.go          # Helper tests
│   ├── completion_test.go       # Shell completion tests
│   └── activity.log             # Runtime activity log (generated)
│
├── internal/                    # Internal packages
│   │
│   ├── commands/                # Built-in command handlers
│   │   ├── built_in_test.go     # Tests for built-in commands
│   │   ├── cleanup.go           # cleanup-pids, cleanup-all commands
│   │   ├── completion.go        # Shell completion generation
│   │   ├── completion_test.go   # Completion tests
│   │   ├── constants.go         # Command constants (max lengths, etc)
│   │   ├── daemon.go            # restart, stop commands
│   │   ├── logs.go              # logs command
│   │   ├── onboard.go           # onboard command (generates CLAUDE.md)
│   │   ├── registry.go          # Built-in command registry
│   │   ├── registry_test.go     # Registry tests
│   │   ├── status.go            # status command (text/JSON output)
│   │   └── validate.go          # validate command
│   │
│   ├── config/                  # Configuration layer
│   │   ├── config.go            # Config loading, project detection
│   │   ├── config_test.go       # Config tests
│   │   ├── config_refactored_test.go  # Legacy test suite
│   │   ├── types.go             # Config struct definitions
│   │   ├── interfaces.go        # ConfigLoader interface
│   │   ├── schema.go            # YAML schema validation
│   │   ├── schema_test.go       # Schema tests
│   │   └── errors.go            # Config-specific errors
│   │
│   ├── executor/                # Execution engine
│   │   ├── executor_command.go  # Main command execution logic
│   │   ├── executor_daemon.go   # Daemon process management
│   │   ├── executor_docker.go   # Docker container management
│   │   ├── executor_services.go # Docker Compose service management
│   │   ├── executor_watch.go    # Watch mode (file watching)
│   │   ├── executor_retry.go    # Retry logic
│   │   ├── executor_validation.go # Input validation
│   │   ├── executor_signals.go  # Signal handling
│   │   ├── executor_test.go     # Executor tests
│   │   ├── executor_coverage_test.go # Coverage tests
│   │   ├── executor_watch_test.go # Watch mode tests
│   │   ├── docker_helpers.go    # Docker utility functions
│   │   ├── interfaces.go        # Executor interface
│   │   ├── types.go             # Executor types
│   │   ├── errors.go            # Executor errors
│   │   ├── errors_test.go       # Error tests
│   │   ├── constants.go         # Executor constants
│   │   └── data/                # Test data files
│   │
│   ├── logger/                  # Logging utilities
│   │   ├── logger.go            # zerolog configuration
│   │   └── logger_test.go       # Logger tests
│   │
│   ├── colors/                  # Terminal color utilities
│   │   ├── colors.go            # ANSI color functions
│   │   └── colors_test.go       # Color tests
│   │
│   └── mocks/                   # Generated mocks (mockery)
│       ├── ConfigLoader.go      # Mock ConfigLoader
│       └── Executor.go          # Mock Executor
│
├── test-tools/                  # Development tools (Python scripts)
│   └── ...
│
└── .beads/                      # Beads issue tracking (not in repo)
```

## Core Components

### 1. Entry Point (`main.go`)

**Purpose**: Minimal entry point - creates root command and handles exit codes.

```go
func main() {
    rootCmd := cmd.NewRootCommand()
    if err := rootCmd.Execute(); err != nil {
        // Handle ExitError with specific codes
        var exitErr *cmd.ExitError
        if errors.As(err, &exitErr) {
            os.Exit(exitErr.Code)
        }
        os.Exit(1)
    }
}
```

**Rules**:
- Keep this file minimal (under 25 lines)
- Only handle exit codes here
- No business logic

### 2. CLI Layer (`cmd/root.go`)

**Purpose**: Cobra command setup, flag parsing, command routing.

**Key Responsibilities**:
- Define root command with version
- Parse global flags (`--verbose`, `--watch`, `--format`, `--project-dir`, `--no-color`)
- Route commands to handlers (built-in vs custom)
- Handle signals (Ctrl+C)
- Manage logging configuration

**Key Functions**:
- `NewRootCommand()` - Creates Cobra root command
- `setupFlagsAndContext()` - Parses flags and creates context
- `routeCommand()` - Routes to built-in or custom command handler

**Adding New Flags**:
1. Add flag to `CommandConfig` struct
2. Parse in `setupFlagsAndContext()`
3. Document in onboard.go template
4. Add tests to `root_test.go`

### 3. Command Layer (`internal/commands/`)

**Purpose**: Built-in command implementations (status, logs, cleanup, etc).

**Key Files**:

- **`registry.go`**: Maps command names to handlers
  ```go
  func GetBuiltInCommands() map[string]CommandHandler
  ```

- **`status.go`**: Shows running daemons (text/JSON output)
  ```go
  func HandleStatusCommand(cmd *cobra.Command, args []string, projectDir string)
  ```
  The output format ("text" or "json") is propagated via the cobra command
  context using `FormatCtxKey`; handlers read it via
  `commands.FormatFromContext(cmd)`. When the `CLAUDE_CODE=1` environment
  variable is set, the default is `json` so Claude Code sessions always get
  structured output.

- **`onboard.go`**: Generates CLAUDE.md for AI assistants
  ```go
  func HandleOnboardCommand(cmd *cobra.Command, args []string, projectDir string)
  ```

- **`completion.go`**: Shell completion generation
  ```go
  func HandleCompletionCommand(cmd *cobra.Command, args []string)
  ```

- **`cleanup.go`**: PID file cleanup
- **`daemon.go`**: Daemon management (restart, stop)
- **`logs.go`**: View activity logs
- **`validate.go`**: Validate `.dev-config.yaml`

**Adding New Built-In Commands**:
1. Create handler in `internal/commands/`
2. Add to registry in `registry.go`
3. Add tests in `<name>_test.go`
4. Update documentation

### 4. Execution Layer (`internal/executor/`)

**Purpose**: Core execution engine - runs commands, manages daemons, handles services.

**Key Files**:

- **`executor_command.go`**: Main command execution
  - `ExecuteCommandWithOptions()` - Main entry point
  - Handles environment loading, directory changes, output capture
  - Integrates retry, watch, services

- **`executor_daemon.go`**: Daemon process management
  - SHA1-based PID tracking (prevents duplicates)
  - Background process spawning
  - PID file management in `/tmp/`

- **`executor_docker.go`**: Docker container management
  - Container creation, starting, stopping
  - Smart container naming from image paths
  - Health check polling

- **`executor_services.go`**: Docker Compose service management
  - Service startup with profiles
  - Health check waiting
  - Automatic cleanup

- **`executor_watch.go`**: Watch mode (file watching)
  - File pattern matching
  - Debouncing (300ms default)
  - Ignore patterns
  - Automatic re-execution on changes

- **`executor_retry.go`**: Retry logic
  - Configurable retry attempts
  - Delay between retries
  - Exit code filtering

**Interfaces**:
```go
type Executor interface {
    ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult
    LoadEnvironmentVariables(envFile string) error
    WatchAndExecute(ctx context.Context, ...) error
}
```

**Key Types**:
```go
type CommandExecutionOptions struct {
    CommandName      string
    Steps            []config.CommandStep
    WorkingDir       string
    PassthroughArgs  []string
    LogOutput        io.Writer
}

type ExecutionResult struct {
    Success    bool
    ExitCode   int
    Output     string
    Error      error
}
```

### 5. Configuration Layer (`internal/config/`)

**Purpose**: YAML parsing, validation, project detection.

**Key Files**:

- **`config.go`**: Config loading and project detection
  - `LoadConfigFromFile()` - Loads `.dev-config.yaml`
  - `DetectProjectType()` - Auto-detects project type
  - `GetDefaultCommandsForProjectType()` - Provides defaults

- **`types.go`**: Configuration struct definitions
  ```go
  type Config struct {
      Commands map[string][]CommandStep `yaml:"commands"`
  }

  type CommandStep struct {
      Run              RunCommand
      Services         ServicesConfig
      Background       bool
      Daemon           bool
      Directory        string
      Retry            int
      RetryDelay       string
      RetryOnExitCodes []int
      Watch            *WatchConfig
  }
  ```

- **`schema.go`**: YAML schema validation
  - JSON Schema validation with detailed errors
  - Validates all config fields
  - Provides helpful error messages

- **`interfaces.go`**: ConfigLoader interface (for mocking)

**Project Types Supported**:
- Go (go.mod)
- Python (requirements.txt, pyproject.toml, setup.py)
- Node.js (package.json)
- Rust (Cargo.toml)
- Java (pom.xml, build.gradle)
- .NET (*.csproj, *.sln)
- PHP (composer.json)
- Ruby (Gemfile)
- Unknown (fallback)

### 6. Utilities

**Logger** (`internal/logger/`):
- Structured logging with zerolog
- Writes to `activity.log`
- Log levels: Debug, Info, Warn, Error

**Colors** (`internal/colors/`):
- Terminal ANSI color helpers
- Functions: Success(), Error(), Info(), Warning(), Command(), Subtle()
- Respects `--no-color` flag

## Development Workflow

### Setting Up Development Environment

```bash
# Clone repository
git clone git@github.com:Arjile-Estate/dev-tools.git
cd dev-tools

# Install dependencies
go mod tidy

# Build binary
dev-tools build
# or: go build -o dev-tools .

# Run tests
dev-tools test
# or: go test ./... -v

# Run with coverage
dev-tools test:coverage
# or: go test -cover ./...

# Run linter
dev-tools lint
# or: golangci-lint run && go fmt ./...

# Validate config
dev-tools validate
```

### Watch Mode for TDD

```bash
# Re-run tests on file changes (requires watch config in .dev-config.yaml)
dev-tools --watch test
```

### Debugging

```bash
# Enable verbose logging
dev-tools --verbose test

# View activity logs
dev-tools logs

# Check daemon status
dev-tools status

# Clean up stale PIDs
dev-tools cleanup-pids
```

### Building and Installing

```bash
# Build
go build -o dev-tools .

# Install locally
cp dev-tools ~/.local/bin/

# Install globally (requires sudo)
sudo cp dev-tools /usr/local/bin/
```

## Testing Guidelines

### Test Organization

- **Unit tests**: Test individual functions in isolation
- **Integration tests**: Test component interactions
- **Coverage tests**: Ensure code paths are covered

### Test File Naming

- Place tests in same package as code
- Name: `<package>_test.go` or `<feature>_test.go`
- Example: `executor_test.go`, `config_test.go`

### Writing Tests

```go
func TestFeature(t *testing.T) {
    t.Run("descriptive test case name", func(t *testing.T) {
        // Arrange
        input := setupTestData()

        // Act
        result := FunctionUnderTest(input)

        // Assert
        assert.Equal(t, expectedValue, result)
        assert.NoError(t, err)
    })
}
```

### Using Mocks

Mocks are generated with mockery:

```go
import "dev-tools/internal/mocks"

mockLoader := new(mocks.ConfigLoader)
mockLoader.On("LoadConfig", mock.Anything).Return(cfg, nil)
```

Generate mocks:
```bash
go generate ./...
```

### Test Coverage

```bash
# Run with coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Helpers

Use testify/assert for cleaner assertions:

```go
import (
    "testing"
    "github.com/stretchr/testify/assert"
)

assert.Equal(t, expected, actual)
assert.NoError(t, err)
assert.Contains(t, output, "expected string")
assert.True(t, condition)
```

## Contributing New Features

### Process Overview

1. **Understand the feature** - Read existing code, understand architecture
2. **Write tests first** (TDD) - Define expected behavior
3. **Implement feature** - Keep it simple, follow patterns
4. **Update documentation** - README.md, CLAUDE.md (via onboard), DEVELOPMENT.md
5. **Test thoroughly** - Unit tests, integration tests, manual testing
6. **Commit** - Follow conventional commits format

### Adding a New Built-In Command

**Example**: Adding a `logs-json` command

1. **Create handler** in `internal/commands/logs_json.go`:
   ```go
   package commands

   func HandleLogsJSONCommand(cmd *cobra.Command, args []string) error {
       // Implementation
   }
   ```

2. **Add to registry** in `internal/commands/registry.go`:
   ```go
   func GetBuiltInCommands() map[string]CommandHandler {
       return map[string]CommandHandler{
           // ...
           "logs-json": HandleLogsJSONCommand,
       }
   }
   ```

3. **Add tests** in `internal/commands/logs_json_test.go`

4. **Update documentation**:
   - Add to `internal/commands/onboard.go` template (Built-in Dev-Tools Commands)
   - Run `dev-tools onboard` to regenerate CLAUDE.md
   - Update README.md if user-facing

### Adding a New Flag

**Example**: Adding `--timeout` flag

1. **Add to CommandConfig** in `cmd/root.go`:
   ```go
   type CommandConfig struct {
       Verbose    bool
       ProjectDir string
       NoColor    bool
       Format     string
       Watch      bool
       Timeout    int  // NEW
   }
   ```

2. **Parse flag** in `setupFlagsAndContext()`:
   ```go
   timeout := 0
   for i, arg := range args {
       if arg == "--timeout" && i+1 < len(args) {
           timeout, _ = strconv.Atoi(args[i+1])
           break
       }
   }
   ```

3. **Use flag** in relevant code

4. **Document** in `internal/commands/onboard.go` template (Flags section)

5. **Add tests** in `cmd/root_test.go`

6. **Update README.md**

### Adding a New Executor Feature

**Example**: Adding "command retry with exponential backoff"

1. **Add config fields** in `internal/config/types.go`:
   ```go
   type CommandStep struct {
       // ...
       RetryBackoff string `yaml:"retry_backoff,omitempty"` // "exponential" or "linear"
   }
   ```

2. **Implement logic** in `internal/executor/executor_retry.go`

3. **Integrate** in `internal/executor/executor_command.go`

4. **Add tests** in `internal/executor/executor_retry_test.go`

5. **Update schema** in `internal/config/schema.go`

6. **Document**:
   - Add to onboard.go template (Custom Configuration capabilities)
   - Update README.md (Configuration section)
   - Update DEVELOPMENT.md (this file)

### Adding a New Project Type

**Example**: Adding Swift support

1. **Add enum** in `internal/config/config.go`:
   ```go
   const (
       // ...
       ProjectSwift ProjectType = "Swift"
   )
   ```

2. **Add detection** in `DetectProjectType()`:
   ```go
   if fileExists(filepath.Join(projectDir, "Package.swift")) {
       return ProjectSwift
   }
   ```

3. **Add defaults** in `GetDefaultCommandsForProjectType()`:
   ```go
   case ProjectSwift:
       return Config{
           Commands: map[string][]CommandStep{
               "test":  {{Run: RunCommand{"swift test"}}},
               "build": {{Run: RunCommand{"swift build"}}},
               "lint":  {{Run: RunCommand{"swiftlint"}}},
           },
       }
   ```

4. **Add tests** in `internal/config/config_test.go`

5. **Update documentation**

## Code Organization Principles

### 1. Single Responsibility Principle

Each file/package should have ONE clear purpose:
- `executor_daemon.go` - Only daemon management
- `executor_docker.go` - Only Docker container management
- `executor_services.go` - Only Docker Compose services

### 2. Dependency Injection

Use interfaces for testability:
```go
// Define interface
type Executor interface {
    ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult
}

// Inject in tests
func TestSomething(t *testing.T) {
    mockExec := new(mocks.Executor)
    mockExec.On("ExecuteCommandWithOptions", mock.Anything).Return(result)
}
```

### 3. Error Handling

- Always handle errors explicitly
- Use custom error types when needed (`internal/config/errors.go`, `internal/executor/errors.go`)
- Provide helpful error messages
- Use `fmt.Errorf()` with context

```go
if err != nil {
    return fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

### 4. Logging

Use structured logging:
```go
logger.Info("Starting command execution", "command", commandName)
logger.Error("Command failed", "error", err, "exitCode", exitCode)
```

### 5. Testing

- Write tests for all new code
- Use table-driven tests for multiple cases
- Mock external dependencies
- Test error paths

### 6. Documentation

- Add godoc comments to exported functions
- Update README.md for user-facing features
- Update CLAUDE.md via onboard command
- Update DEVELOPMENT.md for architecture changes

## Common Patterns

### Pattern 1: Command Handler

```go
func HandleMyCommand(cmd *cobra.Command, args []string, config MyConfig) error {
    // 1. Validate inputs
    if err := validateInputs(args); err != nil {
        return err
    }

    // 2. Load configuration
    cfg, err := loadConfig()
    if err != nil {
        return err
    }

    // 3. Execute business logic
    result, err := doWork(cfg)
    if err != nil {
        return err
    }

    // 4. Format and output results
    fmt.Fprintln(cmd.OutOrStdout(), formatResult(result))
    return nil
}
```

### Pattern 2: Options Struct

Use options struct for functions with many parameters:

```go
type CommandExecutionOptions struct {
    CommandName      string
    Steps            []config.CommandStep
    WorkingDir       string
    PassthroughArgs  []string
    LogOutput        io.Writer
}

func ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult {
    // Implementation
}
```

### Pattern 3: Result Struct

Return structured results instead of multiple return values:

```go
type ExecutionResult struct {
    Success    bool
    ExitCode   int
    Output     string
    Error      error
}

func ExecuteCommand() ExecutionResult {
    // Implementation
    return ExecutionResult{
        Success: true,
        ExitCode: 0,
        Output: "success",
    }
}
```

### Pattern 4: Interface for Mocking

Define interfaces for external dependencies:

```go
// interfaces.go
type Executor interface {
    ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult
}

// Production code
type RealExecutor struct{}
func (e *RealExecutor) ExecuteCommandWithOptions(opts CommandExecutionOptions) ExecutionResult {
    // Real implementation
}

// Test code
mockExec := new(mocks.Executor)
mockExec.On("ExecuteCommandWithOptions", mock.Anything).Return(result)
```

### Pattern 5: Context for Cancellation

Use context for signal handling:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Handle signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigChan
    cancel()
}()

// Use context in operations
select {
case <-ctx.Done():
    return ctx.Err()
case result := <-workChan:
    return result
}
```

## AI Assistant Guidelines

### For AI Assistants Working on This Project

**When asked to add features or fix bugs**:

1. **Read first, code second**
   - Use Read tool to understand existing code
   - Check this DEVELOPMENT.md for architecture
   - Look at similar existing features

2. **Follow TDD**
   - Write tests first
   - Implement minimal code to pass
   - Refactor

3. **Check existing patterns**
   - Look for similar code in the same package
   - Follow the same structure
   - Use established helper functions

4. **Update documentation**
   - Regenerate CLAUDE.md: `dev-tools onboard`
   - Update README.md if user-facing
   - Update DEVELOPMENT.md if architecture changes

5. **Test thoroughly**
   - Run `dev-tools test`
   - Run `dev-tools lint`
   - Test manually with `dev-tools --verbose <command>`

6. **File locations guide**:
   - New CLI flag? → `cmd/root.go` + `cmd/root_test.go`
   - New built-in command? → `internal/commands/` + registry
   - New executor feature? → `internal/executor/`
   - New config field? → `internal/config/types.go` + schema
   - New utility? → `internal/logger/` or `internal/colors/`

7. **Common tasks**:
   - Add flag: CommandConfig struct → setupFlagsAndContext() → onboard.go → tests
   - Add command: Create handler → Add to registry → Add tests → Update docs
   - Add config field: types.go → schema.go → executor → tests
   - Add project type: config.go enum → DetectProjectType() → GetDefaultCommandsForProjectType()

8. **Testing commands**:
   ```bash
   dev-tools test              # Run all tests
   dev-tools test:coverage     # Coverage report
   dev-tools lint              # Run linter
   dev-tools --verbose test    # Debug mode
   dev-tools validate          # Validate config
   ```

9. **Debugging workflow**:
   ```bash
   dev-tools logs              # View activity logs
   dev-tools status            # Check running daemons
   dev-tools cleanup-pids      # Clean stale processes
   dev-tools --verbose <cmd>   # Run with verbose logging
   ```

10. **Key files to reference**:
    - Architecture: `DEVELOPMENT.md` (this file)
    - User docs: `README.md`
    - AI guide: `CLAUDE.md` (auto-generated)
    - Config example: `.dev-config.yaml`
    - Main entry: `main.go` → `cmd/root.go`

### Common Mistakes to Avoid

1. ❌ **Don't** add logic to main.go (keep it minimal)
2. ❌ **Don't** bypass interfaces (breaks testing)
3. ❌ **Don't** skip error handling
4. ❌ **Don't** forget to update documentation
5. ❌ **Don't** write code without tests
6. ❌ **Don't** ignore existing patterns
7. ❌ **Don't** modify generated files (CLAUDE.md, mocks) directly
8. ❌ **Don't** add features without understanding existing code

### Quick Reference: File Responsibility

| File/Directory | Purpose | When to Modify |
|---------------|---------|---------------|
| `main.go` | Entry point | Rarely (only for exit code handling) |
| `cmd/root.go` | CLI setup, flags | Adding global flags, command routing |
| `internal/commands/` | Built-in commands | Adding new commands (status, logs, etc) |
| `internal/executor/` | Execution engine | Adding execution features (retry, watch, etc) |
| `internal/config/` | Config & detection | Adding config fields, project types |
| `internal/logger/` | Logging | Changing log format or behavior |
| `internal/colors/` | Terminal colors | Adding color functions |
| `internal/mocks/` | Test mocks | Auto-generated (don't edit directly) |
| `CLAUDE.md` | AI assistant guide | Auto-generated (edit onboard.go template) |

## Version Control

### Commit Message Format

Follow conventional commits:

```
<type>: <description>

[optional body]

[optional footer]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding or updating tests
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `chore`: Maintenance tasks

**Examples**:
```
feat: add --timeout flag for command execution

Add configurable timeout for long-running commands.
Timeout can be set via --timeout flag (in seconds).

feat: support Swift project detection

Add Swift project type with Package.swift detection.
Includes default commands for test, build, and lint.

fix: prevent duplicate daemon processes

Use SHA1-based PID tracking to prevent duplicate daemons
when command is run multiple times.

docs: update DEVELOPMENT.md with testing guidelines

Add comprehensive testing section with examples and
best practices for contributors.
```

### Git Workflow

```bash
# 1. Create feature branch (if needed)
git checkout -b feature/my-feature

# 2. Make changes, test thoroughly
dev-tools test
dev-tools lint

# 3. Commit changes
git add <files>
git commit -m "feat: add my feature"

# 4. Push to remote
git push origin feature/my-feature

# 5. Create pull request (if using GitHub)
```

## Getting Help

### Internal Resources

1. **This file (DEVELOPMENT.md)** - Architecture and contribution guidelines
2. **README.md** - User documentation
3. **CLAUDE.md** - AI assistant quick reference (auto-generated)
4. **Code comments** - Godoc comments on exported functions
5. **Tests** - Comprehensive test suite showing usage examples

### External Resources

- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [Testify Assert](https://github.com/stretchr/testify)
- [Zerolog Logging](https://github.com/rs/zerolog)
- [Go Testing Guide](https://go.dev/doc/tutorial/add-a-test)

### Questions?

- Check existing code for similar patterns
- Look at test files for usage examples
- Read this DEVELOPMENT.md thoroughly
- Review CLAUDE.md for quick reference

---

**Last Updated**: 2026-02-11
**Version**: 0.40.2
