# oapi-codegen NCSC API client generation

Date: 2026-06-23
Status: Accepted

## Context

The toolkit must interact with the NCSC Data Analysis API from Go using OAuth client credentials. The API client needs to stay aligned with the NCSC API contract as it changes over time, while toolkit-owned behavior such as OAuth handling, memory-only secret handling, error normalization, and stable application call sites must remain under project control. The canonical NCSC OpenAPI specification is available at `https://api.ncsc.nl/data-analysis/openapi.json`.

## Decision

Use `oapi-codegen` to generate Go code for interacting with the NCSC Data Analysis API from the canonical OpenAPI specification at `https://api.ncsc.nl/data-analysis/openapi.json`. Pin and document the `oapi-codegen` version and generation command used by the project. Keep the generated API binding behind toolkit-owned wrappers that handle OAuth client credentials, bearer-token injection, memory-only token handling, error normalization, and stable application-level methods.

## Consequences

NCSC API contract changes can be incorporated by intentionally refreshing the generated client from the canonical OpenAPI specification and reviewing the resulting diff. The project must maintain a reproducible code-generation setup and avoid hand-rolling request/response structs or endpoint calls when they can be generated from the OpenAPI contract. Tests should focus on the toolkit-owned wrapper behavior and integration boundaries rather than treating generated code as product logic. Any future API-client story must preserve the separation between generated transport/spec code and toolkit-owned OAuth, secret-handling, and error behavior.
