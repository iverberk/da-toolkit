# CLI analysis retrieval subcommands

## Type

feature

## Context

Upload stories stop at successful upload. Retrieving data-analysis status and results should be a separate user action built on the API client.

## Functional Requirements

- Provide CLI commands to list data-analyses.
- Provide a CLI command to read one data-analysis by ID.
- Provide CLI subcommands to download a result archive and a specific result file.
- Let users choose output paths for downloaded results.

## Technical Requirements

- Reuse the NCSC API client from the API story.
- Keep OAuth credentials and tokens in memory only.
- Do not assume an analysis is complete unless the API response indicates results are available.
- Avoid overwriting local files unless the user explicitly confirms or passes an explicit overwrite option.
- Follow `work/guidelines/cli-workflows.md` and `work/guidelines/api-upload.md`.

## Acceptance Criteria

- A user can list analyses with pagination inputs.
- A user can inspect one analysis by ID and see file/result metadata returned by the API.
- A user can download a result archive to a selected path.
- A user can download a named result file to a selected path.
- Missing results and not-found responses are reported clearly.
- No credentials or tokens are written to disk.

## Analysis

### Likely Impact

- Primary implementation lane: Cobra `analysis` retrieval subcommands -> `internal/ncscapi.Client` list/read/result methods -> local output-file write/overwrite safeguards.
- `cmd/analysis.go` - command surface for list, read, archive download, and named-file download.
- `cmd/upload.go` - closest existing credential flag wiring and in-memory OAuth client construction pattern (`--base-url`, `--token-url`, `--client-id`, `--client-secret`, env fallbacks) to reuse for analysis commands.
- `internal/ncscapi/client.go` - already exposes `ListAnalyses`, `ReadAnalysis`, `GetResultArchive`, and `GetResultFile`, so command implementation should call these wrappers rather than adding generated-client calls.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` or new analysis command tests - existing command execution helper/reset pattern covers Cobra assertions; analysis flags may need their own reset helper if package-level flags are used.
- `internal/ncscapi/error.go` - existing API error normalization already covers status and auth guidance; update only if result-specific missing/not-found wording needs clearer user-facing messages.
- `internal/ncscapi/generated/client.gen.go` - read-only generated prior art confirms result metadata fields and endpoints, but should not be manually edited.

### Existing Patterns / Prior Art

- `cmd/upload.go` - established pattern for constructing `ncscapi.NewClient` from flags/env without persisting credentials or tokens.
- `internal/ncscapi/client_test.go` - tests prove OAuth token reuse is in memory and cover list/read/archive/file result wrappers with fake HTTP endpoints.
- `internal/ncscapi/generated/client.gen.go` - `DataAnalysisResource.Files[].Results` exposes result metadata (`name`, `size`), and generated result responses return raw bodies for archive/file downloads.
- `cmd/root_test.go` - command help, placeholder, and argument-validation patterns for CLI behavior tests.

### Layer Boundaries

- Touch first: `cmd/analysis.go` and command tests; use `internal/ncscapi.Client` as the API boundary.
- Touch only if evidence emerges: `internal/ncscapi/client.go` wrapper behavior, limited to small helper additions or clearer result/not-found handling.
- Avoid unless regenerated from OpenAPI: `internal/ncscapi/generated/client.gen.go`.
- Avoid widening into upload checkpoint/resume logic: result retrieval is separate from upload workflows, and existing upload tests explicitly reject result retrieval during upload.
- Avoid `internal/config/` for secrets: project config and guidelines require OAuth credentials/tokens to remain memory-only.

### Verification Plan

**Unit Tests**:

- Add CLI command tests for list pagination flag mapping, read-by-ID output including file/result metadata, invalid/missing ID validation, and no credential requirement before pure argument validation fails.
- Add download-path tests that prove existing files are not overwritten unless an explicit overwrite/confirmation path is used.

**Integration Tests**:

- Use fake HTTP token/API servers to exercise `analysis` subcommands through Cobra, including OAuth header use without leaking client secret or bearer token in errors/output.

**E2E / Manual Validation**:

- Manually run analysis list/read/archive/file commands against a test API environment if credentials are available, including incomplete/no-results and not-found analyses.

**Additional Checks (as applicable)**:

- Verify downloaded archive/file bytes are written exactly to the selected output path and parent-directory errors are reported clearly.

## Implementation feedback (2026-06-25 09:30)

* Gate result: FAIL.
* Add story-required verification for not-found analysis responses through the analysis command surface; current tests do not exercise a 404/not-found read or download path even though acceptance requires clear not-found reporting.
* Add story-required verification for archive download when analysis metadata reports no available results; current tests cover missing named file metadata but not the archive/no-results path from the verification plan.

## Implementation update (2026-06-25 09:30)

- Addressed: Added analysis command tests for 404/not-found read and archive download errors, plus archive download when metadata reports no available results.
- Not addressed: None.
- Status: done

## Validation update (2026-06-25 09:51)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -w main.go cmd internal/config internal/ncscapi`, `go test ./cmd ./internal/ncscapi`, `go build -o /tmp/da-toolkit .`, `go test ./...`, and `go vet ./...`.
* Touched-scope coverage: no material regression observed (`go test ./cmd ./internal/ncscapi -cover` reported `cmd` 60.5% and `internal/ncscapi` 80.2%).
* Security review: completed for OAuth credential/token handling and result-file writes; no client secrets or bearer tokens are persisted, and downloads use explicit output paths with `0600` file mode plus overwrite protection unless `--overwrite` is passed.
* Retained exploratory artifacts: not applicable; no UI/browser validation required for this CLI/API story.
* Validated checklist items: list analyses with pagination; read analysis by ID with file/result metadata; download archive to selected path; download named result file to selected path; block downloads when API metadata reports no available matching results; clear not-found reporting for read/download paths; no credential requirement before argument validation; no credential/token leakage in CLI output; reuse of `internal/ncscapi.Client` wrapper; no completion assumption before result metadata is available; local overwrite guard.
* Providers covered: NCSC API via fake HTTP OAuth/API servers and command-level fakes.
