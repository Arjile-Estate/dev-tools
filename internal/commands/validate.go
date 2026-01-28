package commands

import (
	"fmt"
	"path/filepath"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/logger"
	"github.com/spf13/cobra"
)

// HandleValidateCommand validates the configuration file
func HandleValidateCommand(cmd *cobra.Command, args []string, projectDir string) error {
	logger.Info("Validating configuration file")

	configPath := filepath.Join(projectDir, ".dev-config.yaml")

	// Validate the config file
	err := config.ValidateConfigFile(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), colors.Error("✗ Configuration validation failed"))
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), "")
		_, _ = fmt.Fprintln(cmd.OutOrStderr(), err.Error())
		return fmt.Errorf("validation failed")
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success("✓ Configuration is valid"))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Config file: %s\n", colors.Info(configPath))
	return nil
}
