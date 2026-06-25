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

	"da-toolkit/internal/ncscapi"
	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
)

func TestResultListUsesPaginationAndOAuthWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			id, secret, ok := r.BasicAuth()
			if !ok || id != testResultClientID || secret != testResultClientSecret {
				t.Fatalf("basic auth = %q/%q/%v, want configured credentials", id, secret, ok)
			}
			writeResultJSON(t, w, http.StatusOK, map[string]any{"access_token": testResultAccessToken, "token_type": "Bearer", "expires_in": 3600})
		case "/v1/data-analyses":
			if got := r.Header.Get("Authorization"); got != "Bearer "+testResultAccessToken {
				t.Fatalf("Authorization = %q, want bearer token", got)
			}
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("size") != "10" {
				t.Fatalf("query = %s, want page=2&size=10", r.URL.RawQuery)
			}
			writeResultJSON(t, w, http.StatusOK, map[string]any{"items": []any{map[string]any{"id": testResultAnalysisID, "type": "other", "status": "completed", "metadata": map[string]string{"case": "x"}}}, "total": 1, "page": 2, "size": 10, "pages": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stdout, stderr, err := executeCommand("result", "list", "--base-url", server.URL, "--token-url", server.URL+"/oauth/token", "--client-id", testResultClientID, "--client-secret", testResultClientSecret, "--page", "2", "--size", "10")
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	for _, want := range []string{"\"page\": 2", testResultAnalysisID} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	assertResultNoSecrets(t, stdout+stderr)
}

func TestResultReadOutputsFileAndResultMetadata(t *testing.T) {
	withFakeResultAPI(t, fakeResultAPI{
		read: analysisResults{
			Payload: map[string]any{"id": testResultAnalysisID, "files": []map[string]any{{"name": "sample.bin", "results": []map[string]any{{"name": "report.json", "size": 11}}}}},
			Files:   []fileResults{{Name: "sample.bin", Results: []resultMetadata{{Name: "report.json", Size: 11}}}},
		},
	})

	stdout, stderr, err := executeCommand("result", "read", testResultAnalysisID)
	if err != nil {
		t.Fatalf("Execute returned error: %v; stderr=%s", err, stderr)
	}
	for _, want := range []string{"sample.bin", "report.json", "\"size\": 11"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestResultArgumentValidationDoesNotRequireCredentials(t *testing.T) {
	t.Setenv("DA_TOOLKIT_CLIENT_ID", "")
	t.Setenv("DA_TOOLKIT_CLIENT_SECRET", "")
	t.Setenv("DA_TOOLKIT_TOKEN_URL", "")
	_, _, err := executeCommand("result", "read", "not-a-uuid")
	if err == nil || !strings.Contains(err.Error(), "invalid analysis-id") {
		t.Fatalf("error = %v, want invalid UUID", err)
	}
	if strings.Contains(err.Error(), "oauth client id") || strings.Contains(err.Error(), "client secret") || strings.Contains(err.Error(), "token URL") {
		t.Fatalf("argument validation required credentials: %v", err)
	}
}

func TestResultArchiveDownloadDoesNotOverwriteUnlessExplicit(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	if err := os.WriteFile(out, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	withFakeResultAPI(t, fakeResultAPI{read: analysisWithResult("report.json"), archive: ncscapi.Result{Body: []byte("zip-bytes")}})

	_, _, err := executeCommand("result", "archive", testResultAnalysisID, "--output", out)
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

	stdout, stderr, err := executeCommand("result", "archive", testResultAnalysisID, "--output", out, "--overwrite")
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

func TestResultFileDownloadRequiresAvailableNamedResult(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.json")
	withFakeResultAPI(t, fakeResultAPI{read: analysisWithResult("other.json"), file: ncscapi.Result{Body: []byte(`{"ok":true}`)}})

	_, _, err := executeCommand("result", "file", testResultAnalysisID, "report.json", "--output", out)
	if err == nil || !strings.Contains(err.Error(), "does not report result file") {
		t.Fatalf("error = %v, want missing result metadata", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}

	withFakeResultAPI(t, fakeResultAPI{read: analysisWithResult("report.json"), file: ncscapi.Result{Body: []byte(`{"ok":true}`)}})
	if _, stderr, err := executeCommand("result", "file", testResultAnalysisID, "report.json", "--output", out); err != nil {
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

func TestResultReadReportsNotFoundFromAPI(t *testing.T) {
	withFakeResultAPI(t, fakeResultAPI{readErr: &ncscapi.APIError{Operation: "read data-analysis", StatusCode: http.StatusNotFound, Message: http.StatusText(http.StatusNotFound)}})

	stdout, _, err := executeCommand("result", "read", testResultAnalysisID)
	if err == nil || !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("error = %v, want clear not-found API error", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on not-found", stdout)
	}
}

func TestResultDownloadReportsNotFoundFromAPI(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	withFakeResultAPI(t, fakeResultAPI{read: analysisWithResult("report.json"), archiveErr: &ncscapi.APIError{Operation: "download result archive", StatusCode: http.StatusNotFound, Message: http.StatusText(http.StatusNotFound)}})

	_, _, err := executeCommand("result", "archive", testResultAnalysisID, "--output", out)
	if err == nil || !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("error = %v, want clear not-found API error", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}
}

func TestResultArchiveDownloadRequiresAvailableResults(t *testing.T) {
	out := filepath.Join(t.TempDir(), "results.zip")
	withFakeResultAPI(t, fakeResultAPI{read: analysisResults{Payload: map[string]any{"id": testResultAnalysisID}, Files: []fileResults{{Name: "sample.bin"}}}, archive: ncscapi.Result{Body: []byte("zip-bytes")}})

	_, _, err := executeCommand("result", "archive", testResultAnalysisID, "--output", out)
	if err == nil || !strings.Contains(err.Error(), "does not report available results yet") {
		t.Fatalf("error = %v, want unavailable results metadata error", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output stat err = %v, want no file", statErr)
	}
}

const (
	testResultClientID     = "result-client-id"
	testResultClientSecret = "result-client-secret"
	testResultAccessToken  = "result-access-token"
	testResultAnalysisID   = "11111111-1111-1111-1111-111111111111"
)

type fakeResultAPI struct {
	read       analysisResults
	readErr    error
	archive    ncscapi.Result
	archiveErr error
	file       ncscapi.Result
	fileErr    error
}

func (f fakeResultAPI) ListAnalyses(context.Context, ncscapi.ListOptions) (any, error) {
	return generated.PageDataAnalysisCollection{}, nil
}

func (f fakeResultAPI) ReadAnalysis(context.Context, uuid.UUID) (analysisResults, error) {
	if f.readErr != nil {
		return analysisResults{}, f.readErr
	}
	return f.read, nil
}

func (f fakeResultAPI) GetResultArchive(context.Context, uuid.UUID) (ncscapi.Result, error) {
	if f.archiveErr != nil {
		return ncscapi.Result{}, f.archiveErr
	}
	return f.archive, nil
}

func (f fakeResultAPI) GetResultFile(context.Context, uuid.UUID, string) (ncscapi.Result, error) {
	if f.fileErr != nil {
		return ncscapi.Result{}, f.fileErr
	}
	return f.file, nil
}

func withFakeResultAPI(t *testing.T, api fakeResultAPI) {
	t.Helper()
	original := newResultAPI
	newResultAPI = func() (resultAPI, error) { return api, nil }
	t.Cleanup(func() { newResultAPI = original })
}

func analysisWithResult(name string) analysisResults {
	return analysisResults{Payload: map[string]any{"id": testResultAnalysisID}, Files: []fileResults{{Name: "sample.bin", Results: []resultMetadata{{Name: name, Size: 11}}}}}
}

func writeResultJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
}

func assertResultNoSecrets(t *testing.T, value string) {
	t.Helper()
	for _, secret := range []string{testResultClientID, testResultClientSecret, testResultAccessToken, base64.StdEncoding.EncodeToString([]byte(testResultClientSecret))} {
		if strings.Contains(value, secret) {
			t.Fatalf("%q leaked secret %q", value, secret)
		}
	}
}
