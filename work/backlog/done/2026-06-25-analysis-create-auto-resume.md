# Analysis create auto-resume

## Type

feature

## Context

The upload/create workflow persists checkpoint state so interrupted uploads can resume without creating replacement data-analyses. Today resume is an explicit operator choice. If a user forgets the resume flag while a usable incomplete checkpoint exists, the CLI can accidentally create an unnecessary new data-analysis. The create workflow should infer resumable state safely and require an explicit override for fresh creation when checkpoint state is present.

## Functional Requirements

- Keep resume behavior under `analysis create`.
- Make `analysis create` automatically resume when the resolved checkpoint exists and represents a usable incomplete upload.
- Keep `analysis create --resume` as a strict resume mode that fails if no usable checkpoint exists.
- Add `analysis create --new` as the explicit override to intentionally create a fresh data-analysis.
- Do not delete, archive, overwrite, or invalidate existing checkpoint state automatically when `--new` is used.
- If `--new` would use the same checkpoint or workspace as an existing incomplete checkpoint, fail unless the operator chooses a different `--checkpoint` or `--workspace`.
- If a checkpoint exists but is invalid, corrupt, ambiguous, or unusable, fail clearly rather than falling back to fresh creation.
- If a checkpoint exists but the SAS URL is expired, fail clearly and do not create a replacement data-analysis.
- If no checkpoint exists, `analysis create` creates a new data-analysis as usual.

## Technical Requirements

- Apply the behavior to the `analysis create` command after the command hierarchy story introduces it.
- Resolve the effective workspace and checkpoint path before deciding whether the run is a fresh create or resume.
- Inspect checkpoint state before constructing an authenticated API client when the run will resume, preserving the current no-OAuth-needed-before-checkpoint-read behavior for resume paths.
- Define checkpoint states explicitly enough to distinguish at least: missing checkpoint, usable incomplete checkpoint, completed checkpoint, expired-SAS/unresumable checkpoint, invalid/corrupt checkpoint, and fresh-create-safe target.
- Treat invalid, corrupt, completed, expired, or ambiguous checkpoint state as an explicit user-facing condition; do not silently create a new analysis from those states.
- Preserve owner-only checkpoint permission enforcement and sensitive SAS URL handling.
- Preserve memory-only handling for OAuth client secrets, OAuth tokens, AES keys, zip passwords, and disk encryption keys.
- Preserve ADR decisions forbidding automatic replacement data-analysis creation after SAS expiry.
- Update command help so operators understand auto-resume, strict `--resume`, and fresh-create `--new` behavior.
- Add command/workflow tests for inferred resume, strict resume failure, explicit `--new`, same-checkpoint protection, corrupt checkpoint failure, completed checkpoint handling, expired SAS handling, and no accidental API create call during inferred resume.
- Follow `work/adr/2026-06-22-restart-resumable-upload-semantics.md`, `work/adr/2026-06-24-persist-sas-urls-for-restart-resume.md`, `work/guidelines/cli-workflows.md`, `work/guidelines/api-upload.md`, and `work/guidelines/go.md`.
- Decision: default `analysis create` should auto-resume usable incomplete checkpoints to prevent accidental duplicate data-analyses.
- Decision: keep `--resume` as strict resume mode because it lets operators require resume and fail when checkpoint state is absent.
- Decision: use `--new` as the explicit fresh-create override.
- Decision: `--new` does not delete or overwrite existing checkpoints automatically; operators must choose a different checkpoint/workspace when an incomplete checkpoint exists.
- Rejected alternative: silently create a new data-analysis when checkpoint state is invalid or expired; rejected because it can create duplicate analyses and conflicts with restart-resume ADRs.

## Acceptance Criteria

- With a usable incomplete checkpoint at the resolved checkpoint path, `da-toolkit analysis create ...` resumes that analysis without creating a new data-analysis.
- With no checkpoint at the resolved checkpoint path, `da-toolkit analysis create ...` creates a new data-analysis.
- `da-toolkit analysis create --resume ...` fails clearly when no usable checkpoint exists.
- `da-toolkit analysis create --new ...` creates a fresh data-analysis only when it will not overwrite or invalidate an existing incomplete checkpoint.
- `da-toolkit analysis create --new ...` fails clearly if it targets the same checkpoint/workspace as an existing incomplete checkpoint.
- Corrupt, invalid, ambiguous, completed, or expired checkpoint states produce clear errors and do not trigger fresh analysis creation by default.
- Expired SAS checkpoint handling still fails without creating a replacement data-analysis.
- Resume paths do not require OAuth credentials before reading and validating checkpoint state.
- Tests prove no accidental create-analysis API call occurs during inferred resume or invalid checkpoint handling.

## Analysis

### Likely Impact

- Primary implementation lane: Cobra `analysis create` entrypoint -> `internal/upload.Run` option/state decision -> checkpoint classification in `internal/upload/checkpoint.go` -> upload validation in `internal/upload/upload.go`.
- `cmd/upload.go` - current `create` command owns `--resume`, workspace/checkpoint resolution, help text, and API client construction; it currently skips OAuth client creation only when `--resume` is explicitly set, so auto-resume must classify the checkpoint before constructing `ncscapi.NewClient`.
- `internal/upload/workflow.go` - `Run` currently branches only on `Options.Resume` and `Checkpoint.AnalysisID`; adding `--new`/auto-resume likely needs a clearer run mode or checkpoint-state decision before `prepareCheckpoint` encrypts files or creates an analysis.
- `internal/upload/checkpoint.go` - closest place to add explicit checkpoint state inspection for missing, usable incomplete, completed, corrupt/invalid, expired-SAS/unresumable, and fresh-create-safe targets while preserving owner-only save permissions.
- `internal/upload/upload.go` - existing SAS expiry/missing-artifact checks should stay the enforcement path for upload attempts, but classification may need to reuse or mirror this logic to fail before accidental fresh creation.

### Possible Adjacent Touchpoints

- `cmd/analysis.go` - repo evidence shows `analysisCmd.AddCommand(analysisCreateCmd, ...)`, so `analysisCreateCmd` is already the `analysis create` subcommand; only touch hierarchy wiring if the command hierarchy story has changed this shape.
- `internal/upload/workflow_test.go` - has existing fake `CreateAnalysis` call counters and resume/SAS-expiry tests that directly match the new no-accidental-create requirements.
- `cmd/root_test.go` - may need CLI help/flag assertions for `--new`, strict `--resume`, and updated help text.

### Existing Patterns / Prior Art

- `internal/upload/workflow_test.go` `TestResumeContinuesFromPersistedUploadedBlocksAndCommits` - demonstrates strict resume from persisted checkpoint and continued Azure block upload.
- `internal/upload/workflow_test.go` `TestExpiredSASFailsWithoutCreatingReplacementAnalysis` - already proves expired SAS resume does not call `CreateAnalysis`; extend this pattern to inferred resume and invalid checkpoint cases.
- `internal/upload/checkpoint.go` `loadCheckpoint` / `saveCheckpoint` / `isResumable` - existing checkpoint parsing, permission protection, and resumability checks are the closest prior art, but `isResumable` is currently file-level and not enough to classify whole-checkpoint states.

### Layer Boundaries

- Touch first: Cobra command flags/help and pre-client run-mode decision in `cmd/upload.go`; upload orchestration and checkpoint-state classification in `internal/upload`.
- Avoid unless evidence emerges: generated NCSC API bindings under `internal/ncscapi/generated`, OAuth client internals in `internal/ncscapi`, encryption implementation, analysis retrieval subcommands, acquisition, GUI/packaging layers, and ADR/guideline semantics.
- Inference: preserve the existing API wrapper boundary; this story should not require new NCSC endpoint shapes or generated code.

### Verification Plan

**Unit Tests**:

- Add checkpoint-state tests for missing, usable incomplete, completed, corrupt/invalid, expired SAS, missing artifact/unresumable, and same checkpoint/workspace with `--new`.
- Add workflow tests proving inferred resume uses existing checkpoint data and does not call `CreateAnalysis` or require an API client before checkpoint validation.

**Integration Tests**:

- Add CLI command tests for default auto-resume, strict `--resume` failure when no usable checkpoint exists, explicit `--new`, `--resume`/`--new` conflict handling, and updated help text.

**E2E / Manual Validation**:

- Manually exercise `analysis create`, `analysis create --resume`, and `analysis create --new` against a temp workspace/checkpoint to confirm operator-facing messages for auto-resume, invalid checkpoint, completed checkpoint, and expired SAS are clear.

## Implementation update (2026-06-25 00:00)

- Addressed: auto-resume planning, strict `--resume`, explicit `--new`, checkpoint classification, pre-OAuth resume planning, and workflow/CLI tests.
- Not addressed: manual E2E against a live NCSC API; not required for automated repo verification.
- Status: done

## Implementation feedback (2026-06-25 16:15)

* Gate result: FAIL.
* Ambiguous checkpoint states are not rejected clearly: `internal/upload/checkpoint.go` treats any file status other than `uploaded` or `skipped` as resumable when SAS URL and artifact metadata exist, so an unknown/ambiguous status can enter inferred resume instead of failing as required by Functional Requirement 19, Technical Requirement 29, and Acceptance Criterion 49.
* Validation commands passed: `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`, `gofmt -w main.go cmd internal/config && git diff --check`, focused upload/cmd story tests.
* Security review completed for touched upload/resume scope: no new persistence of OAuth client secrets, OAuth tokens, AES keys, zip passwords, or disk encryption keys observed; checkpoint SAS handling remains owner-only via `saveCheckpoint` permissions.

## Implementation update (2026-06-25 16:17)

- Addressed: unknown/ambiguous checkpoint file statuses now fail as invalid/corrupt instead of auto-resuming; added default and strict-resume tests proving no create-analysis call.
- Not addressed: live NCSC manual E2E; not required for automated repo verification.
- Status: done

## Validation update (2026-06-25 16:20)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -w main.go cmd internal/config && git diff --check`, focused story tests, `go test ./...`, `go vet ./...`, and `go build -o /tmp/da-toolkit .`.
* Touched-scope coverage: no material regression observed; `go test ./cmd ./internal/upload -cover` reported `cmd` 58.6% and `internal/upload` 78.7%.
* Security review: completed for CLI/upload/checkpoint changes; no persistence of OAuth client secrets, OAuth tokens, AES keys, zip passwords, or disk encryption keys observed, SAS URLs remain limited to protected checkpoints, and inferred resume plans before API client construction.
* Retained exploratory artifacts: none; no browser/UI flow required for this CLI/API workflow story.
* Validated checklist items: default create auto-resumes usable incomplete checkpoint without `CreateAnalysis`; missing checkpoint creates via normal create path; strict `--resume` fails when no usable checkpoint exists; `--new` is explicit and protected from existing checkpoint overwrite; corrupt/invalid/unknown, completed, missing-artifact, and expired-SAS checkpoints fail clearly without fresh creation; resume planning occurs before OAuth credential validation; help documents auto-resume, strict `--resume`, and `--new`; `--resume`/`--new` conflict is rejected.
* Providers covered: local CLI/upload workflow with fake NCSC API creator and local SAS upload test server; no external provider variant required.
