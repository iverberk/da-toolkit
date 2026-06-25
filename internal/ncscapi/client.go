// Package ncscapi contains the toolkit-owned NCSC Data Analysis API client.
//
// Generated OpenAPI bindings stay in the generated subpackage. This package is
// responsible for OAuth client-credentials authentication, memory-only token
// handling, error normalization, and stable application-level call sites.
package ncscapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	DefaultBaseURL = "https://api.ncsc.nl/data-analysis"
)

type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	HTTPClient   *http.Client
}

type Client struct {
	api *generated.ClientWithResponses
}

type ListOptions struct {
	Filters       *string
	Sort          *string
	Search        *string
	SearchFields  *string
	CaseSensitive *bool
	Page          *int
	Size          *int
}

type Result struct {
	ContentType string
	Body        []byte
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("oauth client id is required")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("oauth client secret is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	tokenURL := strings.TrimSpace(cfg.TokenURL)
	if tokenURL == "" {
		return nil, errors.New("oauth token url is required")
	}

	baseTransport := http.DefaultTransport
	if cfg.HTTPClient != nil && cfg.HTTPClient.Transport != nil {
		baseTransport = cfg.HTTPClient.Transport
	}
	tokenHTTPClient := &http.Client{Transport: baseTransport}
	if cfg.HTTPClient != nil && cfg.HTTPClient.Timeout != 0 {
		tokenHTTPClient.Timeout = cfg.HTTPClient.Timeout
	}

	tokens := &tokenSource{
		tokenURL:     tokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		scopes:       oauthScopes(cfg.Scopes),
		httpClient:   tokenHTTPClient,
	}
	httpClient := &http.Client{
		Transport: &apiCompatibilityTransport{base: &bearerTransport{base: baseTransport, source: tokens}},
	}
	if cfg.HTTPClient != nil && cfg.HTTPClient.Timeout != 0 {
		httpClient.Timeout = cfg.HTTPClient.Timeout
	}

	api, err := generated.NewClientWithResponses(baseURL, generated.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create generated NCSC client: %w", err)
	}
	return &Client{api: api}, nil
}

func (c *Client) CreateAnalysis(ctx context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error) {
	resp, err := c.api.CreateDataAnalysisV1DataAnalysesPostWithResponse(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("create data analysis: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	if resp.StatusCode() == http.StatusAccepted {
		created, err := parseAcceptedCreateAnalysis(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("create data analysis: parse accepted response: %w", err)
		}
		return created, nil
	}
	return nil, normalizeAPIError("create data analysis", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
}

func (c *Client) ListAnalyses(ctx context.Context, opts ListOptions) (*generated.PageDataAnalysisCollection, error) {
	params := &generated.ReadDataAnalysesV1DataAnalysesGetParams{
		Filters:       opts.Filters,
		Sort:          opts.Sort,
		Search:        opts.Search,
		SearchFields:  opts.SearchFields,
		CaseSensitive: opts.CaseSensitive,
		Page:          opts.Page,
		Size:          opts.Size,
	}
	resp, err := c.api.ReadDataAnalysesV1DataAnalysesGetWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list data analyses: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, normalizeAPIError("list data analyses", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
	}
	return resp.JSON200, nil
}

func (c *Client) ReadAnalysis(ctx context.Context, id uuid.UUID) (*generated.DataAnalysisResource, error) {
	resp, err := c.api.ReadDataAnalysisV1DataAnalysesDataAnalysisIdGetWithResponse(ctx, openapi_types.UUID(id))
	if err != nil {
		return nil, fmt.Errorf("read data analysis: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, normalizeAPIError("read data analysis", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
	}
	return resp.JSON200, nil
}

func (c *Client) DeleteAnalysis(ctx context.Context, id uuid.UUID) error {
	resp, err := c.api.DeleteDataAnalysisV1DataAnalysesDataAnalysisIdDeleteWithResponse(ctx, openapi_types.UUID(id))
	if err != nil {
		return fmt.Errorf("delete data analysis: %w", err)
	}
	if resp.StatusCode() != http.StatusAccepted {
		return normalizeAPIError("delete data analysis", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
	}
	return nil
}

func (c *Client) GetResultFile(ctx context.Context, id uuid.UUID, fileName string) (Result, error) {
	resp, err := c.api.DataAnalysisGetResultV1DataAnalysesDataAnalysisIdResultsFileNameGetWithResponse(ctx, openapi_types.UUID(id), fileName)
	if err != nil {
		return Result{}, fmt.Errorf("retrieve result file: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return Result{}, normalizeAPIError("retrieve result file", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
	}
	return Result{ContentType: contentType(resp.HTTPResponse), Body: append([]byte(nil), resp.Body...)}, nil
}

func (c *Client) GetResultArchive(ctx context.Context, id uuid.UUID) (Result, error) {
	resp, err := c.api.DataAnalysisGetResultsArchiveV1DataAnalysesDataAnalysisIdResultsPostWithResponse(ctx, openapi_types.UUID(id))
	if err != nil {
		return Result{}, fmt.Errorf("retrieve result archive: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return Result{}, normalizeAPIError("retrieve result archive", resp.StatusCode(), resp.HTTPResponse, resp.Body, resp.JSON422)
	}
	return Result{ContentType: contentType(resp.HTTPResponse), Body: append([]byte(nil), resp.Body...)}, nil
}

func NewOtherAnalysisRequest(files []generated.DataAnalysisFile, metadata map[string]interface{}) (generated.StoreDataAnalysisRequest, error) {
	var req generated.StoreDataAnalysisRequest
	err := req.FromDataAnalysisOtherData(generated.DataAnalysisOtherData{
		Files:                 files,
		ManualAnalysisAllowed: generated.True,
		Metadata:              optionalMetadata(metadata),
		Type:                  generated.Other,
	})
	return req, err
}

func NewMalwareAnalysisRequest(files []generated.MalwareDataAnalysisFile, metadata map[string]interface{}) (generated.StoreDataAnalysisRequest, error) {
	var req generated.StoreDataAnalysisRequest
	err := req.FromDataAnalysisMalwareData(generated.DataAnalysisMalwareData{
		Files:    files,
		Metadata: optionalMetadata(metadata),
		Type:     generated.Malware,
	})
	return req, err
}

func NewForensicAnalysisRequest(files []generated.ForensicDataAnalysisFile, manualAnalysisAllowed bool, metadata map[string]interface{}) (generated.StoreDataAnalysisRequest, error) {
	var req generated.StoreDataAnalysisRequest
	err := req.FromDataAnalysisForensicData(generated.DataAnalysisForensicData{
		Files:                 files,
		ManualAnalysisAllowed: manualAnalysisAllowed,
		Metadata:              optionalMetadata(metadata),
		Type:                  generated.DataAnalysisForensicDataTypeForensic,
	})
	return req, err
}

func optionalMetadata(metadata map[string]interface{}) *map[string]interface{} {
	if len(metadata) == 0 {
		return nil
	}
	copy := make(map[string]interface{}, len(metadata))
	for k, v := range metadata {
		copy[k] = v
	}
	return &copy
}

func contentType(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get("Content-Type")
}

func parseAcceptedCreateAnalysis(body []byte) (*generated.DataAnalysisCreatedResource, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("empty response body")
	}
	var created generated.DataAnalysisCreatedResource
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

func oauthScopes(configured []string) []string {
	scopes := append([]string{"openid"}, configured...)
	seen := make(map[string]bool, len(scopes))
	unique := scopes[:0]
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		unique = append(unique, scope)
	}
	return unique
}

type bearerTransport struct {
	base   http.RoundTripper
	source *tokenSource
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.source.token(req.Context())
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token.accessToken)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

type apiCompatibilityTransport struct {
	base http.RoundTripper
}

func (t *apiCompatibilityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil || !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return resp, err
	}
	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	normalized := normalizeNaiveAPITimestamps(body)
	resp.Body = io.NopCloser(bytes.NewReader(normalized))
	resp.ContentLength = int64(len(normalized))
	resp.Header.Set("Content-Length", fmt.Sprint(len(normalized)))
	return resp, nil
}

var naiveAPITimestampPattern = regexp.MustCompile(`"((?:completed|created|expired|processing)_at)"\s*:\s*"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?)"`)

func normalizeNaiveAPITimestamps(body []byte) []byte {
	return naiveAPITimestampPattern.ReplaceAll(body, []byte(`"${1}":"${2}Z"`))
}

type tokenSource struct {
	mu           sync.Mutex
	tokenURL     string
	clientID     string
	clientSecret string
	scopes       []string
	httpClient   *http.Client
	cached       oauthToken
}

type oauthToken struct {
	accessToken string
	expiresAt   time.Time
}

func (s *tokenSource) token(ctx context.Context) (oauthToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached.accessToken != "" && time.Now().Before(s.cached.expiresAt.Add(-30*time.Second)) {
		return s.cached, nil
	}
	token, err := s.fetch(ctx)
	if err != nil {
		return oauthToken{}, err
	}
	s.cached = token
	return token, nil
}

func (s *tokenSource) fetch(ctx context.Context) (oauthToken, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if len(s.scopes) > 0 {
		form.Set("scope", strings.Join(s.scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthToken{}, fmt.Errorf("create oauth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.clientID, s.clientSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return oauthToken{}, fmt.Errorf("request oauth token: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return oauthToken{}, fmt.Errorf("read oauth token response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return oauthToken{}, &APIError{Operation: "request oauth token", StatusCode: resp.StatusCode, Message: "oauth token endpoint returned " + resp.Status, Details: oauthStatusDetails(resp.StatusCode)}
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		return oauthToken{}, fmt.Errorf("decode oauth token response: %w", err)
	}
	if payload.AccessToken == "" {
		return oauthToken{}, errors.New("oauth token response did not include access_token")
	}
	if payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer") {
		return oauthToken{}, errors.New("oauth token response used unsupported token_type")
	}
	expiresAt := time.Now().Add(time.Hour)
	if payload.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return oauthToken{accessToken: payload.AccessToken, expiresAt: expiresAt}, nil
}
