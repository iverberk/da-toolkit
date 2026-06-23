package cmd

import (
	"github.com/spf13/cobra"
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "upload prepared files for an analysis",
	Long:  "Upload workflow placeholder. This command does not require network access or credentials.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		placeholder(cmd, "Upload")
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
