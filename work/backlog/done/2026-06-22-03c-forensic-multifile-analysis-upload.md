# Forensic multi-file analysis upload

## Type

feature

## Context

The primary product workflow is forensic data analysis. After the shared multi-file upload engine exists, the toolkit must support the NCSC `forensic` data-analysis type for one or more pre-existing local files.

## Functional Requirements

- Create one `forensic` data-analysis for one or more pre-existing local files.
- Require `requested_analyses` for each file.
- Support optional per-file `disk_encryption_key` input.
- Support `manual_analysis_allowed` for the analysis.
- Reuse the shared encryption, hashing, SAS upload, checkpoint, resume, partial-progress, summary, and cleanup behavior from the upload engine.
- Stop at successful upload; do not retrieve results.

## Technical Requirements

- Treat `disk_encryption_key` as sensitive input and do not persist it unless a later ADR explicitly allows it.
- Preserve memory-only handling for API credentials, OAuth tokens, AES keys, and sensitive per-file inputs.
- Reuse upload checkpoint semantics from `work/adr/2026-06-22-restart-resumable-upload-semantics.md`.
- Follow `work/guidelines/api-upload.md` and `work/guidelines/cli-workflows.md`.

## Acceptance Criteria

- A user can create a `forensic` analysis with multiple local files.
- Each file requires at least one requested analysis before the create request is sent.
- Optional disk encryption keys are included in the create request when provided.
- Upload behavior matches the shared engine behavior for partial progress, restart-resume, missing artifacts, SAS expiry, and cleanup.
- No secrets or disk encryption keys are written to disk.
- No result retrieval occurs.

## Analysis

### Likely Impact

- Primary implementation lane: `cmd/upload.go` CLI flags/validation -> `internal/upload.Options` and create-analysis dispatch -> `internal/ncscapi.NewForensicAnalysisRequest` -> existing shared encryption/checkpoint/SAS upload/cleanup path.
- `cmd/upload.go` - currently supports only `other` and `malware` in validation, help text, and secret-like per-file flag parsing; it likely needs `forensic`, per-file `requested_analyses`, optional per-file `disk_encryption_key`, and `manual_analysis_allowed` inputs wired into `upload.Run`.
- `internal/upload/workflow.go` - currently dispatches only `AnalysisTypeOther` and `AnalysisTypeMalware`; add `AnalysisTypeForensic` and a forensic request builder that maps each checkpoint file to `generated.ForensicDataAnalysisFile` with AES key, hash, required requested analyses, and optional disk encryption key.
- `internal/upload/checkpoint.go` - checkpoint already stores durable resume metadata and AES keys remain memory-only; avoid adding `disk_encryption_key` to `CheckpointFile` because the story and guidelines require sensitive per-file inputs to remain memory-only.
- `internal/ncscapi/client.go` - already has `NewForensicAnalysisRequest(files, manualAnalysisAllowed, metadata)` using generated forensic API models, so implementation should call this wrapper rather than hand-rolling request JSON.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` - reset helper mirrors `uploadFlags`; new upload flag fields will need reset/default coverage so Cobra tests remain isolated.
- `internal/upload/workflow_test.go` - closest place for unit/integration-style workflow checks: forensic create request shape, required requested analyses, optional disk key inclusion, non-persistence of disk keys, and no result retrieval during upload.
- `internal/ncscapi/client_test.go` - may need a narrow request-wrapper test if forensic serialization is not already covered elsewhere.

### Existing Patterns / Prior Art

- `internal/upload/workflow.go` `createOtherAnalysis` / `createMalwareAnalysis` - closest pattern for building generated per-file structs from checkpoint files plus in-memory sensitive values, then calling an `ncscapi` request helper.
- `cmd/upload.go` `--zip-password` handling - closest pattern for per-file sensitive CLI input that is parsed into memory-only maps and rejected for unrelated analysis types.
- `internal/upload/workflow_test.go` `TestRunCreatesMalwareAnalysisWithZipPasswordsWithoutPersistingThem` - closest test pattern for sensitive per-file input inclusion in the create request without checkpoint/stdout leakage.
- `internal/upload/workflow_test.go` resume/SAS/missing-artifact tests - shared upload behavior is already covered and should be reused rather than duplicated broadly for forensic.
- `internal/ncscapi/generated/client.gen.go` defines `DataAnalysisForensicData` and `ForensicDataAnalysisFile` with `manual_analysis_allowed`, `disk_encryption_key`, and `requested_analyses`; generated code should be treated as read-only unless the OpenAPI spec changes.

### Layer Boundaries

- Touch first: `cmd/upload.go` for workflow surface and validation; `internal/upload/workflow.go` / `Options` for forensic request construction and memory-only inputs; tests around those seams.
- Avoid unless evidence emerges: `internal/upload/upload.go` block upload mechanics, encryption implementation, checkpoint permission mechanics, result retrieval command/client methods, acquisition/gui flows, `internal/config`, and generated NCSC API bindings.
- Inference: checkpoint schema likely should not change for `disk_encryption_key` or requested analyses unless implementation needs non-sensitive requested-analysis values persisted for resume visibility; if persisted, do not persist disk encryption keys.

### Verification Plan

**Unit Tests**:

- Forensic workflow builds a create request with multiple files, AES keys, hashes, required requested analyses, optional disk encryption keys, and configured `manual_analysis_allowed`.
- Missing requested analyses for any forensic file fails before `CreateAnalysis` is called.
- Disk encryption keys are not written to checkpoint files or stdout/stderr.

**Integration Tests**:

- Existing shared upload behavior remains exercised for forensic through the same fake SAS server path: successful upload stops after upload and does not call result endpoints.

**E2E / Manual Validation**:

- Run the upload CLI with `--type forensic`, multiple local files, per-file requested analyses, optional per-file disk key, and both manual-analysis allowed states; confirm checkpoint contains only allowed resume metadata and upload summary stops at successful upload.

## Validation update (2026-06-25 08:44)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -w main.go cmd internal/config internal/upload`, `go test ./...`, `go vet ./...`, and `go build -o /tmp/da-toolkit .`.
* Focused checks passed: `go test ./internal/upload -run 'TestRunCreatesForensic|TestRunRequiresRequestedAnalyses|TestResumeContinues|TestRunFailsWhenEncryptedArtifactMissing|TestRunFailsWhenSASURLExpired' -v`, `go test ./cmd -run 'Test' -v`, and `go test ./cmd ./internal/upload -cover`.
* Touched-scope coverage: no material regression observed (`cmd` 45.3%, `internal/upload` 80.4%).
* Security review: completed for sensitive inputs and upload checkpoint handling; disk encryption keys remain memory-only and are not written to checkpoint files or stdout/stderr by the forensic workflow test.
* Retained exploratory artifacts: not applicable; no UI/browser flow required.
* Validated checklist items: forensic multi-file create request includes AES keys, hashes, requested analyses, optional disk encryption key, and manual-analysis flag; missing requested analyses fail before `CreateAnalysis`; shared upload path still uploads via fake SAS server, supports resume proof, and stops without result retrieval; checkpoint schema remains free of disk encryption key fields; CLI flags are scoped to forensic-only usage.
* Providers covered: NCSC Data Analysis API wrapper path using generated forensic request models, with fake SAS upload server for workflow validation.
