"""Integration tests for dev-tools full workflow."""

import os
import tempfile
from pathlib import Path
from unittest.mock import Mock, patch

import yaml

from dev_tools.cli import handle_command_execution
from dev_tools.command_executor import execute_command_with_steps
from dev_tools.config_parser import load_configuration_for_project


class TestFullWorkflowIntegration:
    """Integration tests for complete dev-tools workflow."""

    def test_python_project_detection_and_defaults(self):
        """Test full workflow for Python project detection and default commands."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create pyproject.toml to make it a Python project
            (project_dir / "pyproject.toml").write_text(
                """
[project]
name = "test-project"
version = "0.1.0"
"""
            )

            config = load_configuration_for_project(project_dir)

            assert "commands" in config
            assert "test" in config["commands"]
            assert "logs" in config["commands"]
            assert any("pytest" in str(cmd) for cmd in config["commands"]["test"])

    def test_nodejs_project_detection_and_defaults(self):
        """Test full workflow for Node.js project detection and default commands."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create package.json to make it a Node.js project
            (project_dir / "package.json").write_text(
                """
{
  "name": "test-project",
  "version": "1.0.0"
}
"""
            )

            config = load_configuration_for_project(project_dir)

            assert "commands" in config
            assert "test" in config["commands"]
            assert "build" in config["commands"]
            assert "lint" in config["commands"]
            assert (
                "dev" not in config["commands"]
            )  # dev command should not be included by default
            assert any("npm test" in str(cmd) for cmd in config["commands"]["test"])

    def test_custom_config_overrides_defaults(self):
        """Test that custom .dev-config.yaml overrides default commands."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create pyproject.toml for Python project
            (project_dir / "pyproject.toml").write_text(
                """
[project]
name = "test-project"
"""
            )

            # Create custom config
            custom_config = {
                "commands": {
                    "test": [{"run": "custom test command"}],
                    "custom": [{"run": "custom command"}],
                }
            }

            config_file = project_dir / ".dev-config.yaml"
            with open(config_file, "w") as f:
                yaml.dump(custom_config, f)

            config = load_configuration_for_project(project_dir)

            # Custom commands should override defaults
            assert config["commands"]["test"] == [{"run": "custom test command"}]
            assert config["commands"]["custom"] == [{"run": "custom command"}]
            # Default logs command should still be present
            assert "logs" in config["commands"]

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_command_execution_with_services(self, mock_execute):
        """Test command execution with service startup."""
        mock_execute.return_value = Mock(success=True, stdout="success")

        steps = [{"start_services": ["redis"], "run": "pytest tests/"}]

        result = execute_command_with_steps("test", steps)

        assert result.success is True
        # Should have called docker command and pytest
        assert mock_execute.call_count >= 2

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_command_execution_with_multiple_run_commands(self, mock_execute):
        """Test command execution with multiple run commands."""
        mock_execute.return_value = Mock(success=True)

        steps = [{"run": ["eslint src/", "black ."]}]

        result = execute_command_with_steps("lint", steps)

        assert result.success is True
        assert mock_execute.call_count == 2

    @patch("dev_tools.command_executor.execute_shell_command")
    def test_command_execution_failure_stops_execution(self, mock_execute):
        """Test that command execution stops on first failure."""
        mock_execute.side_effect = [
            Mock(success=False, stderr="first command failed"),
            Mock(success=True),  # This shouldn't be called
        ]

        steps = [{"run": "failing command"}, {"run": "second command"}]

        result = execute_command_with_steps("test", steps)

        assert result.success is False
        assert mock_execute.call_count == 1

    def test_environment_variable_loading(self):
        """Test that .env file loading works correctly."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create .env file
            env_file = project_dir / ".env"
            env_file.write_text("TEST_VAR=test_value\nANOTHER_VAR=another_value")

            # Create minimal config
            (project_dir / "pyproject.toml").write_text(
                """
[project]
name = "test-project"
"""
            )

            original_env = os.environ.copy()
            try:
                # Clear test variables
                os.environ.pop("TEST_VAR", None)
                os.environ.pop("ANOTHER_VAR", None)

                with patch(
                    "dev_tools.command_executor.execute_shell_command"
                ) as mock_execute:
                    mock_execute.return_value = Mock(success=True)

                    handle_command_execution("test", project_dir)

                    # Environment variables should be loaded
                    assert os.environ.get("TEST_VAR") == "test_value"
                    assert os.environ.get("ANOTHER_VAR") == "another_value"

            finally:
                # Restore original environment
                os.environ.clear()
                os.environ.update(original_env)

    @patch("dev_tools.cli.execute_shell_command")
    def test_logs_command_integration(self, mock_execute):
        """Test logs command integration."""
        mock_execute.return_value = Mock(success=True, stdout="log content")

        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create activity.log file
            activity_log = project_dir / "activity.log"
            activity_log.write_text("2024-01-01 10:00:00 - INFO - Test log entry")

            # Change to temp directory
            original_cwd = os.getcwd()
            try:
                os.chdir(temp_dir)

                from dev_tools.cli import handle_logs_command

                result = handle_logs_command()

                assert result.success is True
                mock_execute.assert_called_once_with(
                    "tail -n 50 activity.log", capture_output=True
                )

            finally:
                os.chdir(original_cwd)

    def test_unknown_command_handling(self):
        """Test handling of unknown commands."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create minimal Python project
            (project_dir / "pyproject.toml").write_text(
                """
[project]
name = "test-project"
"""
            )

            result = handle_command_execution("unknown_command", project_dir)

            assert result.success is False
            assert "Unknown command 'unknown_command'" in result.stderr
            assert "Available commands:" in result.stderr
