# Multi-file upload engine for other analysis

## Type

feature

## Context

The first upload workflow should use pre-existing local files and not depend on Dissect/acquire. The `other` analysis type is the initial vehicle for the shared multi-file encryption, hashing, upload, checkpoint, resume, partial-progress, and cleanup behavior.

## Functional Requirements

- Create one `other` data-analysis for one or more pre-existing local files.
- Set `manual_analysis_allowed=true` for `other` analyses.
- Generate an in-memory AES key per file.
- Encrypt and hash each selected file.
- Upload each encrypted artifact through its corresponding returned SAS URL.
- Treat partial progress as valid and report per-file status.
- Support restart-resume for incomplete files when encrypted artifacts and checkpoint metadata remain available.
- Stop at successful upload; do not retrieve results.
- Show one end-of-run per-file summary and prompt once to delete all successfully uploaded encrypted artifacts.

## Technical Requirements

- Persist only non-secret metadata such as analysis ID, source path, encrypted artifact path, file hash, upload checkpoint, phase status, and SAS expiry metadata when known.
- Do not persist client secrets, OAuth tokens, or AES keys.
- Do not automatically create a replacement data-analysis after SAS expiry.
- Do not re-upload files already marked complete during resume.
- Retain failed or incomplete encrypted artifacts for resume.
- Follow `work/adr/2026-06-22-restart-resumable-upload-semantics.md` and `work/guidelines/api-upload.md`.

## Acceptance Criteria

- A user can create an `other` analysis with multiple local files and upload all encrypted artifacts.
- Per-file upload status is visible during and after the run.
- Restart-resume skips completed files and resumes incomplete files.
- Missing encrypted artifact fails only the affected file with a clear remediation message.
- Expired SAS URL fails affected file uploads without creating a new data-analysis.
- End-of-run summary lists each file, status, encrypted artifact path, and resumable/non-resumable state.
- Cleanup prompt appears once and deletes only encrypted artifacts for successfully uploaded files when confirmed.
- No result retrieval occurs.
