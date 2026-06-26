package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"da-toolkit/internal/analysis"
	"da-toolkit/internal/ncscapi"
	"github.com/spf13/cobra"
)

var analysisCreateFlags struct {
	baseURL        string
	tokenURL       string
	clientID       string
	clientSecret   string
	workspaceDir   string
	checkpointPath string
	resume         bool
	new            bool
	yes            bool
	analysisType   string
	zipPasswords   []string
	requested      []string
	diskKeys       []string
	manualAllowed  bool
	deferred       bool
	output         string
}

// analysisCreateCmd represents the analysis create command.
var analysisCreateCmd = &cobra.Command{
	Use:   "create [flags] <file> [file...]",
	Short: "create a data-analysis and upload local files",
	Long:  "Create one explicitly selected other, malware, or forensic data-analysis for local files, encrypt each file, upload encrypted artifacts, and persist protected resume metadata. When a usable incomplete checkpoint exists, create resumes it automatically. Use --type for fresh creation, --resume to require resume, and --new to require fresh creation.",
	Args: func(cmd *cobra.Command, args []string) error {
		if analysisCreateFlags.resume {
			return nil
		}
		if analysisCreateFlags.new && len(args) == 0 {
			return errors.New("at least one local file is required with --new")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		zipPasswords, err := parseZipPasswordFlags(analysisCreateFlags.zipPasswords)
		if err != nil {
			return err
		}
		requested, err := parseRequestedAnalysisFlags(analysisCreateFlags.requested)
		if err != nil {
			return err
		}
		diskKeys, err := parseDiskEncryptionKeyFlags(analysisCreateFlags.diskKeys)
		if err != nil {
			return err
		}
		if len(zipPasswords) > 0 && analysisCreateFlags.analysisType != analysis.AnalysisTypeMalware {
			return errors.New("--zip-password can only be used with --type malware")
		}
		if len(requested) > 0 && analysisCreateFlags.analysisType != analysis.AnalysisTypeForensic {
			return errors.New("--requested-analysis can only be used with --type forensic")
		}
		if len(diskKeys) > 0 && analysisCreateFlags.analysisType != analysis.AnalysisTypeForensic {
			return errors.New("--disk-encryption-key can only be used with --type forensic")
		}
		if cmd.Flags().Changed("manual-analysis-allowed") && analysisCreateFlags.analysisType != analysis.AnalysisTypeForensic {
			return errors.New("--manual-analysis-allowed can only be used with --type forensic")
		}
		if analysisCreateFlags.deferred {
			return runAnalysisCreateDeferred(cmd, args, zipPasswords, requested, diskKeys)
		}
		workspace := analysisCreateFlags.workspaceDir
		if workspace == "" {
			workspace = defaultAnalysisWorkspace(args)
		}
		opts, mode, err := analysis.Plan(analysis.Options{
			Files:                 args,
			WorkspaceDir:          workspace,
			CheckpointPath:        analysisCreateFlags.checkpointPath,
			Resume:                analysisCreateFlags.resume,
			New:                   analysisCreateFlags.new,
			AssumeYes:             analysisCreateFlags.yes,
			AnalysisType:          analysisCreateFlags.analysisType,
			ZipPasswords:          zipPasswords,
			RequestedAnalyses:     requested,
			DiskEncryptionKeys:    diskKeys,
			ManualAnalysisAllowed: analysisCreateFlags.manualAllowed,
			Stdin:                 os.Stdin,
			Stdout:                cmd.OutOrStdout(),
		})
		if err != nil {
			return err
		}
		if mode == analysis.RunModeCreate {
			if !cmd.Flags().Changed("type") {
				return errors.New("--type is required for fresh data-analysis creation (supported: other, malware, forensic)")
			}
			switch analysisCreateFlags.analysisType {
			case analysis.AnalysisTypeOther, analysis.AnalysisTypeMalware, analysis.AnalysisTypeForensic:
			default:
				return fmt.Errorf("unsupported analysis type %q (supported: other, malware, forensic)", analysisCreateFlags.analysisType)
			}
		}
		if mode == analysis.RunModeCreate && len(args) == 0 {
			return errors.New("at least one local file is required unless a usable checkpoint can be resumed")
		}
		var client analysis.AnalysisCreator
		if mode == analysis.RunModeCreate {
			apiClient, err := ncscapi.NewClient(ncscapi.Config{
				BaseURL:      firstNonEmpty(analysisCreateFlags.baseURL, analysisFlags.baseURL, os.Getenv("DA_TOOLKIT_API_BASE_URL"), ncscapi.DefaultBaseURL),
				TokenURL:     firstNonEmpty(analysisCreateFlags.tokenURL, analysisFlags.tokenURL, os.Getenv("DA_TOOLKIT_TOKEN_URL")),
				ClientID:     firstNonEmpty(analysisCreateFlags.clientID, analysisFlags.clientID, os.Getenv("DA_TOOLKIT_CLIENT_ID")),
				ClientSecret: firstNonEmpty(analysisCreateFlags.clientSecret, analysisFlags.clientSecret, os.Getenv("DA_TOOLKIT_CLIENT_SECRET")),
				HTTPClient:   &http.Client{Timeout: 0},
			})
			if err != nil {
				return err
			}
			client = apiClient
		}
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		opts.API = client
		_, err = analysis.Run(ctx, opts)
		return err
	},
}

func init() {
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.baseURL, "base-url", "", "NCSC Data Analysis API base URL (or DA_TOOLKIT_API_BASE_URL)")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.tokenURL, "token-url", "", "OAuth token URL (or DA_TOOLKIT_TOKEN_URL)")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.clientID, "client-id", "", "OAuth client ID (or DA_TOOLKIT_CLIENT_ID)")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.clientSecret, "client-secret", "", "OAuth client secret (or DA_TOOLKIT_CLIENT_SECRET; memory-only)")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.workspaceDir, "workspace", "", "directory for encrypted artifacts and checkpoint metadata")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.checkpointPath, "checkpoint", "", "checkpoint file for restart-resume metadata")
	analysisCreateCmd.Flags().StringVar(&analysisCreateFlags.analysisType, "type", "", "data-analysis type to create (required for fresh creation: other, malware, or forensic)")
	analysisCreateCmd.Flags().StringArrayVar(&analysisCreateFlags.zipPasswords, "zip-password", nil, "malware zip password as file=password; repeat per protected file (memory-only)")
	analysisCreateCmd.Flags().StringArrayVar(&analysisCreateFlags.requested, "requested-analysis", nil, "forensic requested analyses as file=analysis[,analysis...]; repeat per file")
	analysisCreateCmd.Flags().StringArrayVar(&analysisCreateFlags.diskKeys, "disk-encryption-key", nil, "forensic disk encryption key as file=key; repeat per encrypted disk image (memory-only)")
	analysisCreateCmd.Flags().BoolVar(&analysisCreateFlags.manualAllowed, "manual-analysis-allowed", false, "allow analysts to view forensic source data")
	analysisCreateCmd.Flags().BoolVar(&analysisCreateFlags.resume, "resume", false, "strictly resume from a usable incomplete checkpoint and fail if none exists")
	analysisCreateCmd.Flags().BoolVar(&analysisCreateFlags.new, "new", false, "create a fresh data-analysis only when no checkpoint exists at the resolved target")
	analysisCreateCmd.Flags().BoolVar(&analysisCreateFlags.yes, "yes", false, "delete successfully uploaded encrypted artifacts without prompting")
	analysisCreateCmd.Flags().BoolVar(&analysisCreateFlags.deferred, "deferred", false, "create upload handoff JSON without reading local files")
	analysisCreateCmd.Flags().StringVarP(&analysisCreateFlags.output, "output", "o", "", "write deferred handoff JSON to this path instead of stdout")
}

func runAnalysisCreateDeferred(cmd *cobra.Command, args []string, zipPasswords map[string]string, requested map[string][]string, diskKeys map[string]string) error {
	if len(args) == 0 {
		return errors.New("at least one filename is required with --deferred")
	}
	if !cmd.Flags().Changed("type") {
		return errors.New("--type is required for deferred data-analysis creation (supported: other, malware, forensic)")
	}
	switch analysisCreateFlags.analysisType {
	case analysis.AnalysisTypeOther, analysis.AnalysisTypeMalware, analysis.AnalysisTypeForensic:
	default:
		return fmt.Errorf("unsupported analysis type %q (supported: other, malware, forensic)", analysisCreateFlags.analysisType)
	}
	apiClient, err := ncscapi.NewClient(ncscapi.Config{
		BaseURL:      firstNonEmpty(analysisCreateFlags.baseURL, analysisFlags.baseURL, os.Getenv("DA_TOOLKIT_API_BASE_URL"), ncscapi.DefaultBaseURL),
		TokenURL:     firstNonEmpty(analysisCreateFlags.tokenURL, analysisFlags.tokenURL, os.Getenv("DA_TOOLKIT_TOKEN_URL")),
		ClientID:     firstNonEmpty(analysisCreateFlags.clientID, analysisFlags.clientID, os.Getenv("DA_TOOLKIT_CLIENT_ID")),
		ClientSecret: firstNonEmpty(analysisCreateFlags.clientSecret, analysisFlags.clientSecret, os.Getenv("DA_TOOLKIT_CLIENT_SECRET")),
		HTTPClient:   &http.Client{Timeout: 0},
	})
	if err != nil {
		return err
	}
	handoff, err := analysis.CreateDeferred(cmd.Context(), analysis.Options{
		Files:                 args,
		AnalysisType:          analysisCreateFlags.analysisType,
		ZipPasswords:          zipPasswords,
		RequestedAnalyses:     requested,
		DiskEncryptionKeys:    diskKeys,
		ManualAnalysisAllowed: analysisCreateFlags.manualAllowed,
		API:                   apiClient,
		Stdout:                cmd.OutOrStdout(),
	})
	if err != nil {
		return err
	}
	return writeHandoff(cmd, handoff, analysisCreateFlags.output)
}

func writeHandoff(cmd *cobra.Command, handoff analysis.Handoff, output string) error {
	encoded, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return fmt.Errorf("format handoff: %w", err)
	}
	warning := "Warning: handoff JSON contains AES keys and SAS bearer URLs; treat it as secret-bearing."
	if strings.TrimSpace(output) == "" {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), warning); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return err
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write handoff %q: %w", output, err)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\nWrote deferred upload handoff to %s\n", warning, output)
	return err
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

func defaultAnalysisWorkspace(files []string) string {
	if len(files) == 0 {
		return filepath.Join(".da-toolkit", "analysis")
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
	return filepath.Join(".da-toolkit", fmt.Sprintf("analysis-%s", id))
}
