# Analysis command hierarchy

## Type

feature

## Context

The CLI currently exposes `upload` and `result` as separate top-level commands even though the NCSC API domain object is a data-analysis. Users create, inspect, list, and retrieve artifacts for analyses; uploading local files is part of creating an analysis, and result retrieval is part of working with an existing analysis. The top-level CLI surface should therefore be organized around `analysis` rather than transport-oriented or output-oriented command groups.

## Functional Requirements

- Replace the old top-level `upload` command with `analysis create`.
- Replace the old top-level `result` command with `analysis` subcommands.
- Provide `analysis list` for listing data-analyses.
- Provide `analysis read <analysis-id>` for reading a single data-analysis metadata payload.
- Provide `analysis archive <analysis-id>` for downloading a result archive.
- Provide `analysis file <analysis-id> <result-name>` for downloading a named result file.
- Remove the old `upload` and `result` top-level commands immediately rather than keeping aliases.
- Keep the existing `acquisition` top-level command unchanged.
- Keep existing upload/create flags and result retrieval flags unless renamed command placement requires help text updates.

## Technical Requirements

- Implement a new Cobra top-level `analysis` command with subcommands `create`, `list`, `read`, `archive`, and `file`.
- Move current `upload` command behavior under `analysis create` without changing upload engine semantics.
- Move current `result list`, `result read`, `result archive`, and `result file` behavior under the corresponding `analysis` subcommands.
- Remove old root command registrations for `upload` and `result`; do not add hidden or deprecated aliases.
- Keep `analysis read` metadata-only for now; do not add result summary formatting beyond the existing API payload behavior in this story.
- Preserve memory-only handling for OAuth client secrets, OAuth tokens, AES keys, zip passwords, and disk encryption keys.
- Preserve existing archive/file output-path and overwrite behavior.
- Preserve the existing rule that create/upload workflows stop after successful upload and do not retrieve results.
- Update root/help text and command tests to reflect the `analysis` command hierarchy.
- Follow `work/guidelines/cli-workflows.md`, `work/guidelines/api-upload.md`, and `work/guidelines/go.md`.
- Decision: use `analysis` as the only top-level data-analysis API command because it matches the API/domain object.
- Decision: use `analysis archive` and `analysis file` for retrieval commands because the names are concise and match the requested retrieval artifacts.
- Rejected alternative: keep `upload` and `result` as deprecated aliases; rejected because the desired CLI cleanup removes old commands immediately.
- Rejected alternative: use `analysis result archive/file`; rejected because the extra hierarchy is unnecessary for the current surface.

## Acceptance Criteria

- `da-toolkit analysis create [flags] <file> [file...]` creates an analysis and uploads local files using the existing supported analysis types.
- `da-toolkit analysis list` lists analyses with the existing pagination/filter/search options.
- `da-toolkit analysis read <analysis-id>` outputs the analysis metadata payload only.
- `da-toolkit analysis archive <analysis-id> --output <path>` downloads the result archive with existing overwrite protection.
- `da-toolkit analysis file <analysis-id> <result-name> --output <path>` downloads a named result file with existing overwrite protection.
- `da-toolkit upload` is not available.
- `da-toolkit result` is not available.
- Root help lists `analysis` and `acquisition`, and no longer lists `upload` or `result`.
- Existing secret-handling, checkpoint, cleanup, and no-result-retrieval-during-create guarantees are preserved.
