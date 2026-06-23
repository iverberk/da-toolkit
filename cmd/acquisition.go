package cmd

import (
	"github.com/spf13/cobra"
)

// acquisitionCmd represents the acquisition command
var acquisitionCmd = &cobra.Command{
	Use:   "acquisition",
	Short: "run live-system acquisition workflows",
	Long:  "Acquisition workflow placeholder. This command does not require network access or credentials.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		placeholder(cmd, "Acquisition")
	},
}

func init() {
	rootCmd.AddCommand(acquisitionCmd)
}
