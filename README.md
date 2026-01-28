# Dev Tools

A unified command runner for development workflows that provides consistent interfaces across different project types.

## Overview

Dev Tools automatically detects your project type (Go, Python, Node.js, Rust, etc.) and provides sensible defaults for common development commands like `test`, `lint`, `build`, and `dev`. It can also be customized with project-specific configurations.

## Features

- **Smart Project Detection**: Automatically detects project type (Go, Python, Node.js, Rust, Java, .NET, PHP, Ruby)
- **Consistent Interface**: Same commands work across all project types
- **YAML Schema Validation**: Built-in configuration validation with detailed error messages
- **Retry Logic**: Configurable retry with delays and exit code filtering for transient failures
- **Watch Mode**: Automatic command re-execution on file changes with debouncing
- **Advanced Service Management**: Full Docker Compose and Docker container support with health checks
- **Intelligent Docker Service Management**: Automatically detects existing containers and restarts stopped ones
- **Advanced Daemon Support**: SHA1-based PID tracking prevents duplicate daemon instances
- **Structured Logging**: High-performance JSON logging with zerolog for easy parsing
- **JSON Output**: Machine-readable output (`--format json`) for automation and monitoring
- **Security Hardening**: Shell injection prevention, POSIX escaping, thread-safe operations
- **Dynamic Help System**: Context-aware help showing available commands from configuration
- **Shell Completion**: Bash, Zsh, and Fish completion with dynamic command discovery
- **Container Naming**: Smart container naming from complex image paths (e.g., `registry.com/user/app` → `app`)
- **Environment Handling**: Automatically loads `.env` files
- **Parallel Execution**: Handle multiple commands and services efficiently
- **Activity Logging**: Comprehensive logging to `activity.log`
- **Single Binary**: No runtime dependencies or virtual environments needed

## Installation

### Prerequisites

- Go 1.21 or higher

### Build from Source

```bash
git clone <repository-url>
cd dev-tools
go mod tidy
go build -o dev-tools .
```

### Install Globally

```bash
# Copy binary to your PATH
sudo cp dev-tools /usr/local/bin/
```

## Usage

### Basic Commands

```bash
# Run tests for any project type
dev-tools test

# Start development server
dev-tools dev

# Run linting
dev-tools lint

# Build project
dev-tools build

# View recent logs
dev-tools logs

# Clean up stale daemon processes
dev-tools cleanup-pids

# Clean up all daemon processes (including running ones)
dev-tools cleanup-all

# View daemon status
dev-tools status

# View status in JSON format (machine-readable)
dev-tools status --format json

# Restart a specific daemon
dev-tools restart <daemon-name>

# Stop a specific daemon
dev-tools stop <daemon-name>

# Validate configuration file
dev-tools validate

# Run with verbose logging
dev-tools --verbose test

# Use in different directory
dev-tools --project-dir /path/to/project test
```

### Passthrough Arguments

You can pass additional arguments to the underlying commands using the `--` separator:

```bash
# Pass test flags to the underlying test command
dev-tools test -- --verbose --timeout=30s

# Pass build flags
dev-tools build -- -ldflags="-X main.version=1.0.0"

# Pass multiple arguments
dev-tools test -- --run TestSpecific --count=3 --parallel=4

# Works with any custom command
dev-tools custom-command -- --extra --flags
```

The arguments after `--` are appended to each `run` command defined in your configuration. For example, if your `.dev-config.yaml` defines:

```yaml
commands:
  test:
    - run: "go test ./..."
```

Then `dev-tools test -- --verbose` becomes `go test ./... --verbose`.

## Shell Completion

Dev Tools supports automatic command completion for popular shells, making it faster and easier to use commands.

### Installation

#### Bash

**Option 1: System-wide installation (recommended)**
```bash
# Generate and install completion script
dev-tools completion bash | sudo tee /etc/bash_completion.d/dev-tools

# Restart your shell or source the file
source /etc/bash_completion.d/dev-tools
```

**Option 2: User-specific installation**
```bash
# Generate and save completion script
dev-tools completion bash > ~/.dev-tools-completion.bash

# Add to your .bashrc or .bash_profile
echo 'source ~/.dev-tools-completion.bash' >> ~/.bashrc
source ~/.bashrc
```

#### Zsh

**Option 1: Using fpath (recommended)**
```bash
# Create completions directory if it doesn't exist
mkdir -p ~/.zsh/completions

# Generate completion script
dev-tools completion zsh > ~/.zsh/completions/_dev-tools

# Add to your .zshrc if not already present
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc

# Restart your shell
exec zsh
```

**Option 2: Direct sourcing**
```bash
# Generate and source completion
dev-tools completion zsh > ~/.dev-tools-completion.zsh
echo 'source ~/.dev-tools-completion.zsh' >> ~/.zshrc
source ~/.zshrc
```

#### Fish

```bash
# Create fish completions directory if it doesn't exist
mkdir -p ~/.config/fish/completions

# Generate completion script
dev-tools completion fish > ~/.config/fish/completions/dev-tools.fish

# Restart your fish shell or reload completions
exec fish
```

### Features

Shell completion provides intelligent suggestions for:

- **Commands**: All available commands from both built-in and your `.dev-config.yaml`
- **Daemon Names**: For `restart` and `stop` commands, shows currently running daemons
- **Flags**: Global flags like `--verbose`, `--project-dir`, `--no-color`
- **Shell Types**: For `completion` command, suggests `bash`, `zsh`, `fish`

### Dynamic Command Discovery

The completion system automatically discovers commands from:

1. **Built-in commands**: `logs`, `status`, `validate`, `version`, `completion`, etc.
2. **Project defaults**: Based on detected project type (Go, Python, Node.js, Rust)
3. **Custom commands**: From your project's `.dev-config.yaml`

### Examples

```bash
# Complete available commands
dev-tools <TAB>
# Shows: build test lint dev logs status version completion ...

# Complete daemon names for restart
dev-tools restart <TAB>  
# Shows: web-server worker api-daemon ...

# Complete flags
dev-tools --<TAB>
# Shows: --verbose --project-dir --no-color --version

# Complete partial commands
dev-tools cus<TAB>
# Shows: custom-build custom-test (if defined in config)
```

### Troubleshooting

**Completion not working:**
1. Verify the completion script is properly installed
2. Restart your shell completely
3. Check that dev-tools is in your PATH
4. Test with `dev-tools completion bash` to ensure it generates output

**Completions not updating:**
- Completions are cached for 5 seconds for performance
- They automatically update when you change directories or modify `.dev-config.yaml`

### Project Types & Default Commands

#### Go Projects
Detected by presence of `go.mod`:
- `test`: `go test ./...`
- `lint`: `golangci-lint run`
- `build`: `go build ./...`

#### Python Projects
Detected by presence of `pyproject.toml` or `requirements.txt`:
- `test`: `uv run pytest tests/`
- `lint`: `uv run ruff check .` and `uv run black .`

#### Node.js Projects
Detected by presence of `package.json`:
- `test`: `npm test`
- `lint`: `npm run lint`
- `build`: `npm run build`

#### Rust Projects
Detected by presence of `Cargo.toml`:
- `test`: `cargo test`
- `lint`: `cargo clippy`
- `dev`: `cargo run`
- `build`: `cargo build`

#### Java/Maven Projects
Detected by presence of `pom.xml`:
- `test`: `mvn test`
- `build`: `mvn package`

#### .NET Projects
Detected by presence of `*.csproj` or `*.sln`:
- `test`: `dotnet test`
- `build`: `dotnet build`

#### PHP Projects
Detected by presence of `composer.json`:
- `test`: `composer test`

#### Ruby Projects
Detected by presence of `Gemfile`:
- `test`: `bundle exec rspec`

## Configuration

Create a `.dev-config.yaml` file in your project root to customize commands with powerful service and execution management.

### Configuration Validation

Dev Tools includes built-in YAML schema validation to help catch configuration errors early:

**Automatic Validation:**
- Configuration is automatically validated when loaded
- Invalid configurations are rejected with detailed error messages
- Prevents runtime errors from malformed configs

**Manual Validation:**
```bash
# Validate your configuration file
dev-tools validate

# Example output on success:
✓ Configuration is valid
  Config file: /path/to/project/.dev-config.yaml

# Example output on error:
✗ Configuration validation failed
- commands.test[0].run: Invalid type. Expected: string, given: integer
```

**Validation Rules:**
- At least one command must be defined
- `run` commands must be strings or arrays of strings
- Service configurations must have required fields (e.g., `compose.file`)
- Retry delays and timeouts must match duration patterns (e.g., "5s", "1m")
- Exit codes in `retry_on_exit_codes` must be integers

**Benefits:**
- Catch typos and structural errors before execution
- Clear error messages pinpointing issues
- Schema-based validation ensures consistency
- IDE support with JSON schema integration (coming soon)

### YAML Configuration Format

The configuration file uses a hierarchical structure where each command consists of multiple steps that can start services, execute commands, and control execution flow.

#### Basic Structure

```yaml
commands:
  command-name:
    - step-option: value
    - step-option: value
    # ... additional steps
```

#### Complete Example

Here's a comprehensive example showcasing all available options:

```yaml
commands:
  # Full-featured test command with modern services configuration
  test:
    - services:
        containers:
          - database:
              image: postgres:15
              command: "postgres -c log_statement=all"
              environment:
                POSTGRES_PASSWORD: "password"
                POSTGRES_DB: "testdb"
              volumes: ["./data:/var/lib/postgresql/data"]
              ports: ["5432:5432"]
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "30s"
          - cache:
              image: redis:alpine
              ports: ["6379:6379"]
          - worker:
              image: "myapp/worker"
              command: "python worker.py --dev"
              volumes: ["./app:/app"]
              environment:
                WORKER_MODE: "dev"
        wait_for_health: true
        timeout: 45
    - run: "go test ./... -v"

  # Development server with Docker Compose
  dev:
    - services:
        compose:
          file: "docker-compose.dev.yml"
          services: ["postgres", "redis"]
          profiles: ["dev"]
        wait_for_health: true
    - run: "go run main.go"
      background: true
      daemon: true
      directory: "backend"

  # Multi-command linting with different directories
  lint:
    - run:
        - "golangci-lint run"
        - "go fmt ./..."
        - "go vet ./..."
      directory: "backend"
    - run: "npm run lint"
      directory: "frontend"

  # Build command with sequential steps
  build:
    - run: "npm run build"
      directory: "frontend"
    - run: "go build -o app ."
      directory: "backend"
    - run: "docker build -t myapp ."

  # Custom deployment command with service cleanup
  deploy:
    - services:
        containers:
          - deployment-db:
              image: postgres:15
              ports: ["5433:5432"]
              environment:
                POSTGRES_PASSWORD: "deploy_password"
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "10s"
        cleanup: true  # Clean up services after deployment
        wait_for_health: true
    - run: "go run scripts/migrate.go"
    - run: "go run scripts/deploy.go --env=prod"
      background: true
```

### Available Configuration Options

#### Command Step Options

Each command step is a dictionary that can contain the following options:

##### `services` (Object) - **Modern Service Management**
The recommended way to manage Docker services with comprehensive features including Docker Compose support, health checks, and cleanup management.

**Full Services Configuration:**
```yaml
services:
  # Docker Compose support
  compose:
    file: "docker-compose.yml"           # Path to compose file
    services: ["redis", "postgres"]      # Optional: specific services
    profiles: ["dev", "testing"]         # Optional: compose profiles

  # Individual Docker containers
  containers:
    - "redis"                           # Simple predefined service
    - database:                         # Advanced container config
        image: "postgres:15"
        environment:
          POSTGRES_PASSWORD: "password"
          POSTGRES_DB: "myapp"
        ports: ["5432:5432"]
        volumes: ["./data:/var/lib/postgresql/data"]
        networks: ["myapp-network"]
        restart: "unless-stopped"
        memory: "512m"
        cpus: "0.5"
        healthcheck:
          test: "pg_isready -U postgres"
          interval: "30s"
          timeout: "10s"
          retries: "3"

  # Service management options
  cleanup: false                        # Default: false - keep services running
  wait_for_health: true                 # Default: true - wait for health checks
  timeout: 30                          # Default: 30 seconds startup timeout
```

**Docker Compose Examples:**
```yaml
services:
  compose:
    file: "docker-compose.yml"
    services: ["api", "database", "cache"]
    profiles: ["dev"]
  wait_for_health: true
  timeout: 60
```

**Container Configuration Fields:**
- `image` (required): Docker image name
- `environment` (optional): Environment variables as key-value pairs
- `command` (optional): Custom command to run in container
- `volumes` (optional): Array of volume mounts `["host:container"]`
- `ports` (optional): Array of port mappings `["host:container"]`
- `networks` (optional): Array of networks to connect to
- `restart` (optional): Restart policy (`no`, `always`, `unless-stopped`, `on-failure`)
- `memory` (optional): Memory limit (e.g., `512m`, `1g`)
- `cpus` (optional): CPU limit (e.g., `0.5`, `2.0`)
- `healthcheck` (optional): Health check configuration

**Health Check Configuration:**
```yaml
healthcheck:
  test: "curl -f http://localhost:8080/health"  # Health check command
  interval: "30s"                              # Check interval
  timeout: "10s"                               # Command timeout
  retries: "3"                                 # Retry attempts
```

##### `start_services` (Array) - **Legacy (Deprecated)**
⚠️ **DEPRECATED**: Use `services` configuration instead. This option will be removed in a future version.

**String Format (Simple):**
```yaml
start_services: ["redis", "postgres", "mysql"]
```

**Named Service Format (Advanced):**
```yaml
start_services:
  - database:
      image: postgres:15
      command: "postgres -c log_statement=all"
      volumes: ["./data:/var/lib/postgresql/data"]
      ports: ["5432:5432"]
```

##### `run` (String or Array)
Commands to execute. Can be a single command or multiple commands.

```yaml
# Single command
run: "go test ./..."

# Multiple commands (executed sequentially)
run:
  - "golangci-lint run"
  - "go fmt ./..."
  - "go test ./..."
```

##### `background` (Boolean)
Run the command in the background (non-blocking).

```yaml
run: "go run main.go"
background: true
```

##### `daemon` (Boolean)
Store PID file for single-instance processes. Prevents multiple instances of the same command.

```yaml
run: "go run main.go"
background: true
daemon: true  # Creates SHA1-based PID file (e.g., .a1b2c3d4.pid)
```

##### `directory` (String)
Working directory for command execution. Can be absolute or relative path.

```yaml
run: "npm run build"
directory: "frontend"  # Relative to project root

# Or absolute path
run: "make build"
directory: "/opt/myproject/backend"
```

**Note:** PID files are always stored in the project root regardless of the directory option.

##### `retry` (Integer)
Number of times to retry a command if it fails. Useful for handling transient failures.

```yaml
run: "./flaky-script.sh"
retry: 3  # Retry up to 3 times on failure
```

##### `retry_delay` (String)
Delay between retry attempts. Supports time units: `ms` (milliseconds), `s` (seconds), `m` (minutes), `h` (hours).

```yaml
run: "curl https://api.example.com/health"
retry: 5
retry_delay: "10s"  # Wait 10 seconds between retries
```

##### `retry_on_exit_codes` (Array of Integers)
Only retry if the command exits with specific exit codes. Useful for retrying only on certain failures.

```yaml
run: "npm install"
retry: 3
retry_delay: "5s"
retry_on_exit_codes: [1, 2]  # Only retry on exit codes 1 and 2
```

**Retry Example:**
```yaml
commands:
  flaky-test:
    - run: "go test -race ./..."
      retry: 3
      retry_delay: "2s"
      retry_on_exit_codes: [1]  # Retry only on test failures, not on compilation errors

  deploy:
    - run: "./deploy.sh"
      retry: 5
      retry_delay: "30s"  # Retry with longer delays for deployment
```

##### `watch` (Object)
Enable file watching mode that automatically re-runs the command when files change. Perfect for development workflows.

**Watch Configuration:**
```yaml
watch:
  patterns: ["**/*.go", "**/*.ts"]     # Glob patterns for files to watch
  debounce: "300ms"                     # Debounce delay (ms, s, m)
  ignore: ["**/node_modules/**", "**/dist/**"]  # Patterns to ignore
```

**Watch Example:**
```yaml
commands:
  test-watch:
    - run: "go test ./..."
      watch:
        patterns: ["**/*.go"]
        debounce: "500ms"
        ignore: ["**/*_test.go", "**/vendor/**"]

  dev-watch:
    - run: "npm run build"
      watch:
        patterns: ["src/**/*.ts", "src/**/*.tsx"]
        debounce: "300ms"
        ignore: ["**/node_modules/**", "**/dist/**"]
```

**Watch Mode Features:**
- **Automatic re-execution**: Commands run automatically when matching files change
- **Debouncing**: Prevents rapid re-runs during multiple file changes
- **Glob pattern support**: Flexible file matching with `**`, `*`, `?`
- **Ignore patterns**: Exclude directories like node_modules, vendor, build outputs
- **Real-time feedback**: See test results immediately as you save files

### Advanced Service Management

#### Docker Compose Integration

Dev Tools provides comprehensive Docker Compose support:

**Features:**
- **Multi-format support**: Automatically detects and uses `docker compose` (new) or `docker-compose` (legacy)
- **Service selection**: Start specific services from compose file
- **Profile support**: Use Docker Compose profiles for different environments
- **Health monitoring**: Wait for services to be healthy before proceeding

**Example Docker Compose configuration:**
```yaml
services:
  compose:
    file: "docker-compose.yml"
    services: ["api", "database", "cache"]  # Optional: specific services
    profiles: ["dev", "testing"]           # Optional: compose profiles
  wait_for_health: true
  timeout: 60
```

#### Enhanced Container Management

Individual containers support extensive configuration options:

**Resource Management:**
- **Memory limits**: Control container memory usage
- **CPU limits**: Set CPU allocation
- **Restart policies**: Configure container restart behavior

**Networking:**
- **Custom networks**: Connect containers to specific networks
- **Port mapping**: Flexible port configuration
- **Environment variables**: Set container environment

**Health Monitoring:**
- **Health checks**: Configure container health validation
- **Startup timeout**: Control service startup time
- **Health validation**: Wait for services to be ready

#### Intelligent Service Lifecycle

Services are automatically managed with smart container lifecycle handling:

1. **Container Detection**: Checks if container already exists
2. **State Management**:
   - **Doesn't exist**: Creates new container with `docker run`
   - **Exists but stopped**: Restarts with `docker start`
   - **Already running**: Skips (no action needed)
3. **Container Naming**: Uses service name as container name
4. **Health Validation**: Waits for services to be healthy (if enabled)
5. **Cleanup Management**: Optional service cleanup after command completion

#### Predefined Services

The following services have predefined configurations for convenience:

```yaml
# These are equivalent to the detailed configurations below
start_services: ["redis", "postgres", "mysql"]

# Predefined service configurations:
# redis → docker run -d --name redis -p 6379:6379 redis:latest
# postgres → docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=<env or default> postgres:latest
# mysql → docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=<env or default> mysql:latest
```

**Security: Environment Variable Support**

⚠️ **IMPORTANT**: The predefined `postgres` and `mysql` services use default passwords (`password`) for development convenience.

**Always set environment variables in production:**

```bash
# Set before running dev-tools
export POSTGRES_PASSWORD="your-secure-password"
export MYSQL_ROOT_PASSWORD="your-secure-password"

# Then run your command
dev-tools dev
```

When using default passwords, dev-tools will log a **warning** to remind you to set environment variables for production use.

#### Custom Service Examples

**Simple Custom Service:**
```yaml
start_services: ["myuser/myapp"]  # Uses myuser/myapp:latest, container name: myapp
```

**Advanced Custom Service:**
```yaml
start_services:
  - api:
      image: "myregistry.com/api:v1.2.3"
      command: "gunicorn app:app --workers 4"
      volumes: ["./app:/app", "./logs:/var/log/app"]
      ports: ["8000:8000", "9000:127.0.0.1:9090"]
```

### Configuration Examples by Use Case

#### Test-Driven Development with Watch Mode
```yaml
commands:
  test-watch:
    - run: "go test ./... -v"
      watch:
        patterns: ["**/*.go"]
        debounce: "500ms"
        ignore: ["**/vendor/**", "**/.git/**"]

  lint-watch:
    - run:
        - "golangci-lint run"
        - "go fmt ./..."
      watch:
        patterns: ["**/*.go"]
        debounce: "300ms"
        ignore: ["**/vendor/**"]
```

#### Handling Flaky Tests with Retry Logic
```yaml
commands:
  test:
    - run: "go test -race ./..."
      retry: 3
      retry_delay: "2s"
      retry_on_exit_codes: [1]  # Only retry test failures

  integration-test:
    - services:
        containers: ["postgres", "redis"]
        wait_for_health: true
    - run: "go test ./integration/..."
      retry: 5
      retry_delay: "10s"  # Network issues may need longer delays

  e2e-test:
    - run: "npm run test:e2e"
      directory: "frontend"
      retry: 3
      retry_delay: "5s"
      retry_on_exit_codes: [1, 2]  # Retry on specific failures
```

#### Microservices Development with Docker Compose
```yaml
commands:
  dev:
    - services:
        compose:
          file: "docker-compose.dev.yml"
          services: ["postgres", "redis", "rabbitmq"]
          profiles: ["dev"]
        wait_for_health: true
        timeout: 60
    - run: "go run cmd/api/main.go"
      background: true
      daemon: true
      directory: "services/api"
    - run: "go run cmd/worker/main.go"
      background: true
      daemon: true
      directory: "services/worker"
```

#### Testing with Service Dependencies and Health Checks
```yaml
commands:
  test:
    - services:
        containers:
          - test-db:
              image: postgres:15
              command: "postgres -c fsync=off -c synchronous_commit=off"
              environment:
                POSTGRES_PASSWORD: "test_password"
                POSTGRES_DB: "testdb"
              ports: ["5433:5432"]
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "5s"
                timeout: "3s"
                retries: "5"
          - test-redis:
              image: redis:alpine
              ports: ["6380:6379"]
              healthcheck:
                test: "redis-cli ping"
                interval: "5s"
        wait_for_health: true
        cleanup: true  # Clean up test services after testing
        timeout: 30
    - run: "go test ./... -v"
```

#### Multi-Environment Build Pipeline
```yaml
commands:
  build-dev:
    - run: "npm run build:dev"
      directory: "frontend"
    - run: "go build -tags dev -o app-dev ."
      directory: "backend"

  build-prod:
    - run: "npm run build:prod"
      directory: "frontend"
    - run: "go build -ldflags='-s -w' -o app ."
      directory: "backend"
    - run: "docker build -t myapp:latest ."
```

## Daemon & PID Management

Dev Tools provides advanced process management for long-running commands:

```yaml
commands:
  dev:
    - run: "go run main.go"
      background: true
      daemon: true
```

**Features:**
- **SHA1-based PID files**: Uses 8-character SHA1 hash of command (e.g., `.392a9a8c.pid`)
- **Duplicate prevention**: Prevents multiple instances of the same daemon command
- **Stale cleanup**: Automatically removes PID files for stopped processes
- **Process tracking**: Monitors daemon status and provides clear error messages

**Example behavior:**
```bash
dev-tools dev          # Starts daemon, creates PID file
dev-tools dev          # Error: "Daemon process already running with PID 12345"
# (kill process manually)
dev-tools dev          # Cleans stale PID file, starts new daemon
```

## Environment Variables

Place a `.env` file in your project root to automatically load environment variables:

```env
DATABASE_URL=postgresql://localhost/mydb
REDIS_URL=redis://localhost:6379
DEBUG=true
```

## Security Features

Dev Tools includes multiple security hardening measures to protect against common vulnerabilities:

### Shell Injection Prevention

**POSIX Argument Escaping**: All command arguments are properly escaped using POSIX shell quoting rules to prevent shell injection attacks.

```yaml
# Safe: Arguments are properly escaped
commands:
  deploy:
    - run: "deploy.sh --target=${TARGET}"  # ${TARGET} is safely escaped
```

**Command Execution**: Commands are executed with proper argument separation, preventing shell metacharacter injection.

### Thread-Safe Operations

**Atomic Operations**: Color output and logging use atomic operations to prevent race conditions in concurrent environments.

### Secure Defaults

- **No Shell Interpolation**: Commands are executed directly without shell interpretation unless explicitly required
- **PID File Integrity**: Daemon PID files use SHA1 hashing to prevent name collisions and ensure uniqueness
- **Environment Isolation**: Each command runs in its own process with controlled environment variables
- **Error Handling**: Comprehensive error handling prevents crashes and ensures proper cleanup

### Docker Security

**Password Management**:
- Default development passwords trigger warnings
- Support for environment variables to override defaults
- Never hardcode production credentials in configuration files

```bash
# Secure way to use Docker services
export POSTGRES_PASSWORD="your-secure-password"
export MYSQL_ROOT_PASSWORD="your-secure-password"
dev-tools dev
```

## Activity Logging

Dev Tools logs all its activity to help you monitor and debug your development workflow.

### Logging Features

Dev Tools provides comprehensive logging with detailed execution information:

- **Activity Logging**: All commands logged with timestamps
- **Command Options**: Logs show execution options (e.g., `background=True, daemon=True`)
- **Docker Commands**: Full Docker commands logged for debugging
- **Process Tracking**: PID information for background processes
- **Service Status**: Container lifecycle events and status changes

### Viewing Logs

- **`--verbose` flag**: Output logs to stdout in addition to the log file
- **`dev-tools logs`**: View the last 50 lines of activity from the log file
- **Manual access**: `./activity.log` in the project directory

### Structured Logging

Dev Tools uses **zerolog** for high-performance structured logging:

- **JSON Format**: Logs are structured as JSON for easy parsing and analysis
- **Context-Rich**: Each log entry includes timestamps, levels, and contextual data
- **Performance**: Minimal overhead with zero-allocation JSON encoding
- **Filtering**: Easy to filter and search with tools like `jq`

**Example log entry:**
```json
{"level":"info","time":"2024-01-15T10:30:45Z","message":"Executing command","command":"test","step":1}
```

**Parse logs with jq:**
```bash
# View only error logs
cat activity.log | jq 'select(.level=="error")'

# View logs for specific command
cat activity.log | jq 'select(.command=="test")'

# Extract all executed commands
cat activity.log | jq -r 'select(.message=="Executing command") | .command'
```

## Machine-Readable Output

The `status` command supports JSON output for integration with automation tools:

```bash
# Get status in JSON format
dev-tools status --format json

# Example output
{
  "daemons": [
    {
      "command": "dev",
      "pid": 12345,
      "status": "running",
      "uptime": "2h15m30s"
    }
  ],
  "docker_services": [
    {
      "name": "postgres",
      "status": "running",
      "ports": ["5432:5432"],
      "image": "postgres:15"
    }
  ]
}
```

**Use cases:**
- **Monitoring**: Integrate with monitoring tools (Prometheus, Datadog)
- **CI/CD**: Validate service health in pipelines
- **Automation**: Script daemon management and health checks
- **Dashboards**: Build custom development environment dashboards

## Development

### Setup Development Environment

```bash
# Clone and setup
git clone <repository-url>
cd dev-tools
go mod tidy

# Install golangci-lint for linting
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest
```

### Running Tests

The project includes comprehensive test coverage for all functionality:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/config
go test ./internal/executor
go test ./cmd
```

**Test Coverage Includes:**
- SHA1 PID filename generation and collision testing
- Docker service lifecycle management (start/restart/already running)
- Container naming from complex image paths
- Daemon duplicate prevention and stale cleanup
- Enhanced logging with command options
- Service integration and failure handling
- CLI argument parsing and help generation
- Configuration loading and project detection

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Build binary
go build -o dev-tools .
```

## Project Structure

```
dev-tools/
├── main.go                    # Main entry point
├── go.mod                     # Go module definition
├── cmd/                       # CLI commands
│   ├── root.go               # Root command and CLI setup
│   └── root_test.go          # CLI tests
├── internal/                  # Internal packages
│   ├── config/               # Configuration management
│   │   ├── config.go         # Config parsing logic
│   │   └── config_test.go    # Config tests
│   └── executor/             # Command execution engine
│       ├── executor.go       # Execution logic
│       └── executor_test.go  # Execution tests
├── CLAUDE.md                 # Development guide
└── README.md                 # This file
```

## Examples

### Go REST API Project

```yaml
# .dev-config.yaml
commands:
  test:
    - services:
        compose:
          file: "docker-compose.test.yml"
          services: ["postgres", "redis"]
        wait_for_health: true
        cleanup: true
    - run: "go test ./... -v -cover"

  dev:
    - services:
        containers:
          - postgres:
              image: postgres:15
              environment:
                POSTGRES_PASSWORD: "dev_password"
                POSTGRES_DB: "myapp_dev"
              ports: ["5432:5432"]
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "30s"
          - redis:
              image: redis:alpine
              ports: ["6379:6379"]
              healthcheck:
                test: "redis-cli ping"
                interval: "30s"
        wait_for_health: true
    - run: "go run cmd/server/main.go"
      background: true
      daemon: true

  lint:
    - run:
        - "golangci-lint run"
        - "go fmt ./..."
        - "go vet ./..."

  build:
    - run: "go build -o app cmd/server/main.go"
```

### Python FastAPI Project

```yaml
# .dev-config.yaml
commands:
  test:
    - services:
        compose:
          file: "docker-compose.yml"
          services: ["postgres", "redis"]
          profiles: ["test"]
        wait_for_health: true
        cleanup: true
    - run: "uv run pytest tests/ -v --cov=app"

  dev:
    - services:
        containers:
          - postgres:
              image: postgres:15
              environment:
                POSTGRES_PASSWORD: "dev_password"
                POSTGRES_DB: "fastapi_dev"
              ports: ["5432:5432"]
              volumes: ["./data:/var/lib/postgresql/data"]
              healthcheck:
                test: "pg_isready -U postgres"
                interval: "30s"
          - redis:
              image: redis:alpine
              ports: ["6379:6379"]
        wait_for_health: true
    - run: "uvicorn app.main:app --reload --host 0.0.0.0 --port 8000"
      background: true
      daemon: true
      directory: "backend"

  lint:
    - run:
        - "uv run ruff check ."
        - "uv run ruff format ."
        - "uv run mypy app/"
```

### Node.js React Project

```yaml
# .dev-config.yaml
commands:
  test:
    - run: "npm run test:unit && npm run test:e2e"

  dev:
    - run: "npm run dev"
      background: true
      daemon: true
      directory: "frontend"

  build:
    - run:
        - "npm run build"
        - "npm run build:analyze"
      directory: "frontend"

  deploy:
    - run: "npm run build"
    - run: "npm run deploy:prod"
```

### Rust CLI Project

```yaml
# .dev-config.yaml
commands:
  test:
    - run: "cargo test -- --test-threads=1"

  dev:
    - run: "cargo run -- --dev"
      background: true
      daemon: true

  lint:
    - run:
        - "cargo clippy -- -D warnings"
        - "cargo fmt --check"

  build:
    - run: "cargo build --release"
```

## Migration Guide: `start_services` to `services`

### Why Migrate?

The new `services` configuration provides:
- **Docker Compose support**: Full integration with compose files
- **Health checks**: Wait for services to be ready before proceeding
- **Service cleanup**: Automatic cleanup after command completion
- **Enhanced container options**: Environment variables, resource limits, networking
- **Better error handling**: More robust service management

### Migration Examples

#### Simple Migration
```yaml
# Old (deprecated)
start_services: ["redis", "postgres"]

# New (recommended)
services:
  containers: ["redis", "postgres"]
  wait_for_health: true
```

#### Complex Migration
```yaml
# Old (deprecated)
start_services:
  - database:
      image: postgres:15
      volumes: ["./data:/var/lib/postgresql/data"]
      ports: ["5432:5432"]

# New (recommended)
services:
  containers:
    - database:
        image: postgres:15
        environment:
          POSTGRES_PASSWORD: "password"
          POSTGRES_DB: "myapp"
        volumes: ["./data:/var/lib/postgresql/data"]
        ports: ["5432:5432"]
        healthcheck:
          test: "pg_isready -U postgres"
          interval: "30s"
  wait_for_health: true
  timeout: 60
```

#### Docker Compose Migration
```yaml
# Old (not possible with start_services)
# Had to manage each service individually

# New (recommended)
services:
  compose:
    file: "docker-compose.yml"
    services: ["redis", "postgres", "nginx"]
    profiles: ["dev"]
  wait_for_health: true
  timeout: 45
```

### Migration Strategy

1. **Gradual migration**: Both `start_services` and `services` can coexist
2. **Test thoroughly**: Ensure services start correctly with new configuration
3. **Leverage new features**: Add health checks and environment variables
4. **Consider Docker Compose**: Migrate complex setups to compose files

### Backward Compatibility

The `start_services` configuration continues to work but shows deprecation warnings:
```
WARNING: 'start_services' is deprecated. Please migrate to 'services' configuration.
```

## Version History

### v0.35.0 - YAML Schema Validation
- Added JSON schema validation for `.dev-config.yaml` files
- New `dev-tools validate` command for explicit configuration validation
- Automatic validation on config load with detailed error messages
- Schema enforces correct types, required fields, and patterns

### v0.34.0 - Code Quality Improvements
- Extracted magic numbers to named constants
- Improved code maintainability and readability
- Better timeout and display width management

### v0.33.0 - Code Organization
- Split monolithic `built_in.go` into focused modules
- Separated commands: logs, cleanup, daemon, status, onboard
- Improved single-responsibility principle adherence

### v0.32.0 - Structured Logging
- Implemented zerolog for high-performance structured logging
- JSON-formatted logs for easy parsing and analysis
- Context-rich log entries with timestamps and metadata
- Zero-allocation JSON encoding for minimal overhead

### v0.31.0 - Error Handling Standardization
- Standardized error handling patterns across codebase
- Custom error types for better error context
- Improved error messages and debugging information
- Replaced `os.Exit()` with proper error returns

### v0.23.0 - Watch Mode & Context Support
- File watch mode with automatic command re-execution
- Debouncing to prevent rapid re-runs
- Glob pattern support for file matching
- Added `context.Context` support for command cancellation
- Graceful shutdown and cleanup on interrupts

### v0.22.0 - Security Hardening
- Eliminated shell injection vulnerabilities
- Implemented POSIX shell argument escaping
- Thread-safe color output with atomic operations
- Secure command execution without shell interpolation

### v0.21.0 - Code Refactoring
- Split `executor_shell.go` into focused modules
- Extracted helper functions for better modularity
- Improved test coverage and code organization

### v0.20.0 - Type Safety & Configuration
- Replaced `map[string]interface{}` with typed Docker config structs
- Type-safe configuration handling
- Better IDE support and compile-time checking

### v0.19.0 - Retry Logic
- Implemented configurable retry logic for transient failures
- Retry delays with configurable time units
- Selective retry based on exit codes
- Helpful for flaky tests and network operations

### v0.16.0 - Extended Project Support & JSON Output
- Added support for Java/Maven, .NET, PHP, and Ruby projects
- JSON output format for `status` command (`--format json`)
- Machine-readable output for automation and monitoring
- Enhanced status command with comprehensive system information

### Earlier Versions
- v0.15.0: Onboard command for AI assistant documentation
- v0.14.0: Shell completion for Bash, Zsh, and Fish
- v0.13.0: Docker Compose integration
- v0.12.0: Enhanced service management with health checks
- v0.11.0: Daemon process management with PID tracking

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Ensure linting passes: `golangci-lint run`
6. Submit a pull request

## License

This project is licensed under the MIT License.
