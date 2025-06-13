"""Command execution engine with service management and PID tracking."""

import logging
import os
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from dotenv import load_dotenv

logger = logging.getLogger(__name__)


@dataclass
class CommandResult:
    """Result of command execution."""

    success: bool
    stdout: str = ""
    stderr: str = ""
    returncode: int = 0
    pid: int | None = None


def execute_shell_command(
    command: str,
    background: bool = False,
    timeout: int | None = None,
    capture_output: bool = False,
    cwd: Path | None = None,
) -> CommandResult:
    """
    Execute a shell command with optional background execution.

    Args:
        command: Shell command to execute
        background: Whether to run in background
        timeout: Optional timeout in seconds
        capture_output: Whether to capture output or stream to stdout
        cwd: Working directory for command execution

    Returns:
        CommandResult with execution details
    """
    logger.info(f"Executing command: {command}")

    try:
        if background:
            process = subprocess.Popen(
                command,
                shell=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                cwd=cwd,
            )
            logger.info(f"Started background process with PID {process.pid}")
            return CommandResult(success=True, pid=process.pid)
        elif capture_output:
            # Used for tests and internal commands like logs
            result = subprocess.run(
                command,
                shell=True,
                capture_output=True,
                text=True,
                timeout=timeout,
                cwd=cwd,
            )

            success = result.returncode == 0
            if success:
                logger.info("Command completed successfully")
            else:
                logger.error(f"Command failed with return code {result.returncode}")

            return CommandResult(
                success=success,
                stdout=result.stdout,
                stderr=result.stderr,
                returncode=result.returncode,
            )
        else:
            # Stream output directly to stdout/stderr for user commands
            result = subprocess.run(command, shell=True, timeout=timeout, cwd=cwd)

            success = result.returncode == 0
            if success:
                logger.info("Command completed successfully")
            else:
                logger.error(f"Command failed with return code {result.returncode}")

            return CommandResult(success=success, returncode=result.returncode)

    except subprocess.TimeoutExpired as e:
        logger.error(f"Command timed out after {timeout} seconds: {e}")
        return CommandResult(
            success=False, stderr=f"TimeoutExpired: {str(e)}", returncode=-1
        )
    except Exception as e:
        logger.error(f"Failed to execute command: {e}")
        return CommandResult(success=False, stderr=str(e), returncode=-1)


def start_docker_service(service_name: str) -> CommandResult:
    """
    Start a Docker service container.

    Args:
        service_name: Name of the service to start

    Returns:
        CommandResult indicating success/failure
    """
    logger.info(f"Starting Docker service: {service_name}")

    service_configs = {
        "redis": "docker run -d --name redis -p 6379:6379 redis:latest",
        "postgres": "docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=password postgres:latest",
        "mysql": "docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password mysql:latest",
    }

    command = service_configs.get(
        service_name, f"docker run -d --name {service_name} {service_name}:latest"
    )
    result = execute_shell_command(command, capture_output=True)

    if not result.success and "already in use" in result.stderr:
        logger.info(f"Service {service_name} is already running")
        return CommandResult(success=True, stdout="Service already running")

    return result


def stop_docker_service(service_name: str) -> CommandResult:
    """
    Stop and remove a Docker service container.

    Args:
        service_name: Name of the service to stop

    Returns:
        CommandResult indicating success/failure
    """
    logger.info(f"Stopping Docker service: {service_name}")
    command = f"docker stop {service_name} && docker rm {service_name}"
    return execute_shell_command(command, capture_output=True)


def execute_command_step(
    step: dict[str, Any], cwd: Path | None = None
) -> CommandResult:
    """
    Execute a single command step with all its components.

    Args:
        step: Command step configuration
        cwd: Working directory for command execution

    Returns:
        CommandResult with execution details
    """
    logger.info("Executing command step")

    if "start_services" in step:
        for service in step["start_services"]:
            result = start_docker_service(service)
            if not result.success:
                logger.error(f"Failed to start service {service}")
                return result

    if "run" in step:
        commands = step["run"]
        if isinstance(commands, str):
            commands = [commands]

        background = step.get("background", False)

        for command in commands:
            # For regular user commands, don't capture output (stream to stdout)
            # For background commands, we need to capture for PID tracking
            capture = background
            result = execute_shell_command(
                command, background=background, capture_output=capture, cwd=cwd
            )
            if not result.success and not background:
                return result
            elif background and result.pid:
                daemon = step.get("daemon", False)
                if daemon:
                    pid_file = Path(f".{command.replace(' ', '_')}.pid")
                    create_pid_file(pid_file, result.pid)
                    logger.info(f"Created PID file {pid_file} for daemon process")
                return result

    return CommandResult(success=True)


def execute_command_with_steps(
    command_name: str, steps: list[dict[str, Any]], cwd: Path | None = None
) -> CommandResult:
    """
    Execute a command consisting of multiple steps.

    Args:
        command_name: Name of the command being executed
        steps: List of command steps
        cwd: Working directory for command execution

    Returns:
        CommandResult with overall execution status
    """
    logger.info(f"Executing command '{command_name}' with {len(steps)} steps")

    for i, step in enumerate(steps, 1):
        logger.info(f"Executing step {i}/{len(steps)}")
        result = execute_command_step(step, cwd)
        if not result.success:
            logger.error(f"Step {i} failed, aborting command execution")
            return result

    logger.info(f"Command '{command_name}' completed successfully")
    return CommandResult(success=True)


def load_environment_variables(env_file: Path) -> None:
    """
    Load environment variables from a .env file.

    Args:
        env_file: Path to the .env file
    """
    if env_file.exists():
        logger.info(f"Loading environment variables from {env_file}")
        load_dotenv(env_file)
    else:
        logger.debug(f"No .env file found at {env_file}")


def create_pid_file(pid_file: Path, pid: int) -> None:
    """
    Create a PID file for daemon process tracking.

    Args:
        pid_file: Path to the PID file
        pid: Process ID to store
    """
    with open(pid_file, "w") as f:
        f.write(str(pid))
    logger.debug(f"Created PID file {pid_file} with PID {pid}")


def read_pid_file(pid_file: Path) -> int | None:
    """
    Read PID from a PID file.

    Args:
        pid_file: Path to the PID file

    Returns:
        Process ID or None if file doesn't exist
    """
    if not pid_file.exists():
        return None

    try:
        with open(pid_file) as f:
            pid = int(f.read().strip())
        logger.debug(f"Read PID {pid} from {pid_file}")
        return pid
    except (OSError, ValueError) as e:
        logger.error(f"Failed to read PID file {pid_file}: {e}")
        return None


def remove_pid_file(pid_file: Path) -> None:
    """
    Remove a PID file.

    Args:
        pid_file: Path to the PID file to remove
    """
    if pid_file.exists():
        pid_file.unlink()
        logger.debug(f"Removed PID file {pid_file}")


def is_process_running(pid: int) -> bool:
    """
    Check if a process with given PID is running.

    Args:
        pid: Process ID to check

    Returns:
        True if process is running, False otherwise
    """
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
