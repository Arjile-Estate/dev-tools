package commands

import (
	"fmt"
	"path/filepath"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleValidateCommand validates the configuration file. Honors --format json.
func HandleValidateCommand(cmd *cobra.Command, args []string, projectDir string) error {
	logger.Info("Validating configuration file")

	configPath := filepath.Join(projectDir, ".dev-config.yaml")
	jsonMode := FormatFromContext(cmd) == "json"

	err := config.ValidateConfigFile(configPath)
	if err != nil {
		if jsonMode {
			_ = EmitJSON(cmd, map[string]any{
				"valid":       false,
				"config_file": configPath,
				"errors":      []string{err.Error()},
				"warnings":    []string{},
			})
			return fmt.Errorf("validation failed: %s", err.Error())
		}
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), colors.Error("✗ Configuration validation failed"))
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), "")
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), err.Error())
		return fmt.Errorf("validation failed")
	}

	if jsonMode {
		return EmitJSON(cmd, map[string]any{
			"valid":       true,
			"config_file": configPath,
			"errors":      []string{},
			"warnings":    []string{},
		})
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success("✓ Configuration is valid"))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Config file: %s\n", colors.Info(configPath))
	return nil
}
