package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"da-toolkit/internal/ncscapi"
	"da-toolkit/internal/upload"
	"github.com/spf13/cobra"
)

var uploadFlags struct {
	baseURL        string
	tokenURL       string
	clientID       string
	clientSecret   string
	workspaceDir   string
	checkpointPath string
	resume         bool
	yes            bool
}

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload [flags] <file> [file...]",
	Short: "create an other analysis and upload local files",
	Long:  "Create one other data-analysis for local files, encrypt each file, upload encrypted artifacts, and persist non-secret resume metadata.",
	Args: func(cmd *cobra.Command, args []string) error {
		if uploadFlags.resume {
			return nil
		}
		if len(args) == 0 {
			return errors.New("at least one local file is required unless --resume is used")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var client upload.AnalysisCreator
		if !uploadFlags.resume {
			apiClient, err := ncscapi.NewClient(ncscapi.Config{
				BaseURL:      firstNonEmpty(uploadFlags.baseURL, os.Getenv("DA_TOOLKIT_API_BASE_URL"), ncscapi.DefaultBaseURL),
				TokenURL:     firstNonEmpty(uploadFlags.tokenURL, os.Getenv("DA_TOOLKIT_TOKEN_URL")),
				ClientID:     firstNonEmpty(uploadFlags.clientID, os.Getenv("DA_TOOLKIT_CLIENT_ID")),
				ClientSecret: firstNonEmpty(uploadFlags.clientSecret, os.Getenv("DA_TOOLKIT_CLIENT_SECRET")),
				HTTPClient:   &http.Client{Timeout: 0},
			})
			if err != nil {
				return err
			}
			client = apiClient
		}
		workspace := uploadFlags.workspaceDir
		if workspace == "" {
			workspace = defaultUploadWorkspace(args)
		}
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		_, err := upload.Run(ctx, upload.Options{
			Files:          args,
			WorkspaceDir:   workspace,
			CheckpointPath: uploadFlags.checkpointPath,
			Resume:         uploadFlags.resume,
			AssumeYes:      uploadFlags.yes,
			API:            client,
			Stdin:          os.Stdin,
			Stdout:         cmd.OutOrStdout(),
		})
		return err
	},
}

func init() {
	uploadCmd.Flags().StringVar(&uploadFlags.baseURL, "base-url", "", "NCSC Data Analysis API base URL (or DA_TOOLKIT_API_BASE_URL)")
	uploadCmd.Flags().StringVar(&uploadFlags.tokenURL, "token-url", "", "OAuth token URL (or DA_TOOLKIT_TOKEN_URL)")
	uploadCmd.Flags().StringVar(&uploadFlags.clientID, "client-id", "", "OAuth client ID (or DA_TOOLKIT_CLIENT_ID)")
	uploadCmd.Flags().StringVar(&uploadFlags.clientSecret, "client-secret", "", "OAuth client secret (or DA_TOOLKIT_CLIENT_SECRET; memory-only)")
	uploadCmd.Flags().StringVar(&uploadFlags.workspaceDir, "workspace", "", "directory for encrypted artifacts and checkpoint metadata")
	uploadCmd.Flags().StringVar(&uploadFlags.checkpointPath, "checkpoint", "", "checkpoint file for restart-resume metadata")
	uploadCmd.Flags().BoolVar(&uploadFlags.resume, "resume", false, "resume from the checkpoint without creating a replacement data-analysis")
	uploadCmd.Flags().BoolVar(&uploadFlags.yes, "yes", false, "delete successfully uploaded encrypted artifacts without prompting")
	rootCmd.AddCommand(uploadCmd)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultUploadWorkspace(files []string) string {
	if len(files) == 0 {
		return filepath.Join(".da-toolkit", "upload")
	}
	hash := sha256.New()
	for _, file := range files {
		path, err := filepath.Abs(file)
		if err != nil {
			path = file
		}
		_, _ = hash.Write([]byte(path))
		_, _ = hash.Write([]byte{0})
	}
	id := hex.EncodeToString(hash.Sum(nil))[:16]
	return filepath.Join(".da-toolkit", fmt.Sprintf("upload-%s", id))
}
