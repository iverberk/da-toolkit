# Wait for Analysis and Download Results

## Type

feature

## Context

Users can currently read a data-analysis metadata payload once with `analysis read <analysis-id>`, download a completed analysis archive with `analysis archive`, or download a named result file with `analysis file`.

Operators need a way to wait for a data-analysis to complete while respecting API rate-limit guidance. When requested, available result files should be downloaded as soon as they appear in metadata instead of waiting for the whole analysis to complete and then downloading an archive.

## Functional Requirements

- Add `--wait` to `analysis read <analysis-id>`.
- `analysis read <analysis-id> --wait` must poll indefinitely until the data-analysis metadata reports `status: "completed"`, unless the user interrupts the command.
- Polling must respect the `Retry-After` response header from the metadata read API response when it is present.
- `analysis read <analysis-id> --wait` without `--download` must print the final metadata JSON after completion.
- Add `--download` to `analysis read <analysis-id>` for use only with `--wait`.
- `analysis read <analysis-id> --wait --download` must download newly observed result files while waiting continues.
- Download mode must use each result name as the local path exactly, including path separators, and create parent directories as needed.
- If a target result file already exists locally, skip it and count it as complete permanently for the current wait session.
- Failed result downloads must be reported and retried on later polls.
- Download mode must continue retrying observed failed downloads after the analysis reaches `completed` until every observed result has been downloaded or skipped, unless the user interrupts the command.
- Download mode output must use stdout and include progress while waiting plus a final summary only; it must not print the final metadata JSON.
- `--download` without `--wait` must fail with helpful guidance that `--download` is only available while waiting and that `analysis archive` and `analysis file` can download results from a completed analysis.
- Preserve the existing `analysis archive` and `analysis file` command behavior.

## Technical Requirements

- Keep generated NCSC API bindings behind toolkit-owned wrappers; do not expose generated response types directly from the command layer.
- Extend the toolkit API wrapper used by `analysis read` so successful metadata reads expose the response metadata needed for wait behavior, including `Retry-After`.
- Treat `Retry-After` as authoritative for the next metadata poll delay when present; define a conservative fallback interval for cases where the header is absent or unusable.
- Determine completion from the data-analysis-level `status` field, not from file-level statuses.
- Track per-session result state so successfully downloaded and skipped-existing results are not retried.
- Treat failed downloads as pending so they are retried on later polls.
- Use the existing named-result retrieval logic/API path for result downloads rather than archive retrieval.
- Create local parent directories before writing result files whose names include path separators.
- Preserve local overwrite protection by skipping existing files rather than replacing them.
- Keep OAuth client secrets and access tokens memory-only; do not persist credentials, tokens, or wait/download state for this feature.
- Rejected alternative: download the archive after completion; rejected because users should receive individual results as soon as they become available.
- Rejected alternative: wait for all file sections to report `status: "completed"`; rejected in favor of the data-analysis-level `status: "completed"` completion rule.
- Rejected alternative: abort waiting on a failed result download; rejected because transient download failures should be retried while waiting continues.
- Rejected alternative: sanitize result names down to basenames; rejected because result names may intentionally include subdirectories.
- Rejected alternative: print final metadata JSON in download mode; rejected because download mode should provide progress and a final summary only.

## Acceptance Criteria

- Running `analysis read <analysis-id> --wait` polls until the metadata reports `status: "completed"` and then prints the completed metadata JSON.
- Polling uses the `Retry-After` header from metadata read responses when present.
- Running `analysis read <analysis-id> --wait --download` downloads each newly observed result file before overall completion when the result appears in metadata.
- Downloaded files are written using result names as paths, including creation of parent directories.
- Existing local result paths are skipped, reported, and considered complete for the rest of the wait session.
- Failed result downloads are reported and retried on later polls.
- If the analysis reaches `completed` while observed result downloads are still failing, the command keeps retrying those downloads until they succeed or are skipped, unless interrupted.
- Download mode prints progress and a final summary to stdout and does not print metadata JSON.
- Running `analysis read <analysis-id> --download` without `--wait` fails with guidance mentioning `analysis archive` and `analysis file` for completed-analysis result retrieval.
- Existing `analysis read` without `--wait`, `analysis archive`, and `analysis file` behaviors remain unchanged.

## Analysis

### Likely Impact

- Primary implementation lane: Cobra `analysis read` flags/wait orchestration in `cmd/analysis.go` -> toolkit-owned `internal/ncscapi.Client` read wrapper exposing response metadata -> existing named-result download path via `GetResultFile` -> local file write/skip helpers.
- `cmd/analysis.go` - owns `analysisReadCmd`, `analysisFlags`, command API interface, metadata-to-result mapping, `hasResultNamed`, and `writeResultFile`; this is the first command surface to extend for `--wait`, `--download`, progress output, per-session result state, and skip-existing behavior.
- `internal/ncscapi/client.go` - current `ReadAnalysis` returns only `*generated.DataAnalysisResource` and drops `resp.HTTPResponse`; add a toolkit-owned response/metadata wrapper or companion method so command code can use `Retry-After` without exposing generated response types.
- `internal/ncscapi/generated/client.gen.go` - confirms `ReadDataAnalysis...Response` carries `HTTPResponse` and `DataAnalysisResource.Status`, while `DataAnalysisResource.Files[].Results[].Name` carries result names; use as read-only evidence, not an edit target.

### Possible Adjacent Touchpoints

- `cmd/analysis_test.go` - existing fake `analysisAPI`, read/download tests, and command fixtures will need extension for repeated reads, retry-after behavior, failed result retries, existing-file skips, and `--download` without `--wait` guidance.
- `cmd/root_test.go` - `resetAnalysisCommandFlags` must include any new `analysisFlags` fields and reset new `analysis read` flags.
- `internal/ncscapi/client_test.go` - likely needs a focused wrapper test proving successful metadata reads preserve `Retry-After` while still returning normalized errors and without leaking secrets.
- Inference: `writeResultFile` currently opens the exact output path and does not create parent directories; download-wait mode likely needs a sibling helper or option that creates parents and treats `os.ErrExist` as a permanent skip instead of an error.

### Existing Patterns / Prior Art

- `cmd/analysis.go` `analysisArchiveCmd`/`analysisFileCmd` - closest result download pattern: read metadata first, verify named result availability, call `GetResultFile`/`GetResultArchive`, then write bytes with overwrite protection.
- `cmd/analysis_test.go` - established command-level fake API pattern (`withFakeAnalysisAPI`, `fakeAnalysisAPI`) and assertions for metadata JSON output, not-found reporting, and overwrite protection.
- `internal/ncscapi/client.go` `GetResultFile` - existing named-result retrieval wrapper required by the story; avoid archive retrieval for wait-download mode.
- `work/backlog/done/2026-06-22-04-cli-result-retrieval-commands.md` - prior retrieval scope and validation notes emphasize wrapper reuse, memory-only OAuth handling, and generated-client boundaries.

### Layer Boundaries

- Touch first: `cmd/analysis.go` command flags/orchestration and tests; `internal/ncscapi/client.go` only for a stable toolkit-owned read response that includes header metadata.
- Touch if needed: small file-writing helper in `cmd/analysis.go` or a narrow internal helper if wait/download logic grows beyond Cobra orchestration; keep behavior aligned with existing `analysis file` output protections.
- Avoid unless evidence emerges: `internal/analysis` upload/checkpoint workflow packages; this feature must not persist wait/download state or alter upload resume semantics.
- Avoid manual edits: `internal/ncscapi/generated/client.gen.go`; generated bindings are read-only unless regenerated from OpenAPI.
- Avoid widening: `analysis archive` and existing `analysis file` behavior except shared helper refactors that preserve current overwrite semantics.

### Verification Plan

**Unit Tests**:

- Cover `analysis read --wait` polling through multiple metadata states, honoring `Retry-After` when present and using a fallback when absent/unusable.
- Cover final output split: wait-only prints completed metadata JSON; wait-download prints progress/final summary and no metadata JSON.
- Cover `--download` without `--wait` failing before API construction with guidance mentioning `analysis archive` and `analysis file`.
- Cover wait-download result state: newly observed results download once, existing local paths are skipped permanently for the session, failed downloads remain pending and retry on later polls, and parent directories are created for result names with separators.
- Cover completed-analysis-with-pending-failed-download behavior: command continues retrying observed pending results after analysis status becomes `completed` until all are downloaded or skipped.

**Integration Tests**:

- Use fake OAuth/API HTTP servers to verify `Retry-After` is captured from real metadata read responses through the `internal/ncscapi` wrapper and that named result downloads still use authenticated `GetResultFile` requests without leaking client secrets or bearer tokens.

**E2E / Manual Validation**:

- Manually exercise `analysis read <id> --wait` and `analysis read <id> --wait --download` against a test API if credentials are available, including interruption behavior and result names containing subdirectories.

**Additional Checks (as applicable)**:

- Security/file-system check: confirm result names are used exactly as local paths per story, existing files are never overwritten in wait-download mode, parent directories are created with no credential/token persistence, and no wait/download session state is written to disk.

## Implementation notes (2026-06-26 22:47)

- Added `analysis read --wait` polling with `Retry-After` support and fallback delay.
- Added `analysis read --wait --download` progress output, per-session result tracking, parent directory creation, existing-file skips, and retry of failed observed downloads after completion.
- Added wrapper support for metadata read response headers without exposing generated response types to command code.
- Verified with `gofmt`, `go test ./...`, `go vet ./...`, and `scripts/build-binary.sh /tmp/da-toolkit`.

## Validation update (2026-06-26 22:50)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -w main.go cmd internal/config`, `go test ./...`, `go vet ./...`, and `scripts/build-binary.sh /tmp/da-toolkit`.
* Touched-scope coverage: no material regression observed; `go test -cover ./cmd ./internal/ncscapi` reported cmd 56.3% and internal/ncscapi 79.2%.
* Security review: completed for OAuth/token memory-only handling, toolkit-owned API wrapper boundaries, result file overwrite protection, parent directory creation, and exact result-name path behavior required by the story.
* Retained exploratory artifacts: none; CLI/API behavior was validated through automated tests and focused code review, with no browser/UI surface required.
* Validated checklist items: wait flag polling until completed, Retry-After/fallback delay selection, final metadata JSON for wait-only, download flag dependency on wait with archive/file guidance, newly observed named-result downloads, parent directory creation, existing file skip/per-session completion, failed download retry including after analysis completion, download-mode progress/final summary without metadata JSON, existing read/archive/file behavior preservation, and internal wrapper exposure of read response metadata.
* Providers covered: NCSC Data Analysis API wrapper with fake HTTP/API test surfaces; no provider variants configured for this CLI story.
