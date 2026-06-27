package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"da-toolkit/internal/ncscapi"
	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
)

func TestAnalysisListUsesPaginationAndOAuthWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			id, secret, ok := r.BasicAuth()
			if !ok || id != testAnalysisClientID || secret != testAnalysisClientSecret {
				t.Fatalf("basic auth = %q/%q/%v, want configured credentials", id, secret, ok)
			}
			writeAnalysisJSON(t, w, http.StatusOK, map[string]any{"access_token": testAnalysisAccessToken, "token_type": "Bearer", "expires_in": 3600})
		case "/v1/data-analyses":
			if got := r.Header.Get("Authorization"); got != "Bearer "+testAnalysisAccessToken {
				t.Fatalf("Authorization = %q, want bearer token", got)
			}
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("size") != "10" {
				t.Fatalf("query = %s, want page=2&size=10", r.URL.RawQuery)
			}
			writeAnalysisJSON(t, w, http.StatusOK, map[string]any{"items": []any{map[string]any{"id": testAnalysisID, "type": "other", "status": "completed", "metadata": map[string]string{"case": "x"}}}, "total": 1, "page": 2, "size": 10, "pages": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stdout, stderr, err := executeCommand("analysis", "list", "--base-url", server.URL, "--token-url", server.URL+"/oauth/token", "--client-id", testAnalysisClientID, "--client-secret", testAnalysisClientSecret, "--page", "2", "--size", "10")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	for _, want := range []string{"\"page\": 2", testAnalysisID} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	assertAnalysisNoSecrets(t, stdout+stderr)
}

func TestAnalysisReadOutputsMetadataPayload(t *testing.T) {
	withFakeAnalysisAPI(t, fakeAnalysisAPI{
		read: analysisResults{
			Payload: map[string]any{"id": testAnalysisID, "files": []map[string]any{{"name": "sample.bin", "results": []map[string]any{{"name": "report.json", "size": 11}}}}},
			Files:   []fileResults{{Name: "sample.bin", Results: []resultMetadata{{Name: "report.json", Size: 11}}}},
		},
	})

	stdout, stderr, err := executeCommand("analysis", "read", testAnalysisID)
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	for _, want := range []string{"sample.bin", "report.json", "\"size\": 11"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestAnalysisReadWaitPollsUntilCompletedAndPrintsFinalMetadata(t *testing.T) {
	var slept []time.Duration
	originalSleep := analysisWaitSleep
	analysisWaitSleep = func(_ context.Context, delay time.Duration) error {
		slept = append(slept, delay)
		return nil
	}
	t.Cleanup(func() { analysisWaitSleep = originalSleep })
	calls := 0
	withFakeAnalysisAPI(t, fakeAnalysisAPI{readCalls: &calls, readSeq: []analysisResults{
		{Payload: map[string]any{"status": "processing"}, Status: "processing", RetryAfter: durationPtr(2 * time.Second)},
		{Payload: map[string]any{"status": "submitted"}, Status: "submitted"},
		{Payload: map[string]any{"status": "completed", "id": testAnalysisID}, Status: "completed"},
	}})

	stdout, stderr, err := executeCommand("analysis", "read", testAnalysisID, "--wait")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if calls != 3 {
		t.Fatalf("read calls = %d, want 3", calls)
	}
	if len(slept) != 2 || slept[0] != 2*time.Second || slept[1] != analysisWaitFallbackInterval {
		t.Fatalf("slept = %v, want retry-after then fallback", slept)
	}
	if !strings.Contains(stdout, "\"status\": \"completed\"") {
		t.Fatalf("stdout missing completed metadata:\n%s", stdout)
	}
}

func TestAnalysisReadDownloadWithoutWaitFailsBeforeAPIConstruction(t *testing.T) {
	created := false
	original := newAnalysisAPI
	newAnalysisAPI = func() (analysisAPI, error) {
		created = true
		return fakeAnalysisAPI{}, nil
	}
	t.Cleanup(func() { newAnalysisAPI = original })

	_, _, err := executeCommand("analysis", "read", testAnalysisID, "--download")
	if err == nil || !strings.Contains(err.Error(), "--download is only available with --wait") || !strings.Contains(err.Error(), "analysis archive") || !strings.Contains(err.Error(), "analysis file") {
		t.Fatalf("error = %v, want helpful download guidance", err)
	}
	if created {
		t.Fatal("newAnalysisAPI was constructed before flag validation")
	}
}

func TestAnalysisReadWaitDownloadTracksResultsAndRetriesFailures(t *testing.T) {
	var slept []time.Duration
	originalSleep := analysisWaitSleep
	analysisWaitSleep = func(_ context.Context, delay time.Duration) error {
		slept = append(slept, delay)
		return nil
	}
	t.Cleanup(func() { analysisWaitSleep = originalSleep })
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.WriteFile("existing.txt", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fileAttempts := map[string]int{}
	withFakeAnalysisAPI(t, fakeAnalysisAPI{
		readCalls: &calls,
		readSeq: []analysisResults{
			{Payload: map[string]any{"status": "processing"}, Status: "processing", RetryAfter: durationPtr(time.Second), Files: []fileResults{{Results: []resultMetadata{{Name: "nested/report.json"}, {Name: "existing.txt"}, {Name: "flaky.bin"}}}}},
			{Payload: map[string]any{"status": "completed"}, Status: "completed", RetryAfter: durationPtr(time.Second), Files: []fileResults{{Results: []resultMetadata{{Name: "nested/report.json"}, {Name: "existing.txt"}, {Name: "flaky.bin"}}}}},
			{Payload: map[string]any{"status": "completed"}, Status: "completed", Files: []fileResults{{Results: []resultMetadata{{Name: "nested/report.json"}, {Name: "existing.txt"}, {Name: "flaky.bin"}}}}},
		},
		fileFunc: func(_ context.Context, _ uuid.UUID, name string) (ncscapi.Result, error) {
			fileAttempts[name]++
			if name == "flaky.bin" && fileAttempts[name] < 3 {
				return ncscapi.Result{}, errors.New("temporary failure")
			}
			return ncscapi.Result{Body: []byte("body:" + name)}, nil
		},
	})

	stdout, stderr, err := executeCommand("analysis", "read", testAnalysisID, "--wait", "--download")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s stdout=%s", err, stderr, stdout)
	}
	if calls != 3 || len(slept) != 2 {
		t.Fatalf("calls=%d slept=%v, want retry after completion until pending succeeds", calls, slept)
	}
	assertFileContent(t, filepath.Join(dir, "nested", "report.json"), "body:nested/report.json")
	assertFileContent(t, filepath.Join(dir, "existing.txt"), "keep")
	assertFileContent(t, filepath.Join(dir, "flaky.bin"), "body:flaky.bin")
	if fileAttempts["nested/report.json"] != 1 || fileAttempts["existing.txt"] != 0 || fileAttempts["flaky.bin"] != 3 {
		t.Fatalf("file attempts = %#v, want one nested, zero existing, three flaky", fileAttempts)
	}
	for _, want := range []string{"Downloaded result nested/report.json", "Skipped existing result existing.txt", "Failed to download flaky.bin", "Download complete: downloaded=2 skipped=1 failed-attempts=2"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "\"status\"") {
		t.Fatalf("download mode printed metadata JSON:\n%s", stdout)
	}
}

func TestAnalysisDeleteRequiresConfirmationAndCallsAPI(t *testing.T) {
	var deleted uuid.UUID
	withFakeAnalysisAPI(t, fakeAnalysisAPI{delete: func(_ context.Context, id uuid.UUID) error {
		deleted = id
		return nil
	}})

	stdout, stderr, err := executeCommand("analysis", "delete", testAnalysisID, "--yes")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	if deleted != uuid.MustParse(testAnalysisID) {
		t.Fatalf("deleted id = %s, want %s", deleted, testAnalysisID)
	}
	if !strings.Contains(stdout, "Deleted data-analysis "+testAnalysisID) {
		t.Fatalf("stdout missing delete confirmation:\n%s", stdout)
	}
}

func TestAnalysisArgumentValidationDoesNotRequireCredentials(t *testing.T) {
	t.Setenv("DA_TOOLKIT_CLIENT_ID", "")
	t.Setenv("DA_TOOLKIT_CLIENT_SECRET", "")
	t.Setenv("DA_TOOLKIT_TOKEN_URL", "")
	_, _, err := executeCommand("analysis", "read", "not-a-uuid")
	if err == nil || !strings.Contains(err.Error(), "invalid analysis-id") {
		t.Fatalf("error = %v, want invalid UUID", err)
	}
	if strings.Contains(err.Error(), "oauth client id") || strings.Contains(err.Error(), "client secret") || strings.Contains(err.Error(), "token URL") {
		t.Fatalf("argument validation required credentials: %v", err)
	}
}

func TestAnalysisArchiveDownloadDoesNotOverwriteUnlessExplicit(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	if err := os.WriteFile(out, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFakeAnalysisAPI(t, fakeAnalysisAPI{read: analysisWithResult("report.json"), archive: ncscapi.Result{Body: []byte("zip-bytes")}})

	_, _, err := executeCommand("analysis", "archive", testAnalysisID, "--output", out)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want overwrite protection", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("file content = %q, want existing", got)
	}

	stdout, stderr, err := executeCommand("analysis", "archive", testAnalysisID, "--output", out, "--overwrite")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	got, err = os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "zip-bytes" || !strings.Contains(stdout, "Wrote 9 bytes") {
		t.Fatalf("download result = %q stdout=%q, want archive bytes", got, stdout)
	}
}

func TestAnalysisFileDownloadRequiresAvailableNamedResult(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.json")
	withFakeAnalysisAPI(t, fakeAnalysisAPI{read: analysisWithResult("other.json"), file: ncscapi.Result{Body: []byte(`{"ok":true}`)}})

	_, _, err := executeCommand("analysis", "file", testAnalysisID, "report.json", "--output", out)
	if err == nil || !strings.Contains(err.Error(), "does not report result file") {
		t.Fatalf("error = %v, want missing result metadata", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}

	withFakeAnalysisAPI(t, fakeAnalysisAPI{read: analysisWithResult("report.json"), file: ncscapi.Result{Body: []byte(`{"ok":true}`)}})
	if _, stderr, err := executeCommand("analysis", "file", testAnalysisID, "report.json", "--output", out); err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("file content = %q, want result bytes", got)
	}
}

func TestAnalysisReadReportsNotFoundFromAPI(t *testing.T) {
	withFakeAnalysisAPI(t, fakeAnalysisAPI{readErr: &ncscapi.APIError{Operation: "read data-analysis", StatusCode: http.StatusNotFound, Message: http.StatusText(http.StatusNotFound)}})

	stdout, _, err := executeCommand("analysis", "read", testAnalysisID)
	if err == nil || !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("error = %v, want clear not-found API error", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on not-found", stdout)
	}
}

func TestAnalysisDownloadReportsNotFoundFromAPI(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	withFakeAnalysisAPI(t, fakeAnalysisAPI{read: analysisWithResult("report.json"), archiveErr: &ncscapi.APIError{Operation: "download result archive", StatusCode: http.StatusNotFound, Message: http.StatusText(http.StatusNotFound)}})

	_, _, err := executeCommand("analysis", "archive", testAnalysisID, "--output", out)
	if err == nil || !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("error = %v, want clear not-found API error", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}
}

func TestAnalysisArchiveDownloadRequiresAvailableResults(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	withFakeAnalysisAPI(t, fakeAnalysisAPI{read: analysisResults{Payload: map[string]any{"id": testAnalysisID}, Files: []fileResults{{Name: "sample.bin"}}}, archive: ncscapi.Result{Body: []byte("zip-bytes")}})

	_, _, err := executeCommand("analysis", "archive", testAnalysisID, "--output", out)
	if err == nil || !strings.Contains(err.Error(), "does not report available results yet") {
		t.Fatalf("error = %v, want unavailable results metadata error", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}
}

const (
	testAnalysisClientID     = "analysis-client-id"
	testAnalysisClientSecret = "analysis-client-secret"
	testAnalysisAccessToken  = "analysis-access-token"
	testAnalysisID           = "11111111-1111-1111-1111-111111111111"
)

type fakeAnalysisAPI struct {
	read       analysisResults
	readSeq    []analysisResults
	readCalls  *int
	readErr    error
	delete     func(context.Context, uuid.UUID) error
	deleteErr  error
	archive    ncscapi.Result
	archiveErr error
	file       ncscapi.Result
	fileFunc   func(context.Context, uuid.UUID, string) (ncscapi.Result, error)
	fileErr    error
}

func (f fakeAnalysisAPI) ListAnalyses(context.Context, ncscapi.ListOptions) (any, error) {
	return generated.PageDataAnalysisCollection{}, nil
}

func (f fakeAnalysisAPI) ReadAnalysis(context.Context, uuid.UUID) (analysisResults, error) {
	if f.readErr != nil {
		return analysisResults{}, f.readErr
	}
	if f.readCalls != nil {
		index := *f.readCalls
		(*f.readCalls)++
		if len(f.readSeq) > 0 {
			if index >= len(f.readSeq) {
				index = len(f.readSeq) - 1
			}
			return f.readSeq[index], nil
		}
	}
	return f.read, nil
}

func (f fakeAnalysisAPI) DeleteAnalysis(ctx context.Context, id uuid.UUID) error {
	if f.delete != nil {
		return f.delete(ctx, id)
	}
	return f.deleteErr
}

func (f fakeAnalysisAPI) GetResultArchive(context.Context, uuid.UUID) (ncscapi.Result, error) {
	if f.archiveErr != nil {
		return ncscapi.Result{}, f.archiveErr
	}
	return f.archive, nil
}

func (f fakeAnalysisAPI) GetResultFile(ctx context.Context, id uuid.UUID, name string) (ncscapi.Result, error) {
	if f.fileFunc != nil {
		return f.fileFunc(ctx, id, name)
	}
	if f.fileErr != nil {
		return ncscapi.Result{}, f.fileErr
	}
	return f.file, nil
}

func withFakeAnalysisAPI(t *testing.T, api analysisAPI) {
	t.Helper()
	original := newAnalysisAPI
	newAnalysisAPI = func() (analysisAPI, error) { return api, nil }
	t.Cleanup(func() { newAnalysisAPI = original })
}

func analysisWithResult(name string) analysisResults {
	return analysisResults{Payload: map[string]any{"id": testAnalysisID}, Files: []fileResults{{Name: "sample.bin", Results: []resultMetadata{{Name: name, Size: 11}}}}}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func durationPtr(value time.Duration) *time.Duration {
	return &value
}

func writeAnalysisJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
}

func assertAnalysisNoSecrets(t *testing.T, value string) {
	t.Helper()
	for _, secret := range []string{testAnalysisClientID, testAnalysisClientSecret, testAnalysisAccessToken, base64.StdEncoding.EncodeToString([]byte(testAnalysisClientSecret))} {
		if strings.Contains(value, secret) {
			t.Fatalf("%q leaked secret %q", value, secret)
		}
	}
}
