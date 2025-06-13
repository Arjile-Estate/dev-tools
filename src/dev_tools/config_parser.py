"""Configuration file parsing and project type detection."""

import logging
from pathlib import Path
from typing import Any

import yaml

logger = logging.getLogger(__name__)


def load_dev_config_from_file(config_path: Path) -> dict[str, Any] | None:
    """
    Load configuration from a .dev-config.yaml file.

    Args:
        config_path: Path to the configuration file

    Returns:
        Parsed configuration dictionary or None if file doesn't exist

    Raises:
        yaml.YAMLError: If the YAML file is malformed
    """
    if not config_path.exists():
        logger.debug(f"Configuration file {config_path} does not exist")
        return None

    try:
        with open(config_path, encoding="utf-8") as file:
            config = yaml.safe_load(file)
            logger.info(f"Loaded configuration from {config_path}")
            return config
    except yaml.YAMLError as e:
        logger.error(f"Failed to parse YAML configuration file {config_path}: {e}")
        raise
    except Exception as e:
        logger.error(f"Failed to read configuration file {config_path}: {e}")
        raise


def detect_project_type(project_dir: Path) -> str:
    """
    Detect the project type based on the presence of specific files.

    Args:
        project_dir: Directory to analyze

    Returns:
        Project type string ('python', 'nodejs', 'rust', or 'unknown')
    """
    detection_patterns = [
        ("python", ["pyproject.toml", "requirements.txt", "setup.py", "Pipfile"]),
        ("nodejs", ["package.json"]),
        ("rust", ["Cargo.toml"]),
    ]

    for project_type, patterns in detection_patterns:
        for pattern in patterns:
            if (project_dir / pattern).exists():
                logger.info(f"Detected {project_type} project based on {pattern}")
                return project_type

    logger.info("Could not detect project type, using 'unknown'")
    return "unknown"


def get_default_commands_for_project_type(project_type: str) -> dict[str, Any]:
    """
    Get default commands based on project type.

    Args:
        project_type: The detected project type

    Returns:
        Dictionary with default commands configuration
    """
    defaults = {
        "python": {
            "commands": {
                "test": [{"run": "uv run pytest tests/"}],
                "lint": [{"run": ["uv run ruff check .", "uv run black --check ."]}],
                "logs": [{"run": "tail -n 50 activity.log"}],
            }
        },
        "nodejs": {
            "commands": {
                "test": [{"run": "npm test"}],
                "lint": [{"run": "npm run lint"}],
                "dev": [{"run": "npm run dev", "daemon": True}],
                "build": [{"run": "npm run build"}],
                "logs": [{"run": "tail -n 50 activity.log"}],
            }
        },
        "rust": {
            "commands": {
                "test": [{"run": "cargo test"}],
                "lint": [{"run": "cargo clippy"}],
                "dev": [{"run": "cargo run"}],
                "build": [{"run": "cargo build"}],
                "logs": [{"run": "tail -n 50 activity.log"}],
            }
        },
        "unknown": {"commands": {"logs": [{"run": "tail -n 50 activity.log"}]}},
    }

    result = defaults.get(project_type, defaults["unknown"])
    logger.debug(f"Using default commands for {project_type} project")
    return result


def merge_config_with_defaults(
    user_config: dict[str, Any], defaults: dict[str, Any]
) -> dict[str, Any]:
    """
    Merge user configuration with defaults, with user config taking precedence.

    Args:
        user_config: User-provided configuration
        defaults: Default configuration

    Returns:
        Merged configuration dictionary
    """
    merged = defaults.copy()

    if "commands" in user_config:
        if "commands" not in merged:
            merged["commands"] = {}

        merged["commands"].update(user_config["commands"])

    logger.debug("Merged user configuration with defaults")
    return merged


def load_configuration_for_project(project_dir: Path) -> dict[str, Any]:
    """
    Load complete configuration for a project, combining user config with defaults.

    Args:
        project_dir: Project directory to analyze

    Returns:
        Complete configuration dictionary
    """
    config_path = project_dir / ".dev-config.yaml"
    user_config = load_dev_config_from_file(config_path) or {}

    project_type = detect_project_type(project_dir)
    defaults = get_default_commands_for_project_type(project_type)

    final_config = merge_config_with_defaults(user_config, defaults)
    logger.info(f"Loaded complete configuration for {project_type} project")

    return final_config
