# Dev Tools - Development Guide

## Project Overview

This is a Go application that provides a series of convenient tools to make it easier for agentic systems to build, test and run software.

## Architecture

Reads a config file (`.dev-config.yaml`) from the current directory and maps commands to the appropriate actions for that repository. It comes with a series of defaults for Go, Python, Node.js, and Rust projects and can handle prerequisites like starting servers, setting up databases, etc.

Example of `.dev-config.yaml`:
```yaml
commands:
  test:
    - start_services: ["redis", "postgres"]
    - run: "uv run pytest tests/"
    - cleanup: true

  lint:
    - run: ["eslint src/", "black --check ."]

  dev:
    - start_services: ["redis"]
    - run: "npm run dev"
    - background: true
    - directory: "frontend"

  build-frontend:
    - run: "npm run build"
    - directory: "./frontend"

  logs:
    - run "tail -n 50 activity.log"
```

## Available Command Options

- `start_services` - an array of services to start with docker
- `run` - an array of shell commands to execute
- `background` - a boolean specifying if the command is to be run in the background or in the foreground
- `daemon` - if true, a PID file is stored and the command can have only one instance running at any given time
- `directory` - a string specifying the working directory for command execution (can be absolute or relative path). Note: PID files are always stored in the project root regardless of this option

## Key Features

1. Smart Discovery: Detects project type (presence of `go.mod`, `package.json`, `requirements.txt`, `Cargo.toml`, etc.) and uses sensible defaults
2. Environment Handling: Automatically loads .env files
3. PID tracking: When starting processes in daemon mode, stores the PID to ensure only one instance runs
4. Single binary deployment with no runtime dependencies

## Ways of working: Test Driven Development
* See @~/.claude/go-snippet.md
