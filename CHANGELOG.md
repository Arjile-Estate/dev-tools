# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.5.2] - 2026-06-05

### Fixed
- Installer no longer reports a spurious failure on a successful install: the `EXIT` cleanup trap referenced a `main()`-local `tmp_dir` that was out of scope when the trap fired, tripping `set -u` (`tmp_dir: unbound variable`). `tmp_dir` is now a script global so cleanup runs cleanly.

## [1.5.0] - 2026-05-15

### Added
- Automatic JSON output when running inside a Claude Code session (detected via `CLAUDE_CODE=1` env var). Pass `--format text` to override.
- `--format json` support extended to all built-in commands that previously only emitted text: `logs`, `restart`, `stop`, `cleanup-pids`, `cleanup-all`, `validate`, and `version`.
- New `internal/commands/format.go` helper exposing `FormatCtxKey`, `FormatFromContext(cmd)`, and `EmitJSON(cmd, payload)` for consistent JSON output across handlers.
- `executor.CleanupSummary` struct returned alongside `ExecutionResult` from `CleanupStalePIDFiles` / `CleanupStalePIDFilesWithTermination`, providing structured cleaned-files/active-processes/errors arrays for JSON output.

### Changed
- `status` command no longer sniffs `--format=json` from `args`; the output format is now propagated through the cobra command context.
- `CleanupStalePIDFiles` and `CleanupStalePIDFilesWithTermination` now return `(ExecutionResult, CleanupSummary)` instead of just `ExecutionResult`.

## [1.2.0] - 2026-03-02

### Added
- User-defined commands in `.dev-config.yaml` now take precedence over built-in commands, allowing users to override commands like `logs` with their own definitions

## [1.1.1] - 2026-03-02

### Fixed
- Background daemon processes no longer leak stdout/stderr to the terminal; output is explicitly redirected to /dev/null

## [1.1.0] - 2026-03-02

### Fixed
- Daemon restart/stop from another terminal now properly terminates foreground daemon instances
- Process group signaling: foreground daemons now run in their own process group, allowing `stop` and `restart` to kill the entire process tree (shell + children)
- PID file race condition: restart no longer causes the old foreground instance to delete the new daemon's PID file
- Signal forwarding: Ctrl+C now propagates to the entire daemon process group instead of just the shell process

### Added
- `signalProcessGroup` helper for platform-specific process group signaling (Unix sends to group leader; Windows falls back to single-process signal)
- `RemovePIDFileIfOwned` function to prevent PID file ownership races during restart
- Termination feedback: foreground daemons now print a message when killed externally (e.g., "Daemon 'test-daemon' was terminated by signal: terminated")

## [1.0.0] - 2026-02-11

### Added
- MIT LICENSE file
- CHANGELOG.md following keepachangelog format
- `dev-tools schema` command to export JSON schema for `.dev-config.yaml`
- GitHub Actions CI/CD workflows (test, lint, release)
- GoReleaser configuration for cross-platform binary releases
- `install.sh` one-line installer script
- Build-time version injection via ldflags

### Changed
- Version bumped from 0.50.0 to 1.0.0
- Installation section in README updated with quick install via curl
- Version history moved from README to CHANGELOG.md

## [0.50.0]

### Added
- Closed all 50 tracked issues
- Comprehensive test coverage (73-96%)

## [0.40.0]

### Removed
- **BREAKING**: Removed deprecated `ExecuteCommandWithSteps` method (use `ExecuteCommandWithOptions`)
- **BREAKING**: Removed deprecated `start_services` configuration (use `services` configuration)
- **BREAKING**: Removed deprecated `StartDockerService` function (use `StartDockerServiceTyped`)
- **BREAKING**: Removed Docker Compose V1 (`docker-compose`) fallback support (use Docker Compose V2 built into Docker CLI)

### Changed
- Changed legacy daemon status marker from "(legacy)" to "(unknown)"
- Removed migration guide for `start_services` as it is no longer supported

## [0.35.0]

### Added
- JSON schema validation for `.dev-config.yaml` files
- `dev-tools validate` command for explicit configuration validation
- Automatic validation on config load with detailed error messages
- Schema enforces correct types, required fields, and patterns

## [0.34.0]

### Changed
- Extracted magic numbers to named constants
- Improved code maintainability and readability
- Better timeout and display width management

## [0.33.0]

### Changed
- Split monolithic `built_in.go` into focused modules
- Separated commands: logs, cleanup, daemon, status, onboard
- Improved single-responsibility principle adherence

## [0.32.0]

### Added
- Implemented zerolog for high-performance structured logging
- JSON-formatted logs for easy parsing and analysis
- Context-rich log entries with timestamps and metadata
- Zero-allocation JSON encoding for minimal overhead

## [0.31.0]

### Changed
- Standardized error handling patterns across codebase
- Custom error types for better error context
- Improved error messages and debugging information
- Replaced `os.Exit()` with proper error returns

## [0.23.0]

### Added
- File watch mode with automatic command re-execution
- Debouncing to prevent rapid re-runs
- Glob pattern support for file matching
- `context.Context` support for command cancellation
- Graceful shutdown and cleanup on interrupts

## [0.22.0]

### Changed
- Eliminated shell injection vulnerabilities
- Implemented POSIX shell argument escaping
- Thread-safe color output with atomic operations
- Secure command execution without shell interpolation

## [0.21.0]

### Changed
- Split `executor_shell.go` into focused modules
- Extracted helper functions for better modularity
- Improved test coverage and code organization

## [0.20.0]

### Changed
- Replaced `map[string]interface{}` with typed Docker config structs
- Type-safe configuration handling
- Better IDE support and compile-time checking

## [0.19.0]

### Added
- Configurable retry logic for transient failures
- Retry delays with configurable time units
- Selective retry based on exit codes

## [0.16.0]

### Added
- Support for Java/Maven, .NET, PHP, and Ruby projects
- JSON output format for `status` command (`--format json`)
- Machine-readable output for automation and monitoring
- Enhanced status command with comprehensive system information

## [0.15.0]

### Added
- Onboard command for AI assistant documentation

## [0.14.0]

### Added
- Shell completion for Bash, Zsh, and Fish

## [0.13.0]

### Added
- Docker Compose integration

## [0.12.0]

### Added
- Enhanced service management with health checks

## [0.11.0]

### Added
- Daemon process management with PID tracking

[1.5.0]: https://github.com/Arjile-Estate/dev-tools/compare/v1.2.0...v1.5.0
[1.2.0]: https://github.com/Arjile-Estate/dev-tools/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/Arjile-Estate/dev-tools/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/Arjile-Estate/dev-tools/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.50.0...v1.0.0
[0.50.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.40.0...v0.50.0
[0.40.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.35.0...v0.40.0
[0.35.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.34.0...v0.35.0
[0.34.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.33.0...v0.34.0
[0.33.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.32.0...v0.33.0
[0.32.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.31.0...v0.32.0
[0.31.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.23.0...v0.31.0
[0.23.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.22.0...v0.23.0
[0.22.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.16.0...v0.19.0
[0.16.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/Arjile-Estate/dev-tools/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/Arjile-Estate/dev-tools/releases/tag/v0.11.0
