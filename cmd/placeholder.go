package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func placeholder(cmd *cobra.Command, name string) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s workflow is not implemented yet. This placeholder does not require network access or credentials.\n", name)
}
