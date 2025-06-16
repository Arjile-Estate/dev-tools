"""Tests for command execution functionality."""

import os
import subprocess
from pathlib import Path
from unittest.mock import Mock, patch

from dev_tools.command_executor import (
    create_pid_file,
    execute_command_step,
    execute_shell_command,
    generate_pid_filename,
    is_process_running,
    load_environment_variables,
    read_pid_file,
    remove_pid_file,
    start_docker_service,
    stop_docker_service,
)


class TestShellCommandExecution:
    """Test suite for shell command execution."""

    @patch("subprocess.run")
    def test_execute_shell_command_success(self, mock_run):
        """Test successful shell command execution."""
        mock_run.return_value = Mock(returncode=0, stdout="success output", stderr="")

        result = execute_shell_command("echo 'hello'", capture_output=True)

        assert result.success is True
        assert result.stdout == "success output"
        assert result.stderr == ""
        assert result.returncode == 0
        mock_run.assert_called_once()

    @patch("subprocess.run")
    def test_execute_shell_command_failure(self, mock_run):
        """Test failed shell command execution."""
        mock_run.return_value = Mock(returncode=1, stdout="", stderr="error message")

        result = execute_shell_command("false", capture_output=True)

        assert result.success is False
        assert result.stderr == "error message"
        assert result.returncode == 1

    @patch("subprocess.run")
    def test_execute_shell_command_timeout(self, mock_run):
        """Test shell command execution with timeout."""
        mock_run.side_effect = subprocess.TimeoutExpired("cmd", 30)

        result = execute_shell_command("sleep 60", timeout=30, capture_output=True)

        assert result.success is False
        assert "TimeoutExpired" in result.stderr

    @patch("subprocess.Popen")
    def test_execute_shell_command_background(self, mock_popen):
        """Test background shell command execution."""
        mock_process = Mock()
        mock_process.pid = 12345
        mock_popen.return_value = mock_process

        result = execute_shell_command("long_running_process", background=True)

        assert result.success is True
        assert result.pid == 12345
        mock_popen.assert_called_once()


class TestDockerServiceManagement:
    """Test suite for Docker service management."""

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_success(self, mock_execute):
        """Test successful Docker service startup."""
        mock_execute.return_value = Mock(success=True, stdout="service started")

        result = start_docker_service("redis")

        assert result.success is True
        mock_execute.assert_called_with(
            "docker run -d --name redis -p 6379:6379 redis:latest", capture_output=True
        )

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_already_running_old_logic(self, mock_execute):
        """Test starting Docker service (legacy test for old logic)."""
        # This test is kept for backward compatibility but the new logic
        # is tested in TestImprovedDockerServiceManagement
        mock_execute.side_effect = [
            Mock(success=True, stdout="redis"),  # Container exists
            Mock(success=True, stdout="redis"),  # Container is running
        ]

        result = start_docker_service("redis")

        assert result.success is True
        assert result.stdout == "Container already running"

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_stop_docker_service(self, mock_execute):
        """Test stopping Docker service."""
        mock_execute.return_value = Mock(success=True)

        result = stop_docker_service("redis")

        assert result.success is True
        mock_execute.assert_called_with(
            "docker stop redis && docker rm redis", capture_output=True
        )


class TestCommandStepExecution:
    """Test suite for command step execution."""

    @patch("dev_tools.command_executor.start_docker_service")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_services(self, mock_execute, mock_start_service):
        """Test executing command step with service startup."""
        mock_start_service.return_value = Mock(success=True)
        mock_execute.return_value = Mock(success=True, stdout="test output")

        step = {"start_services": ["redis", "postgres"], "run": "pytest tests/"}

        result = execute_command_step(step)

        assert result.success is True
        assert mock_start_service.call_count == 2
        mock_execute.assert_called_with(
            "pytest tests/",
            background=False,
            capture_output=False,
            cwd=None,
            daemon=False,
        )

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_multiple_commands(self, mock_execute):
        """Test executing command step with multiple run commands."""
        mock_execute.return_value = Mock(success=True)

        step = {"run": ["eslint src/", "black ."]}

        result = execute_command_step(step)

        assert result.success is True
        assert mock_execute.call_count == 2

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_background(self, mock_execute):
        """Test executing command step in background."""
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "npm run dev", "background": True}

        result = execute_command_step(step)

        assert result.success is True
        assert result.pid == 12345
        mock_execute.assert_called_with(
            "npm run dev", background=True, capture_output=True, cwd=None, daemon=False
        )


class TestEnvironmentLoading:
    """Test suite for environment variable loading."""

    @patch("dev_tools.command_executor.load_dotenv")
    @patch("pathlib.Path.exists")
    def test_load_environment_variables_success(self, mock_exists, mock_load_dotenv):
        """Test successful .env file loading."""
        mock_exists.return_value = True

        load_environment_variables(Path(".env"))
        mock_load_dotenv.assert_called_once_with(Path(".env"))

    @patch("pathlib.Path.exists")
    def test_load_environment_variables_no_file(self, mock_exists):
        """Test handling when .env file doesn't exist."""
        mock_exists.return_value = False

        with patch.dict(os.environ, {}, clear=True):
            load_environment_variables(Path(".env"))

            assert len(os.environ) == 0


class TestPIDManagement:
    """Test suite for PID file management."""

    @patch("builtins.open")
    def test_create_pid_file(self, mock_open):
        """Test PID file creation."""
        create_pid_file(Path("test.pid"), 12345)

        mock_open.assert_called_once_with(Path("test.pid"), "w")
        mock_open.return_value.__enter__.return_value.write.assert_called_with("12345")

    @patch("pathlib.Path.exists")
    @patch("builtins.open")
    def test_read_pid_file_success(self, mock_open, mock_exists):
        """Test successful PID file reading."""
        mock_exists.return_value = True
        mock_open.return_value.__enter__.return_value.read.return_value = "12345"

        pid = read_pid_file(Path("test.pid"))

        assert pid == 12345

    @patch("pathlib.Path.exists")
    def test_read_pid_file_nonexistent(self, mock_exists):
        """Test reading nonexistent PID file."""
        mock_exists.return_value = False

        pid = read_pid_file(Path("test.pid"))

        assert pid is None

    @patch("pathlib.Path.unlink")
    @patch("pathlib.Path.exists")
    def test_remove_pid_file(self, mock_exists, mock_unlink):
        """Test PID file removal."""
        mock_exists.return_value = True

        remove_pid_file(Path("test.pid"))

        mock_unlink.assert_called_once()

    @patch("os.kill")
    def test_is_process_running_true(self, mock_kill):
        """Test checking if process is running (process exists)."""
        mock_kill.return_value = None

        result = is_process_running(12345)

        assert result is True
        mock_kill.assert_called_once_with(12345, 0)

    @patch("os.kill")
    def test_is_process_running_false(self, mock_kill):
        """Test checking if process is running (process doesn't exist)."""
        mock_kill.side_effect = ProcessLookupError()

        result = is_process_running(12345)

        assert result is False


class TestPidFilenameGeneration:
    """Test suite for SHA1-based PID filename generation."""

    def test_generate_pid_filename_simple_command(self):
        """Test PID filename generation for simple command."""
        command = "sleep 3600"
        filename = generate_pid_filename(command)

        # Should be 8 chars + .pid
        assert len(filename) == 13  # .{8chars}.pid
        assert filename.startswith(".")
        assert filename.endswith(".pid")
        assert filename == ".392a9a8c.pid"  # Known SHA1 for "sleep 3600"

    def test_generate_pid_filename_complex_command(self):
        """Test PID filename generation for complex command."""
        command = "echo 'Starting...' && sleep 3600"
        filename = generate_pid_filename(command)

        assert len(filename) == 13
        assert filename.startswith(".")
        assert filename.endswith(".pid")
        assert filename == ".ff41d59f.pid"  # Known SHA1 for this command

    def test_generate_pid_filename_consistency(self):
        """Test that same command always generates same filename."""
        command = "test command"
        filename1 = generate_pid_filename(command)
        filename2 = generate_pid_filename(command)

        assert filename1 == filename2

    def test_generate_pid_filename_different_commands(self):
        """Test that different commands generate different filenames."""
        command1 = "test command 1"
        command2 = "test command 2"
        filename1 = generate_pid_filename(command1)
        filename2 = generate_pid_filename(command2)

        assert filename1 != filename2


class TestImprovedDockerServiceManagement:
    """Test suite for improved Docker service management."""

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_new_container(self, mock_execute):
        """Test starting a new Docker container."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        result = start_docker_service("redis")

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called
        args, kwargs = mock_execute.call_args_list[1]
        assert "docker run -d --name redis -p 6379:6379 redis:latest" in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_container_already_running(self, mock_execute):
        """Test when container already exists and is running."""
        # Mock container exists and is running
        mock_execute.side_effect = [
            Mock(success=True, stdout="redis"),  # Container exists
            Mock(success=True, stdout="redis"),  # Container is running
        ]

        result = start_docker_service("redis")

        assert result.success is True
        assert result.stdout == "Container already running"
        assert mock_execute.call_count == 2

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_container_stopped(self, mock_execute):
        """Test restarting a stopped container."""
        # Mock container exists but is stopped
        mock_execute.side_effect = [
            Mock(success=True, stdout="redis"),  # Container exists
            Mock(success=True, stdout=""),  # Container not running
            Mock(success=True, stdout="redis\n"),  # Docker start success
        ]

        result = start_docker_service("redis")

        assert result.success is True
        assert mock_execute.call_count == 3
        # Check that docker start was called
        args, kwargs = mock_execute.call_args_list[2]
        assert "docker start redis" in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_custom_image_naming(self, mock_execute):
        """Test container naming for custom images with paths."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        result = start_docker_service("registry.example.com/namespace/app")

        assert result.success is True
        # Check that container name is extracted correctly
        args, kwargs = mock_execute.call_args_list[1]
        assert (
            "docker run -d --name app registry.example.com/namespace/app:latest"
            in args[0]
        )

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_user_repo_format(self, mock_execute):
        """Test container naming for user/repo format images."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        result = start_docker_service("rebelthor/sleep")

        assert result.success is True
        # Check that container name is extracted correctly (sleep from rebelthor/sleep)
        args, kwargs = mock_execute.call_args_list[1]
        assert "docker run -d --name sleep rebelthor/sleep:latest" in args[0]


class TestDaemonImprovements:
    """Test suite for daemon improvements with SHA1 PID files."""

    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("dev_tools.command_executor.read_pid_file")
    @patch("dev_tools.command_executor.is_process_running")
    @patch("pathlib.Path.exists")
    def test_daemon_prevents_duplicate_with_sha1_pid(
        self, mock_exists, mock_running, mock_read_pid, mock_execute
    ):
        """Test that daemon prevents duplicate instances using SHA1 PID files."""
        # Mock PID file exists and process is running
        mock_exists.return_value = True
        mock_read_pid.return_value = 12345
        mock_running.return_value = True

        step = {"run": "sleep 3600", "daemon": True, "background": True}

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is False
        assert "Daemon process already running with PID 12345" in result.stderr
        assert ".392a9a8c.pid" in result.stderr  # SHA1 hash of "sleep 3600"

    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.read_pid_file")
    @patch("dev_tools.command_executor.is_process_running")
    @patch("pathlib.Path.exists")
    def test_daemon_cleans_stale_pid_file(
        self, mock_exists, mock_running, mock_read_pid, mock_remove_pid, mock_execute
    ):
        """Test that daemon cleans up stale PID files."""
        # Mock stale PID file (exists but process not running)
        mock_exists.return_value = True
        mock_read_pid.return_value = 12345
        mock_running.return_value = False
        mock_execute.return_value = Mock(success=True, pid=54321)

        step = {"run": "sleep 3600", "daemon": True, "background": True}

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is True
        mock_remove_pid.assert_called_once()

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_command_logging_with_options(self, mock_execute):
        """Test that command execution logs include options."""
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "test command", "daemon": True, "background": True}

        with patch("dev_tools.command_executor.logger") as mock_logger:
            execute_command_step(step, Path("/tmp"))

            # Check that logging includes options
            mock_logger.info.assert_any_call(
                "Executing command: test command (background=True, daemon=True)"
            )

    def test_command_logging_without_options(self):
        """Test that command execution logs work without options."""
        with (
            patch("dev_tools.command_executor.execute_shell_command") as mock_execute,
            patch("dev_tools.command_executor.logger") as mock_logger,
        ):
            mock_execute.return_value = Mock(success=True)

            step = {"run": "test command"}

            execute_command_step(step, Path("/tmp"))

            # Check that logging works without options
            mock_logger.info.assert_any_call("Executing command: test command")


class TestServiceIntegration:
    """Test suite for start_services integration."""

    @patch("dev_tools.command_executor.start_docker_service")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_start_services(
        self, mock_execute, mock_start_service
    ):
        """Test command step execution with start_services."""
        mock_start_service.return_value = Mock(success=True)
        mock_execute.return_value = Mock(success=True)

        step = {"start_services": ["redis", "postgres"], "run": "test command"}

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is True
        assert mock_start_service.call_count == 2
        mock_start_service.assert_any_call("redis")
        mock_start_service.assert_any_call("postgres")

    @patch("dev_tools.command_executor.start_docker_service")
    def test_execute_command_step_service_failure_stops_execution(
        self, mock_start_service
    ):
        """Test that service startup failure stops command execution."""
        mock_start_service.return_value = Mock(
            success=False, stderr="Service failed to start"
        )

        step = {"start_services": ["redis"], "run": "test command"}

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is False
        assert "Service failed to start" in result.stderr


class TestDaemonFunctionality:
    """Test suite for daemon functionality improvements."""

    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("dev_tools.command_executor.create_pid_file")
    @patch("pathlib.Path.exists")
    def test_daemon_background_creates_pid_file(
        self, mock_exists, mock_create_pid, mock_execute
    ):
        """Test that daemon with background=True creates PID file."""
        mock_exists.return_value = False  # No existing PID file
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "npm run dev", "daemon": True, "background": True}

        result = execute_command_step(step)

        assert result.success is True
        assert result.pid == 12345
        mock_execute.assert_called_with(
            "npm run dev", background=True, capture_output=True, cwd=None, daemon=True
        )
        mock_create_pid.assert_called_once()

    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("pathlib.Path.exists")
    def test_daemon_foreground_creates_pid_file(self, mock_exists, mock_execute):
        """Test that daemon with background=False creates PID file."""
        mock_exists.return_value = False  # No existing PID file
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "npm run dev", "daemon": True, "background": False}

        result = execute_command_step(step)

        assert result.success is True
        assert result.pid == 12345
        mock_execute.assert_called_with(
            "npm run dev", background=False, capture_output=False, cwd=None, daemon=True
        )
        # PID file creation is now handled inside execute_shell_command for foreground daemons

    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.create_pid_file")
    @patch("subprocess.Popen")
    def test_execute_shell_command_daemon_foreground(
        self, mock_popen, mock_create_pid, mock_remove_pid
    ):
        """Test daemon execution in foreground mode."""
        mock_process = Mock()
        mock_process.pid = 12345
        mock_process.returncode = 0
        mock_process.wait.return_value = None
        mock_popen.return_value = mock_process

        result = execute_shell_command("test command", daemon=True, background=False)

        assert result.success is True
        assert result.pid == 12345
        assert result.returncode == 0
        mock_popen.assert_called_once()
        mock_process.wait.assert_called_once()
        mock_create_pid.assert_called_once()
        mock_remove_pid.assert_called_once()

    @patch("subprocess.Popen")
    def test_execute_shell_command_daemon_background(self, mock_popen):
        """Test daemon execution in background mode."""
        mock_process = Mock()
        mock_process.pid = 12345
        mock_popen.return_value = mock_process

        result = execute_shell_command("test command", daemon=True, background=True)

        assert result.success is True
        assert result.pid == 12345
        mock_popen.assert_called_once()
        # Should not wait for background processes
        mock_process.wait.assert_not_called()


class TestBackgroundJobMessaging:
    """Test suite for background job stdout messaging."""

    @patch("builtins.print")
    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("dev_tools.command_executor.create_pid_file")
    @patch("pathlib.Path.exists")
    def test_background_daemon_stdout_message(
        self, mock_exists, mock_create_pid, mock_execute, mock_print
    ):
        """Test that background daemon jobs display stdout message with PID and file info."""
        mock_exists.return_value = False  # No existing PID file
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "npm run dev", "daemon": True, "background": True}

        result = execute_command_step(step)

        assert result.success is True
        assert result.pid == 12345
        mock_print.assert_called_once_with(
            "Running job 'npm run dev' in the background. PID: 12345, PID file: .4eedebe9.pid"
        )

    @patch("builtins.print")
    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("pathlib.Path.exists")
    def test_background_non_daemon_stdout_message(
        self, mock_exists, mock_execute, mock_print
    ):
        """Test that background non-daemon jobs display stdout message without PID file info."""
        mock_execute.return_value = Mock(success=True, pid=54321)

        step = {"run": "long_running_task", "background": True}

        result = execute_command_step(step)

        assert result.success is True
        assert result.pid == 54321
        mock_print.assert_called_once_with(
            "Running job 'long_running_task' in the background"
        )

    @patch("builtins.print")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_foreground_job_no_stdout_message(self, mock_execute, mock_print):
        """Test that foreground jobs do not display stdout message."""
        mock_execute.return_value = Mock(success=True)

        step = {"run": "pytest tests/"}

        result = execute_command_step(step)

        assert result.success is True
        mock_print.assert_not_called()

    @patch("builtins.print")
    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.create_pid_file")
    @patch("subprocess.Popen")
    def test_foreground_daemon_stdout_message(
        self, mock_popen, mock_create_pid, mock_remove_pid, mock_print
    ):
        """Test that foreground daemon jobs display stdout message with PID and file info."""
        mock_process = Mock()
        mock_process.pid = 98765
        mock_process.returncode = 0
        mock_process.wait.return_value = None
        mock_popen.return_value = mock_process

        result = execute_shell_command(
            "echo toot && sleep 10", daemon=True, background=False
        )

        assert result.success is True
        assert result.pid == 98765
        mock_print.assert_called_once_with(
            "Running job 'echo toot && sleep 10' in the foreground. PID: 98765, PID file: .8a8dff0d.pid"
        )


class TestDirectoryOption:
    """Test suite for directory option functionality."""

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_absolute_directory(self, mock_execute):
        """Test executing command step with absolute directory path."""
        mock_execute.return_value = Mock(success=True)

        step = {"run": "ls -la", "directory": "/tmp"}

        result = execute_command_step(step, Path("/home/user"))

        assert result.success is True
        mock_execute.assert_called_with(
            "ls -la",
            background=False,
            capture_output=False,
            cwd=Path("/tmp"),
            daemon=False,
        )

    @patch("pathlib.Path.iterdir")
    @patch("pathlib.Path.is_dir")
    @patch("pathlib.Path.exists")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_relative_directory(
        self, mock_execute, mock_exists, mock_is_dir, mock_iterdir
    ):
        """Test executing command step with relative directory path."""
        mock_execute.return_value = Mock(success=True)
        mock_exists.return_value = True
        mock_is_dir.return_value = True
        mock_iterdir.return_value = []

        step = {"run": "npm install", "directory": "frontend"}

        result = execute_command_step(step, Path("/home/user/project"))

        assert result.success is True
        mock_execute.assert_called_with(
            "npm install",
            background=False,
            capture_output=False,
            cwd=Path("/home/user/project/frontend"),
            daemon=False,
        )

    @patch("pathlib.Path.iterdir")
    @patch("pathlib.Path.is_dir")
    @patch("pathlib.Path.exists")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_string_directory(
        self, mock_execute, mock_exists, mock_is_dir, mock_iterdir
    ):
        """Test executing command step with directory as string."""
        mock_execute.return_value = Mock(success=True)
        mock_exists.return_value = True
        mock_is_dir.return_value = True
        mock_iterdir.return_value = []

        step = {"run": "pytest", "directory": "tests"}

        result = execute_command_step(step, Path("/home/user/project"))

        assert result.success is True
        mock_execute.assert_called_with(
            "pytest",
            background=False,
            capture_output=False,
            cwd=Path("/home/user/project/tests"),
            daemon=False,
        )

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_without_directory_option(self, mock_execute):
        """Test executing command step without directory option uses original cwd."""
        mock_execute.return_value = Mock(success=True)

        step = {"run": "make build"}

        result = execute_command_step(step, Path("/home/user/project"))

        assert result.success is True
        mock_execute.assert_called_with(
            "make build",
            background=False,
            capture_output=False,
            cwd=Path("/home/user/project"),
            daemon=False,
        )

    # NOTE: Daemon with directory test is covered by real-world testing
    # The mocking is complex due to multiple pathlib.Path.exists() calls
    # and the functionality is verified by manual testing

    def test_execute_command_step_directory_not_exists(self):
        """Test error handling when specified directory doesn't exist."""
        step = {"run": "ls -la", "directory": "/nonexistent/path"}

        result = execute_command_step(step, Path("/home/user"))

        assert result.success is False
        assert "Directory '/nonexistent/path' does not exist" in result.stderr

    @patch("pathlib.Path.is_dir")
    @patch("pathlib.Path.exists")
    def test_execute_command_step_path_not_directory(self, mock_exists, mock_is_dir):
        """Test error handling when specified path is not a directory."""
        mock_exists.return_value = True
        mock_is_dir.return_value = False

        step = {"run": "ls -la", "directory": "/path/to/file.txt"}

        result = execute_command_step(step, Path("/home/user"))

        assert result.success is False
        assert "Path '/path/to/file.txt' is not a directory" in result.stderr

    @patch("pathlib.Path.iterdir")
    @patch("pathlib.Path.is_dir")
    @patch("pathlib.Path.exists")
    def test_execute_command_step_directory_permission_denied(
        self, mock_exists, mock_is_dir, mock_iterdir
    ):
        """Test error handling when directory access is denied."""
        mock_exists.return_value = True
        mock_is_dir.return_value = True
        mock_iterdir.side_effect = PermissionError("Permission denied")

        step = {"run": "ls -la", "directory": "/restricted/path"}

        result = execute_command_step(step, Path("/home/user"))

        assert result.success is False
        assert (
            "Directory '/restricted/path' is not accessible (permission denied)"
            in result.stderr
        )

    @patch("pathlib.Path.iterdir")
    @patch("pathlib.Path.is_dir")
    @patch("pathlib.Path.exists")
    def test_execute_command_step_directory_other_access_error(
        self, mock_exists, mock_is_dir, mock_iterdir
    ):
        """Test error handling when directory has other access issues."""
        mock_exists.return_value = True
        mock_is_dir.return_value = True
        mock_iterdir.side_effect = OSError("Device not ready")

        step = {"run": "ls -la", "directory": "/problematic/path"}

        result = execute_command_step(step, Path("/home/user"))

        assert result.success is False
        assert (
            "Directory '/problematic/path' is not accessible: Device not ready"
            in result.stderr
        )
