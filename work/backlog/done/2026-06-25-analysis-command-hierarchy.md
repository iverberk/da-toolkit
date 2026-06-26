# Analysis command hierarchy

## Type

feature

## Context

The CLI previously exposed separate top-level creation and retrieval surfaces even though the NCSC API domain object is a data-analysis. Users create, inspect, list, and retrieve artifacts for analyses; uploading local files is part of creating an analysis, and result retrieval is part of working with an existing analysis. The top-level CLI surface should therefore be organized around `analysis` rather than transport-oriented or output-oriented command groups.

## Functional Requirements

- Replace the old top-level analysis creation route with `analysis create`.
- Replace the old top-level retrieval surface with `analysis` subcommands.
- Provide `analysis list` for listing data-analyses.
- Provide `analysis read <analysis-id>` for reading a single data-analysis metadata payload.
- Provide `analysis archive <analysis-id>` for downloading a result archive.
- Provide `analysis file <analysis-id> <result-name>` for downloading a named result file.
- Remove the old create and retrieval top-level commands immediately rather than keeping aliases.
- Keep the existing `acquisition` top-level command unchanged.
- Keep existing upload/create flags and result retrieval flags unless renamed command placement requires help text updates.

## Technical Requirements

- Implement a new Cobra top-level `analysis` command with subcommands `create`, `list`, `read`, `archive`, and `file`.
- Move current analysis upload behavior under `analysis create` without changing upload engine semantics.
- Move current retrieval behavior under the corresponding `analysis` subcommands.
- Remove old root command registrations for `upload` and retrieval; do not add hidden or deprecated aliases.
- Keep `analysis read` metadata-only for now; do not add result summary formatting beyond the existing API payload behavior in this story.
- Preserve memory-only handling for OAuth client secrets, OAuth tokens, AES keys, zip passwords, and disk encryption keys.
- Preserve existing archive/file output-path and overwrite behavior.
- Preserve the existing rule that create/upload workflows stop after successful upload and do not retrieve results.
- Update root/help text and command tests to reflect the `analysis` command hierarchy.
- Follow `work/guidelines/cli-workflows.md`, `work/guidelines/api-upload.md`, and `work/guidelines/go.md`.
- Decision: use `analysis` as the only top-level data-analysis API command because it matches the API/domain object.
- Decision: use `analysis archive` and `analysis file` for retrieval commands because the names are concise and match the requested retrieval artifacts.
- Rejected alternative: keep the old create and retrieval surfaces as deprecated aliases; rejected because the desired CLI cleanup removes old commands immediately.
- Rejected alternative: use `analysis result archive/file`; rejected because the extra hierarchy is unnecessary for the current surface.

## Acceptance Criteria

- `da-toolkit analysis create [flags] <file> [file...]` creates an analysis and uploads local files using the existing supported analysis types.
- `da-toolkit analysis list` lists analyses with the existing pagination/filter/search options.
- `da-toolkit analysis read <analysis-id>` outputs the analysis metadata payload only.
- `da-toolkit analysis archive <analysis-id> --output <path>` downloads the result archive with existing overwrite protection.
- `da-toolkit analysis file <analysis-id> <result-name> --output <path>` downloads a named result file with existing overwrite protection.
- The former top-level create route is not available.
- The former top-level retrieval route is not available.
- Root help lists `analysis` and `acquisition`, and no longer lists the former create or retrieval routes.
- Existing secret-handling, checkpoint, cleanup, and no-result-retrieval-during-create guarantees are preserved.

## Analysis

### Likely Impact

- Primary implementation lane: Cobra command wiring in `cmd/` -> existing upload workflow in `internal/upload` and NCSC API wrapper in `internal/ncscapi` -> command tests.
- `cmd/upload.go` - current `analysisCreateCmd` already owns `create analysis + upload local files` behavior, flags, validation, secret env lookup, default workspace, and calls `upload.Run`; this should move under `analysis create` without changing `internal/upload` semantics.
- `cmd/analysis.go` - owns `list/read/archive/file` subcommands and result download helpers under `analysis list/read/archive/file`.
- `cmd/root.go` - root long/help text should describe the analysis-centered workflow.
- `cmd/root_test.go` and `cmd/analysis_test.go` - tests assert old top-level surfaces are unavailable and execute `analysis ...` paths.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` reset helpers - reset `analysisCreateCmd` and `analysisCmd` flags directly; keep reset coverage for `analysis` and its subcommands so tests remain isolated.
- `cmd/analysis.go` helper/type names - use analysis-oriented names where they describe command wiring; result-specific names remain appropriate for downloaded result artifacts and metadata.
- `cmd/upload.go` helper names/default workspace - `defaultUploadWorkspace` and parse helpers are upload-engine-specific and can remain if only command placement changes; update only if help text or tests require `create` terminology.

### Existing Patterns / Prior Art

- `cmd/analysis.go` - parent command with subcommands and persistent auth flags: `analysisCmd.AddCommand(analysisCreateCmd, analysisListCmd, analysisReadCmd, analysisArchiveCmd, analysisFileCmd)` plus root registration.
- `cmd/upload.go` - closest pattern for create-analysis upload behavior: argument validation before credentials, memory-only secret inputs, `ncscapi.NewClient`, and `upload.Run` option wiring.
- `cmd/acquisition.go` - existing top-level command registration pattern that must remain unchanged: it adds child commands and registers only `acquisitionCmd` on `rootCmd`.
- `internal/ncscapi/client.go` - API wrapper already exposes `CreateAnalysis`, `ListAnalyses`, `ReadAnalysis`, `GetResultArchive`, and `GetResultFile`; no new generated API calls appear needed for this hierarchy change.

### Layer Boundaries

- Touch first: Cobra command definitions/registration in `cmd/`, root/help text, and command tests.
- Touch only if tests expose a gap: command helper names and test reset/fake API wiring in `cmd/`.
- Avoid unless evidence emerges: `internal/upload` workflow, checkpoint/resume/encryption behavior, generated OpenAPI bindings under `internal/ncscapi/generated`, OAuth/token handling in `internal/ncscapi`, and `internal/acquisition`.

### Verification Plan

**Unit Tests**:

- Cover `analysis create` missing-file/resume validation using the existing analysis creation checks.
- Cover `analysis list/read/archive/file` by updating existing command tests to execute through the new hierarchy while preserving pagination, metadata JSON, availability checks, and overwrite protection.
- Cover old top-level routes returning unknown command, while root help lists `analysis` and `acquisition` only for this domain.

**E2E / Manual Validation**:

- Manually inspect `da-toolkit --help` and `da-toolkit analysis --help` to confirm the hierarchy and help text match the story.

## Implementation notes (2026-06-25 14:58)

- Added `analysis` as the data-analysis command hierarchy and moved create/list/read/archive/file behavior beneath it.
- Removed root registration for old create and retrieval commands; root help now lists `analysis` and `acquisition`.
- Updated command tests to execute `analysis create`, `analysis list`, `analysis read`, `analysis archive`, and `analysis file`, including old-command removal checks.
- Verified with `gofmt -w main.go cmd internal/config`, `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`, `/tmp/da-toolkit --help`, `/tmp/da-toolkit analysis --help`, and `/tmp/da-toolkit analysis create --help`.

## Validation update (2026-06-25 15:01)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`.
* Touched-scope coverage: no material regression observed; `go test ./cmd -cover` reported 59.7% statement coverage for the changed command package.
* Security review: completed for CLI/API credential and upload/result retrieval paths; memory-only secret handling and overwrite protections remain covered by focused tests and diff review.
* Retained exploratory artifacts: none; CLI/API-level validation only, no browser/UI proof required.
* Validated checklist items: `analysis create` placement and validation, `analysis list/read/archive/file` behavior, old top-level route removal, root and analysis help hierarchy, acquisition command retention, no result retrieval during create, existing output overwrite protection, and guideline compliance for CLI/Cobra/API-upload changes.
* Providers covered: NCSC Data Analysis API command surface with fake/test API coverage; no alternate provider dimension configured.
