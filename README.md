# Dev Tools

A unified command runner for development workflows that provides consistent interfaces across different project types.

## Overview

Dev Tools automatically detects your project type (Python, Node.js, Rust, etc.) and provides sensible defaults for common development commands like `test`, `lint`, `build`, and `dev`. It can also be customized with project-specific configurations.

## Features

- **Smart Project Detection**: Automatically detects project type from files like `pyproject.toml`, `package.json`, `Cargo.toml`
- **Consistent Interface**: Same commands work across all project types
- **Intelligent Docker Service Management**: Automatically detects existing containers and restarts stopped ones
- **Advanced Daemon Support**: SHA1-based PID tracking prevents duplicate daemon instances
- **Enhanced Logging**: Detailed execution logs with command options and Docker commands
- **Dynamic Help System**: Context-aware help showing available commands from configuration
- **Container Naming**: Smart container naming from complex image paths (e.g., `registry.com/user/app` → `app`)
- **Environment Handling**: Automatically loads `.env` files
- **Parallel Execution**: Handle multiple commands and services efficiently
- **Activity Logging**: Comprehensive logging to `activity.log`

## Installation

### Prerequisites

- Python 3.12 or higher
- [uv](https://docs.astral.sh/uv/) package manager

### System Installation (Recommended)

Use the provided install script to install dev-tools system-wide:

```bash
git clone <repository-url>
cd dev-tools
./install.sh
```

This will install the tool using `uv tool install` and make the `dev-tools` command available globally.

### Development Installation

For development or if you prefer to run from source:

```bash
git clone <repository-url>
cd dev-tools
uv sync
```

## Usage

### Basic Commands

```bash
# After system installation with ./install.sh
dev-tools test
dev-tools dev
dev-tools lint
dev-tools build
dev-tools logs
dev-tools --verbose test

# Or if running from source
uv run dev-tools.py test
uv run dev-tools.py dev
uv run dev-tools.py lint
uv run dev-tools.py build
uv run dev-tools.py logs
uv run dev-tools.py --verbose test
```

### Project Types & Default Commands

#### Python Projects
Detected by presence of `pyproject.toml` or `requirements.txt`:
- `test`: `uv run pytest tests/`
- `lint`: `uv run ruff check .`
- `build`: `uv build`

#### Node.js Projects
Detected by presence of `package.json`:
- `test`: `npm test`
- `lint`: `npm run lint`
- `build`: `npm run build`

#### Rust Projects
Detected by presence of `Cargo.toml`:
- `test`: `cargo test`
- `lint`: `cargo clippy`
- `build`: `cargo build`

## Configuration

Create a `.dev-config.yaml` file in your project root to customize commands with powerful service and execution management.

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
  # Full-featured test command with services and multiple steps
  test:
    - start_services:
        - database:
            image: postgres:15
            command: "postgres -c log_statement=all"
            volumes: ["./data:/var/lib/postgresql/data"]
            ports: ["5432:5432"]
        - cache:
            image: redis:alpine
            ports: ["6379:6379"]
        - worker:
            image: "myapp/worker"
            command: "python worker.py --dev"
            volumes: ["./app:/app"]
    - run: "uv run pytest tests/ -v --cov=app"
    - cleanup: true

  # Development server with background execution
  dev:
    - start_services: ["postgres", "redis"]  # Simple string format
    - run: "uvicorn app.main:app --reload --host 0.0.0.0"
      background: true
      daemon: true
      directory: "backend"

  # Multi-command linting with different directories
  lint:
    - run:
        - "uv run ruff check ."
        - "uv run ruff format ."
        - "uv run mypy app/"
      directory: "backend"
    - run: "npm run lint"
      directory: "frontend"

  # Build command with sequential steps
  build:
    - run: "npm run build"
      directory: "frontend"
    - run: "uv build"
      directory: "backend"
    - run: "docker build -t myapp ."

  # Custom deployment command
  deploy:
    - start_services:
        - deployment-db:
            image: postgres:15
            ports: ["5433:5432"]
    - run: "python scripts/migrate.py"
    - run: "python scripts/deploy.py --env=prod"
      background: true
```

### Available Configuration Options

#### Command Step Options

Each command step is a dictionary that can contain the following options:

##### `start_services` (Array)
Start Docker services before executing commands. Supports multiple formats:

**String Format (Simple):**
```yaml
start_services: ["redis", "postgres", "mysql"]
```

**Named Service Format (Advanced):**
```yaml
start_services:
  - database:
      image: postgres:15
      command: "postgres -c log_statement=all"  # Custom startup command
      volumes: ["./data:/var/lib/postgresql/data", "./config:/etc/postgresql"]
      ports: ["5432:5432", "5433:127.0.0.1:5432"]
  - cache:
      image: redis:alpine
      ports: ["6379:6379"]
  - worker:
      image: "myregistry.com/myapp/worker:latest"
      command: "python worker.py --dev"
      volumes: ["./app:/app", "./logs:/logs"]
      ports: ["8080:8080"]
```

**Service Configuration Fields:**
- `image` (required): Docker image name
- `command` (optional): Custom command to run in container
- `volumes` (optional): Array of volume mounts in format `["host:container"]`
- `ports` (optional): Array of port mappings in format `["host:container"]` or `["container:host_ip:host_port"]`

**Port Mapping Examples:**
- `"80:80"` → `-p 80:80`
- `"81:127.0.0.1:443"` → `-p 127.0.0.1:443:81`

##### `run` (String or Array)
Commands to execute. Can be a single command or multiple commands.

```yaml
# Single command
run: "uv run pytest tests/"

# Multiple commands (executed sequentially)
run:
  - "uv run ruff check ."
  - "uv run ruff format ."
  - "uv run pytest tests/"
```

##### `background` (Boolean)
Run the command in the background (non-blocking).

```yaml
run: "uvicorn app.main:app --reload"
background: true
```

##### `daemon` (Boolean)
Store PID file for single-instance processes. Prevents multiple instances of the same command.

```yaml
run: "uvicorn app.main:app --reload"
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

##### `cleanup` (Boolean)
Clean up services after command execution.

```yaml
- start_services: ["postgres", "redis"]
- run: "uv run pytest tests/"
- cleanup: true  # Stops and removes containers
```

### Docker Service Management

#### Intelligent Service Lifecycle

Services are automatically managed with smart container lifecycle handling:

1. **Container Detection**: Checks if container already exists
2. **State Management**:
   - **Doesn't exist**: Creates new container with `docker run`
   - **Exists but stopped**: Restarts with `docker start`
   - **Already running**: Skips (no action needed)
3. **Container Naming**: Uses service name as container name

#### Predefined Services

The following services have predefined configurations for convenience:

```yaml
# These are equivalent to the detailed configurations below
start_services: ["redis", "postgres", "mysql"]

# Predefined service configurations:
# redis → docker run -d --name redis -p 6379:6379 redis:latest
# postgres → docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=password postgres:latest  
# mysql → docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password mysql:latest
```

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

#### Microservices Development
```yaml
commands:
  dev:
    - start_services:
        - postgres:
            image: postgres:15
            ports: ["5432:5432"]
        - redis:
            image: redis:alpine
            ports: ["6379:6379"]
        - rabbitmq:
            image: rabbitmq:3-management
            ports: ["5672:5672", "15672:15672"]
    - run: "uvicorn api.main:app --reload"
      background: true
      daemon: true
      directory: "services/api"
    - run: "python worker.py"
      background: true 
      daemon: true
      directory: "services/worker"
```

#### Testing with Service Dependencies
```yaml
commands:
  test:
    - start_services:
        - test-db:
            image: postgres:15
            command: "postgres -c fsync=off -c synchronous_commit=off"
            ports: ["5433:5432"]
        - test-redis:
            image: redis:alpine
            ports: ["6380:6379"]
    - run: "uv run pytest tests/ --maxfail=1 -v"
    - cleanup: true
```

#### Multi-Environment Build Pipeline
```yaml
commands:
  build-dev:
    - run: "npm run build:dev"
      directory: "frontend"
    - run: "uv build"
      directory: "backend"
    
  build-prod:
    - run: "npm run build:prod"
      directory: "frontend"
    - run: "uv build --wheel"
      directory: "backend"
    - run: "docker build -t myapp:latest ."
```

## Daemon & PID Management

Dev Tools provides advanced process management for long-running commands:

```yaml
commands:
  dev:
    - run: "uvicorn app.main:app --reload"
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

## Activity Logging

Dev Tools logs all its activity to help you monitor and debug your development workflow:

### Log Location
- **When running installed version**: Logs are written to `~/Library/Logs/dev-tools/activity.log`
- **When running from source**: Logs are written to `activity.log` in the current working directory

### Logging Features

Dev Tools provides comprehensive logging with detailed execution information:

- **Activity Logging**: All commands logged with timestamps
- **Command Options**: Logs show execution options (e.g., `background=True, daemon=True`)
- **Docker Commands**: Full Docker commands logged for debugging
- **Process Tracking**: PID information for background processes
- **Service Status**: Container lifecycle events and status changes

**Logging Output Examples:**
```
2025-06-13 12:34:03 - dev_tools.command_executor - INFO - Executing command: sleep 3600 (background=True, daemon=True)
2025-06-13 12:34:03 - dev_tools.command_executor - INFO - Started background process with PID 44447
2025-06-13 12:34:03 - dev_tools.command_executor - INFO - Creating new container: docker run -d --name redis -p 6379:6379 redis:latest
```

### Viewing Logs

- **`--verbose` flag**: Output logs to stdout in addition to the log file
- **`dev-tools logs`**: View the last 50 lines of activity from the log file
- **Manual access**:
  - Installed version: `~/Library/Logs/dev-tools/activity.log`
  - Source version: `./activity.log`

## Dynamic Help System

Dev Tools provides context-aware help that automatically discovers available commands:

```bash
dev-tools --help
```

**Features:**
- **Auto-discovery**: Reads `.dev-config.yaml` to show actual available commands
- **Dynamic examples**: Shows examples using your project's specific commands
- **Fallback support**: Shows default commands if configuration loading fails
- **Command listing**: Displays all available commands with descriptions

**Example Output:**
```
Available commands: test, lint, dev, build, logs, custom-command

Examples:
  uv run dev-tools.py test
  uv run dev-tools.py lint
  uv run dev-tools.py dev
  uv run dev-tools.py build
  uv run dev-tools.py --verbose test  # Run with verbose logging
```

## Development

### Setup Development Environment

```bash
# Clone and setup
git clone <repository-url>
cd dev-tools
uv sync

# Install development dependencies
uv sync --group dev
```

### Running Tests

The project includes comprehensive test coverage for all functionality:

```bash
# Run all tests (67+ tests)
uv run pytest

# Run with coverage
uv run pytest --cov=src

# Run specific test categories
uv run pytest -m unit
uv run pytest -m integration

# Test specific features
uv run pytest tests/test_command_executor.py::TestPidFilenameGeneration
uv run pytest tests/test_command_executor.py::TestImprovedDockerServiceManagement
uv run pytest tests/test_command_executor.py::TestDaemonImprovements
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
uv run ruff format .

# Lint code
uv run ruff check .

# Fix auto-fixable issues
uv run ruff check --fix .
```

## Project Structure

```
dev-tools/
├── src/dev_tools/           # Main package
│   ├── cli.py              # Command-line interface
│   ├── command_executor.py # Command execution engine
│   ├── config_parser.py    # Configuration management
│   └── logger_setup.py     # Logging configuration
├── tests/                  # Test suite
│   ├── test_cli.py
│   ├── test_command_executor.py
│   ├── test_config_parser.py
│   └── test_integration.py
├── dev-tools.py           # Main entry point
├── install.sh             # System installation script
├── pyproject.toml         # Project configuration
└── README.md             # This file
```

## Examples

### Python FastAPI Project

```yaml
# .dev-config.yaml
commands:
  test:
    - start_services: ["postgres", "redis"]
    - run: "uv run pytest tests/ -v --cov=app"

  dev:
    - start_services: ["postgres", "redis"]
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

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `uv run pytest`
5. Submit a pull request

## License

This project is licensed under the MIT License.
