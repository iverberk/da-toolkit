package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"da-toolkit/internal/ncscapi"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type resultAPI interface {
	ListAnalyses(context.Context, ncscapi.ListOptions) (any, error)
	ReadAnalysis(context.Context, uuid.UUID) (analysisResults, error)
	GetResultArchive(context.Context, uuid.UUID) (ncscapi.Result, error)
	GetResultFile(context.Context, uuid.UUID, string) (ncscapi.Result, error)
}

type ncscResultAPI struct{ client *ncscapi.Client }

func (a ncscResultAPI) ListAnalyses(ctx context.Context, opts ncscapi.ListOptions) (any, error) {
	return a.client.ListAnalyses(ctx, opts)
}

func (a ncscResultAPI) ReadAnalysis(ctx context.Context, id uuid.UUID) (analysisResults, error) {
	analysis, err := a.client.ReadAnalysis(ctx, id)
	if err != nil {
		return analysisResults{}, err
	}
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
	return analysisResults{Payload: analysis, Files: files}, nil
}

func (a ncscResultAPI) GetResultArchive(ctx context.Context, id uuid.UUID) (ncscapi.Result, error) {
	return a.client.GetResultArchive(ctx, id)
}

func (a ncscResultAPI) GetResultFile(ctx context.Context, id uuid.UUID, name string) (ncscapi.Result, error) {
	return a.client.GetResultFile(ctx, id, name)
}

type analysisResults struct {
	Payload any
	Files   []fileResults
}

type fileResults struct {
	Name    string
	Results []resultMetadata
}

type resultMetadata struct {
	Name string
	Size int
}

var resultFlags struct {
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
	output        string
	overwrite     bool
}

var newResultAPI = func() (resultAPI, error) {
	client, err := ncscapi.NewClient(ncscapi.Config{
		BaseURL:      firstNonEmpty(resultFlags.baseURL, os.Getenv("DA_TOOLKIT_API_BASE_URL"), ncscapi.DefaultBaseURL),
		TokenURL:     firstNonEmpty(resultFlags.tokenURL, os.Getenv("DA_TOOLKIT_TOKEN_URL")),
		ClientID:     firstNonEmpty(resultFlags.clientID, os.Getenv("DA_TOOLKIT_CLIENT_ID")),
		ClientSecret: firstNonEmpty(resultFlags.clientSecret, os.Getenv("DA_TOOLKIT_CLIENT_SECRET")),
		HTTPClient:   &http.Client{Timeout: 0},
	})
	if err != nil {
		return nil, err
	}
	return ncscResultAPI{client: client}, nil
}

// resultCmd represents the result command
var resultCmd = &cobra.Command{
	Use:   "result",
	Short: "retrieve analysis results",
	Long:  "List data-analyses, inspect result metadata, and download available result archives or files.",
	Args:  cobra.NoArgs,
}

var resultListCmd = &cobra.Command{
	Use:   "list",
	Short: "list data-analyses",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		api, err := newResultAPI()
		if err != nil {
			return err
		}
		var page, size *int
		if cmd.Flags().Changed("page") {
			page = &resultFlags.page
		}
		if cmd.Flags().Changed("size") {
			size = &resultFlags.size
		}
		listed, err := api.ListAnalyses(cmd.Context(), ncscapi.ListOptions{
			Filters:       optionalString(resultFlags.filters),
			Sort:          optionalString(resultFlags.sort),
			Search:        optionalString(resultFlags.search),
			SearchFields:  optionalString(resultFlags.searchFields),
			CaseSensitive: optionalBool(cmd.Flags().Changed("case-sensitive"), resultFlags.caseSensitive),
			Page:          page,
			Size:          size,
		})
		if err != nil {
			return err
		}
		return writeJSON(cmd, listed)
	},
}

var resultReadCmd = &cobra.Command{
	Use:   "read <analysis-id>",
	Short: "read one data-analysis",
	Args:  uuidArg("analysis-id"),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		api, err := newResultAPI()
		if err != nil {
			return err
		}
		analysis, err := api.ReadAnalysis(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeJSON(cmd, analysis.Payload)
	},
}

var resultArchiveCmd = &cobra.Command{
	Use:   "archive <analysis-id>",
	Short: "download a result archive",
	Args:  uuidArg("analysis-id"),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := uuid.MustParse(args[0])
		if strings.TrimSpace(resultFlags.output) == "" {
			return errors.New("--output is required")
		}
		api, err := newResultAPI()
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
		return writeResultFile(cmd, resultFlags.output, result.Body, resultFlags.overwrite)
	},
}

var resultFileCmd = &cobra.Command{
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
		if strings.TrimSpace(resultFlags.output) == "" {
			return errors.New("--output is required")
		}
		api, err := newResultAPI()
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
		return writeResultFile(cmd, resultFlags.output, result.Body, resultFlags.overwrite)
	},
}

func init() {
	resultCmd.PersistentFlags().StringVar(&resultFlags.baseURL, "base-url", "", "NCSC Data Analysis API base URL (or DA_TOOLKIT_API_BASE_URL)")
	resultCmd.PersistentFlags().StringVar(&resultFlags.tokenURL, "token-url", "", "OAuth token URL (or DA_TOOLKIT_TOKEN_URL)")
	resultCmd.PersistentFlags().StringVar(&resultFlags.clientID, "client-id", "", "OAuth client ID (or DA_TOOLKIT_CLIENT_ID)")
	resultCmd.PersistentFlags().StringVar(&resultFlags.clientSecret, "client-secret", "", "OAuth client secret (or DA_TOOLKIT_CLIENT_SECRET; memory-only)")

	resultListCmd.Flags().IntVar(&resultFlags.page, "page", 0, "page number to request")
	resultListCmd.Flags().IntVar(&resultFlags.size, "size", 0, "page size to request")
	resultListCmd.Flags().StringVar(&resultFlags.filters, "filters", "", "API filter expression")
	resultListCmd.Flags().StringVar(&resultFlags.sort, "sort", "", "API sort expression")
	resultListCmd.Flags().StringVar(&resultFlags.search, "search", "", "search query")
	resultListCmd.Flags().StringVar(&resultFlags.searchFields, "search-fields", "", "comma-separated fields to search")
	resultListCmd.Flags().BoolVar(&resultFlags.caseSensitive, "case-sensitive", false, "use case-sensitive search")

	for _, command := range []*cobra.Command{resultArchiveCmd, resultFileCmd} {
		command.Flags().StringVarP(&resultFlags.output, "output", "o", "", "local output path for downloaded result bytes")
		command.Flags().BoolVar(&resultFlags.overwrite, "overwrite", false, "overwrite an existing output file")
	}

	resultCmd.AddCommand(resultListCmd, resultReadCmd, resultArchiveCmd, resultFileCmd)
	rootCmd.AddCommand(resultCmd)
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
