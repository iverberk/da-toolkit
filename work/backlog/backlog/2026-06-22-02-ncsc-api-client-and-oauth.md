# NCSC API client and OAuth client credentials

## Type

feature

## Context

The toolkit must interact with the NCSC Data Analysis API using OAuth client credentials. Later stories need a tested client for creating analyses, reading analyses, and retrieving results, but upload orchestration is handled separately.

## Functional Requirements

- Authenticate to the NCSC API using OAuth client credentials.
- Create data-analysis requests for supported API types.
- List data-analyses with pagination inputs.
- Read a single data-analysis by ID.
- Delete a data-analysis by ID.
- Provide client functions for retrieving a result file and a result archive.

## Technical Requirements

- Keep client ID and client secret input out of persistent storage.
- Keep OAuth access tokens in memory only.
- Make the API client testable with mocked HTTP responses.
- Surface API validation errors with actionable messages and trace IDs when present.
- Follow `work/guidelines/api-upload.md` for secret handling.

## Acceptance Criteria

- OAuth token acquisition is covered by mocked tests.
- Create/list/read/delete analysis client methods are covered by mocked tests.
- Result file/archive client methods are covered by mocked tests but are not exposed as final user-facing workflow commands in this story.
- API errors are returned without logging secrets.
- No credentials or tokens are written to disk.
