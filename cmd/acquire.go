package cmd

import (
	"context"

	"da-toolkit/internal/acquire"

	"github.com/spf13/cobra"
)

var runAcquire = acquire.Run

// acquireCmd represents the acquire command.
var acquireCmd = &cobra.Command{
	Use:                "acquire [acquire args/options...]",
	Short:              "run Dissect/acquire through the embedded runtime",
	Long:               "Run Dissect/acquire through the embedded Python runtime.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		return runAcquire(ctx, acquire.Options{
			Args:   args,
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
	},
}

func init() {
	rootCmd.AddCommand(acquireCmd)
}
