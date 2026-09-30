package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"da-toolkit/internal/analysis"
	"github.com/spf13/cobra"
)

var analysisDecryptFlags struct {
	aesKey    string
	output    string
	overwrite bool
}

var analysisDecryptCmd = &cobra.Command{
	Use:   "decrypt <encrypted-file>",
	Short: "decrypt a local encrypted analysis file",
	Long:  "Decrypt a local file encrypted with the data-analysis upload chunk format using a base64 AES-256 key. The AES key is used in memory only.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		if strings.TrimSpace(args[0]) == "" {
			return errors.New("encrypted-file is required")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(analysisDecryptFlags.aesKey) == "" {
			return errors.New("--aes-key is required")
		}
		if strings.TrimSpace(analysisDecryptFlags.output) == "" {
			return errors.New("--output is required")
		}
		if !analysisDecryptFlags.overwrite {
			if _, err := os.Stat(analysisDecryptFlags.output); err == nil {
				return fmt.Errorf("output file %q already exists; pass --overwrite to replace it", analysisDecryptFlags.output)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("stat output file %q: %w", analysisDecryptFlags.output, err)
			}
		}
		if err := analysis.DecryptFile(args[0], analysisDecryptFlags.output, analysisDecryptFlags.aesKey); err != nil {
			return fmt.Errorf("decrypt file: %w", err)
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Decrypted %s to %s\n", args[0], analysisDecryptFlags.output)
		return err
	},
}

func init() {
	analysisDecryptCmd.Flags().StringVar(&analysisDecryptFlags.aesKey, "aes-key", "", "base64 AES-256 key from deferred handoff (memory-only)")
	analysisDecryptCmd.Flags().StringVarP(&analysisDecryptFlags.output, "output", "o", "", "local output path for decrypted plaintext")
	analysisDecryptCmd.Flags().BoolVar(&analysisDecryptFlags.overwrite, "overwrite", false, "overwrite an existing output file")
}
