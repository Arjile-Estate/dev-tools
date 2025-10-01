# Dev Tools - Development Guide

## Project Overview

This is a Go application that provides a unified command runner for development workflows across different project types (Go, Python, Node.js, Rust). It automatically detects project types and provides consistent interfaces while allowing extensive customization.

## Architecture

### Core Components

```
dev-tools/
├── main.go                    # Main entry point
├── cmd/                       # CLI commands and command handling
│   ├── root.go               # Root command, CLI setup, flag parsing
│   ├── helpers.go            # Helper functions for command execution
│   ├── logging.go            # Logging setup and management
│   └── completion_test.go    # Shell completion tests
├── internal/
│   ├── config/               # Configuration management
│   │   ├── config.go         # Config parsing, project detection
│   │   ├── types.go          # Config type definitions
│   │   └── interfaces.go     # Config interfaces
│   ├── executor/             # Command execution engine
│   │   ├── executor.go       # Core execution logic
│   │   ├── types.go          # Executor types
│   │   └── interfaces.go     # Executor interfaces
│   ├── commands/             # Command implementations
│   │   ├── built_in.go       # Built-in commands (status, logs, cleanup, etc.)
│   │   └── completion.go     # Shell completion logic
│   ├── colors/               # Terminal color utilities
│   └── errors/               # Error handling utilities
└── .dev-config.yaml          # Project configuration example
```

### Configuration System

Reads a `.dev-config.yaml` from the current directory (or specified with `--project-dir`) and maps commands to actions. Supports both modern `services` and legacy `start_services` configurations.

#### Modern Configuration Example:
```yaml
commands:
  test:
    - services:
        containers:
          - postgres:
              image: postgres:15
              environment:
                POSTGRES_PASSWORD: "password"
                POSTGRES_DB: "testdb"
              ports: ["5432:5432"]
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "30s"
        wait_for_health: true
        cleanup: true
    - run: "go test ./... -v"

  dev:
    - services:
        compose:
          file: "docker-compose.yml"
          services: ["redis", "postgres"]
          profiles: ["dev"]
        wait_for_health: true
    - run: "go run main.go"
      background: true
      daemon: true

  lint:
    - run:
        - "golangci-lint run"
        - "go fmt ./..."

  build:
    - run: "go build -o app ."
```

## Available Command Options

### Service Configuration (Modern - Recommended)
- `services` - Object containing service definitions:
  - `compose` - Docker Compose configuration:
    - `file` - Compose file path
    - `services` - Array of specific services to start
    - `profiles` - Array of compose profiles to use
  - `containers` - Array of container definitions (string names or objects with full config)
  - `wait_for_health` - Boolean to wait for health checks (default: true)
  - `cleanup` - Boolean to clean up services after command (default: false)
  - `timeout` - Timeout in seconds for service startup (default: 30)

### Legacy Options (Deprecated)
- `start_services` - Array of services to start with docker (shows deprecation warning)

### Execution Options
- `run` - String or array of shell commands to execute
- `background` - Boolean to run command in background (non-blocking)
- `daemon` - Boolean to enable PID tracking for single-instance processes (uses SHA1-based PID files)
- `directory` - String specifying working directory (relative or absolute path)

### Passthrough Arguments
Commands support passthrough arguments using `--` separator:
```bash
dev-tools test -- --verbose --timeout=30s
dev-tools build -- -ldflags="-X main.version=1.0.0"
```

## Key Features

1. **Smart Project Detection**: Automatically detects project type from `go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`
2. **Docker Compose Support**: Full integration with compose files, profiles, and service selection
3. **Advanced Service Management**:
   - Health checks with automatic waiting
   - Intelligent container lifecycle (start/restart/reuse)
   - Service cleanup after execution
   - Resource limits (memory, CPU)
4. **Daemon Management**: SHA1-based PID tracking prevents duplicate daemon instances
5. **Enhanced Logging**: Comprehensive activity logging to `activity.log` with verbose mode
6. **Shell Completion**: Dynamic command completion for bash, zsh, and fish
7. **Environment Handling**: Automatically loads `.env` files
8. **Built-in Commands**: logs, status, cleanup-pids, cleanup-all, restart, stop, version
9. **Single Binary**: No runtime dependencies required

## Built-in Commands

- `dev-tools logs` - View last 50 lines of activity.log
- `dev-tools status` - Show running daemon processes
- `dev-tools restart <daemon-name>` - Restart a specific daemon
- `dev-tools stop <daemon-name>` - Stop a specific daemon
- `dev-tools cleanup-pids` - Clean up stale PID files
- `dev-tools cleanup-all` - Clean up all daemon processes
- `dev-tools version` - Show version information
- `dev-tools completion <shell>` - Generate shell completion script

## Command Line Flags

- `--verbose` or `-v` - Enable verbose logging to stdout
- `--project-dir` or `-p` - Specify project directory (default: current directory)
- `--no-color` - Disable colored output
- `--version` - Show version information
- `--help` or `-h` - Show help message

## Development Workflow

### Running Commands
```bash
# From project root
dev-tools test
dev-tools lint
dev-tools build

# From subdirectory or different location
dev-tools --project-dir /path/to/project test

# With verbose output
dev-tools --verbose dev

# With passthrough arguments
dev-tools test -- --run TestSpecific --count=3
```

### Ways of Working: Test Driven Development
* See @~/.claude/go-snippet.md
* Always write tests before implementation
* Run `dev-tools test` to verify all tests pass
* Run `dev-tools lint` after any code changes
* Use `dev-tools test:coverage` to check coverage
