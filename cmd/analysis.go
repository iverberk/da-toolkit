package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"da-toolkit/internal/ncscapi"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type analysisAPI interface {
	ListAnalyses(context.Context, ncscapi.ListOptions) (any, error)
	ReadAnalysis(context.Context, uuid.UUID) (analysisResults, error)
	DeleteAnalysis(context.Context, uuid.UUID) error
	GetResultArchive(context.Context, uuid.UUID) (ncscapi.Result, error)
	GetResultFile(context.Context, uuid.UUID, string) (ncscapi.Result, error)
}

type ncscAnalysisAPI struct{ client *ncscapi.Client }

const analysisWaitFallbackInterval = 30 * time.Second

var analysisWaitSleep = func(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a ncscAnalysisAPI) ListAnalyses(ctx context.Context, opts ncscapi.ListOptions) (any, error) {
	return a.client.ListAnalyses(ctx, opts)
}

func (a ncscAnalysisAPI) ReadAnalysis(ctx context.Context, id uuid.UUID) (analysisResults, error) {
	read, err := a.client.ReadAnalysisWithResponse(ctx, id)
	if err != nil {
		return analysisResults{}, err
	}
	analysis := read.Analysis
	files := make([]fileResults, 0, len(analysis.Files))
	for _, file := range analysis.Files {
		results := []resultMetadata(nil)
		if file.Results != nil {
			results = make([]resultMetadata, 0, len(*file.Results))
			for _, result := range *file.Results {
				results = append(results, resultMetadata{Name: result.Name, Size: result.Size})
			}
		}
		files = append(files, fileResults{Name: file.Name, Results: results})
	}
	return analysisResults{Payload: analysis, Status: string(analysis.Status), Files: files, RetryAfter: parseRetryAfter(read.RetryAfter)}, nil
}

func (a ncscAnalysisAPI) DeleteAnalysis(ctx context.Context, id uuid.UUID) error {
	return a.client.DeleteAnalysis(ctx, id)
}

func (a ncscAnalysisAPI) GetResultArchive(ctx context.Context, id uuid.UUID) (ncscapi.Result, error) {
	return a.client.GetResultArchive(ctx, id)
}

func (a ncscAnalysisAPI) GetResultFile(ctx context.Context, id uuid.UUID, name string) (ncscapi.Result, error) {
	return a.client.GetResultFile(ctx, id, name)
}

type analysisResults struct {
	Payload    any
	Status     string
	Files      []fileResults
	RetryAfter *time.Duration
}

type fileResults struct {
	Name    string
	Results []resultMetadata
}

type resultMetadata struct {
	Name string
	Size int
}

var analysisFlags struct {
	baseURL       string
	tokenURL      string
	clientID      string
	clientSecret  string
	page          int
	size          int
	filters       string
	sort          string
	search        string
	searchFields  string
	caseSensitive bool
	yes           bool
	output        string
	overwrite     bool
	wait          bool
	download      bool
}

var newAnalysisAPI = func() (analysisAPI, error) {
	client, err := ncscapi.NewClient(ncscapi.Config{
		BaseURL:      firstNonEmpty(analysisFlags.baseURL, os.Getenv("DA_TOOLKIT_API_BASE_URL"), ncscapi.DefaultBaseURL),
		TokenURL:     firstNonEmpty(analysisFlags.tokenURL, os.Getenv("DA_TOOLKIT_TOKEN_URL")),
		ClientID:     firstNonEmpty(analysisFlags.clientID, os.Getenv("DA_TOOLKIT_CLIENT_ID")),
		ClientSecret: firstNonEmpty(analysisFlags.clientSecret, os.Getenv("DA_TOOLKIT_CLIENT_SECRET")),
		HTTPClient:   &http.Client{Timeout: 0},
	})
	if err != nil {
		return nil, err
	}
	return ncscAnalysisAPI{client: client}, nil
}

// analysisCmd represents the analysis command.
var analysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "work with data-analyses",
	Long:  "Create data-analyses, list and read data-analysis metadata, and download available result archives or files.",
	Args:  cobra.NoArgs,
}

var analysisListCmd = &cobra.Command{
	Use:   "list",
	Short: "list data-analyses",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		api, err := newAnalysisAPI()
		if err != nil {
			return err
		}
		var page, size *int
		if cmd.Flags().Changed("page") {
			page = &analysisFlags.page
		}
		if cmd.Flags().Changed("size") {
			size = &analysisFlags.size
		}
		listed, err := api.ListAnalyses(cmd.Context(), ncscapi.ListOptions{
			Filters:       optionalString(analysisFlags.filters),
			Sort:          optionalString(analysisFlags.sort),
			Search:        optionalString(analysisFlags.search),
			SearchFields:  optionalString(analysisFlags.searchFields),
			CaseSensitive: optionalBool(cmd.Flags().Changed("case-sensitive"), analysisFlags.caseSensitive),
			Page:          page,
			Size:          size,
		})
		if err != nil {
			return err
		}
		return writeJSON(cmd, listed)
	},
}

var analysisReadCmd = &cobra.Command{
	Use:   "read <analysis-id>",
	Short: "read one data-analysis",
	Args:  uuidArg("analysis-id"),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		if analysisFlags.download && !analysisFlags.wait {
			return errors.New("--download is only available with --wait; use analysis archive or analysis file to download results from a completed analysis")
		}
		api, err := newAnalysisAPI()
		if err != nil {
			return err
		}
		if analysisFlags.wait {
			return waitForAnalysis(cmd, api, id, analysisFlags.download)
		}
		analysis, err := api.ReadAnalysis(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(cmd, analysis.Payload)
	},
}

var analysisDeleteCmd = &cobra.Command{
	Use:   "delete <analysis-id>",
	Short: "delete one data-analysis",
	Long:  "Delete one data-analysis after explicit confirmation. Pass --yes to confirm non-interactively.",
	Args:  uuidArg("analysis-id"),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		confirmed := analysisFlags.yes
		if !confirmed {
			fmt.Fprintf(cmd.OutOrStdout(), "Delete data-analysis %s? [y/N]: ", id)
			var response string
			_, _ = fmt.Fscanln(cmd.InOrStdin(), &response)
			confirmed = strings.EqualFold(strings.TrimSpace(response), "y") || strings.EqualFold(strings.TrimSpace(response), "yes")
		}
		if !confirmed {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Delete cancelled.")
			return err
		}

		api, err := newAnalysisAPI()
		if err != nil {
			return err
		}
		if err := api.DeleteAnalysis(cmd.Context(), id); err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Deleted data-analysis %s\n", id)
		return err
	},
}

var analysisArchiveCmd = &cobra.Command{
	Use:   "archive <analysis-id>",
	Short: "download a result archive",
	Args:  uuidArg("analysis-id"),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		if strings.TrimSpace(analysisFlags.output) == "" {
			return errors.New("--output is required")
		}
		api, err := newAnalysisAPI()
		if err != nil {
			return err
		}
		analysis, err := api.ReadAnalysis(cmd.Context(), id)
		if err != nil {
			return err
		}
		if !hasAnyResult(analysis) {
			return errors.New("data-analysis does not report available results yet")
		}
		result, err := api.GetResultArchive(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeResultFile(cmd, analysisFlags.output, result.Body, analysisFlags.overwrite)
	},
}

var analysisFileCmd = &cobra.Command{
	Use:   "file <analysis-id> <result-name>",
	Short: "download a named result file",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(2)(cmd, args); err != nil {
			return err
		}
		if _, err := uuid.Parse(args[0]); err != nil {
			return fmt.Errorf("invalid analysis-id %q: %w", args[0], err)
		}
		if strings.TrimSpace(args[1]) == "" {
			return errors.New("result-name is required")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		name := args[1]
		if strings.TrimSpace(analysisFlags.output) == "" {
			return errors.New("--output is required")
		}
		api, err := newAnalysisAPI()
		if err != nil {
			return err
		}
		analysis, err := api.ReadAnalysis(cmd.Context(), id)
		if err != nil {
			return err
		}
		if !hasResultNamed(analysis, name) {
			return fmt.Errorf("data-analysis does not report result file %q as available", name)
		}
		result, err := api.GetResultFile(cmd.Context(), id, name)
		if err != nil {
			return err
		}
		return writeResultFile(cmd, analysisFlags.output, result.Body, analysisFlags.overwrite)
	},
}

func init() {
	analysisCmd.PersistentFlags().StringVar(&analysisFlags.baseURL, "base-url", "", "NCSC Data Analysis API base URL (or DA_TOOLKIT_API_BASE_URL)")
	analysisCmd.PersistentFlags().StringVar(&analysisFlags.tokenURL, "token-url", "", "OAuth token URL (or DA_TOOLKIT_TOKEN_URL)")
	analysisCmd.PersistentFlags().StringVar(&analysisFlags.clientID, "client-id", "", "OAuth client ID (or DA_TOOLKIT_CLIENT_ID)")
	analysisCmd.PersistentFlags().StringVar(&analysisFlags.clientSecret, "client-secret", "", "OAuth client secret (or DA_TOOLKIT_CLIENT_SECRET; memory-only)")

	analysisListCmd.Flags().IntVar(&analysisFlags.page, "page", 0, "page number to request")
	analysisListCmd.Flags().IntVar(&analysisFlags.size, "size", 0, "page size to request")
	analysisListCmd.Flags().StringVar(&analysisFlags.filters, "filters", "", "API filter expression")
	analysisListCmd.Flags().StringVar(&analysisFlags.sort, "sort", "", "API sort expression")
	analysisListCmd.Flags().StringVar(&analysisFlags.search, "search", "", "search query")
	analysisListCmd.Flags().StringVar(&analysisFlags.searchFields, "search-fields", "", "comma-separated fields to search")
	analysisListCmd.Flags().BoolVar(&analysisFlags.caseSensitive, "case-sensitive", false, "use case-sensitive search")
	analysisDeleteCmd.Flags().BoolVar(&analysisFlags.yes, "yes", false, "confirm deletion without prompting")
	analysisReadCmd.Flags().BoolVar(&analysisFlags.wait, "wait", false, "poll until the data-analysis completes")
	analysisReadCmd.Flags().BoolVar(&analysisFlags.download, "download", false, "with --wait, download newly observed result files using result names as local paths")

	for _, command := range []*cobra.Command{analysisArchiveCmd, analysisFileCmd} {
		command.Flags().StringVarP(&analysisFlags.output, "output", "o", "", "local output path for downloaded result bytes")
		command.Flags().BoolVar(&analysisFlags.overwrite, "overwrite", false, "overwrite an existing output file")
	}

	analysisCmd.AddCommand(analysisCreateCmd, analysisUploadCmd, analysisDecryptCmd, analysisListCmd, analysisReadCmd, analysisDeleteCmd, analysisArchiveCmd, analysisFileCmd)
	rootCmd.AddCommand(analysisCmd)
}

func uuidArg(name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := uuid.Parse(args[0]); err != nil {
			return fmt.Errorf("invalid %s %q: %w", name, args[0], err)
		}
		return nil
	}
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func optionalBool(changed bool, value bool) *bool {
	if !changed {
		return nil
	}
	return &value
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("format response: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
	return err
}

func waitForAnalysis(cmd *cobra.Command, api analysisAPI, id uuid.UUID, download bool) error {
	state := newWaitDownloadState()
	completed := false
	for {
		analysis, err := api.ReadAnalysis(cmd.Context(), id)
		if err != nil {
			return err
		}
		if download {
			if err := state.observeAndDownload(cmd, api, id, analysis); err != nil {
				return err
			}
		}
		completed = completed || strings.EqualFold(analysis.Status, "completed")
		if completed && (!download || state.pendingCount() == 0) {
			if download {
				return state.printSummary(cmd)
			}
			return writeJSON(cmd, analysis.Payload)
		}
		delay := analysisWaitFallbackInterval
		if analysis.RetryAfter != nil {
			delay = *analysis.RetryAfter
		}
		if delay < 0 {
			delay = analysisWaitFallbackInterval
		}
		if download {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Waiting for data-analysis %s; status=%s; downloaded=%d skipped=%d pending=%d; next poll in %s\n", id, analysis.Status, state.downloaded, state.skipped, state.pendingCount(), delay)
		}
		if err := analysisWaitSleep(cmd.Context(), delay); err != nil {
			return err
		}
	}
}

type waitDownloadState struct {
	complete   map[string]bool
	observed   map[string]bool
	downloaded int
	skipped    int
	failed     int
}

func newWaitDownloadState() *waitDownloadState {
	return &waitDownloadState{complete: map[string]bool{}, observed: map[string]bool{}}
}

func (s *waitDownloadState) observeAndDownload(cmd *cobra.Command, api analysisAPI, id uuid.UUID, analysis analysisResults) error {
	for _, file := range analysis.Files {
		for _, result := range file.Results {
			name := result.Name
			if strings.TrimSpace(name) != "" {
				s.observed[name] = true
			}
		}
	}
	for name := range s.observed {
		if s.complete[name] {
			continue
		}
		written, err := downloadObservedResult(cmd, api, id, name)
		if err != nil {
			s.failed++
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Failed to download %s: %v; will retry\n", name, err)
			continue
		}
		s.complete[name] = true
		if written {
			s.downloaded++
		} else {
			s.skipped++
		}
	}
	return nil
}

func (s *waitDownloadState) pendingCount() int {
	pending := 0
	for name := range s.observed {
		if !s.complete[name] {
			pending++
		}
	}
	return pending
}

func (s *waitDownloadState) printSummary(cmd *cobra.Command) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Download complete: downloaded=%d skipped=%d failed-attempts=%d\n", s.downloaded, s.skipped, s.failed)
	return err
}

func downloadObservedResult(cmd *cobra.Command, api analysisAPI, id uuid.UUID, name string) (bool, error) {
	if _, err := os.Stat(name); err == nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skipped existing result %s\n", name)
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat result path %q: %w", name, err)
	}
	result, err := api.GetResultFile(cmd.Context(), id, name)
	if err != nil {
		return false, err
	}
	if err := writeResultFileCreatingParents(cmd, name, result.Body); err != nil {
		return false, err
	}
	return true, nil
}

func writeResultFileCreatingParents(cmd *cobra.Command, path string, body []byte) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skipped existing result %s\n", path)
			return nil
		}
		return fmt.Errorf("open output file %q: %w", path, err)
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write output file %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close output file %q: %w", path, closeErr)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded result %s (%d bytes)\n", path, len(body))
	return err
}

func parseRetryAfter(value string) *time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		return &delay
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return nil
	}
	if delay := time.Until(when); delay > 0 {
		return &delay
	}
	delay := time.Duration(0)
	return &delay
}

func hasAnyResult(analysis analysisResults) bool {
	for _, file := range analysis.Files {
		if len(file.Results) > 0 {
			return true
		}
	}
	return false
}

func hasResultNamed(analysis analysisResults, name string) bool {
	for _, file := range analysis.Files {
		for _, result := range file.Results {
			if result.Name == name {
				return true
			}
		}
	}
	return false
}

func writeResultFile(cmd *cobra.Command, path string, body []byte, overwrite bool) error {
	flag := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file %q already exists; pass --overwrite to replace it", path)
		}
		return fmt.Errorf("open output file %q: %w", path, err)
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write output file %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close output file %q: %w", path, closeErr)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d bytes to %s\n", len(body), path)
	return err
}
