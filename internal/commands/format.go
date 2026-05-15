package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// FormatOutputKey is the unexported context-key type used to propagate the
// --format value from cmd/root.go down to built-in command handlers.
// Using a named type avoids collisions with other context keys.
type FormatOutputKey struct{}

// FormatCtxKey is the singleton key value used with context.WithValue to
// carry the resolved output format ("text" or "json") into built-in
// command handlers.
var FormatCtxKey = FormatOutputKey{}

// FormatFromContext returns the output format stored on the cobra command's
// context. It defaults to "text" when no value is set, when the value has the
// wrong type, or when it is empty — keeping callers safe from nil context
// values during tests or direct invocations.
func FormatFromContext(cmd *cobra.Command) string {
	ctx := cmd.Context()
	if ctx == nil {
		return "text"
	}
	v, ok := ctx.Value(FormatCtxKey).(string)
	if !ok || v == "" {
		return "text"
	}
	return v
}

// EmitJSON marshals payload as indented JSON and writes it (with a trailing
// newline) to the command's stdout writer. Centralizing this avoids
// duplicating the json.MarshalIndent + Fprintln pattern across every
// built-in command's JSON branch.
func EmitJSON(cmd *cobra.Command, payload any) error {
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes)); err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}
	return nil
}
