"""Command-line interface for dev-tools application."""

import argparse
import sys
from pathlib import Path

from dev_tools.command_executor import (
    CommandResult,
    execute_command_with_steps,
    execute_shell_command,
    load_environment_variables,
)
from dev_tools.config_parser import load_configuration_for_project
from dev_tools.logger_setup import get_logger, setup_application_logging

logger = get_logger(__name__)


def create_argument_parser() -> argparse.ArgumentParser:
    """
    Create and configure the command-line argument parser.

    Returns:
        Configured ArgumentParser instance
    """
    # Load configuration to get available commands for help text
    try:
        config = load_configuration_for_project(Path("."))
        available_commands = list(config.get("commands", {}).keys())
        commands_str = ", ".join(available_commands)

        # Generate dynamic examples based on available commands
        examples = []
        for cmd in available_commands[:4]:  # Show first 4 commands as examples
            examples.append(f"  uv run dev-tools.py {cmd}")
        if "test" in available_commands:
            examples.append("  uv run dev-tools.py --verbose test  # Run with verbose logging")

        epilog_text = f"""
Available commands: {commands_str}

Examples:
{chr(10).join(examples)}
        """
    except Exception:
        # Fallback to static help if configuration loading fails
        available_commands = ["test", "lint", "dev", "build", "logs"]
        commands_str = ", ".join(available_commands)
        epilog_text = """
Examples:
  uv run dev-tools.py test        # Run tests
  uv run dev-tools.py lint        # Run linting
  uv run dev-tools.py dev         # Start development server
  uv run dev-tools.py logs        # Show recent logs
  uv run dev-tools.py --verbose test  # Run tests with verbose logging
        """

    parser = argparse.ArgumentParser(
        description="Dev Tools - A command runner for development workflows",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=epilog_text,
    )

    parser.add_argument(
        "command", help=f"Command to execute ({commands_str})"
    )

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

    return parser


def handle_logs_command(project_dir: Path = Path(".")) -> CommandResult:
    """
    Handle the special 'logs' command to display recent activity.

    Args:
        project_dir: Project directory to look for activity.log

    Returns:
        CommandResult with log content or error
    """
    activity_log = project_dir / "activity.log"

    if not activity_log.exists():
        logger.warning(f"No activity.log file found in {project_dir}")
        return CommandResult(
            success=False, stderr=f"No activity.log file found in {project_dir}"
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
    parser = create_argument_parser()
    args = parser.parse_args()

    setup_application_logging(verbose=args.verbose)
    logger.info(f"Starting dev-tools with command: {args.command}")

    try:
        if args.command == "logs":
            result = handle_logs_command(args.project_dir)
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
