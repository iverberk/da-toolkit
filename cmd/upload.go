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
	analysisType   string
	zipPasswords   []string
	requested      []string
	diskKeys       []string
	manualAllowed  bool
}

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload [flags] <file> [file...]",
	Short: "create an analysis and upload local files",
	Long:  "Create one other, malware, or forensic data-analysis for local files, encrypt each file, upload encrypted artifacts, and persist non-secret resume metadata.",
	Args: func(cmd *cobra.Command, args []string) error {
		if uploadFlags.resume {
			return nil
		}
		if len(args) == 0 {
			return errors.New("at least one local file is required unless --resume is used")
		}
		switch uploadFlags.analysisType {
		case upload.AnalysisTypeOther, upload.AnalysisTypeMalware, upload.AnalysisTypeForensic:
			return nil
		default:
			return fmt.Errorf("unsupported analysis type %q (supported: other, malware, forensic)", uploadFlags.analysisType)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		zipPasswords, err := parseZipPasswordFlags(uploadFlags.zipPasswords)
		if err != nil {
			return err
		}
		requested, err := parseRequestedAnalysisFlags(uploadFlags.requested)
		if err != nil {
			return err
		}
		diskKeys, err := parseDiskEncryptionKeyFlags(uploadFlags.diskKeys)
		if err != nil {
			return err
		}
		if len(zipPasswords) > 0 && uploadFlags.analysisType != upload.AnalysisTypeMalware {
			return errors.New("--zip-password can only be used with --type malware")
		}
		if len(requested) > 0 && uploadFlags.analysisType != upload.AnalysisTypeForensic {
			return errors.New("--requested-analysis can only be used with --type forensic")
		}
		if len(diskKeys) > 0 && uploadFlags.analysisType != upload.AnalysisTypeForensic {
			return errors.New("--disk-encryption-key can only be used with --type forensic")
		}
		if cmd.Flags().Changed("manual-analysis-allowed") && uploadFlags.analysisType != upload.AnalysisTypeForensic {
			return errors.New("--manual-analysis-allowed can only be used with --type forensic")
		}
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
		_, err = upload.Run(ctx, upload.Options{
			Files:                 args,
			WorkspaceDir:          workspace,
			CheckpointPath:        uploadFlags.checkpointPath,
			Resume:                uploadFlags.resume,
			AssumeYes:             uploadFlags.yes,
			AnalysisType:          uploadFlags.analysisType,
			ZipPasswords:          zipPasswords,
			RequestedAnalyses:     requested,
			DiskEncryptionKeys:    diskKeys,
			ManualAnalysisAllowed: uploadFlags.manualAllowed,
			API:                   client,
			Stdin:                 os.Stdin,
			Stdout:                cmd.OutOrStdout(),
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
	uploadCmd.Flags().StringVar(&uploadFlags.analysisType, "type", upload.AnalysisTypeOther, "data-analysis type to create (other, malware, or forensic)")
	uploadCmd.Flags().StringArrayVar(&uploadFlags.zipPasswords, "zip-password", nil, "malware zip password as file=password; repeat per protected file (memory-only)")
	uploadCmd.Flags().StringArrayVar(&uploadFlags.requested, "requested-analysis", nil, "forensic requested analyses as file=analysis[,analysis...]; repeat per file")
	uploadCmd.Flags().StringArrayVar(&uploadFlags.diskKeys, "disk-encryption-key", nil, "forensic disk encryption key as file=key; repeat per encrypted disk image (memory-only)")
	uploadCmd.Flags().BoolVar(&uploadFlags.manualAllowed, "manual-analysis-allowed", false, "allow analysts to view forensic source data")
	uploadCmd.Flags().BoolVar(&uploadFlags.resume, "resume", false, "resume from the checkpoint without creating a replacement data-analysis")
	uploadCmd.Flags().BoolVar(&uploadFlags.yes, "yes", false, "delete successfully uploaded encrypted artifacts without prompting")
	rootCmd.AddCommand(uploadCmd)
}

func parseZipPasswordFlags(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	passwords := make(map[string]string, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		file, password, ok := strings.Cut(entry, "=")
		file = strings.TrimSpace(file)
		if !ok || file == "" || password == "" {
			return nil, errors.New("--zip-password must use file=password with a non-empty file and password")
		}
		passwords[file] = password
		if abs, err := filepath.Abs(file); err == nil {
			passwords[abs] = password
		}
	}
	if len(passwords) == 0 {
		return nil, nil
	}
	return passwords, nil
}

func parseRequestedAnalysisFlags(entries []string) (map[string][]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	requested := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		file, analysesValue, ok := strings.Cut(entry, "=")
		file = strings.TrimSpace(file)
		if !ok || file == "" || strings.TrimSpace(analysesValue) == "" {
			return nil, errors.New("--requested-analysis must use file=analysis[,analysis...] with a non-empty file and analysis list")
		}
		analyses, err := splitRequestedAnalyses(analysesValue)
		if err != nil {
			return nil, err
		}
		requested[file] = analyses
		if abs, err := filepath.Abs(file); err == nil {
			requested[abs] = analyses
		}
	}
	if len(requested) == 0 {
		return nil, nil
	}
	return requested, nil
}

func splitRequestedAnalyses(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	analyses := make([]string, 0, len(parts))
	for _, part := range parts {
		analysis := strings.TrimSpace(part)
		if analysis == "" {
			return nil, errors.New("--requested-analysis entries must not contain empty analyses")
		}
		analyses = append(analyses, analysis)
	}
	return analyses, nil
}

func parseDiskEncryptionKeyFlags(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	keys := make(map[string]string, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		file, key, ok := strings.Cut(entry, "=")
		file = strings.TrimSpace(file)
		if !ok || file == "" || key == "" {
			return nil, errors.New("--disk-encryption-key must use file=key with a non-empty file and key")
		}
		keys[file] = key
		if abs, err := filepath.Abs(file); err == nil {
			keys[abs] = key
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return keys, nil
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
