package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatFromContext(t *testing.T) {
	t.Run("returns the format stored in context", func(t *testing.T) {
		cmd := &cobra.Command{}
		ctx := context.WithValue(context.Background(), FormatCtxKey, "json")
		cmd.SetContext(ctx)

		assert.Equal(t, "json", FormatFromContext(cmd))
	})

	t.Run("returns text when no format is set in context", func(t *testing.T) {
		cmd := &cobra.Command{}
		// No SetContext call; cobra returns context.Background()

		assert.Equal(t, "text", FormatFromContext(cmd))
	})

	t.Run("returns text when context value is the wrong type", func(t *testing.T) {
		cmd := &cobra.Command{}
		ctx := context.WithValue(context.Background(), FormatCtxKey, 42)
		cmd.SetContext(ctx)

		assert.Equal(t, "text", FormatFromContext(cmd))
	})

	t.Run("returns text when context value is empty string", func(t *testing.T) {
		cmd := &cobra.Command{}
		ctx := context.WithValue(context.Background(), FormatCtxKey, "")
		cmd.SetContext(ctx)

		assert.Equal(t, "text", FormatFromContext(cmd))
	})
}

func TestEmitJSON(t *testing.T) {
	t.Run("writes indented JSON with trailing newline", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		payload := map[string]any{"hello": "world", "count": 3}
		err := EmitJSON(cmd, payload)
		require.NoError(t, err)

		out := buf.String()
		assert.True(t, len(out) > 0)
		assert.Equal(t, byte('\n'), out[len(out)-1], "output should end with a newline")

		var decoded map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &decoded))
		assert.Equal(t, "world", decoded["hello"])
		assert.Equal(t, float64(3), decoded["count"])
	})

	t.Run("supports nested structures", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		payload := map[string]any{
			"items": []map[string]any{
				{"name": "a"},
				{"name": "b"},
			},
		}
		err := EmitJSON(cmd, payload)
		require.NoError(t, err)

		var decoded map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))

		items, ok := decoded["items"].([]any)
		require.True(t, ok)
		assert.Len(t, items, 2)
	})

	t.Run("returns an error for unmarshalable payloads", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		// channels are not JSON-marshalable
		err := EmitJSON(cmd, map[string]any{"ch": make(chan int)})
		assert.Error(t, err)
	})
}
