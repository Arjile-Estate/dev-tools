"""Command execution engine with service management and PID tracking."""

import hashlib
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


def generate_pid_filename(command: str) -> str:
    """
    Generate a PID filename using SHA1 hash of the command.

    Args:
        command: The command string to hash

    Returns:
        PID filename in format '.{first_8_chars_of_sha1}.pid'
    """
    sha1_hash = hashlib.sha1(command.encode("utf-8")).hexdigest()
    return f".{sha1_hash[:8]}.pid"


def execute_shell_command(
    command: str,
    background: bool = False,
    timeout: int | None = None,
    capture_output: bool = False,
    cwd: Path | None = None,
    daemon: bool = False,
) -> CommandResult:
    """
    Execute a shell command with optional background execution.

    Args:
        command: Shell command to execute
        background: Whether to run in background
        timeout: Optional timeout in seconds
        capture_output: Whether to capture output or stream to stdout
        cwd: Working directory for command execution
        daemon: Whether this is a daemon process (for PID tracking)

    Returns:
        CommandResult with execution details
    """

    try:
        if background:
            # For background processes, redirect output to /dev/null to prevent hanging
            # and properly detach from parent process
            process = subprocess.Popen(
                command,
                shell=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                stdin=subprocess.DEVNULL,
                cwd=cwd,
                start_new_session=True,  # Properly detach from parent process
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
            # For daemon processes running in foreground, we need to track PID
            if daemon:
                process = subprocess.Popen(
                    command,
                    shell=True,
                    cwd=cwd,
                )
                logger.info(f"Started foreground daemon process with PID {process.pid}")

                # Create PID file immediately for daemon tracking
                pid_file = Path(generate_pid_filename(command))
                create_pid_file(pid_file, process.pid)
                logger.info(
                    f"Created PID file {pid_file} for foreground daemon process"
                )
                # Display message to stdout for user
                print(
                    f"Running job '{command}' in the foreground. PID: {process.pid}, PID file: {pid_file}"
                )

                try:
                    # Wait for process to complete
                    process.wait()
                    success = process.returncode == 0
                    if success:
                        logger.info("Command completed successfully")
                    else:
                        logger.error(
                            f"Command failed with return code {process.returncode}"
                        )
                finally:
                    # Clean up PID file when process completes
                    remove_pid_file(pid_file)
                    logger.info(f"Removed PID file {pid_file} after process completion")

                return CommandResult(
                    success=success, returncode=process.returncode, pid=process.pid
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

    First checks if container exists:
    - If exists and running: do nothing
    - If exists but stopped: start it with docker start
    - If doesn't exist: create and start with docker run

    Args:
        service_name: Name of the service to start

    Returns:
        CommandResult indicating success/failure
    """
    logger.info(f"Starting Docker service: {service_name}")

    # Extract container name from service name (use last part after slash)
    container_name = service_name.split("/")[-1]

    # Check if container already exists
    check_cmd = (
        f"docker ps -a --format '{{{{.Names}}}}' --filter name=^{container_name}$"
    )
    logger.info(f"Checking if container exists: {check_cmd}")
    check_result = execute_shell_command(check_cmd, capture_output=True)

    if check_result.success and container_name.strip() in check_result.stdout.strip():
        # Container exists, check if it's running
        status_cmd = (
            f"docker ps --format '{{{{.Names}}}}' --filter name=^{container_name}$"
        )
        logger.info(f"Checking container status: {status_cmd}")
        status_result = execute_shell_command(status_cmd, capture_output=True)

        if (
            status_result.success
            and container_name.strip() in status_result.stdout.strip()
        ):
            logger.info(f"Container {container_name} is already running")
            return CommandResult(success=True, stdout="Container already running")
        else:
            # Container exists but is stopped, start it
            start_cmd = f"docker start {container_name}"
            logger.info(f"Starting existing container: {start_cmd}")
            return execute_shell_command(start_cmd, capture_output=True)

    # Container doesn't exist, create and start it
    service_configs = {
        "redis": "docker run -d --name redis -p 6379:6379 redis:latest",
        "postgres": "docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=password postgres:latest",
        "mysql": "docker run -d --name mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password mysql:latest",
    }

    run_cmd = service_configs.get(
        service_name, f"docker run -d --name {container_name} {service_name}:latest"
    )
    logger.info(f"Creating new container: {run_cmd}")
    return execute_shell_command(run_cmd, capture_output=True)


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
    background = step.get("background", False)
    daemon = step.get("daemon", False)

    # Handle directory option - if specified, use it as the working directory
    # but keep PID files in the root (original cwd)
    step_directory = step.get("directory")
    if step_directory:
        if not isinstance(step_directory, Path):
            step_directory = Path(step_directory)
        # Make relative paths relative to the original cwd
        if not step_directory.is_absolute() and cwd:
            step_directory = cwd / step_directory

        # Validate directory exists and is accessible
        if not step_directory.exists():
            error_msg = f"Directory '{step_directory}' does not exist"
            logger.error(error_msg)
            return CommandResult(success=False, stderr=error_msg)

        if not step_directory.is_dir():
            error_msg = f"Path '{step_directory}' is not a directory"
            logger.error(error_msg)
            return CommandResult(success=False, stderr=error_msg)

        # Check if directory is accessible (readable and executable)
        try:
            # Test access by attempting to list directory contents
            list(step_directory.iterdir())
        except PermissionError:
            error_msg = (
                f"Directory '{step_directory}' is not accessible (permission denied)"
            )
            logger.error(error_msg)
            return CommandResult(success=False, stderr=error_msg)
        except Exception as e:
            error_msg = f"Directory '{step_directory}' is not accessible: {e}"
            logger.error(error_msg)
            return CommandResult(success=False, stderr=error_msg)

        execution_cwd = step_directory
        logger.info(f"Using directory: {step_directory}")
    else:
        execution_cwd = cwd

    logger.info(f"Executing command step (background={background}, daemon={daemon})")

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

        for command in commands:
            # Log command options
            options = []
            if background:
                options.append("background=True")
            if daemon:
                options.append("daemon=True")
            options_str = f" ({', '.join(options)})" if options else ""
            logger.info(f"Executing command: {command}{options_str}")

            # Check if daemon instance is already running before starting
            # PID files are always stored in the original cwd (root)
            if daemon:
                pid_file = Path(generate_pid_filename(command))
                if pid_file.exists():
                    existing_pid = read_pid_file(pid_file)
                    if existing_pid and is_process_running(existing_pid):
                        error_msg = f"Daemon process already running with PID {existing_pid} (pid file: {pid_file})"
                        logger.warning(error_msg)
                        return CommandResult(success=False, stderr=error_msg)
                    else:
                        # Clean up stale PID file
                        logger.info(f"Removing stale PID file {pid_file}")
                        remove_pid_file(pid_file)

            # For regular user commands, don't capture output (stream to stdout)
            # For background commands, we need to capture for PID tracking
            capture = background
            result = execute_shell_command(
                command,
                background=background,
                capture_output=capture,
                cwd=execution_cwd,
                daemon=daemon,
            )
            if not result.success and not background:
                return result
            elif result.pid and daemon and background:
                # Handle background daemon processes (foreground daemons are handled in execute_shell_command)
                logger.info(f"Background daemon process with PID {result.pid}")
                pid_file = Path(generate_pid_filename(command))
                create_pid_file(pid_file, result.pid)
                logger.info(
                    f"Created PID file {pid_file} for background daemon process"
                )
                # Display message to stdout for user
                print(
                    f"Running job '{command}' in the background. PID: {result.pid}, PID file: {pid_file}"
                )
                return result
            elif result.pid and daemon and not background:
                # Foreground daemon - PID file already handled in execute_shell_command
                logger.info(
                    f"Foreground daemon process completed with PID {result.pid}"
                )
                return result
            elif background and result.pid:
                logger.info(f"Command started with PID {result.pid}")
                # Display message to stdout for user
                print(f"Running job '{command}' in the background")
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


def cleanup_stale_pid_files(project_dir: Path = Path(".")) -> CommandResult:
    """
    Clean up stale PID files for processes that are no longer running.

    Args:
        project_dir: Directory to search for PID files

    Returns:
        CommandResult with cleanup summary
    """
    logger.info("Starting PID file cleanup")

    # Find all PID files in the project directory
    pid_files = list(project_dir.glob("*.pid"))
    if not pid_files:
        message = "No PID files found to clean up"
        logger.info(message)
        return CommandResult(success=True, stdout=message)

    cleaned_files = []
    active_processes = []
    errors = []

    for pid_file in pid_files:
        try:
            pid = read_pid_file(pid_file)
            if pid is None:
                logger.warning(f"Could not read PID from {pid_file}")
                errors.append(f"Could not read PID from {pid_file}")
                continue

            if is_process_running(pid):
                logger.info(f"Process {pid} from {pid_file} is still running")
                active_processes.append((pid_file.name, pid))
            else:
                logger.info(
                    f"Process {pid} from {pid_file} is not running, removing PID file"
                )
                remove_pid_file(pid_file)
                cleaned_files.append((pid_file.name, pid))

        except Exception as e:
            error_msg = f"Error processing {pid_file}: {e}"
            logger.error(error_msg)
            errors.append(error_msg)

    # Prepare summary message
    summary_lines = []

    if cleaned_files:
        summary_lines.append(f"Cleaned up {len(cleaned_files)} stale PID file(s):")
        for filename, pid in cleaned_files:
            summary_lines.append(f"  - {filename} (PID {pid})")

    if active_processes:
        summary_lines.append(f"Found {len(active_processes)} active process(es):")
        for filename, pid in active_processes:
            summary_lines.append(f"  - {filename} (PID {pid})")

    if errors:
        summary_lines.append(f"Encountered {len(errors)} error(s):")
        for error in errors:
            summary_lines.append(f"  - {error}")

    if not cleaned_files and not active_processes and not errors:
        summary_lines.append("No PID files found to process")

    summary = "\n".join(summary_lines)
    logger.info(f"PID cleanup completed. Summary: {summary}")

    # Return success if we cleaned files or found active processes, error only if all operations failed
    success = len(errors) == 0 or len(cleaned_files) > 0 or len(active_processes) > 0

    return CommandResult(
        success=success, stdout=summary, stderr="\n".join(errors) if errors else ""
    )
