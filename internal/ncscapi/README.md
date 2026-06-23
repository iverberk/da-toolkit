# NCSC Data Analysis API client generation

- Source OpenAPI specification: `https://api.ncsc.nl/data-analysis/openapi.json`.
- Reviewable source copy: `internal/ncscapi/spec/openapi.json`.
- oapi-codegen-compatible copy: `internal/ncscapi/spec/openapi.oapi-codegen.json`.
- Generator: `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0`.

`oapi-codegen` does not fully support OpenAPI 3.1 nullable/const schema forms yet, so the compatible copy preserves the NCSC contract while translating nullable unions to OpenAPI 3.0 `nullable` and `const` to one-value `enum` for generation.

Refresh command:

```sh
curl -fsSL "https://api.ncsc.nl/data-analysis/openapi.json" -o internal/ncscapi/spec/openapi.json
node internal/ncscapi/spec/normalize-for-oapi-codegen.mjs
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 \
  -generate types,client \
  -package generated \
  -o internal/ncscapi/generated/client.gen.go \
  internal/ncscapi/spec/openapi.oapi-codegen.json
```

- Keep generated code in `internal/ncscapi/generated`.
- Keep OAuth, bearer-token injection, memory-only token caching, and API error normalization in `internal/ncscapi`.
- Do not persist OAuth client credentials, access tokens, AES keys, or per-file passwords.
