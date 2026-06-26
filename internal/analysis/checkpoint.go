package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RunMode int

const (
	RunModeCreate RunMode = iota
	RunModeResume
)

type checkpointState int

const (
	checkpointMissing checkpointState = iota
	checkpointUsableIncomplete
	checkpointCompleted
	checkpointExpiredSAS
	checkpointInvalid
	checkpointUnresumable
)

type checkpointInspection struct {
	state checkpointState
	cp    Checkpoint
	err   error
}

func Plan(opts Options) (Options, RunMode, error) {
	opts = validateOptions(opts)
	if opts.Resume && opts.New {
		return opts, RunModeCreate, errors.New("--resume and --new cannot be used together")
	}
	inspection := inspectCheckpoint(opts.CheckpointPath, opts.Now())
	if opts.New {
		if inspection.state == checkpointMissing {
			return opts, RunModeCreate, nil
		}
		return opts, RunModeCreate, fmt.Errorf("checkpoint already exists at %s; choose a different --checkpoint or --workspace before using --new", opts.CheckpointPath)
	}
	if opts.Resume {
		if inspection.state == checkpointUsableIncomplete {
			return opts, RunModeResume, nil
		}
		return opts, RunModeResume, checkpointStateError("--resume requires a usable incomplete checkpoint", opts.CheckpointPath, inspection)
	}
	if inspection.state == checkpointMissing {
		return opts, RunModeCreate, nil
	}
	if inspection.state == checkpointUsableIncomplete {
		return opts, RunModeResume, nil
	}
	return opts, RunModeCreate, checkpointStateError("checkpoint is not safe for fresh creation", opts.CheckpointPath, inspection)
}

type Checkpoint struct {
	Version    int              `json:"version"`
	AnalysisID string           `json:"analysis_id,omitempty"`
	Files      []CheckpointFile `json:"files"`
}

type CheckpointFile struct {
	Name                  string `json:"name"`
	SourcePath            string `json:"source_path"`
	EncryptedArtifactPath string `json:"encrypted_artifact_path"`
	Hash                  string `json:"hash,omitempty"`
	SASURL                string `json:"sas_url,omitempty"`
	SASExpiresAt          string `json:"sas_expires_at,omitempty"`
	Status                string `json:"status"`
	Message               string `json:"message,omitempty"`
	UploadBlockSize       int64  `json:"upload_block_size,omitempty"`
	TotalBlocks           int    `json:"total_blocks,omitempty"`
	UploadedBlocks        []int  `json:"uploaded_blocks,omitempty"`
	Committed             bool   `json:"committed,omitempty"`
}

func prepareCheckpoint(opts Options) (Checkpoint, map[string]string, error) {
	if opts.Resume {
		cp, err := loadCheckpoint(opts.CheckpointPath)
		return cp, nil, err
	}
	if len(opts.Files) == 0 {
		return Checkpoint{}, nil, errors.New("at least one local file is required")
	}
	if err := os.MkdirAll(opts.WorkspaceDir, 0o700); err != nil {
		return Checkpoint{}, nil, fmt.Errorf("create workspace: %w", err)
	}
	cp := Checkpoint{Version: 1}
	aesKeys := make(map[string]string, len(opts.Files))
	nameCounts := make(map[string]int, len(opts.Files))
	for _, source := range opts.Files {
		abs, err := filepath.Abs(source)
		if err != nil {
			return Checkpoint{}, nil, fmt.Errorf("resolve source path %q: %w", source, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return Checkpoint{}, nil, fmt.Errorf("stat source file %q: %w", source, err)
		}
		if info.IsDir() {
			return Checkpoint{}, nil, fmt.Errorf("source path %q is a directory", source)
		}
		artifact := filepath.Join(opts.WorkspaceDir, uniqueArtifactName(abs, len(cp.Files)))
		key, hash, err := EncryptFile(abs, artifact)
		if err != nil {
			return Checkpoint{}, nil, fmt.Errorf("encrypt %q: %w", source, err)
		}
		name := uniqueUploadName(abs, nameCounts)
		cp.Files = append(cp.Files, CheckpointFile{Name: name, SourcePath: abs, EncryptedArtifactPath: artifact, Hash: hash, Status: StatusPrepared})
		aesKeys[name] = key
	}
	return cp, aesKeys, nil
}

func inspectCheckpoint(path string, now time.Time) checkpointInspection {
	cp, err := loadCheckpoint(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return checkpointInspection{state: checkpointMissing, err: err}
		}
		return checkpointInspection{state: checkpointInvalid, err: err}
	}
	if cp.AnalysisID == "" {
		return checkpointInspection{state: checkpointInvalid, cp: cp, err: errors.New("checkpoint is missing analysis_id")}
	}
	if len(cp.Files) == 0 {
		return checkpointInspection{state: checkpointInvalid, cp: cp, err: errors.New("checkpoint contains no files")}
	}
	completed := 0
	usable := 0
	for _, file := range cp.Files {
		if strings.TrimSpace(file.Name) == "" || strings.TrimSpace(file.SourcePath) == "" || strings.TrimSpace(file.EncryptedArtifactPath) == "" || strings.TrimSpace(file.Status) == "" {
			return checkpointInspection{state: checkpointInvalid, cp: cp, err: errors.New("checkpoint contains incomplete file metadata")}
		}
		if !isKnownCheckpointStatus(file.Status) {
			return checkpointInspection{state: checkpointInvalid, cp: cp, err: fmt.Errorf("checkpoint contains unknown file status %q for %s", file.Status, file.Name)}
		}
		if file.Status == StatusUploaded || file.Status == StatusSkipped {
			completed++
			continue
		}
		if expired(file.SASExpiresAt, now) || expiredURL(file.SASURL, now) {
			return checkpointInspection{state: checkpointExpiredSAS, cp: cp, err: fmt.Errorf("SAS URL expired for %s", file.Name)}
		}
		if file.SASURL == "" {
			return checkpointInspection{state: checkpointUnresumable, cp: cp, err: fmt.Errorf("missing SAS upload URL for %s", file.Name)}
		}
		if _, err := os.Stat(file.EncryptedArtifactPath); err != nil {
			return checkpointInspection{state: checkpointUnresumable, cp: cp, err: fmt.Errorf("encrypted artifact for %s is unavailable: %w", file.Name, err)}
		}
		usable++
	}
	if completed == len(cp.Files) {
		return checkpointInspection{state: checkpointCompleted, cp: cp}
	}
	if usable > 0 {
		return checkpointInspection{state: checkpointUsableIncomplete, cp: cp}
	}
	return checkpointInspection{state: checkpointUnresumable, cp: cp, err: errors.New("checkpoint has no resumable files")}
}

func checkpointStateError(prefix, path string, inspection checkpointInspection) error {
	switch inspection.state {
	case checkpointMissing:
		return fmt.Errorf("%s: no checkpoint exists at %s", prefix, path)
	case checkpointCompleted:
		return fmt.Errorf("%s: checkpoint at %s is already completed; use a different --checkpoint or --workspace for a new analysis", prefix, path)
	case checkpointExpiredSAS:
		return fmt.Errorf("%s: checkpoint at %s has expired SAS upload credentials; %w", prefix, path, inspection.err)
	case checkpointInvalid:
		return fmt.Errorf("%s: checkpoint at %s is invalid or corrupt; %w", prefix, path, inspection.err)
	case checkpointUnresumable:
		return fmt.Errorf("%s: checkpoint at %s is not resumable; %w", prefix, path, inspection.err)
	default:
		return fmt.Errorf("%s: checkpoint at %s is ambiguous", prefix, path)
	}
}

func loadCheckpoint(path string) (Checkpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("read checkpoint: %w", err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return Checkpoint{}, fmt.Errorf("parse checkpoint: %w", err)
	}
	if cp.Version != 1 {
		return Checkpoint{}, errors.New("unsupported checkpoint version")
	}
	return cp, nil
}

func saveCheckpoint(path string, cp Checkpoint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("protect checkpoint permissions: %w", err)
	}
	return nil
}

func resultFromCheckpoint(cp Checkpoint) Result {
	var id uuid.UUID
	if cp.AnalysisID != "" {
		id, _ = uuid.Parse(cp.AnalysisID)
	}
	res := Result{AnalysisID: id}
	for _, file := range cp.Files {
		res.Files = append(res.Files, FileSummary{SourcePath: file.SourcePath, EncryptedArtifactPath: file.EncryptedArtifactPath, Status: file.Status, Resumable: isResumable(file), Message: file.Message})
	}
	return res
}

func isResumable(file CheckpointFile) bool {
	if !isKnownCheckpointStatus(file.Status) {
		return false
	}
	if file.Status == StatusUploaded || file.Status == StatusSkipped {
		return false
	}
	if file.Status == StatusFailed && strings.Contains(file.Message, "missing") {
		return false
	}
	if _, err := os.Stat(file.EncryptedArtifactPath); err != nil {
		return false
	}
	return file.SASURL != ""
}

func isKnownCheckpointStatus(status string) bool {
	switch status {
	case StatusPrepared, StatusUploaded, StatusFailed, StatusSkipped:
		return true
	default:
		return false
	}
}
