package upload

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
)

func uploadOne(ctx context.Context, opts Options, file *CheckpointFile, saveProgress func()) {
	if !validateUploadFile(opts, file) {
		reportStatus(opts.Stdout, file)
		return
	}
	reportStatus(opts.Stdout, file)
	fh, err := os.Open(file.EncryptedArtifactPath)
	if err != nil {
		file.Status = StatusFailed
		file.Message = "open encrypted artifact: " + err.Error()
		reportStatus(opts.Stdout, file)
		return
	}
	defer fh.Close()
	client, err := blockblob.NewClientWithNoCredential(file.SASURL, nil)
	if err != nil {
		file.Status = StatusFailed
		file.Message = "create Azure Blob SAS client: " + err.Error()
		reportStatus(opts.Stdout, file)
		return
	}
	uploaded := uploadedBlockSet(file.UploadedBlocks)
	for index := 0; index < file.TotalBlocks; index++ {
		if uploaded[index] {
			continue
		}
		body, err := readBlock(fh, int64(index)*file.UploadBlockSize, file.UploadBlockSize)
		if err != nil {
			file.Status = StatusFailed
			file.Message = "read encrypted artifact block: " + err.Error()
			reportStatus(opts.Stdout, file)
			return
		}
		if _, err := client.StageBlock(ctx, blockIDForIndex(index), body, nil); err != nil {
			file.Status = StatusFailed
			file.Message = "stage Azure Blob block: " + err.Error()
			reportStatus(opts.Stdout, file)
			return
		}
		file.UploadedBlocks = appendUploadedBlock(file.UploadedBlocks, index)
		file.Message = fmt.Sprintf("uploaded %d/%d blocks", len(file.UploadedBlocks), file.TotalBlocks)
		reportStatus(opts.Stdout, file)
		if saveProgress != nil {
			saveProgress()
		}
	}
	if _, err := client.CommitBlockList(ctx, blockIDs(file.TotalBlocks), nil); err != nil {
		file.Status = StatusFailed
		file.Message = "commit Azure Blob block list: " + err.Error()
		reportStatus(opts.Stdout, file)
		return
	}
	file.Committed = true
	file.Status = StatusUploaded
	file.Message = "uploaded successfully"
	reportStatus(opts.Stdout, file)
}

func validateUploadFile(opts Options, file *CheckpointFile) bool {
	info, err := os.Stat(file.EncryptedArtifactPath)
	if err != nil {
		file.Status = StatusFailed
		file.Message = "encrypted artifact is missing; restart the workflow to recreate it"
		return false
	}
	if file.SASURL == "" {
		file.Status = StatusFailed
		file.Message = "missing SAS upload URL in checkpoint; restart the workflow"
		return false
	}
	if expired(file.SASExpiresAt, opts.Now()) || expiredURL(file.SASURL, opts.Now()) {
		file.Status = StatusFailed
		file.Message = "SAS URL expired before upload completion; restart manually to create a new data-analysis"
		return false
	}
	if file.UploadBlockSize <= 0 {
		file.UploadBlockSize = opts.UploadBlockSize
	}
	if file.TotalBlocks <= 0 {
		file.TotalBlocks = totalBlocks(info.Size(), file.UploadBlockSize)
	}
	return true
}

func reportStatus(w io.Writer, file *CheckpointFile) {
	if w == nil {
		return
	}
	if file.Message == "" {
		fmt.Fprintf(w, "Upload status: %s: %s\n", file.SourcePath, file.Status)
		return
	}
	fmt.Fprintf(w, "Upload status: %s: %s; %s\n", file.SourcePath, file.Status, file.Message)
}

type readSeekCloser struct{ *bytes.Reader }

func (r readSeekCloser) Close() error { return nil }

func readBlock(file *os.File, offset, size int64) (io.ReadSeekCloser, error) {
	buf := make([]byte, size)
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return readSeekCloser{bytes.NewReader(buf[:n])}, nil
}

func totalBlocks(size, blockSize int64) int {
	if size == 0 {
		return 1
	}
	return int((size + blockSize - 1) / blockSize)
}

func blockIDForIndex(index int) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("da-toolkit-block-%08d", index)))
}

func blockIDs(total int) []string {
	ids := make([]string, total)
	for i := range ids {
		ids[i] = blockIDForIndex(i)
	}
	return ids
}

func uploadedBlockSet(indexes []int) map[int]bool {
	set := make(map[int]bool, len(indexes))
	for _, index := range indexes {
		set[index] = true
	}
	return set
}

func appendUploadedBlock(indexes []int, index int) []int {
	for _, existing := range indexes {
		if existing == index {
			return indexes
		}
	}
	return append(indexes, index)
}
