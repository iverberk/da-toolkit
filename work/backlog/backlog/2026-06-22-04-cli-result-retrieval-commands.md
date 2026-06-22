# CLI result retrieval commands

## Type

feature

## Context

Upload stories stop at successful upload. Retrieving data-analysis status and results should be a separate user action built on the API client.

## Functional Requirements

- Provide CLI commands to list data-analyses.
- Provide a CLI command to read one data-analysis by ID.
- Provide CLI commands to download a result archive and a specific result file.
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
