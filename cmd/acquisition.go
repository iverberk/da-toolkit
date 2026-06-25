package cmd

import (
	"context"
	"strings"

	"da-toolkit/internal/acquisition"

	"github.com/spf13/cobra"
)

var acquisitionPocFlags struct {
	runtimeDir      string
	sitePackagesDir string
	bootstrap       bool
	cleanup         bool
	acquireArgs     []string
}

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

var acquisitionPocCmd = &cobra.Command{
	Use:   "poc",
	Short: "run the embedded Python acquire proof of concept",
	Long: strings.Join([]string{
		"Run the embedded Python acquire proof of concept using go-embed-python.",
		"The command extracts a pinned embedded Python runtime, optionally bootstraps the pinned embedded acquire packages into an app-owned site-packages directory, and invokes acquire as a Python subprocess.",
		"The --bootstrap path uses committed embedded artifacts and does not require Internet access.",
	}, "\n"),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		return acquisition.RunPoC(ctx, acquisition.Options{
			RuntimeDir:       acquisitionPocFlags.runtimeDir,
			SitePackagesDir:  acquisitionPocFlags.sitePackagesDir,
			BootstrapAcquire: acquisitionPocFlags.bootstrap,
			CleanupRuntime:   acquisitionPocFlags.cleanup,
			AcquireArgs:      acquisitionPocFlags.acquireArgs,
			Stdout:           cmd.OutOrStdout(),
			Stderr:           cmd.ErrOrStderr(),
		})
	},
}

func init() {
	acquisitionPocCmd.Flags().StringVar(&acquisitionPocFlags.runtimeDir, "runtime-dir", "", "app-owned directory for extracted embedded Python runtime")
	acquisitionPocCmd.Flags().StringVar(&acquisitionPocFlags.sitePackagesDir, "site-packages", "", "directory containing or receiving pinned acquire Python packages")
	acquisitionPocCmd.Flags().BoolVar(&acquisitionPocFlags.bootstrap, "bootstrap", false, "extract pinned embedded acquire dependencies into --site-packages for this PoC")
	acquisitionPocCmd.Flags().BoolVar(&acquisitionPocFlags.cleanup, "cleanup", false, "delete extracted embedded Python runtime after the PoC run")
	acquisitionPocCmd.Flags().StringSliceVar(&acquisitionPocFlags.acquireArgs, "acquire-arg", nil, "argument passed to acquire; repeat to build argv (defaults to --help after bootstrap)")
	acquisitionCmd.AddCommand(acquisitionPocCmd)
	rootCmd.AddCommand(acquisitionCmd)
}
