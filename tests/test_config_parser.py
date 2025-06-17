"""Tests for configuration file parsing functionality."""

import tempfile
from pathlib import Path
from unittest.mock import Mock, mock_open, patch

import pytest

from dev_tools.config_parser import (
    detect_project_type,
    get_default_commands_for_project_type,
    load_configuration_for_project,
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
                result = load_dev_config_from_file(Path(".dev-config.yaml"))
                assert result is None


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
        assert "build" in result["commands"]
        assert "lint" in result["commands"]
        assert "logs" in result["commands"]
        assert (
            "dev" not in result["commands"]
        )  # dev command should not be included by default
        assert any("npm test" in str(cmd) for cmd in result["commands"]["test"])

    def test_get_default_commands_for_unknown_project(self):
        """Test default commands for unknown project type."""
        result = get_default_commands_for_project_type("unknown")

        assert "logs" in result["commands"]
        assert len(result["commands"]) == 1


class TestLoadConfigurationForProject:
    """Test suite for load_configuration_for_project function."""

    def test_load_configuration_python_project_with_custom_config(self):
        """Test loading configuration for Python project with custom .dev-config.yaml."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create pyproject.toml
            (project_dir / "pyproject.toml").write_text('[project]\nname = "test"')

            # Create custom config
            (project_dir / ".dev-config.yaml").write_text(
                """
commands:
  custom-test:
    - run: "custom test command"
  build:
    - run: "custom build command"
"""
            )

            config = load_configuration_for_project(project_dir)

            assert "custom-test" in config["commands"]
            assert "build" in config["commands"]
            # Custom config should override defaults
            assert any(
                "custom build command" in str(cmd)
                for cmd in config["commands"]["build"]
            )

    def test_load_configuration_nodejs_project_defaults_only(self):
        """Test loading configuration for Node.js project with only defaults."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create package.json
            (project_dir / "package.json").write_text('{"name": "test"}')

            config = load_configuration_for_project(project_dir)

            assert "test" in config["commands"]
            assert "build" in config["commands"]
            assert "lint" in config["commands"]
            assert "dev" not in config["commands"]  # Should not have dev by default

    def test_load_configuration_rust_project_with_custom_config(self):
        """Test loading configuration for Rust project with custom config."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create Cargo.toml
            (project_dir / "Cargo.toml").write_text('[package]\nname = "test"')

            # Create custom config that adds to defaults
            (project_dir / ".dev-config.yaml").write_text(
                """
commands:
  test:
    - run: "cargo test --verbose"
  deploy:
    - run: "cargo build --release"
    - run: "docker build ."
"""
            )

            config = load_configuration_for_project(project_dir)

            # Should have both default and custom commands
            assert "test" in config["commands"]
            assert "deploy" in config["commands"]
            # Custom test should override default
            assert any(
                "cargo test --verbose" in str(cmd) for cmd in config["commands"]["test"]
            )

    def test_load_configuration_unknown_project_type(self):
        """Test loading configuration for unknown project type."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # No project files, should be unknown type
            config = load_configuration_for_project(project_dir)

            # Should only have logs command
            assert "logs" in config["commands"]
            assert len(config["commands"]) == 1

    def test_load_configuration_custom_config_only(self):
        """Test loading configuration with only custom config file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # No project type files, but has custom config
            (project_dir / ".dev-config.yaml").write_text(
                """
commands:
  custom-only:
    - run: "echo 'custom only'"
  another:
    - run: "echo 'another'"
    - background: true
"""
            )

            config = load_configuration_for_project(project_dir)

            # Should have custom commands
            assert "custom-only" in config["commands"]
            assert "another" in config["commands"]
            # Should still have default logs
            assert "logs" in config["commands"]

    def test_load_configuration_invalid_yaml_file(self):
        """Test loading configuration with invalid YAML file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create Python project
            (project_dir / "pyproject.toml").write_text('[project]\nname = "test"')

            # Create invalid YAML
            (project_dir / ".dev-config.yaml").write_text(
                """
commands:
  test:
    - run: "test"
    invalid_yaml: [unclosed
"""
            )

            # Should fall back to defaults when YAML is invalid
            config = load_configuration_for_project(project_dir)

            # Should have Python defaults since custom config failed to load
            assert "test" in config["commands"]
            assert any("pytest" in str(cmd) for cmd in config["commands"]["test"])

    def test_load_configuration_empty_yaml_file(self):
        """Test loading configuration with empty YAML file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create Python project
            (project_dir / "pyproject.toml").write_text('[project]\nname = "test"')

            # Create empty YAML file
            (project_dir / ".dev-config.yaml").write_text("")

            config = load_configuration_for_project(project_dir)

            # Should have Python defaults
            assert "test" in config["commands"]
            assert any("pytest" in str(cmd) for cmd in config["commands"]["test"])

    def test_load_configuration_yaml_with_only_whitespace(self):
        """Test loading configuration with YAML file containing only whitespace."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create Node.js project
            (project_dir / "package.json").write_text('{"name": "test"}')

            # Create YAML with only whitespace
            (project_dir / ".dev-config.yaml").write_text("   \n  \t  \n  ")

            config = load_configuration_for_project(project_dir)

            # Should have Node.js defaults
            assert "test" in config["commands"]
            assert any("npm test" in str(cmd) for cmd in config["commands"]["test"])

    def test_load_configuration_multiple_project_types(self):
        """Test loading configuration when multiple project type files exist."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)

            # Create multiple project files
            (project_dir / "pyproject.toml").write_text('[project]\nname = "test"')
            (project_dir / "package.json").write_text('{"name": "test"}')
            (project_dir / "Cargo.toml").write_text('[package]\nname = "test"')

            config = load_configuration_for_project(project_dir)

            # Should detect one of them (order is defined in detect_project_type)
            assert "test" in config["commands"]
            assert len(config["commands"]) > 1  # Should have more than just logs


class TestConfigParserEdgeCases:
    """Test suite for edge cases in config parser."""

    def test_load_dev_config_file_not_found(self):
        """Test load_dev_config_from_file with non-existent file."""
        non_existent_file = Path("/non/existent/file.yaml")
        result = load_dev_config_from_file(non_existent_file)
        assert result is None

    def test_load_dev_config_permission_denied(self):
        """Test load_dev_config_from_file with permission denied."""
        with tempfile.TemporaryDirectory() as temp_dir:
            config_file = Path(temp_dir) / "restricted.yaml"
            config_file.write_text("commands:\n  test:\n    - run: 'test'")

            # Mock permission error
            with patch(
                "builtins.open", side_effect=PermissionError("Permission denied")
            ):
                result = load_dev_config_from_file(config_file)
                assert result is None

    def test_load_dev_config_generic_exception(self):
        """Test load_dev_config_from_file with generic exception."""
        with tempfile.TemporaryDirectory() as temp_dir:
            config_file = Path(temp_dir) / "error.yaml"
            config_file.write_text("commands:\n  test:\n    - run: 'test'")

            # Mock generic exception during file reading
            with patch("builtins.open", side_effect=OSError("Disk error")):
                result = load_dev_config_from_file(config_file)
                assert result is None

    def test_detect_project_type_with_symlinks(self):
        """Test project type detection with symbolic links."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            real_file = project_dir / "real_pyproject.toml"
            symlink_file = project_dir / "pyproject.toml"

            # Create real file and symlink
            real_file.write_text('[project]\nname = "test"')
            try:
                symlink_file.symlink_to(real_file)

                project_type = detect_project_type(project_dir)
                assert project_type == "python"
            except OSError:
                # Skip test if symlinks not supported on this system
                pytest.skip("Symlinks not supported on this system")

    def test_yaml_with_unicode_content(self):
        """Test loading YAML config with unicode content."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            config_file = project_dir / ".dev-config.yaml"

            # Create config with unicode content
            unicode_config = """
commands:
  test:
    - run: "echo 'Testing with unicode: 日本語 🚀 café'"
  build:
    - run: "echo 'Building with émojis: 🔨 ⚡'"
"""
            config_file.write_text(unicode_config, encoding="utf-8")

            result = load_dev_config_from_file(config_file)
            assert result is not None
            assert "test" in result["commands"]
            assert "build" in result["commands"]

    def test_very_large_config_file(self):
        """Test loading a very large config file."""
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir)
            config_file = project_dir / ".dev-config.yaml"

            # Generate a large config with many commands
            large_config = "commands:\n"
            for i in range(100):
                large_config += f"  command-{i}:\n    - run: 'echo command {i}'\n"

            config_file.write_text(large_config)

            result = load_dev_config_from_file(config_file)
            assert result is not None
            assert len(result["commands"]) == 100
            assert "command-0" in result["commands"]
            assert "command-99" in result["commands"]


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
