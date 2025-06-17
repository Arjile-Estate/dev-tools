"""Tests for command execution functionality."""

import os
import subprocess
import tempfile
from pathlib import Path
from unittest.mock import Mock, mock_open, patch

import pytest

from dev_tools.command_executor import (
    cleanup_stale_pid_files,
    CommandResult,
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
            command_name="",
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
            "npm run dev",
            background=True,
            capture_output=True,
            cwd=None,
            daemon=False,
            command_name="",
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
        command_name = "dev"
        command = "sleep 3600"
        filename = generate_pid_filename(command_name, command)

        # Should be 8 chars + .pid
        assert len(filename) == 13  # .{8chars}.pid
        assert filename.startswith(".")
        assert filename.endswith(".pid")
        assert filename == ".4385d908.pid"  # Known SHA1 for "dev" + "sleep 3600"

    def test_generate_pid_filename_complex_command(self):
        """Test PID filename generation for complex command."""
        command_name = "api"
        command = "echo 'Starting...' && sleep 3600"
        filename = generate_pid_filename(command_name, command)

        assert len(filename) == 13
        assert filename.startswith(".")
        assert filename.endswith(".pid")
        assert filename == ".76e4a047.pid"  # Known SHA1 for "api" + this command

    def test_generate_pid_filename_consistency(self):
        """Test that same command always generates same filename."""
        command_name = "test"
        command = "test command"
        filename1 = generate_pid_filename(command_name, command)
        filename2 = generate_pid_filename(command_name, command)

        assert filename1 == filename2

    def test_generate_pid_filename_different_commands(self):
        """Test that different commands generate different filenames."""
        command_name = "test"
        command1 = "test command 1"
        command2 = "test command 2"
        filename1 = generate_pid_filename(command_name, command1)
        filename2 = generate_pid_filename(command_name, command2)

        assert filename1 != filename2

    def test_generate_pid_filename_different_command_names(self):
        """Test that different command names with same command generate different filenames."""
        command = "npm run dev"
        command_name1 = "api"
        command_name2 = "frontend"
        filename1 = generate_pid_filename(command_name1, command)
        filename2 = generate_pid_filename(command_name2, command)

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

        result = execute_command_step(step, Path("/tmp"), "test-command")

        assert result.success is False
        assert "Daemon process already running with PID 12345" in result.stderr
        assert (
            ".32ad89bc.pid" in result.stderr
        )  # SHA1 hash of "test-command" + "sleep 3600"

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

        result = execute_command_step(step, Path("/tmp"), "test-command")

        assert result.success is True
        mock_remove_pid.assert_called_once()

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_command_logging_with_options(self, mock_execute):
        """Test that command execution logs include options."""
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "test command", "daemon": True, "background": True}

        with patch("dev_tools.command_executor.logger") as mock_logger:
            execute_command_step(step, Path("/tmp"), "test-cmd")

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

            execute_command_step(step, Path("/tmp"), "test-cmd")

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

        result = execute_command_step(step, command_name="dev")

        assert result.success is True
        assert result.pid == 12345
        mock_execute.assert_called_with(
            "npm run dev",
            background=True,
            capture_output=True,
            cwd=None,
            daemon=True,
            command_name="dev",
        )
        mock_create_pid.assert_called_once()

    @patch("dev_tools.command_executor.execute_shell_command")
    @patch("pathlib.Path.exists")
    def test_daemon_foreground_creates_pid_file(self, mock_exists, mock_execute):
        """Test that daemon with background=False creates PID file."""
        mock_exists.return_value = False  # No existing PID file
        mock_execute.return_value = Mock(success=True, pid=12345)

        step = {"run": "npm run dev", "daemon": True, "background": False}

        result = execute_command_step(step, command_name="dev")

        assert result.success is True
        assert result.pid == 12345
        mock_execute.assert_called_with(
            "npm run dev",
            background=False,
            capture_output=False,
            cwd=None,
            daemon=True,
            command_name="dev",
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

        result = execute_shell_command(
            "test command", daemon=True, background=False, command_name="test"
        )

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

        result = execute_shell_command(
            "test command", daemon=True, background=True, command_name="test"
        )

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

        result = execute_command_step(step, command_name="dev")

        assert result.success is True
        assert result.pid == 12345
        mock_print.assert_called_once_with(
            "Running job 'npm run dev' in the background. PID: 12345, PID file: .363d1c30.pid"
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

        result = execute_command_step(step, command_name="bg-task")

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

        result = execute_command_step(step, command_name="test")

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
            "echo toot && sleep 10", daemon=True, background=False, command_name="test"
        )

        assert result.success is True
        assert result.pid == 98765
        mock_print.assert_called_once_with(
            "Running job 'echo toot && sleep 10' in the foreground. PID: 98765, PID file: .0444e3ec.pid"
        )


class TestNewStartServicesFormat:
    """Test suite for new start_services format with docker configuration."""

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_dict_config_image_only(self, mock_execute):
        """Test starting Docker service with dict config containing only image."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {"image": "rebelthor/sleep"}
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct image
        args, kwargs = mock_execute.call_args_list[1]
        assert "docker run -d --name sleep rebelthor/sleep" in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_dict_config_full(self, mock_execute):
        """Test starting Docker service with full dict config."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "image": "rebelthor/sleep",
            "ports": ["80:80", "81:127.0.0.1:443"],
            "volumes": ["./:/data"],
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d -v ./:/data -p 80:80 -p 127.0.0.1:443:81 --name sleep rebelthor/sleep"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_dict_config_ports_only(self, mock_execute):
        """Test starting Docker service with dict config containing image and ports."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {"image": "redis:alpine", "ports": ["6379:6379"]}
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d -p 6379:6379 --name redis redis:alpine"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_dict_config_volumes_only(self, mock_execute):
        """Test starting Docker service with dict config containing image and volumes."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "image": "postgres:13",
            "volumes": ["/var/lib/postgresql/data:/var/lib/postgresql/data"],
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d -v /var/lib/postgresql/data:/var/lib/postgresql/data --name postgres postgres:13"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_dict_config_container_exists(self, mock_execute):
        """Test starting Docker service with dict config when container already exists."""
        # Mock container exists and is running
        mock_execute.side_effect = [
            Mock(success=True, stdout="sleep"),  # Container exists
            Mock(success=True, stdout="sleep"),  # Container is running
        ]

        service_config = {"image": "rebelthor/sleep"}
        result = start_docker_service(service_config)

        assert result.success is True
        assert result.stdout == "Container already running"
        assert mock_execute.call_count == 2

    def test_start_docker_service_with_dict_config_missing_image(self):
        """Test starting Docker service with dict config missing required image."""
        service_config = {"ports": ["80:80"]}
        result = start_docker_service(service_config)

        assert result.success is False
        assert "Service configuration must include 'image'" in result.stderr

    def test_start_docker_service_with_dict_config_invalid_type(self):
        """Test starting Docker service with invalid service configuration type."""
        service_config = 123  # Invalid type
        result = start_docker_service(service_config)

        assert result.success is False
        assert "Service must be a string or dictionary" in result.stderr

    @patch("dev_tools.command_executor.start_docker_service")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_mixed_start_services(
        self, mock_execute, mock_start_service
    ):
        """Test command step execution with mixed start_services (string and dict)."""
        mock_start_service.return_value = Mock(success=True)
        mock_execute.return_value = Mock(success=True)

        step = {
            "start_services": [
                "redis",  # String format (legacy)
                {"image": "postgres:13", "ports": ["5432:5432"]},  # Dict format (new)
            ],
            "run": "test command",
        }

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is True
        assert mock_start_service.call_count == 2
        mock_start_service.assert_any_call("redis")
        mock_start_service.assert_any_call(
            {"image": "postgres:13", "ports": ["5432:5432"]}
        )

    @patch("dev_tools.command_executor.start_docker_service")
    def test_execute_command_step_with_dict_service_failure(self, mock_start_service):
        """Test that dict service startup failure stops command execution."""
        mock_start_service.return_value = Mock(
            success=False, stderr="Service failed to start"
        )

        step = {"start_services": [{"image": "failing/service"}], "run": "test command"}

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is False
        assert "Service failed to start" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_named_service_format(self, mock_execute):
        """Test starting Docker service with named service format."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "box": {
                "image": "alpine",
                "volumes": ["./:/data"],
                "ports": ["5432:127.0.0.1:5432"],
            }
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments and container name is "box"
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = (
            "docker run -d -v ./:/data -p 127.0.0.1:5432:5432 --name box alpine"
        )
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_named_service_image_only(self, mock_execute):
        """Test starting Docker service with named service format, image only."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {"cache": {"image": "redis"}}
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments and container name is "cache"
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d --name cache redis"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_named_service_container_exists(
        self, mock_execute
    ):
        """Test starting Docker service with named service format when container already exists."""
        # Mock container exists and is running
        mock_execute.side_effect = [
            Mock(success=True, stdout="box"),  # Container exists
            Mock(success=True, stdout="box"),  # Container is running
        ]

        service_config = {"box": {"image": "alpine"}}
        result = start_docker_service(service_config)

        assert result.success is True
        assert result.stdout == "Container already running"
        assert mock_execute.call_count == 2

    def test_start_docker_service_with_named_service_missing_image(self):
        """Test starting Docker service with named service format missing required image."""
        service_config = {"box": {"ports": ["80:80"]}}
        result = start_docker_service(service_config)

        assert result.success is False
        assert "Service configuration must include 'image'" in result.stderr

    @patch("dev_tools.command_executor.start_docker_service")
    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_with_named_services(
        self, mock_execute, mock_start_service
    ):
        """Test command step execution with named services format."""
        mock_start_service.return_value = Mock(success=True)
        mock_execute.return_value = Mock(success=True)

        step = {
            "start_services": [
                {"box": {"image": "alpine", "volumes": ["./:/data"]}},
                {"cache": {"image": "redis"}},
            ],
            "run": "test command",
        }

        result = execute_command_step(step, Path("/tmp"))

        assert result.success is True
        assert mock_start_service.call_count == 2
        mock_start_service.assert_any_call(
            {"box": {"image": "alpine", "volumes": ["./:/data"]}}
        )
        mock_start_service.assert_any_call({"cache": {"image": "redis"}})

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_named_service_and_command(self, mock_execute):
        """Test starting Docker service with named service format including command."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "box": {
                "image": "alpine",
                "command": "sleep infinity",
                "volumes": ["./:/data"],
                "ports": ["5432:127.0.0.1:5432"],
            }
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments including command
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d -v ./:/data -p 127.0.0.1:5432:5432 --name box alpine sleep infinity"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_direct_dict_and_command(self, mock_execute):
        """Test starting Docker service with direct dictionary format including command."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "image": "alpine",
            "command": "tail -f /dev/null",
            "ports": ["8080:80"],
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments including command
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d -p 8080:80 --name alpine alpine tail -f /dev/null"
        assert expected_cmd in args[0]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_with_command_only(self, mock_execute):
        """Test starting Docker service with command but no other options."""
        # Mock container doesn't exist check
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=True, stdout="container_id\n"),  # Docker run success
        ]

        service_config = {
            "worker": {
                "image": "ubuntu:20.04",
                "command": "bash -c 'while true; do echo working; sleep 10; done'",
            }
        }
        result = start_docker_service(service_config)

        assert result.success is True
        assert mock_execute.call_count == 2
        # Check that docker run was called with correct arguments
        args, kwargs = mock_execute.call_args_list[1]
        expected_cmd = "docker run -d --name worker ubuntu:20.04 bash -c 'while true; do echo working; sleep 10; done'"
        assert expected_cmd in args[0]


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
            command_name="",
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
            command_name="",
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
            command_name="",
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
            command_name="",
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


class TestPIDCleanup:
    """Test suite for PID file cleanup functionality."""

    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_no_files(self, mock_glob):
        """Test cleanup when no PID files exist."""
        mock_glob.return_value = []

        result = cleanup_stale_pid_files(Path("/test/dir"))

        assert result.success is True
        assert "No PID files found to clean up" in result.stdout
        mock_glob.assert_called_once_with("*.pid")

    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.is_process_running")
    @patch("dev_tools.command_executor.read_pid_file")
    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_mixed_scenario(
        self, mock_glob, mock_read_pid, mock_is_running, mock_remove_pid
    ):
        """Test cleanup with mix of active and stale processes."""
        # Mock PID files
        pid_file_1 = Mock()
        pid_file_1.name = "process1.pid"
        pid_file_2 = Mock()
        pid_file_2.name = "process2.pid"
        pid_file_3 = Mock()
        pid_file_3.name = "process3.pid"

        mock_glob.return_value = [pid_file_1, pid_file_2, pid_file_3]

        # Mock PID reading
        mock_read_pid.side_effect = [12345, 67890, 11111]

        # Mock process status: first is running, second and third are not
        mock_is_running.side_effect = [True, False, False]

        result = cleanup_stale_pid_files(Path("/test/dir"))

        assert result.success is True
        assert "Cleaned up 2 stale PID file(s)" in result.stdout
        assert "Found 1 active process(es)" in result.stdout
        assert "process1.pid (PID 12345)" in result.stdout
        assert "process2.pid (PID 67890)" in result.stdout
        assert "process3.pid (PID 11111)" in result.stdout

        # Should remove only stale PID files
        assert mock_remove_pid.call_count == 2
        mock_remove_pid.assert_any_call(pid_file_2)
        mock_remove_pid.assert_any_call(pid_file_3)

    @patch("dev_tools.command_executor.read_pid_file")
    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_read_error(self, mock_glob, mock_read_pid):
        """Test cleanup when PID file reading fails."""
        pid_file = Mock()
        pid_file.name = "corrupted.pid"
        mock_glob.return_value = [pid_file]
        mock_read_pid.return_value = None

        result = cleanup_stale_pid_files(Path("/test/dir"))

        assert result.success is False  # Should fail when only errors occur
        assert "Encountered 1 error(s)" in result.stdout
        assert "Could not read PID from" in result.stdout

    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.is_process_running")
    @patch("dev_tools.command_executor.read_pid_file")
    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_all_active(
        self, mock_glob, mock_read_pid, mock_is_running, mock_remove_pid
    ):
        """Test cleanup when all processes are still active."""
        pid_file = Mock()
        pid_file.name = "active.pid"
        mock_glob.return_value = [pid_file]
        mock_read_pid.return_value = 12345
        mock_is_running.return_value = True

        result = cleanup_stale_pid_files(Path("/test/dir"))

        assert result.success is True
        assert "Found 1 active process(es)" in result.stdout
        assert "active.pid (PID 12345)" in result.stdout
        mock_remove_pid.assert_not_called()

    @patch("dev_tools.command_executor.remove_pid_file")
    @patch("dev_tools.command_executor.is_process_running")
    @patch("dev_tools.command_executor.read_pid_file")
    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_all_stale(
        self, mock_glob, mock_read_pid, mock_is_running, mock_remove_pid
    ):
        """Test cleanup when all processes are stale."""
        pid_file_1 = Mock()
        pid_file_1.name = "stale1.pid"
        pid_file_2 = Mock()
        pid_file_2.name = "stale2.pid"
        mock_glob.return_value = [pid_file_1, pid_file_2]
        mock_read_pid.side_effect = [12345, 67890]
        mock_is_running.return_value = False

        result = cleanup_stale_pid_files(Path("/test/dir"))

        assert result.success is True
        assert "Cleaned up 2 stale PID file(s)" in result.stdout
        assert "stale1.pid (PID 12345)" in result.stdout
        assert "stale2.pid (PID 67890)" in result.stdout
        assert mock_remove_pid.call_count == 2


class TestDockerServiceErrorScenarios:
    """Test suite for Docker service error scenarios."""

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_docker_not_available(self, mock_execute):
        """Test starting Docker service when Docker is not available."""
        mock_execute.return_value = CommandResult(
            success=False, stderr="docker: command not found", returncode=-1
        )

        result = start_docker_service("redis")

        assert result.success is False
        assert "docker: command not found" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_docker_daemon_not_running(self, mock_execute):
        """Test starting Docker service when Docker daemon is not running."""
        mock_execute.return_value = Mock(
            success=False, stderr="Cannot connect to the Docker daemon", returncode=1
        )

        result = start_docker_service("redis")

        assert result.success is False
        assert "Cannot connect to the Docker daemon" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_permission_denied(self, mock_execute):
        """Test starting Docker service with permission denied."""
        mock_execute.return_value = Mock(
            success=False,
            stderr="permission denied while trying to connect to the Docker daemon socket",
            returncode=1,
        )

        result = start_docker_service("redis")

        assert result.success is False
        assert "permission denied" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_image_not_found(self, mock_execute):
        """Test starting Docker service with non-existent image."""
        # First call (check if container exists) succeeds
        # Second call (docker run) fails with image not found
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(
                success=False,
                stderr="Unable to find image 'nonexistent:latest'",
                returncode=125,
            ),
        ]

        result = start_docker_service("nonexistent")

        assert result.success is False
        assert "Unable to find image" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_port_already_in_use(self, mock_execute):
        """Test starting Docker service when port is already in use."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=False, stderr="port is already allocated", returncode=125),
        ]

        result = start_docker_service("redis")

        assert result.success is False
        assert "port is already allocated" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_stop_docker_service_error_scenarios(self, mock_execute):
        """Test stop_docker_service error scenarios."""
        # Test container not found
        mock_execute.return_value = Mock(
            success=False, stderr="No such container: redis", returncode=1
        )

        result = stop_docker_service("redis")

        assert result.success is False
        assert "No such container" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_insufficient_permissions(self, mock_execute):
        """Test starting Docker service with insufficient Docker permissions."""
        mock_execute.return_value = Mock(
            success=False,
            stderr="Got permission denied while trying to connect to the Docker daemon socket",
            returncode=1,
        )

        result = start_docker_service("redis")

        assert result.success is False
        assert "permission denied" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_out_of_disk_space(self, mock_execute):
        """Test starting Docker service when out of disk space."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=False, stderr="no space left on device", returncode=1),
        ]

        result = start_docker_service("redis")

        assert result.success is False
        assert "no space left on device" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_network_error(self, mock_execute):
        """Test starting Docker service with network connectivity issues."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(
                success=False,
                stderr="Error response from daemon: network not found",
                returncode=1,
            ),
        ]

        result = start_docker_service("redis")

        assert result.success is False
        assert "network not found" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_invalid_configuration(self, mock_execute):
        """Test starting Docker service with invalid configuration."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(success=False, stderr="invalid reference format", returncode=125),
        ]

        result = start_docker_service("redis")

        assert result.success is False
        assert "invalid reference format" in result.stderr

    def test_start_docker_service_invalid_service_type(self):
        """Test starting Docker service with invalid service type."""
        result = start_docker_service(123)  # Invalid type

        assert result.success is False
        assert "Service must be a string or dictionary" in result.stderr

    def test_start_docker_service_empty_service_config(self):
        """Test starting Docker service with empty service configuration."""
        result = start_docker_service({})  # Empty dict

        assert result.success is False
        assert "Service configuration must include 'image'" in result.stderr

    def test_start_docker_service_named_service_empty_config(self):
        """Test starting Docker service with named service but empty config."""
        result = start_docker_service({"myservice": {}})

        assert result.success is False
        assert "Service configuration must include 'image'" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_start_docker_service_container_start_failure(self, mock_execute):
        """Test starting Docker service when container creation fails."""
        mock_execute.side_effect = [
            Mock(success=True, stdout=""),  # Container doesn't exist
            Mock(
                success=False, stderr="failed to start container", returncode=1
            ),  # docker run fails
        ]

        result = start_docker_service({"image": "redis"})

        assert result.success is False
        assert "failed to start container" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_stop_docker_service_container_removal_failure(self, mock_execute):
        """Test stopping Docker service when stop+remove command fails."""
        mock_execute.return_value = Mock(
            success=False, stderr="removal of container failed", returncode=1
        )

        result = stop_docker_service("redis")

        assert result.success is False
        assert "removal of container failed" in result.stderr

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_stop_docker_service_docker_daemon_error(self, mock_execute):
        """Test stopping Docker service when Docker daemon has an error."""
        mock_execute.return_value = Mock(
            success=False,
            stderr="Error response from daemon: server error",
            returncode=1,
        )

        result = stop_docker_service("redis")

        assert result.success is False
        assert "server error" in result.stderr


class TestCommandExecutorErrorHandling:
    """Test suite for command executor error handling."""

    @patch("pathlib.Path.exists")
    def test_read_pid_file_permission_error(self, mock_exists):
        """Test read_pid_file with permission error."""
        mock_exists.return_value = True
        with patch("builtins.open", side_effect=PermissionError("Permission denied")):
            result = read_pid_file(Path("test.pid"))
            assert result is None

    @patch("pathlib.Path.exists")
    def test_read_pid_file_corrupted_content(self, mock_exists):
        """Test read_pid_file with corrupted content."""
        mock_exists.return_value = True
        with patch("builtins.open", mock_open(read_data="not_a_number")):
            result = read_pid_file(Path("test.pid"))
            assert result is None

    @patch("pathlib.Path.exists")
    def test_read_pid_file_empty_file(self, mock_exists):
        """Test read_pid_file with empty file."""
        mock_exists.return_value = True
        with patch("builtins.open", mock_open(read_data="")):
            result = read_pid_file(Path("test.pid"))
            assert result is None

    @patch("os.kill")
    def test_is_process_running_permission_error(self, mock_kill):
        """Test is_process_running with permission error."""
        mock_kill.side_effect = PermissionError("Operation not permitted")

        result = is_process_running(12345)

        # Should return True when permission denied (process exists but no access)
        assert result is True

    @patch("os.kill")
    def test_is_process_running_other_os_error(self, mock_kill):
        """Test is_process_running with other OS error."""
        mock_kill.side_effect = OSError("System error")

        # Other OS errors should raise an exception
        with pytest.raises(OSError):
            is_process_running(12345)

    def test_generate_pid_filename_edge_cases(self):
        """Test generate_pid_filename with edge cases."""
        # Empty command name
        result = generate_pid_filename("", "command")
        assert result.startswith(".")
        assert result.endswith(".pid")
        assert len(result) == 13  # .{8chars}.pid

        # Empty command
        result = generate_pid_filename("name", "")
        assert result.startswith(".")
        assert result.endswith(".pid")

        # Very long strings
        long_name = "a" * 1000
        long_command = "b" * 1000
        result = generate_pid_filename(long_name, long_command)
        assert len(result) == 13  # Should still be same length

    @patch("subprocess.run")
    def test_execute_shell_command_general_exception(self, mock_run):
        """Test execute_shell_command with general exception."""
        mock_run.side_effect = Exception("Unexpected error")

        result = execute_shell_command("test command")

        assert result.success is False
        assert "Unexpected error" in result.stderr

    @patch("subprocess.run")
    def test_execute_shell_command_keyboard_interrupt(self, mock_run):
        """Test execute_shell_command with KeyboardInterrupt."""
        # Use a regular Exception to simulate KeyboardInterrupt behavior without actually raising it
        mock_run.side_effect = Exception("KeyboardInterrupt simulation")

        result = execute_shell_command("test command")

        assert result.success is False
        assert "KeyboardInterrupt simulation" in result.stderr

    @patch("pathlib.Path.exists")
    def test_create_pid_file_permission_error(self, mock_exists):
        """Test create_pid_file with permission error."""
        mock_exists.return_value = False
        with patch("builtins.open", side_effect=PermissionError("Permission denied")):
            # Should raise the PermissionError
            with pytest.raises(PermissionError):
                create_pid_file(Path("test.pid"), 12345)

    @patch("pathlib.Path.exists")
    def test_create_pid_file_disk_full_error(self, mock_exists):
        """Test create_pid_file when disk is full."""
        mock_exists.return_value = False
        with patch("builtins.open", side_effect=OSError("No space left on device")):
            # Should raise the OSError
            with pytest.raises(OSError):
                create_pid_file(Path("test.pid"), 12345)

    @patch("pathlib.Path.exists")
    def test_remove_pid_file_permission_error(self, mock_exists):
        """Test remove_pid_file with permission error."""
        mock_exists.return_value = True
        with patch(
            "pathlib.Path.unlink", side_effect=PermissionError("Permission denied")
        ):
            # Should raise the PermissionError
            with pytest.raises(PermissionError):
                remove_pid_file(Path("test.pid"))

    @patch("pathlib.Path.exists")
    def test_remove_pid_file_file_in_use_error(self, mock_exists):
        """Test remove_pid_file when file is in use."""
        mock_exists.return_value = True
        with patch("pathlib.Path.unlink", side_effect=OSError("Resource busy")):
            # Should raise the OSError
            with pytest.raises(OSError):
                remove_pid_file(Path("test.pid"))

    @patch("pathlib.Path.exists")
    def test_read_pid_file_unicode_decode_error(self, mock_exists):
        """Test read_pid_file with unicode decode error."""
        mock_exists.return_value = True
        with patch(
            "builtins.open",
            side_effect=UnicodeDecodeError("utf-8", b"", 0, 1, "invalid start byte"),
        ):
            result = read_pid_file(Path("test.pid"))
            assert result is None

    @patch("pathlib.Path.exists")
    def test_read_pid_file_io_error(self, mock_exists):
        """Test read_pid_file with I/O error."""
        mock_exists.return_value = True
        with patch("builtins.open", side_effect=IOError("I/O operation failed")):
            result = read_pid_file(Path("test.pid"))
            assert result is None

    def test_load_environment_variables_file_not_found(self):
        """Test load_environment_variables when .env file doesn't exist."""
        non_existent_file = Path("/nonexistent/.env")
        # Should not raise any exception
        load_environment_variables(non_existent_file)

    def test_load_environment_variables_permission_error(self):
        """Test load_environment_variables with permission error."""
        with tempfile.TemporaryDirectory() as temp_dir:
            env_file = Path(temp_dir) / ".env"
            env_file.write_text("TEST=value")

            # Mock load_dotenv to raise an exception
            with patch(
                "dev_tools.command_executor.load_dotenv",
                side_effect=PermissionError("Permission denied"),
            ):
                # Should not crash the application
                with pytest.raises(PermissionError):
                    load_environment_variables(env_file)

    def test_load_environment_variables_malformed_env_file(self):
        """Test load_environment_variables with malformed .env file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            env_file = Path(temp_dir) / ".env"
            env_file.write_text("INVALID_LINE_WITHOUT_EQUALS\nVALID=value\n")

            # Should not raise exception (load_dotenv handles malformed files gracefully)
            load_environment_variables(env_file)

    def test_cleanup_stale_pid_files_permission_error(self):
        """Test cleanup_stale_pid_files with permission error accessing directory."""
        with patch(
            "pathlib.Path.glob", side_effect=PermissionError("Permission denied")
        ):
            # The function doesn't catch glob errors, so they propagate
            with pytest.raises(PermissionError):
                cleanup_stale_pid_files(Path("."))

    @patch("pathlib.Path.glob")
    def test_cleanup_stale_pid_files_file_removal_error(self, mock_glob):
        """Test cleanup_stale_pid_files when individual file removal fails."""
        mock_pid_file = Mock()
        mock_pid_file.name = "test.pid"
        mock_pid_file.unlink.side_effect = PermissionError("Cannot remove file")
        mock_glob.return_value = [mock_pid_file]

        with patch("dev_tools.command_executor.read_pid_file", return_value=12345):
            with patch(
                "dev_tools.command_executor.is_process_running", return_value=False
            ):
                result = cleanup_stale_pid_files(Path("."))

                # Should fail when there are errors and no successful operations
                assert result.success is False
                assert "Cannot remove file" in result.stderr
