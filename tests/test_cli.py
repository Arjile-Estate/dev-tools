"""Tests for CLI interface functionality."""

import argparse
import tempfile
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

    def test_create_argument_parser_project_dir_flag(self):
        """Test --project-dir flag parsing."""
        parser = create_argument_parser()

        args = parser.parse_args(["--project-dir", "/some/path", "test"])
        assert args.project_dir == Path("/some/path")

        args = parser.parse_args(["-p", "/other/path", "test"])
        assert args.project_dir == Path("/other/path")

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_respects_project_dir_for_help(
        self, mock_load_config
    ):
        """Test that create_argument_parser loads config from project_dir when showing help."""

        # Mock different configs for different directories
        def mock_load_config_side_effect(project_dir):
            if str(project_dir) == "/custom/project":
                return {"commands": {"custom-command": [{"run": "echo custom"}]}}
            else:
                return {"commands": {"default-command": [{"run": "echo default"}]}}

        mock_load_config.side_effect = mock_load_config_side_effect

        # Test with default directory
        parser = create_argument_parser()
        help_text = parser.format_help()
        assert "default-command" in help_text or "logs" in help_text

        # Test with custom project directory
        parser = create_argument_parser(Path("/custom/project"))
        help_text = parser.format_help()
        assert "custom-command" in help_text

    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("sys.argv", ["dev-tools", "--project-dir", "/custom/project", "--help"])
    def test_main_help_with_project_dir_flag(self, mock_load_config):
        """Test that main() respects --project-dir when showing help."""

        # Mock different configs for different directories
        def mock_load_config_side_effect(project_dir):
            if str(project_dir) == "/custom/project":
                return {"commands": {"custom-command": [{"run": "echo custom"}]}}
            else:
                return {"commands": {"default-command": [{"run": "echo default"}]}}

        mock_load_config.side_effect = mock_load_config_side_effect

        # The SystemExit from --help should be raised
        with pytest.raises(SystemExit):
            main()

        # Verify that the custom project config was loaded
        mock_load_config.assert_called_with(Path("/custom/project"))


class TestLogsCommand:
    """Test suite for logs command handling."""

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    @patch("sys.argv", ["dev-tools.py", "logs"])
    def test_handle_logs_command_with_dev_tools_py_script(
        self, mock_execute, mock_exists
    ):
        """Test logs command with dev-tools.py script name uses system log path."""
        mock_exists.return_value = True
        mock_execute.return_value = Mock(success=True, stdout="log content")

        with patch("dev_tools.cli.Path.home") as mock_home:
            mock_home.return_value = Path("/Users/testuser")
            result = handle_logs_command()

        assert result.success is True
        expected_log_path = "/Users/testuser/Library/Logs/dev-tools.log"
        mock_execute.assert_called_once_with(
            f"tail -n 50 {expected_log_path}", capture_output=True
        )

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    @patch("sys.argv", ["other-script.py", "logs"])
    def test_handle_logs_command_with_current_dir_log(self, mock_execute, mock_exists):
        """Test logs command with non-dev-tools script name uses current directory log."""
        mock_exists.return_value = True
        mock_execute.return_value = Mock(success=True, stdout="log content")

        result = handle_logs_command()

        assert result.success is True
        mock_execute.assert_called_once_with(
            "tail -n 50 activity.log", capture_output=True
        )

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    @patch("sys.argv", ["dev-tools.py", "logs"])
    def test_handle_logs_command_no_activity_log(self, mock_execute, mock_exists):
        """Test logs command when activity.log doesn't exist."""
        mock_exists.return_value = False

        result = handle_logs_command()

        assert result.success is False
        assert "No log file found" in result.stderr
        mock_execute.assert_not_called()

    @patch("pathlib.Path.exists")
    @patch("dev_tools.cli.execute_shell_command")
    @patch("sys.argv", ["dev-tools", "logs"])
    def test_handle_logs_command_system_log_path(self, mock_execute, mock_exists):
        """Test logs command uses system log path when command name is 'dev-tools'."""
        mock_exists.return_value = True
        mock_execute.return_value = Mock(success=True, stdout="log content")

        with patch("dev_tools.cli.Path.home") as mock_home:
            mock_home.return_value = Path("/Users/testuser")
            result = handle_logs_command()

        assert result.success is True
        expected_log_path = "/Users/testuser/Library/Logs/dev-tools.log"
        mock_execute.assert_called_once_with(
            f"tail -n 50 {expected_log_path}", capture_output=True
        )


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
        mock_handle_logs.assert_called_once_with(Path("."))
        mock_print.assert_called_with("log output")


class TestCLIExceptionHandling:
    """Test suite for CLI exception handling."""

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_config_loading_failure(self, mock_load_config):
        """Test create_argument_parser when config loading fails."""
        # Mock config loading failure
        mock_load_config.side_effect = Exception("Config loading failed")

        # Should handle exception gracefully and return parser with fallback commands
        parser = create_argument_parser()

        assert parser is not None
        # Should have fallback commands
        help_text = parser.format_help()
        assert "logs" in help_text

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_permission_error(self, mock_load_config):
        """Test create_argument_parser when permission error occurs."""
        mock_load_config.side_effect = PermissionError("Permission denied")

        parser = create_argument_parser()
        assert parser is not None

    @patch("dev_tools.cli.handle_command_execution")
    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.create_argument_parser")
    def test_main_general_exception(self, mock_parser, mock_logging, mock_handle):
        """Test main function with general exception."""
        mock_args = Mock()
        mock_args.command = "test"
        mock_args.project_dir = Path(".")
        mock_args.verbose = False
        mock_args.version = False

        mock_parser.return_value.parse_args.return_value = mock_args
        mock_handle.side_effect = Exception("General error")

        with pytest.raises(SystemExit) as exc_info:
            main()

        assert exc_info.value.code == 1

    @patch("dev_tools.cli.create_argument_parser")
    def test_main_argument_parsing_error(self, mock_parser):
        """Test main function when argument parsing fails."""
        mock_parser.return_value.parse_args.side_effect = SystemExit(2)

        with pytest.raises(SystemExit) as exc_info:
            main()

        assert exc_info.value.code == 2

    def test_handle_command_execution_invalid_project_dir(self):
        """Test handle_command_execution with invalid project directory."""
        invalid_dir = Path("/nonexistent/directory")

        result = handle_command_execution("test", invalid_dir)
        assert result.success is False
        # When project dir doesn't exist, it loads default config which only has 'logs'
        assert "Unknown command 'test'" in result.stderr

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_handle_command_execution_config_loading_exception(self, mock_load_config):
        """Test handle_command_execution when config loading throws exception."""
        mock_load_config.side_effect = OSError("Disk error")

        # Exception should propagate up since CLI doesn't handle it
        with pytest.raises(OSError):
            handle_command_execution("test", Path("."))

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    def test_handle_command_execution_command_execution_failure(
        self, mock_load_config, mock_execute
    ):
        """Test handle_command_execution when command execution fails."""
        mock_load_config.return_value = {"commands": {"test": [{"run": "pytest"}]}}
        mock_execute.return_value = Mock(success=False, stderr="Command failed")

        result = handle_command_execution("test", Path("."))
        assert result.success is False


class TestCLIEdgeCases:
    """Test suite for CLI edge cases."""

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_empty_commands(self, mock_load_config):
        """Test create_argument_parser with empty commands."""
        mock_load_config.return_value = {"commands": {}}

        parser = create_argument_parser()
        help_text = parser.format_help()

        # Should still have logs command as fallback
        assert "logs" in help_text

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_none_config(self, mock_load_config):
        """Test create_argument_parser when config is None."""
        mock_load_config.return_value = None

        parser = create_argument_parser()
        assert parser is not None

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_create_argument_parser_malformed_config(self, mock_load_config):
        """Test create_argument_parser with malformed config."""
        mock_load_config.return_value = {"not_commands": {"test": []}}

        parser = create_argument_parser()
        help_text = parser.format_help()

        # Should handle malformed config gracefully
        assert "logs" in help_text

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_handle_command_execution_empty_command_steps(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test handle_command_execution with empty command steps."""
        mock_load_config.return_value = {"commands": {"test": []}}
        mock_execute.return_value = Mock(success=True)

        # Should not raise exception
        result = handle_command_execution("test", Path("."))
        assert result.success is True

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_handle_command_execution_command_not_list(self, mock_load_config):
        """Test handle_command_execution when command is not a list."""
        mock_load_config.return_value = {"commands": {"test": "not a list"}}

        # Should raise AttributeError when trying to call .get() on string
        with pytest.raises(AttributeError):
            handle_command_execution("test", Path("."))

    @patch("dev_tools.cli.setup_application_logging")
    def test_logging_setup_error_handling(self, mock_logging):
        """Test main function when logging setup fails."""
        mock_logging.side_effect = PermissionError("Cannot write log file")

        # Exception should propagate since main doesn't handle it
        with patch("dev_tools.cli.create_argument_parser") as mock_parser:
            mock_args = Mock()
            mock_args.command = "test"
            mock_args.project_dir = Path(".")
            mock_args.verbose = False
            mock_parser.return_value.parse_args.return_value = mock_args

            with pytest.raises(PermissionError):
                main()


class TestCLIIntegrationScenarios:
    """Test suite for CLI integration scenarios."""

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_logs_command_with_nonexistent_file(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test logs command when activity.log doesn't exist."""
        mock_load_config.return_value = {
            "commands": {"logs": [{"run": "tail -n 50 activity.log"}]}
        }
        mock_execute.return_value = Mock(success=False, stderr="File not found")

        result = handle_command_execution("logs", Path("."))
        assert result.success is False

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_special_characters_in_project_path(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test handling project paths with special characters."""
        mock_load_config.return_value = {"commands": {"test": [{"run": "echo test"}]}}
        mock_execute.return_value = Mock(success=True)

        with tempfile.TemporaryDirectory() as temp_dir:
            # Create directory with special characters
            special_dir = Path(temp_dir) / "project with spaces & symbols"
            special_dir.mkdir()

            # Should handle special characters in path
            result = handle_command_execution("test", special_dir)
            assert result.success is True

    @patch("dev_tools.cli.load_configuration_for_project")
    def test_very_long_command_name(self, mock_load_config):
        """Test handling very long command names."""
        long_command = "a" * 1000  # Very long command name
        mock_load_config.return_value = {"commands": {"test": [{"run": "echo test"}]}}

        result = handle_command_execution(long_command, Path("."))
        assert result.success is False

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.cleanup_stale_pid_files")
    @patch("sys.argv", ["dev-tools.py", "cleanup-pids"])
    def test_main_cleanup_pids_command(self, mock_cleanup, mock_setup_logging):
        """Test main function with cleanup-pids command."""
        mock_cleanup.return_value = Mock(
            success=True, stdout="Cleaned up 2 stale PID files"
        )

        with patch("builtins.print") as mock_print:
            main()

        mock_setup_logging.assert_called_once()
        mock_cleanup.assert_called_once_with(Path("."))
        mock_print.assert_called_with("Cleaned up 2 stale PID files")

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

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_python_project_test_command(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow for Python project test command."""
        # Simulate Python project configuration
        mock_load_config.return_value = {
            "commands": {
                "test": [
                    {"start_services": ["redis"]},
                    {"run": "pytest tests/"},
                    {"cleanup": True},
                ]
            }
        }
        mock_execute.return_value = Mock(success=True, stdout="All tests passed")

        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            # Create pyproject.toml to make it a Python project
            (project_dir / "pyproject.toml").write_text('[project]\nname = "test"')

            result = handle_command_execution("test", project_dir)

            assert result.success is True
            mock_load_env.assert_called_once()
            mock_execute.assert_called_once()

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_nodejs_project_build_command(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow for Node.js project build command."""
        # Simulate Node.js project configuration
        mock_load_config.return_value = {
            "commands": {
                "build": [
                    {"run": "npm ci"},
                    {"run": "npm run build"},
                    {"directory": "./dist"},
                ]
            }
        }
        mock_execute.return_value = Mock(success=True, stdout="Build completed")

        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            # Create package.json to make it a Node.js project
            (project_dir / "package.json").write_text(
                '{"name": "test", "scripts": {"build": "webpack"}}'
            )

            result = handle_command_execution("build", project_dir)

            assert result.success is True
            mock_execute.assert_called_once()

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_with_daemon_background_process(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow with daemon background process."""
        mock_load_config.return_value = {
            "commands": {
                "dev": [
                    {"start_services": [{"db": {"image": "postgres"}}]},
                    {"run": "npm run dev", "background": True, "daemon": True},
                ]
            }
        }
        mock_execute.return_value = Mock(
            success=True, pid=12345, stdout="Development server started"
        )

        result = handle_command_execution("dev", Path("."))

        assert result.success is True
        mock_execute.assert_called_once()

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_multiple_step_command_with_failure(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow with multiple steps where one step fails."""
        mock_load_config.return_value = {
            "commands": {
                "deploy": [
                    {"run": "npm test"},
                    {"run": "npm run build"},
                    {"run": "docker build -t myapp ."},
                    {"run": "docker push myapp"},
                ]
            }
        }
        # Simulate failure in the build step
        mock_execute.return_value = Mock(success=False, stderr="Build failed")

        result = handle_command_execution("deploy", Path("."))

        assert result.success is False

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_with_env_file_loading(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow that loads environment variables from .env file."""
        mock_load_config.return_value = {
            "commands": {"test": [{"run": "echo $DATABASE_URL"}]}
        }
        mock_execute.return_value = Mock(
            success=True, stdout="postgresql://localhost:5432/test"
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            # Create .env file
            (project_dir / ".env").write_text(
                "DATABASE_URL=postgresql://localhost:5432/test\n"
            )

            result = handle_command_execution("test", project_dir)

            assert result.success is True
            mock_load_env.assert_called_once()

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    @patch("sys.argv", ["dev-tools.py", "--verbose", "test"])
    def test_complete_cli_workflow_with_verbose_logging(
        self, mock_load_env, mock_load_config, mock_execute, mock_setup_logging
    ):
        """Test complete CLI workflow from argument parsing to command execution with verbose logging."""
        mock_load_config.return_value = {
            "commands": {"test": [{"run": "pytest tests/"}]}
        }
        mock_execute.return_value = Mock(success=True, stdout="All tests passed")

        with patch(
            "dev_tools.cli.handle_command_execution",
            return_value=Mock(success=True, stdout="All tests passed"),
        ) as mock_handle:
            with patch("builtins.print") as mock_print:
                main()

        # Verify full workflow
        mock_setup_logging.assert_called_once()
        mock_handle.assert_called_once_with("test", Path("."))
        mock_print.assert_called_with("All tests passed")

        # Verify verbose logging was enabled
        call_args = mock_setup_logging.call_args[1]
        assert call_args.get("verbose") is True

    @patch("dev_tools.cli.setup_application_logging")
    @patch("dev_tools.cli.cleanup_stale_pid_files")
    @patch("sys.argv", ["dev-tools.py", "cleanup-pids"])
    def test_complete_cli_workflow_cleanup_pids_command(
        self, mock_cleanup, mock_setup_logging
    ):
        """Test complete CLI workflow for cleanup-pids command."""
        mock_cleanup.return_value = Mock(
            success=True,
            stdout="Cleaned up 3 stale PID files: test1.pid, test2.pid, test3.pid",
        )

        with patch("builtins.print") as mock_print:
            main()

        # Verify full workflow
        mock_setup_logging.assert_called_once()
        mock_cleanup.assert_called_once_with(Path("."))
        mock_print.assert_called_with(
            "Cleaned up 3 stale PID files: test1.pid, test2.pid, test3.pid"
        )

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_rust_project_with_custom_config(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow for Rust project with custom configuration."""
        mock_load_config.return_value = {
            "commands": {
                "test": [
                    {"run": "cargo fmt --check"},
                    {"run": "cargo clippy -- -D warnings"},
                    {"run": "cargo test --release"},
                ],
                "build": [{"run": "cargo build --release"}],
            }
        }
        mock_execute.return_value = Mock(
            success=True, stdout="Tests completed successfully"
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            # Create Cargo.toml to make it a Rust project
            (project_dir / "Cargo.toml").write_text(
                '[package]\nname = "test"\nversion = "0.1.0"'
            )

            result = handle_command_execution("test", project_dir)

            assert result.success is True
            mock_execute.assert_called_once()

    @patch("dev_tools.cli.execute_command_with_steps")
    @patch("dev_tools.cli.load_configuration_for_project")
    @patch("dev_tools.cli.load_environment_variables")
    def test_full_workflow_error_recovery_and_cleanup(
        self, mock_load_env, mock_load_config, mock_execute
    ):
        """Test full workflow error recovery and cleanup behavior."""
        mock_load_config.return_value = {
            "commands": {
                "deploy": [
                    {"start_services": ["database", "redis"]},
                    {"run": "npm test"},
                    {"run": "npm run build"},
                    {"cleanup": True},
                ]
            }
        }

        # Simulate failure after services are started
        mock_execute.return_value = Mock(
            success=False, stderr="Deployment failed during build step"
        )

        result = handle_command_execution("deploy", Path("."))

        assert result.success is False
        mock_execute.assert_called_once()
        # Verify that cleanup would be attempted even on failure
        call_args = mock_execute.call_args
        assert call_args[0][1]  # command steps were passed to executor


class TestLoggingPathLogic:
    """Test suite for conditional log file path logic."""

    @patch("dev_tools.cli.setup_application_logging")
    @patch("sys.argv")
    def test_main_uses_system_log_path_for_dev_tools_command(
        self, mock_argv, mock_setup_logging
    ):
        """Test that 'dev-tools' command name uses ~/Library/Logs/dev-tools.log path."""
        mock_argv.__getitem__.side_effect = lambda x: ["dev-tools", "test"][x]
        mock_argv.__len__.return_value = 2

        with patch("dev_tools.cli.handle_command_execution") as mock_handle:
            mock_handle.return_value = Mock(success=True)
            with patch("dev_tools.cli.Path.home") as mock_home:
                with patch("pathlib.Path.mkdir") as mock_mkdir:
                    mock_home.return_value = Path("/Users/testuser")
                    main()

        # Verify setup_application_logging was called with system log path
        call_args = mock_setup_logging.call_args
        expected_log_path = Path("/Users/testuser/Library/Logs/dev-tools.log")
        assert call_args[1]["log_file"] == expected_log_path
        # Verify directory creation was attempted
        mock_mkdir.assert_called_once_with(parents=True, exist_ok=True)

    @patch("dev_tools.cli.setup_application_logging")
    @patch("sys.argv", ["other-command", "test"])
    def test_main_uses_current_dir_log_path_for_other_commands(
        self, mock_setup_logging
    ):
        """Test that non 'dev-tools' command names use activity.log in current directory."""
        with patch("dev_tools.cli.handle_command_execution") as mock_handle:
            mock_handle.return_value = Mock(success=True)
            main()

        # Verify setup_application_logging was called with current directory log path
        call_args = mock_setup_logging.call_args
        # Should use default log_file=None, which defaults to activity.log
        assert call_args[1].get("log_file") is None

    @patch("dev_tools.cli.setup_application_logging")
    @patch("sys.argv", ["dev-tools.py", "test"])
    def test_main_uses_system_log_path_for_dev_tools_py(self, mock_setup_logging):
        """Test that 'dev-tools.py' script name uses system log path since stem is 'dev-tools'."""
        with patch("dev_tools.cli.handle_command_execution") as mock_handle:
            mock_handle.return_value = Mock(success=True)
            with patch("dev_tools.cli.Path.home") as mock_home:
                with patch("pathlib.Path.mkdir"):
                    mock_home.return_value = Path("/Users/testuser")
                    main()

        # Verify setup_application_logging was called with system log path
        call_args = mock_setup_logging.call_args
        expected_log_path = Path("/Users/testuser/Library/Logs/dev-tools.log")
        assert call_args[1]["log_file"] == expected_log_path
