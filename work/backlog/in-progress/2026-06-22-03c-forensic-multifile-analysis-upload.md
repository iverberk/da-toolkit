# Forensic multi-file analysis upload

## Type

feature

## Context

The primary product workflow is forensic data analysis. After the shared multi-file upload engine exists, the toolkit must support the NCSC `forensic` data-analysis type for one or more pre-existing local files.

## Functional Requirements

- Create one `forensic` data-analysis for one or more pre-existing local files.
- Require `requested_analyses` for each file.
- Support optional per-file `disk_encryption_key` input.
- Support `manual_analysis_allowed` for the analysis.
- Reuse the shared encryption, hashing, SAS upload, checkpoint, resume, partial-progress, summary, and cleanup behavior from the upload engine.
- Stop at successful upload; do not retrieve results.

## Technical Requirements

- Treat `disk_encryption_key` as sensitive input and do not persist it unless a later ADR explicitly allows it.
- Preserve memory-only handling for API credentials, OAuth tokens, AES keys, and sensitive per-file inputs.
- Reuse upload checkpoint semantics from `work/adr/2026-06-22-restart-resumable-upload-semantics.md`.
- Follow `work/guidelines/api-upload.md` and `work/guidelines/cli-workflows.md`.

## Acceptance Criteria

- A user can create a `forensic` analysis with multiple local files.
- Each file requires at least one requested analysis before the create request is sent.
- Optional disk encryption keys are included in the create request when provided.
- Upload behavior matches the shared engine behavior for partial progress, restart-resume, missing artifacts, SAS expiry, and cleanup.
- No secrets or disk encryption keys are written to disk.
- No result retrieval occurs.
