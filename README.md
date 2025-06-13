# Dev Tools

A unified command runner for development workflows that provides consistent interfaces across different project types.

## Overview

Dev Tools automatically detects your project type (Python, Node.js, Rust, etc.) and provides sensible defaults for common development commands like `test`, `lint`, `build`, and `dev`. It can also be customized with project-specific configurations.

## Features

- **Smart Project Detection**: Automatically detects project type from files like `pyproject.toml`, `package.json`, `Cargo.toml`
- **Consistent Interface**: Same commands work across all project types
- **Service Management**: Start/stop Docker services as prerequisites
- **Background Process Support**: Run development servers in the background with PID tracking
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
- `dev`: `npm run dev`

#### Rust Projects
Detected by presence of `Cargo.toml`:
- `test`: `cargo test`
- `lint`: `cargo clippy`
- `build`: `cargo build`

## Configuration

Create a `.dev-config.yaml` file in your project root to customize commands:

```yaml
commands:
  test:
    - start_services: ["redis", "postgres"]
    - run: "uv run pytest tests/ -v"
    - cleanup: true
  
  lint:
    - run: 
        - "uv run ruff check ."
        - "uv run ruff format --check ."
  
  dev:
    - start_services: ["redis"]
    - run: "uvicorn app.main:app --reload"
    - background: true
    - daemon: true
    
  custom-command:
    - run: "echo 'Custom command executed'"
```

### Configuration Options

Each command step supports:

- **`start_services`**: Array of Docker services to start
- **`run`**: Command(s) to execute (string or array)
- **`background`**: Run command in background (boolean)
- **`daemon`**: Store PID file for single-instance processes (boolean)
- **`cleanup`**: Clean up services after execution (boolean)

### Docker Service Management

Services are managed using Docker containers:

```yaml
commands:
  test:
    - start_services: ["redis", "postgres"]
    - run: "pytest tests/"
```

This will:
1. Start Redis container: `docker run -d --name redis redis:alpine`
2. Start PostgreSQL container: `docker run -d --name postgres postgres:alpine`
3. Execute the test command
4. Services continue running for subsequent commands

## Environment Variables

Place a `.env` file in your project root to automatically load environment variables:

```env
DATABASE_URL=postgresql://localhost/mydb
REDIS_URL=redis://localhost:6379
DEBUG=true
```

## Background Processes & PID Management

For development servers and long-running processes:

```yaml
commands:
  dev:
    - run: "uvicorn app.main:app --reload"
    - background: true
    - daemon: true  # Creates .uvicorn_app_main_app___reload.pid
```

This ensures only one instance runs at a time and allows for proper process management.

## Logging

- All activity is logged to `activity.log` in the project directory
- Use `--verbose` flag to also output logs to stdout
- Use `dev-tools logs` to view recent activity

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

```bash
# Run all tests
uv run pytest

# Run with coverage
uv run pytest --cov=src

# Run specific test categories
uv run pytest -m unit
uv run pytest -m integration
```

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
    - background: true
    - daemon: true
  
  lint:
    - run:
        - "uv run ruff check ."
        - "uv run ruff format --check ."
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
    - background: true
    
  build:
    - run: 
        - "npm run build"
        - "npm run build:analyze"
  
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