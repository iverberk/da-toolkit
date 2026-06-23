package cmd

import (
	"github.com/spf13/cobra"
)

// analysisCmd represents the analysis command
var analysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "create and inspect data-analysis requests",
	Long:  "Analysis workflow placeholder. This command does not require network access or credentials.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		placeholder(cmd, "Analysis")
	},
}

func init() {
	rootCmd.AddCommand(analysisCmd)
}
