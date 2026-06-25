package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "da-toolkit",
	Short:         "CLI-first toolkit for data-analysis workflows.",
	Long:          "Create, inspect, and retrieve data-analyses, and run acquire workflows.",
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
