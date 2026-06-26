package cmd

import (
	"context"

	"da-toolkit/internal/vol"

	"github.com/spf13/cobra"
)

var runVol = vol.Run

// volCmd represents the vol command.
var volCmd = &cobra.Command{
	Use:                "vol [volatility3 args/options...]",
	Short:              "run Volatility 3 through the embedded runtime",
	Long:               "Run Volatility 3 through the embedded Python runtime.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		return runVol(ctx, vol.Options{
			Args:   args,
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
	},
}

func init() {
	rootCmd.AddCommand(volCmd)
}
