package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"dev-tools/internal/colors"
	"dev-tools/internal/config"
	"dev-tools/internal/logger"

	"github.com/spf13/cobra"
)

// HandleSchemaCommand exports the JSON schema for .dev-config.yaml
func HandleSchemaCommand(cmd *cobra.Command, args []string, projectDir string) error {
	logger.Info("Exporting JSON schema for .dev-config.yaml")

	outputPath := filepath.Join(projectDir, ".dev-config.schema.json")

	// Parse --output flag from args
	for i, arg := range args {
		if arg == "--output" && i+1 < len(args) {
			outputPath = args[i+1]
			break
		}
	}

	err := os.WriteFile(outputPath, []byte(config.ConfigSchema+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), colors.Success("Schema exported to %s", outputPath))
	logger.Infof("Schema exported to %s", outputPath)
	return nil
}
