# Analysis create auto-resume

## Type

feature

## Context

The upload/create workflow persists checkpoint state so interrupted uploads can resume without creating replacement data-analyses. Today resume is an explicit operator choice. If a user forgets the resume flag while a usable incomplete checkpoint exists, the CLI can accidentally create an unnecessary new data-analysis. The create workflow should infer resumable state safely and require an explicit override for fresh creation when checkpoint state is present.

## Functional Requirements

- Keep resume behavior under `analysis create`.
- Make `analysis create` automatically resume when the resolved checkpoint exists and represents a usable incomplete upload.
- Keep `analysis create --resume` as a strict resume mode that fails if no usable checkpoint exists.
- Add `analysis create --new` as the explicit override to intentionally create a fresh data-analysis.
- Do not delete, archive, overwrite, or invalidate existing checkpoint state automatically when `--new` is used.
- If `--new` would use the same checkpoint or workspace as an existing incomplete checkpoint, fail unless the operator chooses a different `--checkpoint` or `--workspace`.
- If a checkpoint exists but is invalid, corrupt, ambiguous, or unusable, fail clearly rather than falling back to fresh creation.
- If a checkpoint exists but the SAS URL is expired, fail clearly and do not create a replacement data-analysis.
- If no checkpoint exists, `analysis create` creates a new data-analysis as usual.

## Technical Requirements

- Apply the behavior to the `analysis create` command after the command hierarchy story introduces it.
- Resolve the effective workspace and checkpoint path before deciding whether the run is a fresh create or resume.
- Inspect checkpoint state before constructing an authenticated API client when the run will resume, preserving the current no-OAuth-needed-before-checkpoint-read behavior for resume paths.
- Define checkpoint states explicitly enough to distinguish at least: missing checkpoint, usable incomplete checkpoint, completed checkpoint, expired-SAS/unresumable checkpoint, invalid/corrupt checkpoint, and fresh-create-safe target.
- Treat invalid, corrupt, completed, expired, or ambiguous checkpoint state as an explicit user-facing condition; do not silently create a new analysis from those states.
- Preserve owner-only checkpoint permission enforcement and sensitive SAS URL handling.
- Preserve memory-only handling for OAuth client secrets, OAuth tokens, AES keys, zip passwords, and disk encryption keys.
- Preserve ADR decisions forbidding automatic replacement data-analysis creation after SAS expiry.
- Update command help so operators understand auto-resume, strict `--resume`, and fresh-create `--new` behavior.
- Add command/workflow tests for inferred resume, strict resume failure, explicit `--new`, same-checkpoint protection, corrupt checkpoint failure, completed checkpoint handling, expired SAS handling, and no accidental API create call during inferred resume.
- Follow `work/adr/2026-06-22-restart-resumable-upload-semantics.md`, `work/adr/2026-06-24-persist-sas-urls-for-restart-resume.md`, `work/guidelines/cli-workflows.md`, `work/guidelines/api-upload.md`, and `work/guidelines/go.md`.
- Decision: default `analysis create` should auto-resume usable incomplete checkpoints to prevent accidental duplicate data-analyses.
- Decision: keep `--resume` as strict resume mode because it lets operators require resume and fail when checkpoint state is absent.
- Decision: use `--new` as the explicit fresh-create override.
- Decision: `--new` does not delete or overwrite existing checkpoints automatically; operators must choose a different checkpoint/workspace when an incomplete checkpoint exists.
- Rejected alternative: silently create a new data-analysis when checkpoint state is invalid or expired; rejected because it can create duplicate analyses and conflicts with restart-resume ADRs.

## Acceptance Criteria

- With a usable incomplete checkpoint at the resolved checkpoint path, `da-toolkit analysis create ...` resumes that analysis without creating a new data-analysis.
- With no checkpoint at the resolved checkpoint path, `da-toolkit analysis create ...` creates a new data-analysis.
- `da-toolkit analysis create --resume ...` fails clearly when no usable checkpoint exists.
- `da-toolkit analysis create --new ...` creates a fresh data-analysis only when it will not overwrite or invalidate an existing incomplete checkpoint.
- `da-toolkit analysis create --new ...` fails clearly if it targets the same checkpoint/workspace as an existing incomplete checkpoint.
- Corrupt, invalid, ambiguous, completed, or expired checkpoint states produce clear errors and do not trigger fresh analysis creation by default.
- Expired SAS checkpoint handling still fails without creating a replacement data-analysis.
- Resume paths do not require OAuth credentials before reading and validating checkpoint state.
- Tests prove no accidental create-analysis API call occurs during inferred resume or invalid checkpoint handling.
