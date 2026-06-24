package upload

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
)

func TestEncryptFileUsesNCSCChunkFormatAndHash(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.bin")
	content := append(bytes.Repeat([]byte("a"), ChunkSize), []byte("tail")...)
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "source.bin.enc")
	key, hash, err := EncryptFile(source, artifact)
	if err != nil {
		t.Fatalf("EncryptFile returned error: %v", err)
	}
	if key == "" || hash == "" {
		t.Fatalf("key/hash empty: %q %q", key, hash)
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("key length = %d err=%v, want 32 base64 decoded bytes", len(decoded), err)
	}
	decrypted, chunks := decryptArtifact(t, artifact, decoded)
	if chunks != 2 {
		t.Fatalf("chunks = %d, want 2", chunks)
	}
	if !bytes.Equal(decrypted, content) {
		t.Fatal("decrypted artifact does not match source content")
	}
}

func TestRunCreatesAnalysisUploadsAndCleansOnlySuccessfulArtifacts(t *testing.T) {
	dir := t.TempDir()
	sourceA := writeFile(t, dir, "a.txt", "alpha")
	sourceB := writeFile(t, dir, "b.txt", "beta")
	var stagedBlocks int32
	var commits int32
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "results") {
			t.Fatalf("unexpected result retrieval path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		switch r.URL.Query().Get("comp") {
		case "block":
			if r.URL.Query().Get("blockid") == "" {
				t.Fatal("stage block request missing blockid")
			}
			atomic.AddInt32(&stagedBlocks, 1)
		case "blocklist":
			atomic.AddInt32(&commits, 1)
		default:
			t.Fatalf("unexpected Azure Blob operation: %s", r.URL.RawQuery)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer sas.Close()
	fake := &fakeCreator{sasURLs: map[string]string{
		"a.txt": sas.URL + "/a?se=2099-01-01T00:00:00Z",
		"b.txt": sas.URL + "/b?se=2099-01-01T00:00:00Z",
	}}
	var stdout bytes.Buffer
	res, err := Run(context.Background(), Options{
		Files:          []string{sourceA, sourceB},
		WorkspaceDir:   filepath.Join(dir, "workspace"),
		CheckpointPath: filepath.Join(dir, "checkpoint.json"),
		AssumeYes:      true,
		API:            fake,
		Stdout:         &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fake.calls != 1 || atomic.LoadInt32(&stagedBlocks) != 2 || atomic.LoadInt32(&commits) != 2 {
		t.Fatalf("create calls/staged/commits = %d/%d/%d, want 1/2/2", fake.calls, stagedBlocks, commits)
	}
	for _, file := range res.Files {
		if file.Status != StatusUploaded {
			t.Fatalf("status = %s, want uploaded", file.Status)
		}
		if _, err := os.Stat(file.EncryptedArtifactPath); !os.IsNotExist(err) {
			t.Fatalf("artifact %s still exists after confirmed cleanup", file.EncryptedArtifactPath)
		}
	}
	checkpointBytes, err := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(checkpointBytes), "aes_key") || strings.Contains(string(checkpointBytes), "client_secret") {
		t.Fatalf("checkpoint persisted secret-like metadata: %s", checkpointBytes)
	}
	if !strings.Contains(string(checkpointBytes), "sas_url") || !strings.Contains(string(checkpointBytes), sas.URL) {
		t.Fatalf("checkpoint did not persist SAS URL needed for restart-resume: %s", checkpointBytes)
	}
	if !strings.Contains(stdout.String(), "Upload status:") || !strings.Contains(stdout.String(), "uploaded successfully") {
		t.Fatalf("stdout missing per-file status: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Upload summary:") || !strings.Contains(stdout.String(), "Analysis ID: 11111111-1111-1111-1111-111111111111") || strings.Count(stdout.String(), "Delete encrypted artifacts") != 0 {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestResumeContinuesFromPersistedUploadedBlocksAndCommits(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "abcdefghijkl")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	var stagedBlocks int32
	var commits int32
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("comp") {
		case "block":
			atomic.AddInt32(&stagedBlocks, 1)
			if atomic.LoadInt32(&stagedBlocks) == 1 {
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		case "blocklist":
			atomic.AddInt32(&commits, 1)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected Azure Blob operation: %s", r.URL.RawQuery)
		}
	}))
	defer sas.Close()
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: sas.URL + "/blob?se=2099-01-01T00:00:00Z", Status: StatusPrepared, UploadBlockSize: 4,
	}}})

	res, err := Run(context.Background(), Options{Resume: true, CheckpointPath: checkpoint, WorkspaceDir: dir, Stdout: io.Discard})
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	stagedAfterFailure := atomic.LoadInt32(&stagedBlocks)
	if res.Files[0].Status != StatusFailed || stagedAfterFailure < 2 || atomic.LoadInt32(&commits) != 0 {
		t.Fatalf("first run status/staged/commits = %+v/%d/%d, want failed/>=2/0", res.Files[0], stagedBlocks, commits)
	}
	cp, err := loadCheckpoint(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if got := cp.Files[0].UploadedBlocks; len(got) != 1 || got[0] != 0 || cp.Files[0].TotalBlocks != 3 {
		t.Fatalf("checkpoint blocks = %+v total=%d, want [0] total 3", got, cp.Files[0].TotalBlocks)
	}

	sas.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("comp") {
		case "block":
			atomic.AddInt32(&stagedBlocks, 1)
			w.WriteHeader(http.StatusCreated)
		case "blocklist":
			atomic.AddInt32(&commits, 1)
			body, _ := io.ReadAll(r.Body)
			for _, id := range blockIDs(3) {
				if !strings.Contains(string(body), id) {
					t.Fatalf("commit block list missing block id %q in %s", id, body)
				}
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected Azure Blob operation: %s", r.URL.RawQuery)
		}
	})
	res, err = Run(context.Background(), Options{Resume: true, CheckpointPath: checkpoint, WorkspaceDir: dir, Stdout: io.Discard, Stdin: strings.NewReader("n\n")})
	if err != nil {
		t.Fatalf("resume Run returned error: %v", err)
	}
	if res.Files[0].Status != StatusUploaded || atomic.LoadInt32(&stagedBlocks) != stagedAfterFailure+2 || atomic.LoadInt32(&commits) != 1 {
		t.Fatalf("resume status/staged/commits = %+v/%d/%d, want uploaded/%d/1", res.Files[0], stagedBlocks, commits, stagedAfterFailure+2)
	}
}

func TestResumeSkipsCompletedAndFailsOnlyMissingArtifact(t *testing.T) {
	dir := t.TempDir()
	keep := writeFile(t, dir, "keep.enc", "encrypted")
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer sas.Close()
	cp := Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{
		{Name: "done.bin", SourcePath: filepath.Join(dir, "done.bin"), EncryptedArtifactPath: keep, SASURL: sas.URL + "/done", Status: StatusUploaded},
		{Name: "missing.bin", SourcePath: filepath.Join(dir, "missing.bin"), EncryptedArtifactPath: filepath.Join(dir, "missing.enc"), SASURL: sas.URL + "/missing", Status: StatusPrepared},
	}}
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, cp)
	var stdout bytes.Buffer
	res, err := Run(context.Background(), Options{Resume: true, CheckpointPath: checkpoint, WorkspaceDir: dir, API: &fakeCreator{}, Stdout: &stdout, Stdin: strings.NewReader("n\n")})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Files[0].Status != StatusSkipped || res.Files[1].Status != StatusFailed || res.Files[1].Resumable {
		t.Fatalf("resume results = %+v", res.Files)
	}
	if !strings.Contains(res.Files[1].Message, "encrypted artifact is missing") {
		t.Fatalf("missing artifact message = %q", res.Files[1].Message)
	}
}

func TestSaveCheckpointProtectsExistingPermissiveFile(t *testing.T) {
	dir := t.TempDir()
	checkpoint := filepath.Join(dir, "checkpoint.json")
	if err := os.WriteFile(checkpoint, []byte(`{"version":1,"files":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(checkpoint, 0o644); err != nil {
		t.Fatal(err)
	}

	err := saveCheckpoint(checkpoint, Checkpoint{Version: 1, Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: filepath.Join(dir, "file.bin.enc"), SASURL: "https://storage.example/blob?se=2099-01-01T00:00:00Z", Status: StatusPrepared,
	}}})
	if err != nil {
		t.Fatalf("saveCheckpoint returned error: %v", err)
	}
	info, err := os.Stat(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("checkpoint mode = %o, want 600", got)
	}
}

func TestExpiredSASFailsWithoutCreatingReplacementAnalysis(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "file.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2020-01-01T00:00:00Z", SASExpiresAt: "2020-01-01T00:00:00Z", Status: StatusPrepared,
	}}})
	fake := &fakeCreator{}
	res, err := Run(context.Background(), Options{Resume: true, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard, Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0 on expired resume", fake.calls)
	}
	if res.Files[0].Status != StatusFailed || !strings.Contains(res.Files[0].Message, "SAS URL expired") {
		t.Fatalf("expired result = %+v", res.Files[0])
	}
}

func decryptArtifact(t *testing.T, path string, key []byte) ([]byte, int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	chunks := 0
	for len(data) > 0 {
		if len(data) < 8 {
			t.Fatalf("short chunk prefix")
		}
		chunkLen := int(binary.BigEndian.Uint64(data[:8]))
		data = data[8:]
		if chunkLen < gcm.NonceSize()+gcm.Overhead() || chunkLen > len(data) {
			t.Fatalf("invalid chunk length %d for remaining %d", chunkLen, len(data))
		}
		chunk := data[:chunkLen]
		data = data[chunkLen:]
		plain, err := gcm.Open(nil, chunk[:gcm.NonceSize()], chunk[gcm.NonceSize():], nil)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, plain...)
		chunks++
	}
	return out, chunks
}

type fakeCreator struct {
	calls   int
	sasURLs map[string]string
}

func (f *fakeCreator) CreateAnalysis(_ context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error) {
	f.calls++
	other, err := request.AsDataAnalysisOtherData()
	if err != nil {
		return nil, err
	}
	if other.ManualAnalysisAllowed != generated.True {
		return nil, io.ErrUnexpectedEOF
	}
	files := make([]generated.DataAnalysisFileUploadResource, 0, len(other.Files))
	for _, file := range other.Files {
		if file.AesKey == "" || file.Hash == nil || *file.Hash == "" {
			return nil, io.ErrUnexpectedEOF
		}
		files = append(files, generated.DataAnalysisFileUploadResource{Name: file.Name, SasUrl: f.sasURLs[file.Name]})
	}
	return &generated.DataAnalysisCreatedResource{Id: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Type: generated.DataAnalysisTypeOther, ManualAnalysisAllowed: true, Files: files}, nil
}

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCheckpoint(t *testing.T, path string, cp Checkpoint) {
	t.Helper()
	data, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
