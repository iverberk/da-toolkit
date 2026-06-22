# End-to-end CLI forensic workflow

## Type

feature

## Context

After upload, result retrieval, and acquire packaging are validated separately, the toolkit should provide a coordinated CLI workflow for forensic acquisition and upload.

## Functional Requirements

- Run Dissect/acquire exactly to acquire live-system forensic artifacts.
- Create a `forensic` data-analysis for the acquired artifact or artifacts.
- Collect required requested analyses and optional forensic fields.
- Encrypt, hash, and upload acquired artifacts using the existing upload engine.
- Report per-file progress and partial progress.
- Stop at successful upload and show the data-analysis ID for later result retrieval commands.

## Technical Requirements

- Reuse the embedded acquire approach accepted after the PoC.
- Reuse the forensic upload behavior from the forensic upload story.
- Preserve memory-only handling for API credentials, OAuth tokens, AES keys, disk encryption keys, and other sensitive inputs.
- Persist only non-secret resume metadata.
- Follow `work/adr/2026-06-22-cli-first-v1-product-scope.md`, `work/adr/2026-06-22-restart-resumable-upload-semantics.md`, and the embedded packaging ADR after it is accepted.

## Acceptance Criteria

- A user can run a guided forensic CLI workflow that invokes acquire and uploads resulting artifacts.
- The workflow supports restart-resume for upload according to existing upload semantics.
- Administrative/root acquisition failures are reported clearly.
- Successful upload produces a data-analysis ID and explains how to retrieve results separately.
- No secrets are written to disk.
- GUI implementation is not required for this story.
