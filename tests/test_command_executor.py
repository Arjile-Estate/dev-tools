"""Tests for command execution functionality."""

import os
import subprocess
from pathlib import Path
from unittest.mock import Mock, patch

from dev_tools.command_executor import (
    create_pid_file,
    execute_command_step,
    execute_shell_command,
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
    def test_start_docker_service_already_running(self, mock_execute):
        """Test starting Docker service that's already running."""
        mock_execute.return_value = Mock(success=False, stderr="already in use")

        result = start_docker_service("redis")

        assert result.success is True
        assert result.stdout == "Service already running"

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
            "pytest tests/", background=False, capture_output=False, cwd=None
        )

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_execute_command_step_multiple_commands(self, mock_execute):
        """Test executing command step with multiple run commands."""
        mock_execute.return_value = Mock(success=True)

        step = {"run": ["eslint src/", "black --check ."]}

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
            "npm run dev", background=True, capture_output=True, cwd=None
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
