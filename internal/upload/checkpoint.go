package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

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
