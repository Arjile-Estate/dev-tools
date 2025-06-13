"""Tests for CLI interface functionality."""

import argparse
from pathlib import Path
from unittest.mock import Mock, patch

import pytest

from dev_tools.cli import (
    create_argument_parser,
    handle_command_execution,
    handle_logs_command,
    main,
)


class TestArgumentParser:
    """Test suite for CLI argument parsing."""

    def test_create_argument_parser_basic(self):
        """Test basic argument parser creation."""
        parser = create_argument_parser()

        assert isinstance(parser, argparse.ArgumentParser)

        # Test with no arguments (should show help)
        with pytest.raises(SystemExit):
            parser.parse_args([])

    def test_create_argument_parser_verbose_flag(self):
        """Test --verbose flag parsing."""
        parser = create_argument_parser()

        args = parser.parse_args(["--verbose", "test"])
        assert args.verbose is True

        args = parser.parse_args(["test"])
        assert args.verbose is False

    def test_create_argument_parser_command_argument(self):
        """Test command argument parsing."""
        parser = create_argument_parser()

        args = parser.parse_args(["test"])
        assert args.command == "test"

        args = parser.parse_args(["build"])
        assert args.command == "build"


class TestLogsCommand:
    """Test suite for logs command handling."""

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    def test_handle_logs_command_with_activity_log(self, mock_execute, mock_exists):
        """Test logs command when activity.log exists."""
        mock_exists.return_value = True
        mock_execute.return_value = Mock(success=True, stdout="log content")

        result = handle_logs_command()

        assert result.success is True
        mock_execute.assert_called_once_with(
            "tail -n 50 activity.log", capture_output=True
        )

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    def test_handle_logs_command_no_activity_log(self, mock_execute, mock_exists):
        """Test logs command when activity.log doesn't exist."""
        mock_exists.return_value = False

        result = handle_logs_command()

        assert result.success is False
        assert "No activity.log file found" in result.stderr
        mock_execute.assert_not_called()


class TestCommandExecution:
    """Test suite for command execution handling."""

    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_environment_variables")
    def test_handle_command_execution_success(
        self, mock_load_env, mock_execute, mock_load_config
    ):
        """Test successful command execution."""
        mock_load_config.return_value = {
            "commands": {"test": [{"run": "pytest tests/"}]}
        }
        mock_execute.return_value = Mock(success=True)

        result = handle_command_execution("test", Path("."))

        assert result.success is True
        mock_load_env.assert_called_once()
        mock_execute.assert_called_once_with(
            "test", [{"run": "pytest tests/"}], Path(".")
        )

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_handle_command_execution_unknown_command(self, mock_load_config):
        """Test handling of unknown command."""
        mock_load_config.return_value = {
            "commands": {"test": [{"run": "pytest tests/"}]}
        }

        result = handle_command_execution("unknown", Path("."))

        assert result.success is False
        assert "Unknown command 'unknown'" in result.stderr

    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_environment_variables")
    def test_handle_command_execution_failure(
        self, mock_load_env, mock_execute, mock_load_config
    ):
        """Test command execution failure."""
        mock_load_config.return_value = {
            "commands": {"test": [{"run": "pytest tests/"}]}
        }
        mock_execute.return_value = Mock(success=False, stderr="test failed")

        result = handle_command_execution("test", Path("."))

        assert result.success is False


class TestMainFunction:
    """Test suite for main CLI function."""

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.handle_logs_command")
    @patch("sys.argv", ["dev-tools.py", "logs"])
    def test_main_logs_command(self, mock_handle_logs, mock_setup_logging):
        """Test main function with logs command."""
        mock_handle_logs.return_value = Mock(success=True, stdout="log output")

        with patch("builtins.print") as mock_print:
            main()

        mock_setup_logging.assert_called_once()
        mock_handle_logs.assert_called_once()
        mock_print.assert_called_with("log output")

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.handle_command_execution")
    @patch("sys.argv", ["dev-tools.py", "test"])
    def test_main_regular_command(self, mock_handle_command, mock_setup_logging):
        """Test main function with regular command."""
        mock_handle_command.return_value = Mock(success=True, stdout="command output")

        with patch("builtins.print"):
            main()

        mock_setup_logging.assert_called_once()
        mock_handle_command.assert_called_once_with("test", Path("."))

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.handle_command_execution")
    @patch("sys.argv", ["dev-tools.py", "--verbose", "test"])
    def test_main_with_verbose_flag(self, mock_handle_command, mock_setup_logging):
        """Test main function with verbose flag."""
        mock_handle_command.return_value = Mock(success=True)

        main()

        mock_setup_logging.assert_called_once()
        # Check that verbose=True was passed to setup_application_logging
        call_args = mock_setup_logging.call_args[1]
        assert call_args.get("verbose") is True

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.handle_command_execution")
    @patch("sys.argv", ["dev-tools.py", "failing-command"])
    def test_main_command_failure(self, mock_handle_command, mock_setup_logging):
        """Test main function with failing command."""
        mock_handle_command.return_value = Mock(success=False, stderr="command failed")

        with patch("sys.exit") as mock_exit:
            with patch("builtins.print") as mock_print:
                main()

        mock_print.assert_called_with("command failed")
        mock_exit.assert_called_once_with(1)
