# API Upload Guidelines

## Principles
- Use OAuth client credentials for NCSC API authentication.
- Generate Go code for interacting with the NCSC API from the NCSC OpenAPI specification using `oapi-codegen`.
- Keep generated NCSC API bindings behind toolkit-owned wrappers for OAuth, secret handling, error normalization, and stable application call sites.
- Persist resume metadata needed for restart-resume, including SAS URLs as operational secrets, analysis ID, upload checkpoint, file path, and phase status.
- Require the encrypted artifact to remain on disk for restart-surviving upload resume.
- Fail explicitly when a SAS URL expires before upload completion.
- Make upload retry and resume behavior visible to the user.

## Pitfalls
- Do not hand-roll NCSC API request/response structs or endpoint calls when they can be generated from the OpenAPI specification.
- Do not store client secrets, access tokens, or AES keys.
- Treat checkpoint files as sensitive because they contain SAS bearer capability URLs; protect them with owner-only permissions and document that handling.
- Do not assume an expired SAS URL can be refreshed.
- Do not hide partial-upload state from the operator.

## Decisions
- `oapi-codegen` is the mandated generator for Go code that interacts with the NCSC API.
- Do not automatically create a new data-analysis after SAS expiry.
- Ask the user whether to delete encrypted artifacts after successful upload.
- Persist SAS URLs in checkpoints for restart-resume per `work/adr/2026-06-24-persist-sas-urls-for-restart-resume.md`.
