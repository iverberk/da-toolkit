package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "da-toolkit",
	Short: "CLI-first toolkit for data-analysis workflows.",
	Long: `CLI-first toolkit for data-analysis workflows.

Command groups:
  analysis     create and inspect data-analysis requests
  upload       upload prepared files for an analysis
  result       retrieve analysis results
  acquisition  run live-system acquisition workflows
  gui          start the future GUI wrapper over CLI workflows

Configuration:
  Future workflow settings must be non-secret only.
  Do not store API credentials, OAuth tokens, or AES keys in config files.

User-facing errors are written as: Error: <clear message>.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
