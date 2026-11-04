package commands

import (
	"fmt"
	"strings"

	"dev-tools/internal/devresult"

	"github.com/spf13/cobra"
)

func HandleResultCommand(cmd *cobra.Command, args []string, projectDir string) error {
	if len(args) < 1 || len(args) > 2 || args[0] == "--" {
		return fmt.Errorf("usage: dev-tools result <job> [--full|--failures|--warnings]")
	}
	mode := ""
	if len(args) == 2 {
		mode = args[1]
		if mode != "--full" && mode != "--failures" && mode != "--warnings" {
			return fmt.Errorf("result is retrieval-only; expected --full, --failures, or --warnings")
		}
	}
	result, retained, err := devresult.LoadArtifacts(projectDir, args[0])
	if err != nil {
		return err
	}

	var output string
	switch mode {
	case "--full":
		output = retained
	case "--failures":
		output = evidenceText(result.Failures)
	case "--warnings":
		output = evidenceText(result.Warnings)
	default:
		output = devresult.RenderStoredText(result)
	}
	if output != "" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), output)
	return err
}

func evidenceText(items []devresult.Evidence) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.Message)
	}
	return strings.Join(lines, "\n")
}
