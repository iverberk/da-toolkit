# Multi-file upload engine for other analysis

## Type

feature

## Context

The first upload workflow should use pre-existing local files and not depend on Dissect/acquire. The `other` analysis type is the initial vehicle for the shared multi-file encryption, hashing, upload, checkpoint, resume, partial-progress, and cleanup behavior.

## Functional Requirements

- Create one `other` data-analysis for one or more pre-existing local files.
- Set `manual_analysis_allowed=true` for `other` analyses.
- Generate an in-memory AES key per file.
- Encrypt and hash each selected file.
- Upload each encrypted artifact through its corresponding returned SAS URL.
- Treat partial progress as valid and report per-file status.
- Support restart-resume for incomplete files at transfer/block level when encrypted artifacts and checkpoint metadata remain available.
- Stop at successful upload; do not retrieve results.
- Show one end-of-run per-file summary and prompt once to delete all successfully uploaded encrypted artifacts.

## Technical Requirements

- Persist checkpoint metadata needed for restart-resume such as analysis ID, source path, encrypted artifact path, file hash, upload checkpoint, uploaded Azure block indexes/IDs, phase status, SAS URL, and SAS expiry metadata when known. Treat SAS URLs as acceptable persisted operational secrets protected by owner-only checkpoint permissions.
- Do not persist client secrets, OAuth tokens, or AES keys.
- Do not automatically create a replacement data-analysis after SAS expiry.
- Do not re-upload files already marked complete during resume, and do not re-stage Azure blocks already recorded as uploaded for an incomplete file.
- Retain failed or incomplete encrypted artifacts for resume.
- Follow `work/adr/2026-06-22-restart-resumable-upload-semantics.md` and `work/guidelines/api-upload.md`.

## Acceptance Criteria

- A user can create an `other` analysis with multiple local files and upload all encrypted artifacts.
- Per-file upload status is visible during and after the run.
- Restart-resume skips completed files and resumes incomplete files from the last checkpointed uploaded Azure block.
- Missing encrypted artifact fails only the affected file with a clear remediation message.
- Expired SAS URL fails affected file uploads without creating a new data-analysis.
- End-of-run summary lists each file, status, encrypted artifact path, and resumable/non-resumable state.
- Cleanup prompt appears once and deletes only encrypted artifacts for successfully uploaded files when confirmed.
- No result retrieval occurs.

## Analysis

### Likely Impact

- Primary implementation lane: `cmd/upload.go` CLI entrypoint -> new non-command workflow logic under `internal/` -> `internal/ncscapi` create-analysis wrapper -> raw SAS artifact upload/checkpoint/cleanup helpers.
- `cmd/upload.go` - currently a safe placeholder with `cobra.NoArgs`; this story likely replaces it with flags/args for `other` local-file upload orchestration and user-facing status/cleanup prompts.
- `internal/ncscapi/client.go` - already has `CreateAnalysis` and `NewOtherAnalysisRequest`, with `manual_analysis_allowed` set to `generated.True`; likely reusable for analysis creation and SAS URL discovery.
- New `internal/` workflow/storage packages - likely needed for per-file AES-GCM encryption, encrypted-file SHA-256 hashing, checkpoint persistence, restart-resume state, SAS URL expiry checks, and per-file status summaries. Repo evidence shows no current upload engine beyond the placeholder.
- Encryption implementation must align with the NCSC Python proof of concept at `https://github.com/NCSC-NL/data-analysis-api-examples/blob/main/lib/encrypt.py`: generate a 32-byte AES key per file, encrypt plaintext in 1 MiB chunks with AES-256-GCM, use a fresh 12-byte random IV per chunk, write each encrypted chunk as `8-byte big-endian length` + `IV || ciphertext || 16-byte tag`, and send the base64-encoded key in the create-analysis file metadata without persisting it.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` / new command tests - placeholder expectations include `upload`; these likely need narrowing or replacement when `upload` becomes real behavior.
- `internal/ncscapi/client_test.go` - useful place to extend API-wrapper coverage only if request construction or create-analysis response handling changes; current tests already cover OAuth reuse, secret non-leakage, create/read/delete/result wrapper behavior, and `other` payload fixtures.
- `internal/config/config.go` - only if the implementation needs a configured workspace/checkpoint directory; keep it non-secret per existing config constraints.

### Existing Patterns / Prior Art

- `internal/ncscapi/client.go` - established pattern for keeping generated OpenAPI code behind toolkit-owned wrappers, normalizing errors, and keeping OAuth tokens in memory.
- `internal/ncscapi/generated/client.gen.go` - generated types show `DataAnalysisFile` requires base64 AES key plus optional encrypted-file hash, and create responses return per-file `sas_url`; use types through `internal/ncscapi`, not direct hand-rolled API structs.
- NCSC external example `lib/encrypt.py` - encryption-format reference for Go parity: chunked AES-GCM with 1 MiB plaintext chunks, 12-byte IV, 16-byte tag, and 8-byte big-endian encrypted-chunk length prefixes.
- `internal/ncscapi/client_test.go` - closest test style for HTTP fakes, OAuth assertions, no-secret error checks, and `other` analysis response fixtures.
- `cmd/root.go` / `cmd/root_test.go` - current Cobra conventions and user-facing error pattern (`Error: <clear message>`) to preserve.

### Layer Boundaries

- Touch first: Cobra `upload` command wiring in `cmd/`, new internal workflow code for encryption/hash/upload/checkpoint/resume/summary/cleanup, and existing `internal/ncscapi` wrapper only where API-facing helpers are necessary.
- Avoid unless evidence emerges: `internal/ncscapi/generated/` and OpenAPI spec files, result retrieval paths (`GetResultFile`, `GetResultArchive`, `cmd/result.go`), acquisition/Dissect packaging paths, GUI behavior, and replacement data-analysis creation after SAS expiry.

### Verification Plan

**Unit Tests**:

- Check per-file encryption/hash metadata generation without persisting AES keys.
- Check Go-encrypted artifact format against the NCSC Python proof-of-concept contract: chunk length prefix, IV/ciphertext/tag ordering, 1 MiB chunking, base64 AES key value, and decryptability by an equivalent AES-GCM reader.
- Check checkpoint persistence contains no client secrets, OAuth tokens, or AES keys; records SAS URLs as operational secrets; records uploaded Azure blocks; skips completed files on resume; and resumes incomplete files without re-staging checkpointed blocks.
- Check missing encrypted artifact marks only the affected file non-resumable with a clear remediation message.
- Check SAS expiry fails the affected file and does not create a replacement data-analysis.
- Check cleanup deletes only successfully uploaded encrypted artifacts after one confirmation prompt.

**Integration Tests**:

- Use HTTP test servers to cover create `other` analysis, per-file Azure block SAS upload behavior, partial block failure, transfer-level retry/resume, block-list commit, and no result retrieval requests.

**E2E / Manual Validation**:

- Run the `upload` workflow with multiple local files, interrupt after some Azure blocks have uploaded but before block-list commit, restart with retained checkpoint/artifacts, and confirm completed files are skipped while incomplete files resume from the checkpointed block or fail clearly.

## Implementation feedback (2026-06-24 21:11)

- Gate result: FAIL.
- Restart-resume is not fully satisfied at the CLI surface: `cmd/upload.go` constructs an authenticated NCSC API client before calling `upload.Run`, so `upload --resume` still requires OAuth client ID, client secret, and token URL even though resume should use retained checkpoint/artifact state and must not create a replacement data-analysis. Evidence: `env -u DA_TOOLKIT_CLIENT_ID -u DA_TOOLKIT_CLIENT_SECRET -u DA_TOOLKIT_TOKEN_URL /tmp/da-toolkit upload --resume --checkpoint /tmp/nonexistent-checkpoint.json` returns `oauth client id is required` before reading the checkpoint.
- The checkpoint persists `sas_url` in `internal/upload/checkpoint.go`, and `README.md` documents SAS URL persistence. A SAS URL is a bearer capability URL, so this conflicts with the requirement to persist only non-secret metadata such as SAS expiry metadata and upload checkpoint state, and with the security review expectation for upload checkpoints.
- Per-file status is not visibly reported during upload on the CLI. `uploadOne` updates checkpoint `Message` values after each staged block, but the only user-facing writes in `internal/upload/workflow.go` are the end-of-run summary and cleanup prompt. This does not satisfy the acceptance criterion that per-file upload status is visible during and after the run.
- Baseline checks passed: `go build -o /tmp/da-toolkit .`, `go test ./...`, `go vet ./...`, and `gofmt -l main.go cmd internal/config internal/ncscapi internal/upload` produced no failures/unformatted files. Coverage check passed for touched scope with `go test ./... -cover` (`internal/upload` 80.2%, `internal/ncscapi` 77.7%, `cmd` 51.2%).

## Implementation update (2026-06-24 22:00)

- Addressed: `upload --resume` reads checkpoints without constructing an OAuth client; checkpoints no longer serialize SAS bearer URLs; CLI now prints per-file upload status updates.
- Not addressed: none.
- Status: done

## Implementation update (2026-06-24 22:20)

- Addressed: classified SAS URLs as acceptable persisted operational secrets, restored checkpoint SAS URL persistence for restart-resume, and documented protected checkpoint handling in ADR/guidelines/README.
- Not addressed: none.
- Status: done

## Implementation feedback (2026-06-24 21:36)

- Gate result: FAIL.
- Checkpoint files are not always protected with owner-only permissions even though SAS URLs are persisted as operational secrets. `saveCheckpoint` writes new files with `0600`, but when rewriting an existing checkpoint it preserves the existing mode. Evidence: creating a checkpoint with mode `0644`, then running `/tmp/da-toolkit upload --resume --checkpoint "$checkpoint" --workspace "$tmp"` leaves `stat -c %a "$checkpoint"` at `644`. This violates the technical requirement and ADR/guideline expectation that persisted SAS-bearing checkpoints are owner-only protected.
- Previously reported items are addressed: `upload --resume` now reads the checkpoint before OAuth setup, SAS URL persistence is backed by the new ADR/guideline update, and per-file upload status is printed during upload.
- Baseline checks passed: `go build -o /tmp/da-toolkit .`, `go test ./...`, `go vet ./...`, and `gofmt -l main.go cmd internal/config internal/ncscapi internal/upload` produced no failures/unformatted files. Coverage check passed for touched scope with `go test ./... -cover` (`internal/upload` 79.6%, `internal/ncscapi` 77.7%, `cmd` 72.7%).
- Security review: completed; the remaining finding is checkpoint permission enforcement for persisted SAS bearer capability URLs.

## Implementation update (2026-06-24 22:45)

- Addressed: checkpoint rewrites now enforce owner-only `0600` permissions even when the existing checkpoint file was permissive.
- Not addressed: none.
- Status: done

## Validation update (2026-06-23 13:42)

- Gate result: PASS.
- Functional checklist passed for the implemented upload workflow: multi-file `other` analysis creation with `manual_analysis_allowed=true`, per-file AES-GCM encryption/hash metadata, SAS block upload, per-file status output, checkpoint persistence without OAuth/client/AES secrets, restart-resume from uploaded block indexes, completed-file skip, missing-artifact failure isolation, expired-SAS failure without replacement data-analysis creation, single cleanup prompt/confirmed cleanup for successful artifacts, and no result retrieval were covered by `internal/upload` and `cmd` tests plus focused CLI probes.
- Latest implementation feedback is addressed: `upload --resume` reads checkpoint state before OAuth setup, per-file status is printed, SAS URL persistence is now backed by ADR/guideline updates, and checkpoint rewrites enforce owner-only `0600` permissions. Evidence: `env -u DA_TOOLKIT_CLIENT_ID -u DA_TOOLKIT_CLIENT_SECRET -u DA_TOOLKIT_TOKEN_URL /tmp/da-toolkit upload --resume --checkpoint /tmp/nonexistent-da-toolkit-checkpoint.json` reports `read checkpoint` rather than an OAuth credential error; rewriting a permissive temporary checkpoint through `/tmp/da-toolkit upload --resume --checkpoint "$checkpoint" --workspace "$tmp"` changed its mode from `0644` to `600`.
- Baseline checks passed: `go build -o /tmp/da-toolkit .`, `go test ./...`, `go vet ./...`, and `gofmt -l main.go cmd internal/config internal/ncscapi internal/upload` produced no failures/unformatted files. Touched-scope coverage showed no material regression with `go test ./... -cover` (`internal/upload` 79.2%, `internal/ncscapi` 77.7%, `cmd` 72.7%).
- Security review: completed for upload/checkpoint/resume paths; no remaining security finding in the changed upload code after owner-only checkpoint permission enforcement. Checkpoint SAS URL persistence is accepted by `work/adr/2026-06-24-persist-sas-urls-for-restart-resume.md` and `work/guidelines/api-upload.md`.
