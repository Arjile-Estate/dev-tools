"""Logging configuration for dev-tools application."""

import logging
import sys
from pathlib import Path


def setup_application_logging(
    log_file: Path | None = None, verbose: bool = False, log_level: str = "INFO"
) -> None:
    """
    Configure logging for the application.

    Args:
        log_file: Path to log file (defaults to activity.log)
        verbose: Whether to also log to stdout
        log_level: Logging level (DEBUG, INFO, WARNING, ERROR)
    """
    if log_file is None:
        log_file = Path("activity.log")

    log_format = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    date_format = "%Y-%m-%d %H:%M:%S"

    level = getattr(logging, log_level.upper(), logging.INFO)

    logging.basicConfig(
        level=level, format=log_format, datefmt=date_format, handlers=[]
    )

    file_handler = logging.FileHandler(log_file, encoding="utf-8")
    file_handler.setLevel(level)
    file_handler.setFormatter(logging.Formatter(log_format, date_format))

    root_logger = logging.getLogger()
    root_logger.addHandler(file_handler)

    if verbose:
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setLevel(level)
        console_handler.setFormatter(logging.Formatter(log_format, date_format))
        root_logger.addHandler(console_handler)

    logging.info(
        f"Logging configured - file: {log_file}, verbose: {verbose}, level: {log_level}"
    )


def get_logger(name: str) -> logging.Logger:
    """
    Get a logger instance for a specific module.

    Args:
        name: Name of the logger (typically __name__)

    Returns:
        Configured logger instance
    """
    return logging.getLogger(name)
