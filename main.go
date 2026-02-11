package main

import (
	"errors"
	"fmt"
	"os"

	"dev-tools/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		// Check if error is an ExitError with specific exit code
		var exitErr *cmd.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, exitErr.Message)
			os.Exit(exitErr.Code)
		}
		// Default to exit code 1 for other errors
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
