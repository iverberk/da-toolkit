package ncscapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
)

const (
	testClientID     = "client-id-value"
	testClientSecret = "client-secret-value"
	testAccessToken  = "access-token-value"
	testAnalysisID   = "11111111-1111-1111-1111-111111111111"
)

func TestOAuthTokenAcquisitionInjectionAndReuse(t *testing.T) {
	var tokenRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			atomic.AddInt32(&tokenRequests, 1)
			assertTokenRequest(t, r)
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": testAccessToken, "token_type": "Bearer", "expires_in": 3600})
		case "/v1/data-analyses":
			assertBearer(t, r)
			if r.Method == http.MethodPost {
				writeJSON(t, w, http.StatusAccepted, createdPayload())
				return
			}
			if r.Method == http.MethodGet {
				if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("size") != "10" {
					t.Fatalf("query = %s, want page=2&size=10", r.URL.RawQuery)
				}
				writeJSON(t, w, http.StatusOK, map[string]any{"items": []any{}, "total": 0, "page": 2, "size": 10, "pages": 0})
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	request := newOtherRequest(t)
	if _, err := client.CreateAnalysis(context.Background(), request); err != nil {
		t.Fatalf("CreateAnalysis returned error: %v", err)
	}
	page, size := 2, 10
	listed, err := client.ListAnalyses(context.Background(), ListOptions{Page: &page, Size: &size})
	if err != nil {
		t.Fatalf("ListAnalyses returned error: %v", err)
	}
	if listed.Page != 2 || listed.Size != 10 {
		t.Fatalf("listed = %+v, want page/size from response", listed)
	}
	if got := atomic.LoadInt32(&tokenRequests); got != 1 {
		t.Fatalf("token requests = %d, want 1 reused in-memory token", got)
	}
}

func TestOmitNullFileHashesRemovesHashFieldsOnlyWhenNil(t *testing.T) {
	withHash := "encrypted-hash"
	request, err := NewOtherAnalysisRequest([]generated.DataAnalysisFile{
		{Name: "deferred.bin", AesKey: "key", Hash: nil},
		{Name: "immediate.bin", AesKey: "key", Hash: &withHash},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleaned, err := OmitNullFileHashes(request)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload.Files[0]["hash"]; ok {
		t.Fatalf("deferred file still contains hash field: %s", data)
	}
	if payload.Files[1]["hash"] != withHash {
		t.Fatalf("immediate hash = %#v, want %q in %s", payload.Files[1]["hash"], withHash, data)
	}
}

func TestListAnalysesAcceptsNaiveAPITimestamps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			assertTokenRequest(t, r)
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": testAccessToken, "token_type": "Bearer", "expires_in": 3600})
		case "/v1/data-analyses":
			assertBearer(t, r)
			writeJSON(t, w, http.StatusOK, map[string]any{
				"items": []map[string]any{{
					"id":            testAnalysisID,
					"type":          "other",
					"status":        "submitted",
					"created_at":    "2026-06-24T20:39:00.203207",
					"processing_at": "2026-06-24T20:40:00",
				}},
				"total": 1,
				"page":  1,
				"size":  10,
				"pages": 1,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	listed, err := client.ListAnalyses(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("ListAnalyses returned error: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].CreatedAt == nil || listed.Items[0].ProcessingAt == nil {
		t.Fatalf("listed = %+v, want parsed timestamps", listed)
	}
	wantCreated := time.Date(2026, 6, 24, 20, 39, 0, 203207000, time.UTC)
	if !listed.Items[0].CreatedAt.Equal(wantCreated) {
		t.Fatalf("created_at = %s, want %s", listed.Items[0].CreatedAt, wantCreated)
	}
}

func TestOAuthTokenFailureDoesNotLeakCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assertTokenRequest(t, r)
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	_, err := client.ListAnalyses(context.Background(), ListOptions{})
	if err == nil {
		t.Fatal("ListAnalyses error = nil, want oauth failure")
	}
	assertNoSecrets(t, err.Error())
}

func TestAnalysisOperationsAndResultRetrieval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			assertTokenRequest(t, r)
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": testAccessToken, "token_type": "Bearer", "expires_in": 3600})
			return
		}
		assertBearer(t, r)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/data-analyses/"+testAnalysisID:
			w.Header().Set("Retry-After", "7")
			writeJSON(t, w, http.StatusOK, readPayload())
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/data-analyses/"+testAnalysisID:
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/data-analyses/"+testAnalysisID+"/results/report.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/data-analyses/"+testAnalysisID+"/results":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte("zip-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	id := uuid.MustParse(testAnalysisID)
	read, err := client.ReadAnalysis(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadAnalysis returned error: %v", err)
	}
	if read.Id != id || read.Status != generated.DataAnalysisStatusCompleted {
		t.Fatalf("read = %+v, want completed test analysis", read)
	}
	readWithResponse, err := client.ReadAnalysisWithResponse(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadAnalysisWithResponse returned error: %v", err)
	}
	if readWithResponse.Analysis.Id != id || readWithResponse.RetryAfter != "7" {
		t.Fatalf("read response = %+v, want analysis and retry-after", readWithResponse)
	}
	if err := client.DeleteAnalysis(context.Background(), id); err != nil {
		t.Fatalf("DeleteAnalysis returned error: %v", err)
	}
	file, err := client.GetResultFile(context.Background(), id, "report.json")
	if err != nil {
		t.Fatalf("GetResultFile returned error: %v", err)
	}
	if file.ContentType != "application/json" || string(file.Body) != `{"ok":true}` {
		t.Fatalf("file = %+v, want json result", file)
	}
	archive, err := client.GetResultArchive(context.Background(), id)
	if err != nil {
		t.Fatalf("GetResultArchive returned error: %v", err)
	}
	if archive.ContentType != "application/zip" || string(archive.Body) != "zip-bytes" {
		t.Fatalf("archive = %+v, want zip result", archive)
	}
}

func TestValidationErrorNormalizationIncludesTraceAndNoSecrets(t *testing.T) {
	traceID := "22222222-2222-2222-2222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": testAccessToken, "token_type": "Bearer", "expires_in": 3600})
			return
		}
		assertBearer(t, r)
		writeJSON(t, w, http.StatusUnprocessableEntity, map[string]any{
			"message":  "invalid data-analysis request",
			"trace_id": traceID,
			"detail":   []map[string]any{{"loc": []any{"body", "files"}, "msg": "at least one file is required", "type": "value_error"}},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	_, err := client.CreateAnalysis(context.Background(), newOtherRequest(t))
	if err == nil {
		t.Fatal("CreateAnalysis error = nil, want validation error")
	}
	message := err.Error()
	for _, want := range []string{"create data analysis", "status 422", "invalid data-analysis request", traceID, "at least one file is required"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
	assertNoSecrets(t, message)
}

func TestForbiddenAnalysisErrorExplainsAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			assertTokenRequest(t, r)
			writeJSON(t, w, http.StatusOK, map[string]any{"access_token": testAccessToken, "token_type": "Bearer", "expires_in": 3600})
			return
		}
		assertBearer(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"insufficient_scope","error_description":"missing data-analysis:create"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	_, err := client.CreateAnalysis(context.Background(), newOtherRequest(t))
	if err == nil {
		t.Fatal("CreateAnalysis error = nil, want forbidden error")
	}
	message := err.Error()
	for _, want := range []string{"create data analysis", "status 403", "Forbidden", "OAuth token acquisition succeeded", "allowed to create data analyses", "scopes/roles", "API environment"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
	assertNoSecrets(t, message)
}

func TestOAuthForbiddenErrorExplainsTokenAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assertTokenRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_scope"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, server.URL+"/oauth/token")
	_, err := client.CreateAnalysis(context.Background(), newOtherRequest(t))
	if err == nil {
		t.Fatal("CreateAnalysis error = nil, want oauth forbidden error")
	}
	message := err.Error()
	for _, want := range []string{"create data analysis", "request oauth token", "status 403", "oauth token endpoint", "denied token issuance", "client-credentials flow", "token endpoint environment"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
	assertNoSecrets(t, message)
}

func newTestClient(t *testing.T, baseURL, tokenURL string) *Client {
	t.Helper()
	client, err := NewClient(Config{BaseURL: baseURL, TokenURL: tokenURL, ClientID: testClientID, ClientSecret: testClientSecret})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func newOtherRequest(t *testing.T) generated.StoreDataAnalysisRequest {
	t.Helper()
	request, err := NewOtherAnalysisRequest([]generated.DataAnalysisFile{{Name: "sample.bin", AesKey: "memory-only-aes-key"}}, map[string]interface{}{"case": "test"})
	if err != nil {
		t.Fatalf("NewOtherAnalysisRequest returned error: %v", err)
	}
	return request
}

func TestNewMalwareAnalysisRequestIncludesOptionalZipPasswords(t *testing.T) {
	hashA := "hash-a"
	hashB := "hash-b"
	password := "zip-secret"
	request, err := NewMalwareAnalysisRequest([]generated.MalwareDataAnalysisFile{
		{Name: "a.zip", AesKey: "memory-only-aes-key-a", Hash: &hashA, ZipPassword: &password},
		{Name: "b.bin", AesKey: "memory-only-aes-key-b", Hash: &hashB},
	}, nil)
	if err != nil {
		t.Fatalf("NewMalwareAnalysisRequest returned error: %v", err)
	}
	malware, err := request.AsDataAnalysisMalwareData()
	if err != nil {
		t.Fatalf("AsDataAnalysisMalwareData returned error: %v", err)
	}
	if malware.Type != generated.Malware || len(malware.Files) != 2 {
		t.Fatalf("malware request = %+v, want malware with two files", malware)
	}
	if malware.Files[0].ZipPassword == nil || *malware.Files[0].ZipPassword != password {
		t.Fatalf("first zip password = %v, want provided password", malware.Files[0].ZipPassword)
	}
	if malware.Files[1].ZipPassword != nil {
		t.Fatalf("second zip password = %v, want omitted", *malware.Files[1].ZipPassword)
	}
}

func assertTokenRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("token method = %s, want POST", r.Method)
	}
	gotID, gotSecret, ok := r.BasicAuth()
	if !ok || gotID != testClientID || gotSecret != testClientSecret {
		t.Fatalf("token basic auth = %q/%q/%v, want configured credentials", gotID, gotSecret, ok)
	}
	if err := r.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}
	if r.Form.Get("grant_type") != "client_credentials" {
		t.Fatalf("grant_type = %q, want client_credentials", r.Form.Get("grant_type"))
	}
	if r.Form.Get("scope") != "openid" {
		t.Fatalf("scope = %q, want openid", r.Form.Get("scope"))
	}
	if strings.Contains(r.URL.RawQuery, testClientSecret) {
		t.Fatalf("client secret leaked in URL: %s", r.URL.String())
	}
}

func assertBearer(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer "+testAccessToken {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}
	if strings.Contains(r.URL.RawQuery, testAccessToken) {
		t.Fatalf("access token leaked in URL: %s", r.URL.String())
	}
}

func assertNoSecrets(t *testing.T, value string) {
	t.Helper()
	for _, secret := range []string{testClientID, testClientSecret, testAccessToken, "memory-only-aes-key", base64.StdEncoding.EncodeToString([]byte("memory-only-aes-key"))} {
		if strings.Contains(value, secret) {
			t.Fatalf("%q leaked secret %q", value, secret)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
}

func createdPayload() map[string]any {
	return map[string]any{
		"id":                      testAnalysisID,
		"type":                    "other",
		"manual_analysis_allowed": true,
		"metadata":                map[string]string{"case": "test"},
		"files":                   []map[string]string{{"name": "sample.bin", "sas_url": "https://storage.example/upload"}},
	}
}

func readPayload() map[string]any {
	return map[string]any{
		"id":                      testAnalysisID,
		"type":                    "other",
		"status":                  "completed",
		"manual_analysis_allowed": true,
		"metadata":                map[string]string{"case": "test"},
		"files": []map[string]any{{
			"name":         "sample.bin",
			"status":       "completed",
			"completed_at": nil,
			"results":      []map[string]any{{"name": "report.json", "size": 11}},
		}},
		"processing_at": nil,
		"completed_at":  nil,
		"expired_at":    nil,
		"created_at":    nil,
	}
}
