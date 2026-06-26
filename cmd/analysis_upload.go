package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"da-toolkit/internal/analysis"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var analysisUploadFlags struct {
	filename       string
	aesKey         string
	sasURL         string
	handoff        string
	workspaceDir   string
	checkpointPath string
	yes            bool
}

var analysisUploadCmd = &cobra.Command{
	Use:   "upload [flags]",
	Short: "encrypt and upload deferred analysis files",
	Long:  "Encrypt plaintext files with AES keys from a deferred handoff and upload them to provided SAS URLs. Upload checkpoints persist SAS URLs but never AES keys.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		files, analysisID, err := analysisUploadFiles()
		if err != nil {
			return err
		}
		_, err = analysis.Upload(cmd.Context(), analysis.UploadOptions{
			AnalysisID:     analysisID,
			Files:          files,
			WorkspaceDir:   analysisUploadFlags.workspaceDir,
			CheckpointPath: analysisUploadFlags.checkpointPath,
			AssumeYes:      analysisUploadFlags.yes,
			Stdin:          cmd.InOrStdin(),
			Stdout:         cmd.OutOrStdout(),
		})
		return err
	},
}

func init() {
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.filename, "filename", "", "local plaintext source file to encrypt and upload")
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.aesKey, "aes-key", "", "base64 AES-256 key from deferred handoff (memory-only)")
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.sasURL, "sas-url", "", "Azure Blob SAS URL from deferred handoff")
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.handoff, "handoff", "", "deferred handoff JSON path")
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.workspaceDir, "workspace", "", "directory for encrypted artifacts and checkpoint metadata")
	analysisUploadCmd.Flags().StringVar(&analysisUploadFlags.checkpointPath, "checkpoint", "", "checkpoint file for restart-resume metadata")
	analysisUploadCmd.Flags().BoolVar(&analysisUploadFlags.yes, "yes", false, "delete successfully uploaded encrypted artifacts without prompting")
}

func analysisUploadFiles() ([]analysis.HandoffFile, uuid.UUID, error) {
	if strings.TrimSpace(analysisUploadFlags.handoff) != "" {
		data, err := os.ReadFile(analysisUploadFlags.handoff)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("read handoff %q: %w", analysisUploadFlags.handoff, err)
		}
		handoff, err := analysis.ParseHandoff(data)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("parse handoff: %w", err)
		}
		files, err := analysis.SelectHandoffFiles(handoff, analysisUploadFlags.filename)
		if err != nil {
			return nil, uuid.Nil, err
		}
		return files, handoff.AnalysisID, nil
	}
	if strings.TrimSpace(analysisUploadFlags.filename) == "" || strings.TrimSpace(analysisUploadFlags.aesKey) == "" || strings.TrimSpace(analysisUploadFlags.sasURL) == "" {
		return nil, uuid.Nil, errors.New("--filename, --aes-key, and --sas-url are required unless --handoff is used")
	}
	return []analysis.HandoffFile{{Filename: analysisUploadFlags.filename, AESKey: analysisUploadFlags.aesKey, SASURL: analysisUploadFlags.sasURL}}, uuid.Nil, nil
}
