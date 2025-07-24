package errors

import (
	"errors"
	"fmt"
)

// New creates a new error with the given message.
func New(message string) error {
	return errors.New(message)
}

// Errorf creates a new error with the given formatted message.
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Wrap wraps an error with a message.
func Wrap(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}
