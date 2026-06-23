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

## Analysis

### Likely Impact

- Primary implementation lane: NCSC OpenAPI source/documentation -> `oapi-codegen` generated Go binding -> toolkit-owned internal API/OAuth wrapper -> mocked HTTP tests for OAuth, analysis operations, result retrieval, and error normalization.
- New `internal/` API client package(s) - likely first layer because `work/guidelines/go.md` keeps non-command application logic in `internal/`, and `work/guidelines/api-upload.md` requires generated NCSC bindings to stay behind toolkit-owned wrappers.
- Generated binding and generation docs/config - required because `work/adr/2026-06-23-oapi-codegen-ncsc-api-client-generation.md` records the canonical OpenAPI source as `https://api.ncsc.nl/data-analysis/openapi.json` and requires pinned/documented generation.
- OAuth/token transport wrapper - in scope to acquire client-credentials tokens, inject bearer auth, keep tokens memory-only, and normalize API errors with trace IDs when present.

### Possible Adjacent Touchpoints

- `go.mod` / `go.sum` - may need `oapi-codegen` runtime types and OAuth/client dependencies; currently only Cobra is required.
- `internal/config/config.go` and `internal/config/config_test.go` - likely should remain mostly unchanged, but tests may add guard coverage if new credential option types risk persistent config use; existing config already rejects secret-like persisted fields.
- `cmd/analysis.go` and `cmd/result.go` - only update if a minimal compile-time dependency or help wording is needed; acceptance criteria explicitly keep result retrieval client functions from becoming final user-facing workflow commands in this story.

### Existing Patterns / Prior Art

- `work/adr/2026-06-23-oapi-codegen-ncsc-api-client-generation.md` - closest architecture pattern: generate from the canonical NCSC OpenAPI spec, then wrap generated code for OAuth, bearer-token injection, memory-only secrets, error normalization, and stable application methods.
- `internal/config/config.go` - existing secret-handling precedent: comments and validation keep API credentials/OAuth tokens out of persistent config.
- `internal/config/config_test.go` - existing test style for memory-only secret policy using temp files and direct assertions.
- `cmd/root_test.go` - existing lightweight Go test pattern around user-facing behavior and no-credential/no-network safety; useful only if command help remains touched.

### Layer Boundaries

- Touch first: new internal NCSC API/generated-client area, generation documentation/spec traceability, OAuth HTTP transport/token handling, wrapper methods, and mocked HTTP tests.
- Avoid unless evidence emerges: upload orchestration, SAS upload/retry/resume mechanics, encryption/hashing, Cobra command workflows, GUI behavior, acquisition integration, and persistent credential/token storage.

### Verification Plan

**Unit Tests**:

- Mock OAuth token endpoint success/failure, confirm client credentials are sent only over HTTP requests and access tokens remain in memory.
- Mock generated-client API calls for create/list/read/delete analyses and result file/archive retrieval through the wrapper.
- Assert bearer-token injection, token reuse/refresh behavior where implemented, response handling, API validation error normalization, trace ID propagation, and no secret values in returned/loggable errors.

**Integration Tests**:

- Regenerate the NCSC binding from the documented OpenAPI source/command in a controlled check and review that generated code stays separated from wrapper code.

**Additional Checks (as applicable)**:

- Inspect docs and source to confirm no client ID, client secret, or OAuth access token is written to config, checkpoint, logs, or generated fixtures.

## Validation update (2026-06-23 13:42)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -l main.go cmd internal/config internal/ncscapi`, `go test ./...`, `go vet ./...`, and `go build -o /tmp/da-toolkit .`.
* Code generation proof passed: `node internal/ncscapi/spec/normalize-for-oapi-codegen.mjs && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -generate types,client -package generated -o /tmp/opencode/ncsc-client.gen.go internal/ncscapi/spec/openapi.oapi-codegen.json && cmp -s /tmp/opencode/ncsc-client.gen.go internal/ncscapi/generated/client.gen.go`.
* Touched-scope coverage: no material regression; `go test ./internal/ncscapi -cover` reported 76.6% statement coverage for the new wrapper package.
* Security review: completed for OAuth credentials, bearer tokens, AES keys, API errors, and disk-write/logging surfaces; no secret persistence or secret-bearing errors found.
* Retained exploratory artifacts: none; no browser/UI proof required for this API-only story.
* Validated checklist items: OAuth client-credentials acquisition; in-memory token reuse and bearer injection; create/list/read/delete analysis wrapper methods; result file/archive retrieval methods; generated binding traceability and reproducibility; documented pinned generator command; API validation error normalization with trace ID; no credential/token disk persistence; mocked HTTP testability; generated-code separation behind toolkit wrapper.
* Providers covered: NCSC Data Analysis API via mocked `httptest` OAuth/API endpoints and generated-client codegen check.
