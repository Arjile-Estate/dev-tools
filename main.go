package main

import (
	"errors"
	"os"

	"dev-tools/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		// Check if error is an ExitError with specific exit code
		var exitErr *cmd.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		// Default to exit code 1 for other errors
		os.Exit(1)
	}
}
