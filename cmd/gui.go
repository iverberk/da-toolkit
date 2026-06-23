package cmd

import (
	"github.com/spf13/cobra"
)

// guiCmd represents the gui command
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "start the future GUI wrapper over CLI workflows",
	Long:  "GUI workflow placeholder. This command does not require network access or credentials.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		placeholder(cmd, "GUI")
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
