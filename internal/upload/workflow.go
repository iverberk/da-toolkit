package upload

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"da-toolkit/internal/ncscapi"
	"da-toolkit/internal/ncscapi/generated"
	"github.com/google/uuid"
)

const (
	ChunkSize              = 1024 * 1024
	DefaultUploadBlockSize = 8 * 1024 * 1024

	StatusPrepared = "prepared"
	StatusUploaded = "uploaded"
	StatusFailed   = "failed"
	StatusSkipped  = "skipped"

	AnalysisTypeOther    = "other"
	AnalysisTypeMalware  = "malware"
	AnalysisTypeForensic = "forensic"
)

type AnalysisCreator interface {
	CreateAnalysis(ctx context.Context, request generated.StoreDataAnalysisRequest) (*generated.DataAnalysisCreatedResource, error)
}

type Options struct {
	Files                 []string
	WorkspaceDir          string
	CheckpointPath        string
	Resume                bool
	AssumeYes             bool
	AnalysisType          string
	ZipPasswords          map[string]string
	RequestedAnalyses     map[string][]string
	DiskEncryptionKeys    map[string]string
	ManualAnalysisAllowed bool
	API                   AnalysisCreator
	UploadBlockSize       int64
	Now                   func() time.Time
	Stdin                 io.Reader
	Stdout                io.Writer
}

type FileSummary struct {
	SourcePath            string
	EncryptedArtifactPath string
	Status                string
	Resumable             bool
	Message               string
}

type Result struct {
	AnalysisID uuid.UUID
	Files      []FileSummary
}

func Run(ctx context.Context, opts Options) (Result, error) {
	opts = validateOptions(opts)

	cp, aesKeys, err := prepareCheckpoint(opts)
	if err != nil {
		return Result{}, err
	}
	if err := saveCheckpoint(opts.CheckpointPath, cp); err != nil {
		return Result{}, err
	}
	if opts.Resume && cp.AnalysisID == "" {
		return Result{}, errors.New("checkpoint is missing analysis_id; restart the workflow manually")
	}
	if cp.AnalysisID == "" {
		if err := validateCreateInputs(cp, opts); err != nil {
			return Result{}, err
		}
		if opts.API == nil {
			return Result{}, fmt.Errorf("api client is required to create a %s analysis", opts.AnalysisType)
		}
		created, err := createAnalysis(ctx, opts.API, cp, aesKeys, opts)
		if err != nil {
			return Result{}, err
		}
		cp.AnalysisID = created.Id.String()
		for _, returned := range created.Files {
			for i := range cp.Files {
				if cp.Files[i].Name == returned.Name {
					cp.Files[i].SASURL = returned.SasUrl
					cp.Files[i].SASExpiresAt = sasExpiry(returned.SasUrl)
				}
			}
		}
		if err := saveCheckpoint(opts.CheckpointPath, cp); err != nil {
			return Result{}, err
		}
	}

	for i := range cp.Files {
		if cp.Files[i].Status == StatusUploaded {
			cp.Files[i].Status = StatusSkipped
			cp.Files[i].Message = "already uploaded; skipped during resume"
			reportStatus(opts.Stdout, &cp.Files[i])
			continue
		}
		uploadOne(ctx, opts, &cp.Files[i], func() { _ = saveCheckpoint(opts.CheckpointPath, cp) })
		_ = saveCheckpoint(opts.CheckpointPath, cp)
	}

	if err := saveCheckpoint(opts.CheckpointPath, cp); err != nil {
		return Result{}, err
	}
	printSummary(opts.Stdout, cp)
	cleanupUploaded(opts, cp)
	return resultFromCheckpoint(cp), nil
}

func validateOptions(opts Options) Options {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.UploadBlockSize <= 0 {
		opts.UploadBlockSize = DefaultUploadBlockSize
	}
	if opts.AnalysisType == "" {
		opts.AnalysisType = AnalysisTypeOther
	}
	if opts.CheckpointPath == "" {
		opts.CheckpointPath = filepath.Join(opts.WorkspaceDir, "upload-checkpoint.json")
	}
	if opts.WorkspaceDir == "" {
		opts.WorkspaceDir = filepath.Dir(opts.CheckpointPath)
	}
	return opts
}

func createAnalysis(ctx context.Context, api AnalysisCreator, cp Checkpoint, aesKeys map[string]string, opts Options) (*generated.DataAnalysisCreatedResource, error) {
	switch opts.AnalysisType {
	case AnalysisTypeOther:
		return createOtherAnalysis(ctx, api, cp, aesKeys)
	case AnalysisTypeMalware:
		return createMalwareAnalysis(ctx, api, cp, aesKeys, opts.ZipPasswords)
	case AnalysisTypeForensic:
		return createForensicAnalysis(ctx, api, cp, aesKeys, opts.RequestedAnalyses, opts.DiskEncryptionKeys, opts.ManualAnalysisAllowed)
	default:
		return nil, fmt.Errorf("unsupported analysis type %q", opts.AnalysisType)
	}
}

func validateCreateInputs(cp Checkpoint, opts Options) error {
	if opts.AnalysisType != AnalysisTypeForensic {
		return nil
	}
	for _, file := range cp.Files {
		if _, ok := requestedAnalysesForFile(opts.RequestedAnalyses, file); !ok {
			return fmt.Errorf("missing requested analyses for forensic file %s", file.Name)
		}
	}
	return nil
}

func createOtherAnalysis(ctx context.Context, api AnalysisCreator, cp Checkpoint, aesKeys map[string]string) (*generated.DataAnalysisCreatedResource, error) {
	files := make([]generated.DataAnalysisFile, 0, len(cp.Files))
	for _, file := range cp.Files {
		key := aesKeys[file.Name]
		if key == "" {
			return nil, fmt.Errorf("missing in-memory AES key for %s", file.Name)
		}
		hash := file.Hash
		files = append(files, generated.DataAnalysisFile{Name: file.Name, AesKey: key, Hash: &hash})
	}
	req, err := ncscapi.NewOtherAnalysisRequest(files, nil)
	if err != nil {
		return nil, err
	}
	return api.CreateAnalysis(ctx, req)
}

func createMalwareAnalysis(ctx context.Context, api AnalysisCreator, cp Checkpoint, aesKeys map[string]string, zipPasswords map[string]string) (*generated.DataAnalysisCreatedResource, error) {
	files := make([]generated.MalwareDataAnalysisFile, 0, len(cp.Files))
	for _, file := range cp.Files {
		key := aesKeys[file.Name]
		if key == "" {
			return nil, fmt.Errorf("missing in-memory AES key for %s", file.Name)
		}
		hash := file.Hash
		malwareFile := generated.MalwareDataAnalysisFile{Name: file.Name, AesKey: key, Hash: &hash}
		if password, ok := zipPasswordForFile(zipPasswords, file); ok {
			malwareFile.ZipPassword = &password
		}
		files = append(files, malwareFile)
	}
	req, err := ncscapi.NewMalwareAnalysisRequest(files, nil)
	if err != nil {
		return nil, err
	}
	return api.CreateAnalysis(ctx, req)
}

func createForensicAnalysis(ctx context.Context, api AnalysisCreator, cp Checkpoint, aesKeys map[string]string, requestedAnalyses map[string][]string, diskEncryptionKeys map[string]string, manualAnalysisAllowed bool) (*generated.DataAnalysisCreatedResource, error) {
	files := make([]generated.ForensicDataAnalysisFile, 0, len(cp.Files))
	for _, file := range cp.Files {
		key := aesKeys[file.Name]
		if key == "" {
			return nil, fmt.Errorf("missing in-memory AES key for %s", file.Name)
		}
		requested, ok := requestedAnalysesForFile(requestedAnalyses, file)
		if !ok {
			return nil, fmt.Errorf("missing requested analyses for forensic file %s", file.Name)
		}
		hash := file.Hash
		forensicFile := generated.ForensicDataAnalysisFile{Name: file.Name, AesKey: key, Hash: &hash, RequestedAnalyses: requested}
		if diskKey, ok := diskEncryptionKeyForFile(diskEncryptionKeys, file); ok {
			forensicFile.DiskEncryptionKey = &diskKey
		}
		files = append(files, forensicFile)
	}
	req, err := ncscapi.NewForensicAnalysisRequest(files, manualAnalysisAllowed, nil)
	if err != nil {
		return nil, err
	}
	return api.CreateAnalysis(ctx, req)
}

func zipPasswordForFile(zipPasswords map[string]string, file CheckpointFile) (string, bool) {
	if len(zipPasswords) == 0 {
		return "", false
	}
	for _, key := range []string{file.Name, file.SourcePath, filepath.Base(file.SourcePath)} {
		if password, ok := zipPasswords[key]; ok {
			return password, true
		}
	}
	return "", false
}

func requestedAnalysesForFile(requestedAnalyses map[string][]string, file CheckpointFile) ([]string, bool) {
	if len(requestedAnalyses) == 0 {
		return nil, false
	}
	for _, key := range fileLookupKeys(file) {
		if analyses, ok := requestedAnalyses[key]; ok && len(analyses) > 0 {
			return append([]string(nil), analyses...), true
		}
	}
	return nil, false
}

func diskEncryptionKeyForFile(diskEncryptionKeys map[string]string, file CheckpointFile) (string, bool) {
	if len(diskEncryptionKeys) == 0 {
		return "", false
	}
	for _, key := range fileLookupKeys(file) {
		if diskKey, ok := diskEncryptionKeys[key]; ok {
			return diskKey, true
		}
	}
	return "", false
}

func fileLookupKeys(file CheckpointFile) []string {
	return []string{file.Name, file.SourcePath, filepath.Base(file.SourcePath)}
}

func printSummary(w io.Writer, cp Checkpoint) {
	fmt.Fprintln(w, "Upload summary:")
	if cp.AnalysisID != "" {
		fmt.Fprintf(w, "Analysis ID: %s\n", cp.AnalysisID)
	}
	for _, file := range cp.Files {
		fmt.Fprintf(w, "- %s: %s; artifact=%s; resumable=%t", file.SourcePath, file.Status, file.EncryptedArtifactPath, isResumable(file))
		if file.Message != "" {
			fmt.Fprintf(w, "; %s", file.Message)
		}
		fmt.Fprintln(w)
	}
}

func cleanupUploaded(opts Options, cp Checkpoint) {
	uploaded := uploadedArtifacts(cp)
	if len(uploaded) == 0 {
		return
	}
	confirm := opts.AssumeYes
	if !confirm {
		fmt.Fprint(opts.Stdout, "Delete encrypted artifacts for successfully uploaded files? [y/N]: ")
		line, _ := bufio.NewReader(opts.Stdin).ReadString('\n')
		confirm = strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes")
	}
	if !confirm {
		return
	}
	for _, path := range uploaded {
		_ = os.Remove(path)
	}
}

func uploadedArtifacts(cp Checkpoint) []string {
	var paths []string
	for _, file := range cp.Files {
		if file.Status == StatusUploaded || file.Status == StatusSkipped {
			paths = append(paths, file.EncryptedArtifactPath)
		}
	}
	return paths
}

func uniqueArtifactName(source string, index int) string {
	name := filepath.Base(source) + ".enc"
	if index == 0 {
		return name
	}
	return fmt.Sprintf("%03d-%s", index+1, name)
}

func uniqueUploadName(source string, counts map[string]int) string {
	base := filepath.Base(source)
	counts[base]++
	if counts[base] == 1 {
		return base
	}
	return fmt.Sprintf("%03d-%s", counts[base], base)
}

func sasExpiry(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Query().Get("se")
}

func expiredURL(raw string, now time.Time) bool {
	return expired(sasExpiry(raw), now)
}

func expired(value string, now time.Time) bool {
	if value == "" {
		return false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return !now.Before(parsed)
		}
	}
	return false
}
