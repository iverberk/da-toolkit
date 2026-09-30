package analysis

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

func TestDecryptFileReversesUploadEncryptionFormat(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.bin")
	content := append(bytes.Repeat([]byte("b"), ChunkSize), []byte("tail")...)
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "source.bin.enc")
	key, _, err := EncryptFile(source, artifact)
	if err != nil {
		t.Fatalf("EncryptFile returned error: %v", err)
	}
	output := filepath.Join(dir, "decrypted.bin")
	if err := DecryptFile(artifact, output, key); err != nil {
		t.Fatalf("DecryptFile returned error: %v", err)
	}
	decrypted, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, content) {
		t.Fatal("decrypted file does not match source content")
	}
}

func TestDecryptFileAllowsLargerExternalChunks(t *testing.T) {
	dir := t.TempDir()
	key, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(decoded)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain := bytes.Repeat([]byte("x"), ChunkSize+1)
	nonce := bytes.Repeat([]byte{1}, gcm.NonceSize())
	sealed := gcm.Seal(nil, nonce, plain, nil)
	artifact := filepath.Join(dir, "large-chunk.enc")
	var data bytes.Buffer
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], uint64(len(nonce)+len(sealed)))
	data.Write(prefix[:])
	data.Write(nonce)
	data.Write(sealed)
	if err := os.WriteFile(artifact, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "plain.bin")
	if err := DecryptFile(artifact, output, key); err != nil {
		t.Fatalf("DecryptFile returned error: %v", err)
	}
	decrypted, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatal("decrypted file does not match larger chunk plaintext")
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

func TestCreateDeferredAcceptsMissingFilesAndOmitsHashes(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.bin")
	fake := &fakeDeferredCreator{sasURLs: map[string]string{"does-not-exist.bin": "https://storage.example/blob?se=2099-01-01T00:00:00Z"}}
	handoff, err := CreateDeferred(context.Background(), Options{
		Files:        []string{missing},
		AnalysisType: AnalysisTypeOther,
		API:          fake,
		Stdout:       io.Discard,
	})
	if err != nil {
		t.Fatalf("CreateDeferred returned error: %v", err)
	}
	if fake.calls != 1 || handoff.AnalysisID == uuid.Nil || len(handoff.Files) != 1 {
		t.Fatalf("handoff/calls = %+v/%d, want one created handoff", handoff, fake.calls)
	}
	if handoff.Files[0].AESKey == "" || handoff.Files[0].SASURL == "" || handoff.Files[0].Filename != "does-not-exist.bin" {
		t.Fatalf("handoff file = %+v, want filename/key/sas", handoff.Files[0])
	}
}

func TestUploadHandoffEncryptsWithSuppliedKeyAndDoesNotPersistAESKey(t *testing.T) {
	dir := t.TempDir()
	source := writeFile(t, dir, "plain.bin", "plain evidence")
	key, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	var uploaded []byte
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("comp") {
		case "block":
			body, _ := io.ReadAll(r.Body)
			uploaded = append(uploaded, body...)
		case "blocklist":
		default:
			t.Fatalf("unexpected Azure Blob operation: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer sas.Close()
	checkpoint := filepath.Join(dir, "upload-checkpoint.json")
	var stdout bytes.Buffer
	res, err := Upload(context.Background(), UploadOptions{
		AnalysisID:     uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Files:          []HandoffFile{{Filename: source, AESKey: key, SASURL: sas.URL + "/blob?se=2099-01-01T00:00:00Z"}},
		WorkspaceDir:   filepath.Join(dir, "workspace"),
		CheckpointPath: checkpoint,
		AssumeYes:      true,
		Stdout:         &stdout,
	})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, _ := decryptArtifactBytes(t, uploaded, decoded)
	if string(decrypted) != "plain evidence" || res.Files[0].Status != StatusUploaded {
		t.Fatalf("decrypted/status = %q/%s, want plaintext/uploaded", decrypted, res.Files[0].Status)
	}
	checkpointBytes, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(checkpointBytes), key) || strings.Contains(string(checkpointBytes), "aes_key") {
		t.Fatalf("checkpoint persisted AES key: %s", checkpointBytes)
	}
}

func TestSelectHandoffFilesExactBeforeBasenameAndAmbiguity(t *testing.T) {
	handoff := Handoff{AnalysisID: uuid.New(), Files: []HandoffFile{
		{Filename: "a/sample.bin", AESKey: "key-a", SASURL: "sas-a"},
		{Filename: "b/sample.bin", AESKey: "key-b", SASURL: "sas-b"},
		{Filename: "sample.bin", AESKey: "key-exact", SASURL: "sas-exact"},
	}}
	selected, err := SelectHandoffFiles(handoff, "sample.bin")
	if err != nil || len(selected) != 1 || selected[0].AESKey != "key-exact" {
		t.Fatalf("exact selection = %+v err=%v, want exact", selected, err)
	}
	_, err = SelectHandoffFiles(Handoff{AnalysisID: uuid.New(), Files: handoff.Files[:2]}, filepath.Join(t.TempDir(), "sample.bin"))
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v, want ambiguous basename", err)
	}
}

func TestRunCreatesMalwareAnalysisWithZipPasswordsWithoutPersistingThem(t *testing.T) {
	dir := t.TempDir()
	sourceA := writeFile(t, dir, "a.zip", "alpha")
	sourceB := writeFile(t, dir, "b.bin", "beta")
	secretPassword := "malware-zip-secret"
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "results") {
			t.Fatalf("unexpected result retrieval path: %s", r.URL.Path)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer sas.Close()
	fake := &fakeMalwareCreator{sasURLs: map[string]string{
		"a.zip": sas.URL + "/a?se=2099-01-01T00:00:00Z",
		"b.bin": sas.URL + "/b?se=2099-01-01T00:00:00Z",
	}, wantZipPasswords: map[string]string{"a.zip": secretPassword}}
	checkpoint := filepath.Join(dir, "checkpoint.json")
	var stdout bytes.Buffer
	res, err := Run(context.Background(), Options{
		Files:          []string{sourceA, sourceB},
		WorkspaceDir:   filepath.Join(dir, "workspace"),
		CheckpointPath: checkpoint,
		AssumeYes:      true,
		AnalysisType:   AnalysisTypeMalware,
		ZipPasswords:   map[string]string{sourceA: secretPassword},
		API:            fake,
		Stdout:         &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("CreateAnalysis calls = %d, want 1", fake.calls)
	}
	for _, file := range res.Files {
		if file.Status != StatusUploaded {
			t.Fatalf("status = %s, want uploaded", file.Status)
		}
	}
	checkpointBytes, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	for label, value := range map[string]string{"checkpoint": string(checkpointBytes), "stdout": stdout.String()} {
		if strings.Contains(value, secretPassword) || strings.Contains(value, "zip_password") {
			t.Fatalf("%s leaked zip password metadata: %s", label, value)
		}
	}
}

func TestRunCreatesForensicAnalysisWithRequestedAnalysesAndDiskKeysWithoutPersistingSecrets(t *testing.T) {
	dir := t.TempDir()
	sourceA := writeFile(t, dir, "disk-a.E01", "alpha")
	sourceB := writeFile(t, dir, "memory.raw", "beta")
	secretDiskKey := "forensic-disk-secret"
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "results") {
			t.Fatalf("unexpected result retrieval path: %s", r.URL.Path)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer sas.Close()
	fake := &fakeForensicCreator{sasURLs: map[string]string{
		"disk-a.E01": sas.URL + "/a?se=2099-01-01T00:00:00Z",
		"memory.raw": sas.URL + "/b?se=2099-01-01T00:00:00Z",
	}, wantRequestedAnalyses: map[string][]string{
		"disk-a.E01": {"timeline", "filesystem"},
		"memory.raw": {"memory"},
	}, wantDiskKeys: map[string]string{"disk-a.E01": secretDiskKey}, wantManualAllowed: true}
	checkpoint := filepath.Join(dir, "checkpoint.json")
	var stdout bytes.Buffer
	res, err := Run(context.Background(), Options{
		Files:                 []string{sourceA, sourceB},
		WorkspaceDir:          filepath.Join(dir, "workspace"),
		CheckpointPath:        checkpoint,
		AssumeYes:             true,
		AnalysisType:          AnalysisTypeForensic,
		RequestedAnalyses:     map[string][]string{sourceA: {"timeline", "filesystem"}, sourceB: {"memory"}},
		DiskEncryptionKeys:    map[string]string{sourceA: secretDiskKey},
		ManualAnalysisAllowed: true,
		API:                   fake,
		Stdout:                &stdout,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("CreateAnalysis calls = %d, want 1", fake.calls)
	}
	for _, file := range res.Files {
		if file.Status != StatusUploaded {
			t.Fatalf("status = %s, want uploaded", file.Status)
		}
	}
	checkpointBytes, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	for label, value := range map[string]string{"checkpoint": string(checkpointBytes), "stdout": stdout.String()} {
		if strings.Contains(value, secretDiskKey) || strings.Contains(value, "disk_encryption_key") {
			t.Fatalf("%s leaked disk encryption key metadata: %s", label, value)
		}
	}
}

func TestRunRequiresRequestedAnalysesForEveryForensicFileBeforeCreate(t *testing.T) {
	dir := t.TempDir()
	sourceA := writeFile(t, dir, "a.E01", "alpha")
	sourceB := writeFile(t, dir, "b.E01", "beta")
	fake := &fakeForensicCreator{}
	_, err := Run(context.Background(), Options{
		Files:             []string{sourceA, sourceB},
		WorkspaceDir:      filepath.Join(dir, "workspace"),
		CheckpointPath:    filepath.Join(dir, "checkpoint.json"),
		AnalysisType:      AnalysisTypeForensic,
		RequestedAnalyses: map[string][]string{sourceA: {"timeline"}},
		API:               fake,
		Stdout:            io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "missing requested analyses") {
		t.Fatalf("error = %v, want missing requested analyses", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
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

func TestAutoResumeUsesCheckpointWithoutCreatingAnalysis(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "abcdef")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	var stagedBlocks int32
	var commits int32
	sas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("comp") {
		case "block":
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
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: sas.URL + "/blob?se=2099-01-01T00:00:00Z", Status: StatusPrepared, UploadBlockSize: 3,
	}}})
	fake := &fakeCreator{}

	res, err := Run(context.Background(), Options{CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard, Stdin: strings.NewReader("n\n")})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0 during inferred resume", fake.calls)
	}
	if res.Files[0].Status != StatusUploaded || atomic.LoadInt32(&stagedBlocks) != 2 || atomic.LoadInt32(&commits) != 1 {
		t.Fatalf("auto-resume result/staged/commits = %+v/%d/%d", res.Files[0], stagedBlocks, commits)
	}
}

func TestStrictResumeRequiresUsableCheckpoint(t *testing.T) {
	dir := t.TempDir()
	_, err := Run(context.Background(), Options{Resume: true, CheckpointPath: filepath.Join(dir, "missing.json"), WorkspaceDir: dir, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "--resume requires a usable incomplete checkpoint") || !strings.Contains(err.Error(), "no checkpoint exists") {
		t.Fatalf("error = %v, want strict resume missing checkpoint error", err)
	}
}

func TestNewFailsWhenCheckpointAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2099-01-01T00:00:00Z", Status: StatusPrepared,
	}}})
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{New: true, Files: []string{writeFile(t, dir, "source.bin", "source")}, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "checkpoint already exists") {
		t.Fatalf("error = %v, want existing checkpoint protection", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestInvalidCheckpointFailsWithoutCreatingAnalysis(t *testing.T) {
	dir := t.TempDir()
	checkpoint := filepath.Join(dir, "checkpoint.json")
	if err := os.WriteFile(checkpoint, []byte(`{"version":`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{Files: []string{writeFile(t, dir, "source.bin", "source")}, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "invalid or corrupt") {
		t.Fatalf("error = %v, want corrupt checkpoint error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestUnknownCheckpointStatusFailsWithoutCreatingAnalysis(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2099-01-01T00:00:00Z", Status: "queued",
	}}})
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{Files: []string{writeFile(t, dir, "source.bin", "source")}, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "invalid or corrupt") || !strings.Contains(err.Error(), "unknown file status") {
		t.Fatalf("error = %v, want unknown status checkpoint error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestStrictResumeRejectsUnknownCheckpointStatus(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2099-01-01T00:00:00Z", Status: "queued",
	}}})
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{Resume: true, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "--resume requires a usable incomplete checkpoint") || !strings.Contains(err.Error(), "unknown file status") {
		t.Fatalf("error = %v, want strict resume unknown status error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestCompletedCheckpointFailsByDefault(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2099-01-01T00:00:00Z", Status: StatusUploaded,
	}}})
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{Files: []string{writeFile(t, dir, "source.bin", "source")}, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "already completed") {
		t.Fatalf("error = %v, want completed checkpoint error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestExpiredCheckpointFailsByDefaultWithoutCreatingAnalysis(t *testing.T) {
	dir := t.TempDir()
	artifact := writeFile(t, dir, "artifact.enc", "encrypted")
	checkpoint := filepath.Join(dir, "checkpoint.json")
	writeCheckpoint(t, checkpoint, Checkpoint{Version: 1, AnalysisID: uuid.NewString(), Files: []CheckpointFile{{
		Name: "file.bin", SourcePath: filepath.Join(dir, "file.bin"), EncryptedArtifactPath: artifact, SASURL: "https://storage.example/file?se=2020-01-01T00:00:00Z", SASExpiresAt: "2020-01-01T00:00:00Z", Status: StatusPrepared,
	}}})
	fake := &fakeCreator{}
	_, err := Run(context.Background(), Options{Files: []string{writeFile(t, dir, "source.bin", "source")}, CheckpointPath: checkpoint, WorkspaceDir: dir, API: fake, Stdout: io.Discard, Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	if err == nil || !strings.Contains(err.Error(), "expired SAS") {
		t.Fatalf("error = %v, want expired SAS error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0", fake.calls)
	}
}

func TestResumeFailsClearlyWhenCheckpointArtifactIsMissing(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "not resumable") || !strings.Contains(err.Error(), "encrypted artifact") {
		t.Fatalf("error = %v, want missing artifact resume failure; result=%+v", err, res)
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
	if err == nil || !strings.Contains(err.Error(), "expired SAS") {
		t.Fatalf("error = %v, want expired SAS resume failure; result=%+v", err, res)
	}
	if fake.calls != 0 {
		t.Fatalf("CreateAnalysis calls = %d, want 0 on expired resume", fake.calls)
	}
}

func decryptArtifact(t *testing.T, path string, key []byte) ([]byte, int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return decryptArtifactBytes(t, data, key)
}

func decryptArtifactBytes(t *testing.T, data []byte, key []byte) ([]byte, int) {
	t.Helper()
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

type fakeDeferredCreator struct {
	calls   int
	sasURLs map[string]string
}

type fakeMalwareCreator struct {
	calls            int
	sasURLs          map[string]string
	wantZipPasswords map[string]string
}

type fakeForensicCreator struct {
	calls                 int
	sasURLs               map[string]string
	wantRequestedAnalyses map[string][]string
	wantDiskKeys          map[string]string
	wantManualAllowed     bool
}

func (f *fakeForensicCreator) CreateAnalysis(_ context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error) {
	f.calls++
	forensic, err := request.AsDataAnalysisForensicData()
	if err != nil {
		return nil, err
	}
	if forensic.ManualAnalysisAllowed != f.wantManualAllowed {
		return nil, io.ErrUnexpectedEOF
	}
	files := make([]generated.DataAnalysisFileUploadResource, 0, len(forensic.Files))
	for _, file := range forensic.Files {
		if file.AesKey == "" || file.Hash == nil || *file.Hash == "" || len(file.RequestedAnalyses) == 0 {
			return nil, io.ErrUnexpectedEOF
		}
		if want := f.wantRequestedAnalyses[file.Name]; !sameStrings(file.RequestedAnalyses, want) {
			return nil, io.ErrUnexpectedEOF
		}
		wantDiskKey, shouldHaveDiskKey := f.wantDiskKeys[file.Name]
		if shouldHaveDiskKey {
			if file.DiskEncryptionKey == nil || *file.DiskEncryptionKey != wantDiskKey {
				return nil, io.ErrUnexpectedEOF
			}
		} else if file.DiskEncryptionKey != nil {
			return nil, io.ErrUnexpectedEOF
		}
		files = append(files, generated.DataAnalysisFileUploadResource{Name: file.Name, SasUrl: f.sasURLs[file.Name]})
	}
	return &generated.DataAnalysisCreatedResource{Id: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Type: generated.DataAnalysisTypeForensic, ManualAnalysisAllowed: f.wantManualAllowed, Files: files}, nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (f *fakeMalwareCreator) CreateAnalysis(_ context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error) {
	f.calls++
	malware, err := request.AsDataAnalysisMalwareData()
	if err != nil {
		return nil, err
	}
	files := make([]generated.DataAnalysisFileUploadResource, 0, len(malware.Files))
	for _, file := range malware.Files {
		if file.AesKey == "" || file.Hash == nil || *file.Hash == "" {
			return nil, io.ErrUnexpectedEOF
		}
		wantPassword, shouldHavePassword := f.wantZipPasswords[file.Name]
		if shouldHavePassword {
			if file.ZipPassword == nil || *file.ZipPassword != wantPassword {
				return nil, io.ErrUnexpectedEOF
			}
		} else if file.ZipPassword != nil {
			return nil, io.ErrUnexpectedEOF
		}
		files = append(files, generated.DataAnalysisFileUploadResource{Name: file.Name, SasUrl: f.sasURLs[file.Name]})
	}
	return &generated.DataAnalysisCreatedResource{Id: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Type: generated.DataAnalysisTypeMalware, Files: files}, nil
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

func (f *fakeDeferredCreator) CreateAnalysis(_ context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error) {
	f.calls++
	other, err := request.AsDataAnalysisOtherData()
	if err != nil {
		return nil, err
	}
	files := make([]generated.DataAnalysisFileUploadResource, 0, len(other.Files))
	for _, file := range other.Files {
		if file.AesKey == "" || file.Hash != nil {
			return nil, io.ErrUnexpectedEOF
		}
		files = append(files, generated.DataAnalysisFileUploadResource{Name: file.Name, SasUrl: f.sasURLs[file.Name]})
	}
	return &generated.DataAnalysisCreatedResource{Id: uuid.MustParse("44444444-4444-4444-4444-444444444444"), Type: generated.DataAnalysisTypeOther, ManualAnalysisAllowed: true, Files: files}, nil
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
