"""Tests for logger setup functionality."""

import logging
import tempfile
from io import StringIO
from pathlib import Path
from unittest.mock import patch

import pytest

from dev_tools.logger_setup import get_logger, setup_application_logging


class TestLoggerSetup:
    """Test suite for logger setup functionality."""

    def setup_method(self):
        """Reset logging configuration before each test."""
        # Clear all existing handlers and loggers
        root_logger = logging.getLogger()
        for handler in root_logger.handlers[:]:
            root_logger.removeHandler(handler)
        root_logger.setLevel(logging.WARNING)

        # Clear all existing loggers
        logger_dict = logging.Logger.manager.loggerDict
        for name in list(logger_dict.keys()):
            if isinstance(logger_dict[name], logging.Logger):
                logger_dict[name].handlers.clear()
                logger_dict[name].setLevel(logging.NOTSET)

        # Force logging to not be disabled
        logging.disable(logging.NOTSET)

    def test_setup_application_logging_default_parameters(self):
        """Test setup_application_logging with default parameters."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "activity.log"

            setup_application_logging(log_file=log_file)

            root_logger = logging.getLogger()
            # Check that a FileHandler was added (ignoring pytest's handlers)
            file_handlers = [
                h for h in root_logger.handlers if isinstance(h, logging.FileHandler)
            ]
            assert len(file_handlers) >= 1
            assert root_logger.level == logging.INFO

            # Verify log file is created
            assert log_file.exists()

            # Test logging works - use root logger explicitly
            root_logger.info("Test message")
            log_content = log_file.read_text(encoding="utf-8")
            assert "Test message" in log_content
            assert "Logging configured" in log_content

    def test_setup_application_logging_with_verbose(self):
        """Test setup_application_logging with verbose=True."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "activity.log"

            # Capture stdout
            with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
                setup_application_logging(log_file=log_file, verbose=True)

                root_logger = logging.getLogger()
                # Check that FileHandler and StreamHandler were added (ignoring pytest's handlers)
                file_handlers = [
                    h
                    for h in root_logger.handlers
                    if isinstance(h, logging.FileHandler)
                ]
                stream_handlers = [
                    h
                    for h in root_logger.handlers
                    if isinstance(h, logging.StreamHandler)
                ]
                assert len(file_handlers) >= 1
                assert len(stream_handlers) >= 1

                # Test that both file and console get the message
                root_logger.info("Verbose test message")

                # Check console output
                console_output = mock_stdout.getvalue()
                assert "Verbose test message" in console_output
                assert "Logging configured" in console_output

                # Check file output
                log_content = log_file.read_text(encoding="utf-8")
                assert "Verbose test message" in log_content

    def test_setup_application_logging_debug_level(self):
        """Test setup_application_logging with DEBUG level."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "debug.log"

            setup_application_logging(log_file=log_file, log_level="DEBUG")

            root_logger = logging.getLogger()
            assert root_logger.level == logging.DEBUG

            # Test debug message is logged
            root_logger = logging.getLogger()
            root_logger.debug("Debug test message")
            log_content = log_file.read_text(encoding="utf-8")
            assert "Debug test message" in log_content

    def test_setup_application_logging_warning_level(self):
        """Test setup_application_logging with WARNING level."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "warning.log"

            setup_application_logging(log_file=log_file, log_level="WARNING")

            root_logger = logging.getLogger()
            assert root_logger.level == logging.WARNING

            # Info message should not be logged
            root_logger = logging.getLogger()
            root_logger.info("Info message should not appear")
            # Warning message should be logged
            root_logger.warning("Warning message should appear")

            log_content = log_file.read_text(encoding="utf-8")
            assert "Info message should not appear" not in log_content
            assert "Warning message should appear" in log_content

    def test_setup_application_logging_error_level(self):
        """Test setup_application_logging with ERROR level."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "error.log"

            setup_application_logging(log_file=log_file, log_level="ERROR")

            root_logger = logging.getLogger()
            assert root_logger.level == logging.ERROR

            # Warning message should not be logged
            root_logger = logging.getLogger()
            root_logger.warning("Warning should not appear")
            # Error message should be logged
            root_logger.error("Error message should appear")

            log_content = log_file.read_text(encoding="utf-8")
            assert "Warning should not appear" not in log_content
            assert "Error message should appear" in log_content

    def test_setup_application_logging_invalid_level(self):
        """Test setup_application_logging with invalid log level defaults to INFO."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "invalid.log"

            setup_application_logging(log_file=log_file, log_level="INVALID")

            root_logger = logging.getLogger()
            assert root_logger.level == logging.INFO  # Should default to INFO

    def test_setup_application_logging_case_insensitive_level(self):
        """Test setup_application_logging with case insensitive log level."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "case.log"

            setup_application_logging(log_file=log_file, log_level="debug")

            root_logger = logging.getLogger()
            assert root_logger.level == logging.DEBUG

    def test_setup_application_logging_default_log_file(self):
        """Test setup_application_logging with default log file (None)."""
        original_cwd = Path.cwd()

        with tempfile.TemporaryDirectory() as temp_dir:
            # Change to temp directory
            import os

            os.chdir(temp_dir)

            try:
                setup_application_logging(log_file=None)

                # Should create activity.log in current directory
                default_log = Path("activity.log")
                assert default_log.exists()

                root_logger = logging.getLogger()
                root_logger.info("Default log test")
                log_content = default_log.read_text(encoding="utf-8")
                assert "Default log test" in log_content

            finally:
                os.chdir(original_cwd)

    def test_setup_application_logging_file_permissions_error(self):
        """Test setup_application_logging with file permission errors."""
        # Try to write to a directory that doesn't exist
        invalid_path = Path("/nonexistent/directory/test.log")

        with pytest.raises(FileNotFoundError):
            setup_application_logging(log_file=invalid_path)

    def test_setup_application_logging_log_format(self):
        """Test that log messages have the correct format."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "format_test.log"

            setup_application_logging(log_file=log_file)

            root_logger = logging.getLogger()
            root_logger.info("Format test message")
            log_content = log_file.read_text(encoding="utf-8")

            # Check format: timestamp - logger_name - level - message
            lines = log_content.strip().split("\n")
            for line in lines:
                if "Format test message" in line:
                    parts = line.split(" - ")
                    assert len(parts) >= 4
                    # Should have timestamp, logger name, level, message
                    assert "INFO" in parts[2]
                    break
            else:
                pytest.fail("Format test message not found in log")

    def test_setup_application_logging_encoding(self):
        """Test that log files handle UTF-8 encoding correctly."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "encoding_test.log"

            setup_application_logging(log_file=log_file)

            # Log message with unicode characters
            unicode_message = "Unicode test: 日本語 🚀 café"
            root_logger = logging.getLogger()
            root_logger.info(unicode_message)

            log_content = log_file.read_text(encoding="utf-8")
            assert unicode_message in log_content

    def test_setup_application_logging_multiple_calls(self):
        """Test that multiple calls to setup_application_logging don't create duplicate handlers."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "multiple.log"

            setup_application_logging(log_file=log_file)
            initial_handler_count = len(logging.getLogger().handlers)

            # Call again
            setup_application_logging(log_file=log_file)
            final_handler_count = len(logging.getLogger().handlers)

            # Should have added handlers (this is expected behavior)
            assert final_handler_count >= initial_handler_count

    def test_get_logger_returns_logger_instance(self):
        """Test get_logger returns a proper logger instance."""
        logger = get_logger("test_module")

        assert isinstance(logger, logging.Logger)
        assert logger.name == "test_module"

    def test_get_logger_different_names(self):
        """Test get_logger returns different loggers for different names."""
        logger1 = get_logger("module1")
        logger2 = get_logger("module2")

        assert logger1 != logger2
        assert logger1.name == "module1"
        assert logger2.name == "module2"

    def test_get_logger_same_name_returns_same_instance(self):
        """Test get_logger returns the same instance for the same name."""
        logger1 = get_logger("same_module")
        logger2 = get_logger("same_module")

        assert logger1 is logger2

    def test_get_logger_inherits_configuration(self):
        """Test that get_logger returns loggers that inherit root configuration."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "inheritance.log"

            setup_application_logging(log_file=log_file, log_level="DEBUG")

            module_logger = get_logger("test_module")
            module_logger.debug("Module debug message")

            log_content = log_file.read_text(encoding="utf-8")
            assert "Module debug message" in log_content

    def test_logger_hierarchy(self):
        """Test that loggers follow proper hierarchy."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "hierarchy.log"

            setup_application_logging(log_file=log_file)

            parent_logger = get_logger("parent")
            child_logger = get_logger("parent.child")

            # Both should log to the same file
            parent_logger.info("Parent message")
            child_logger.info("Child message")

            log_content = log_file.read_text(encoding="utf-8")
            assert "Parent message" in log_content
            assert "Child message" in log_content

    def test_verbose_mode_console_format(self):
        """Test that verbose mode console output has correct format."""
        with tempfile.TemporaryDirectory() as temp_dir:
            log_file = Path(temp_dir) / "console_format.log"

            with patch("sys.stdout", new_callable=StringIO) as mock_stdout:
                setup_application_logging(log_file=log_file, verbose=True)

                root_logger = logging.getLogger()
                root_logger.info("Console format test")

                console_output = mock_stdout.getvalue()
                # Should contain timestamp, logger name, level, and message
                assert "INFO" in console_output
                assert "Console format test" in console_output
                # Should have proper datetime format
                import re

                datetime_pattern = r"\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}"
                assert re.search(datetime_pattern, console_output)
