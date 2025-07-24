package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New("test error")
	assert.Error(t, err)
	assert.Equal(t, "test error", err.Error())
}

func TestErrorf(t *testing.T) {
	err := Errorf("error with format: %s", "some value")
	assert.Error(t, err)
	assert.Equal(t, "error with format: some value", err.Error())
}

func TestWrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := Wrap(innerErr, "outer error")
	assert.Error(t, err)
	assert.Equal(t, "outer error: inner error", err.Error())
	assert.True(t, errors.Is(err, innerErr))
}
