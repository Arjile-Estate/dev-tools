"""Command-line interface for dev-tools application."""

import argparse
import sys
from pathlib import Path

from dev_tools.command_executor import (
    CommandResult,
    cleanup_stale_pid_files,
    execute_command_with_steps,
    execute_shell_command,
    load_environment_variables,
)
from dev_tools.config_parser import load_configuration_for_project
from dev_tools.logger_setup import get_logger, setup_application_logging
from dev_tools.version import __version__

logger = get_logger(__name__)


def create_argument_parser(project_dir: Path = Path(".")) -> argparse.ArgumentParser:
    """
    Create and configure the command-line argument parser.

    Args:
        project_dir: Project directory to load configuration from

    Returns:
        Configured ArgumentParser instance
    """
    # Determine the correct command name based on how the script is invoked
    script_name = Path(sys.argv[0]).name
    if script_name == "dev-tools.py":
        # Running from source via uv run dev-tools.py
        cmd_prefix = "uv run dev-tools.py"
    else:
        # Running from installed package
        cmd_prefix = "dev-tools"

    # Load configuration to get available commands for help text
    try:
        config = load_configuration_for_project(project_dir)
        available_commands = list(config.get("commands", {}).keys())
        # Add built-in commands that don't require configuration
        available_commands.extend(["logs", "cleanup-pids"])
        commands_str = ", ".join(available_commands)

        # Generate dynamic examples based on available commands
        examples = []
        for cmd in available_commands[:4]:  # Show first 4 commands as examples
            examples.append(f"  {cmd_prefix} {cmd}")
        if "test" in available_commands:
            examples.append(
                f"  {cmd_prefix} --verbose test  # Run with verbose logging"
            )

        epilog_text = f"""
Available commands: {commands_str}

Examples:
{chr(10).join(examples)}
        """
    except Exception:
        # Fallback to static help if configuration loading fails
        available_commands = ["test", "lint", "dev", "build", "logs", "cleanup-pids"]
        commands_str = ", ".join(available_commands)
        epilog_text = f"""
Examples:
  {cmd_prefix} test        # Run tests
  {cmd_prefix} lint        # Run linting
  {cmd_prefix} dev         # Start development server
  {cmd_prefix} logs        # Show recent logs
  {cmd_prefix} cleanup-pids # Clean up stale PID files
  {cmd_prefix} --verbose test  # Run tests with verbose logging
        """

    parser = argparse.ArgumentParser(
        description="Dev Tools - A command runner for development workflows",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=epilog_text,
    )

    parser.add_argument("command", help=f"Command to execute ({commands_str})")

    parser.add_argument(
        "--verbose", "-v", action="store_true", help="Enable verbose logging to stdout"
    )

    parser.add_argument(
        "--project-dir",
        "-p",
        type=Path,
        default=Path("."),
        help="Project directory to run commands in (defaults to current directory)",
    )

    parser.add_argument(
        "--version",
        action="version",
        version=f"dev-tools {__version__}",
    )

    return parser


def handle_logs_command(project_dir: Path = Path(".")) -> CommandResult:
    """
    Handle the special 'logs' command to display recent activity.

    Args:
        project_dir: Project directory to look for activity.log

    Returns:
        CommandResult with log content or error
    """
    # Determine which log file to display based on command name
    command_name = Path(sys.argv[0]).stem
    if command_name == "dev-tools":
        activity_log = Path.home() / "Library" / "Logs" / "dev-tools.log"
    else:
        activity_log = project_dir / "activity.log"

    if not activity_log.exists():
        logger.warning(f"No log file found at {activity_log}")
        return CommandResult(
            success=False, stderr=f"No log file found at {activity_log}"
        )

    logger.info("Displaying recent activity logs")
    return execute_shell_command(f"tail -n 50 {activity_log}", capture_output=True)


def handle_command_execution(command: str, project_dir: Path) -> CommandResult:
    """
    Handle execution of a development command.

    Args:
        command: Command name to execute
        project_dir: Project directory

    Returns:
        CommandResult with execution status
    """
    logger.info(f"Handling command execution: {command}")

    load_environment_variables(project_dir / ".env")

    config = load_configuration_for_project(project_dir)

    if command not in config.get("commands", {}):
        available_commands = list(config.get("commands", {}).keys())
        error_msg = f"Unknown command '{command}'. Available commands: {', '.join(available_commands)}"
        logger.error(error_msg)
        return CommandResult(success=False, stderr=error_msg)

    command_steps = config["commands"][command]
    return execute_command_with_steps(command, command_steps, project_dir)


def main() -> None:
    """Main entry point for the CLI application."""
    # Check if help is requested and extract project_dir for proper help text
    project_dir = Path(".")
    if "--help" in sys.argv or "-h" in sys.argv:
        # Try to extract project_dir from arguments for help display
        try:
            if "--project-dir" in sys.argv:
                idx = sys.argv.index("--project-dir")
                if idx + 1 < len(sys.argv):
                    project_dir = Path(sys.argv[idx + 1])
            elif "-p" in sys.argv:
                idx = sys.argv.index("-p")
                if idx + 1 < len(sys.argv):
                    project_dir = Path(sys.argv[idx + 1])
        except (ValueError, IndexError):
            # If parsing fails, use default project_dir
            pass

    parser = create_argument_parser(project_dir)
    args = parser.parse_args()

    # Determine log file path based on command name
    command_name = Path(sys.argv[0]).stem
    if command_name == "dev-tools":
        log_file = Path.home() / "Library" / "Logs" / "dev-tools.log"
        # Ensure the directory exists
        log_file.parent.mkdir(parents=True, exist_ok=True)
    else:
        log_file = None  # Will default to activity.log in current directory

    setup_application_logging(log_file=log_file, verbose=args.verbose)
    logger.info(f"Starting dev-tools with command: {args.command}")

    try:
        # Handle special commands that don't require configuration
        if args.command == "logs":
            result = handle_logs_command(args.project_dir)
        elif args.command == "cleanup-pids":
            result = cleanup_stale_pid_files(args.project_dir)
        else:
            result = handle_command_execution(args.command, args.project_dir)

        if result.success:
            if result.stdout:
                print(result.stdout)
            logger.info("Command completed successfully")
        else:
            if result.stderr:
                print(result.stderr)
            logger.error("Command failed")
            sys.exit(1)

    except Exception as e:
        logger.exception(f"Unexpected error: {e}")
        print(f"Unexpected error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
