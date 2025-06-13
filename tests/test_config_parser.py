"""Tests for configuration file parsing functionality."""

from pathlib import Path
from unittest.mock import Mock, mock_open, patch

import pytest
import yaml

from dev_tools.config_parser import (
    detect_project_type,
    get_default_commands_for_project_type,
    load_dev_config_from_file,
    merge_config_with_defaults,
)


class TestConfigurationParser:
    """Test suite for configuration file parsing."""

    def test_load_dev_config_from_file_with_valid_yaml(self):
        """Test loading a valid .dev-config.yaml file."""
        config_content = """
commands:
  test:
    - run: "pytest tests/"
  lint:
    - run: ["eslint src/", "black ."]
"""
        with patch("builtins.open", mock_open(read_data=config_content)):
            with patch("pathlib.Path.exists", return_value=True):
                result = load_dev_config_from_file(Path(".dev-config.yaml"))

        expected = {
            "commands": {
                "test": [{"run": "pytest tests/"}],
                "lint": [{"run": ["eslint src/", "black ."]}],
            }
        }
        assert result == expected

    def test_load_dev_config_from_file_nonexistent_file(self):
        """Test handling of nonexistent config file."""
        with patch("pathlib.Path.exists", return_value=False):
            result = load_dev_config_from_file(Path(".dev-config.yaml"))

        assert result is None

    def test_load_dev_config_from_file_invalid_yaml(self):
        """Test handling of invalid YAML content."""
        invalid_yaml = """
commands:
  test:
    - run: "pytest
invalid: yaml: content
"""
        with patch("builtins.open", mock_open(read_data=invalid_yaml)):
            with patch("pathlib.Path.exists", return_value=True):
                with pytest.raises(yaml.YAMLError):
                    load_dev_config_from_file(Path(".dev-config.yaml"))


class TestProjectTypeDetection:
    """Test suite for project type detection."""

    def test_detect_project_type_python_pyproject(self):
        """Test detection of Python project via pyproject.toml."""
        with patch("dev_tools.config_parser.Path") as mock_path:
            mock_project_dir = Mock()
            mock_path.return_value = mock_project_dir

            # Mock the path division and exists check
            def path_div_side_effect(pattern):
                mock_file_path = Mock()
                mock_file_path.exists.return_value = pattern == "pyproject.toml"
                return mock_file_path

            mock_project_dir.__truediv__ = path_div_side_effect
            result = detect_project_type(Path("."))

        assert result == "python"

    def test_detect_project_type_python_requirements(self):
        """Test detection of Python project via requirements.txt."""
        with patch("dev_tools.config_parser.Path") as mock_path:
            mock_project_dir = Mock()
            mock_path.return_value = mock_project_dir

            def path_div_side_effect(pattern):
                mock_file_path = Mock()
                mock_file_path.exists.return_value = pattern == "requirements.txt"
                return mock_file_path

            mock_project_dir.__truediv__ = path_div_side_effect
            result = detect_project_type(Path("."))

        assert result == "python"

    def test_detect_project_type_nodejs(self):
        """Test detection of Node.js project via package.json."""
        # Create a mock project directory
        mock_project_dir = Mock()

        def path_div_side_effect(self, pattern):
            mock_file_path = Mock()
            mock_file_path.exists.return_value = pattern == "package.json"
            return mock_file_path

        mock_project_dir.__truediv__ = path_div_side_effect
        result = detect_project_type(mock_project_dir)

        assert result == "nodejs"

    def test_detect_project_type_rust(self):
        """Test detection of Rust project via Cargo.toml."""
        # Create a mock project directory
        mock_project_dir = Mock()

        def path_div_side_effect(self, pattern):
            mock_file_path = Mock()
            mock_file_path.exists.return_value = pattern == "Cargo.toml"
            return mock_file_path

        mock_project_dir.__truediv__ = path_div_side_effect
        result = detect_project_type(mock_project_dir)

        assert result == "rust"

    def test_detect_project_type_unknown(self):
        """Test handling of unknown project type."""
        with patch("pathlib.Path.exists", return_value=False):
            result = detect_project_type(Path("."))

        assert result == "unknown"


class TestDefaultCommands:
    """Test suite for default command generation."""

    def test_get_default_commands_for_python_project(self):
        """Test default commands for Python project."""
        result = get_default_commands_for_project_type("python")

        assert "test" in result["commands"]
        assert "logs" in result["commands"]
        assert (
            "dev" not in result["commands"]
        )  # Python projects should not have dev command by default
        assert any("pytest" in str(cmd) for cmd in result["commands"]["test"])

    def test_get_default_commands_for_nodejs_project(self):
        """Test default commands for Node.js project."""
        result = get_default_commands_for_project_type("nodejs")

        assert "test" in result["commands"]
        assert "dev" in result["commands"]
        assert "logs" in result["commands"]
        assert any("npm test" in str(cmd) for cmd in result["commands"]["test"])

    def test_get_default_commands_for_unknown_project(self):
        """Test default commands for unknown project type."""
        result = get_default_commands_for_project_type("unknown")

        assert "logs" in result["commands"]
        assert len(result["commands"]) == 1


class TestConfigMerging:
    """Test suite for configuration merging."""

    def test_merge_config_with_defaults_override(self):
        """Test merging user config that overrides defaults."""
        user_config = {
            "commands": {
                "test": [{"run": "custom test command"}],
                "custom": [{"run": "custom command"}],
            }
        }
        defaults = {
            "commands": {
                "test": [{"run": "default test"}],
                "logs": [{"run": "tail -n 50 activity.log"}],
            }
        }

        result = merge_config_with_defaults(user_config, defaults)

        assert result["commands"]["test"] == [{"run": "custom test command"}]
        assert result["commands"]["logs"] == [{"run": "tail -n 50 activity.log"}]
        assert result["commands"]["custom"] == [{"run": "custom command"}]

    def test_merge_config_with_defaults_empty_user_config(self):
        """Test merging with empty user config uses all defaults."""
        user_config = {}
        defaults = {
            "commands": {
                "test": [{"run": "default test"}],
                "logs": [{"run": "tail -n 50 activity.log"}],
            }
        }

        result = merge_config_with_defaults(user_config, defaults)

        assert result == defaults
