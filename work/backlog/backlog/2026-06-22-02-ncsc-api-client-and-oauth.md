# NCSC API client and OAuth client credentials

## Type

feature

## Context

The toolkit must interact with the NCSC Data Analysis API using OAuth client credentials. Later stories need a tested client for creating analyses, reading analyses, and retrieving results, but upload orchestration is handled separately.
The Go API binding should be generated from the NCSC OpenAPI specification with `oapi-codegen` so the client can be refreshed when the API contract changes.

## Functional Requirements

- Authenticate to the NCSC API using OAuth client credentials.
- Create data-analysis requests for supported API types.
- List data-analyses with pagination inputs.
- Read a single data-analysis by ID.
- Delete a data-analysis by ID.
- Provide client functions for retrieving a result file and a result archive.

## Technical Requirements

- Generate the Go NCSC API binding from the NCSC OpenAPI specification using `oapi-codegen`.
- Pin and document the `oapi-codegen` version and generation command used for the client.
- Keep the OpenAPI specification source traceable and reviewable so future API contract changes can be regenerated intentionally.
- Wrap the generated client with toolkit-owned code for OAuth, memory-only token handling, error normalization, and stable call sites.
- Keep client ID and client secret input out of persistent storage.
- Keep OAuth access tokens in memory only.
- Make the API client testable with mocked HTTP responses.
- Surface API validation errors with actionable messages and trace IDs when present.
- Follow `work/guidelines/api-upload.md` for secret handling.

## Acceptance Criteria

- The generated NCSC API binding is produced by `oapi-codegen` from the documented OpenAPI specification source.
- The documented generation command can be run to refresh the generated client code.
- OAuth token acquisition is covered by mocked tests.
- Create/list/read/delete analysis client methods are covered by mocked tests.
- Result file/archive client methods are covered by mocked tests but are not exposed as final user-facing workflow commands in this story.
- Wrapper-level tests cover OAuth injection, response handling, and error normalization around the generated client.
- API errors are returned without logging secrets.
- No credentials or tokens are written to disk.
