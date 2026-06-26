# Split analysis creation from file upload

## Type

feature

## Context

The current `analysis create` workflow creates a data-analysis and immediately uploads local files from the same environment. Some workflows need to create the data-analysis in one place, then hand upload instructions to another environment that has access to the plaintext files.

The split workflow should let creation return the filename, AES key, and SAS URL as handoff data, then let `analysis upload` encrypt and upload the plaintext file later using that handoff data. Existing immediate-upload behavior must remain available and unchanged.

## Functional Requirements

- Add a deferred upload mode to `analysis create`.
- Deferred creation accepts filenames only; local source files do not need to exist where creation runs.
- Deferred creation generates one AES key per file in memory.
- Deferred creation omits encrypted-file hash because the encrypted file cannot exist yet and the API schema marks hash optional.
- Deferred creation creates the data-analysis and receives one SAS URL per file.
- Deferred creation emits handoff JSON to stdout by default.
- Deferred creation supports `--output <path>` to write handoff JSON for automation.
- Deferred creation prints concise human text warning that handoff output contains AES keys and SAS bearer URLs.
- Support existing analysis types only when all required metadata can be provided without local file access.
- Add `analysis upload` as a new analysis subcommand.
- `analysis upload --filename <path> --aes-key <key> --sas-url <url>` uploads one plaintext source file using explicit handoff pieces.
- `analysis upload --handoff <path>` uploads all files described by the handoff JSON by default.
- `analysis upload --handoff <path> --filename <value>` uploads only the selected handoff file.
- For handoff selection, match `--filename` exactly against `files[].filename` first, then match by local basename.
- If basename selection is ambiguous, fail with a clear ambiguity error and require an exact handoff filename.
- `analysis upload --filename` always refers to the local plaintext source file that will be encrypted before upload.
- `analysis upload` supports the same `--workspace` and `--checkpoint` flags as immediate upload, with safe defaults when omitted.
- `analysis upload` supports resume and progress reporting equivalent to immediate upload.

## Technical Requirements

- Handoff JSON must include `analysis_id` and a `files` array containing `filename`, `aes_key`, and `sas_url` for each file.
- Handoff JSON is secret-bearing because it contains AES keys and SAS bearer URLs; warnings and docs must say so explicitly.
- AES keys may be emitted for handoff but must not be persisted in toolkit checkpoints.
- SAS URLs may be persisted only in protected upload checkpoints for restart-resume, consistent with existing upload guidance.
- `analysis upload` must encrypt plaintext with the supplied AES key using the same encryption format as immediate upload: AES-256-GCM, 1 MiB plaintext chunks, fresh IV per chunk, and length-prefixed encrypted chunks.
- `analysis upload` must reuse the existing Azure block upload behavior where practical, including block checkpointing, retry/resume semantics, progress output, block-list commit behavior, SAS expiry failure behavior, and cleanup prompts.
- Immediate `analysis create` upload behavior must remain unchanged.
- Deferred creation must not require stat/open/read access to the named files.
- Deferred creation for forensic and malware analyses must validate that required metadata is supplied through flags rather than discovered from local file content.
- Do not automatically create replacement data-analyses after SAS expiry.
- Do not persist OAuth client secrets, OAuth tokens, or AES keys.

Technical decisions:

- Use plaintext semantics for `analysis upload --filename` so the receiving environment can encrypt with the provided AES key before upload.
- Use filenames-only semantics for deferred creation to separate API creation from the machine holding the files.
- Omit encrypted hash during deferred creation because it cannot be known before encryption and the generated API schema treats hash as optional.
- Include both explicit flags and JSON handoff input in the first story so the feature works for manual and automated handoff workflows.
- Upload all files from handoff JSON by default, with `--filename` as an optional selector.
- Match exact handoff filename before basename to keep automation deterministic while allowing common local-path usage.
- Write handoff JSON to stdout by default and support `--output` for automation.

Alternatives considered and rejected:

- Requiring source files during deferred creation was rejected because it would not separate creation from the upload environment.
- Requiring an encrypted hash during deferred creation was rejected because the encrypted artifact does not exist yet.
- Supporting only flag-based upload was rejected because JSON handoff is needed for safe automation.
- Requiring `--filename` for JSON handoff uploads was rejected because uploading all files by default better supports batch handoff.

## Acceptance Criteria

- A user can create a data-analysis in deferred upload mode using filenames that do not exist locally.
- Deferred creation returns valid JSON handoff data containing `analysis_id`, `filename`, `aes_key`, and `sas_url` per file.
- Deferred creation can write handoff JSON to stdout or to `--output <path>`.
- Deferred creation warns that handoff data contains AES keys and SAS bearer URLs.
- Deferred creation omits file hash values from the API create request.
- `analysis upload` can upload one plaintext file using `--filename`, `--aes-key`, and `--sas-url`.
- `analysis upload` can upload all files from a handoff JSON file.
- `analysis upload --handoff <path> --filename <value>` selects by exact handoff filename first, then basename, and fails on ambiguous basename matches.
- Upload encryption, progress, block upload, resume, SAS expiry handling, and cleanup behavior match immediate upload behavior.
- Upload checkpoints never persist AES keys.
- Existing immediate `analysis create` upload behavior continues to work.

## Analysis

### Likely Impact

- Primary implementation lane: `cmd/analysis_create.go` -> `internal/analysis` planning/create/upload workflow -> `internal/ncscapi` request helpers/Azure block upload reuse.
- `cmd/analysis_create.go` - current `analysis create` requires local files for fresh creation and passes args into `analysis.Plan`/`analysis.Run`; deferred mode needs new create flags and branching without changing the existing immediate path defaults.
- `cmd/analysis.go` / new generated Cobra command file - `analysis upload` should be registered under the existing `analysis` command next to `create/list/read/delete/archive/file`.
- `internal/analysis/checkpoint.go` - current fresh `prepareCheckpoint` stats and encrypts every source file before API creation; deferred creation needs a filenames-only preparation path that generates AES keys and avoids checkpoint persistence of those keys.
- `internal/analysis/workflow.go` - current create helpers always include encrypted hash pointers; deferred creation likely needs request construction that can leave hash unset/null while still supplying required malware/forensic metadata from flags.
- `internal/analysis/encryption.go` and `internal/analysis/upload.go` - `analysis upload` should reuse the AES-256-GCM chunk format and Azure block upload behavior, but needs an entry path that encrypts using a supplied AES key instead of generating one.

### Possible Adjacent Touchpoints

- `internal/analysis/workflow_test.go` - closest coverage for encryption format, create/upload/resume, checkpoint secret checks, SAS expiry, cleanup, and progress output.
- `cmd/root_test.go` - command flag reset/help tests will need updates for new `analysis create` flags and the new `analysis upload` subcommand.
- `internal/ncscapi/client.go` / `internal/ncscapi/client_test.go` - likely only if request helper tests need to prove hash-optional payloads for supported analysis types; keep generated bindings read-only.
- `README.md` - user-facing CLI docs currently describe only immediate `analysis create`; deferred handoff warnings and upload examples likely need documentation.

### Existing Patterns / Prior Art

- `internal/analysis/encryption.go` - implements the required encryption format: AES-256-GCM, 1 MiB plaintext chunks, fresh nonce per chunk, length-prefixed encrypted chunks, and encrypted artifact hash.
- `internal/analysis/upload.go` - implements Azure block upload from encrypted artifact, block checkpointing, progress reporting, block-list commit, and SAS expiry failure behavior.
- `internal/analysis/checkpoint.go` - implements owner-only checkpoint writes and resume inspection; follow this for protected SAS persistence while ensuring AES keys are never serialized.
- `cmd/analysis_create.go` - existing Cobra flag parsing and API client construction pattern for create workflows.
- `cmd/analysis.go` result subcommands - existing pattern for analysis subcommand registration, JSON/stdout output helpers, and output file handling.

### Layer Boundaries

- Touch first: Cobra command wiring in `cmd/`, internal workflow/options in `internal/analysis`, upload/encryption reuse points, and tests around those paths.
- Avoid unless evidence emerges: `internal/ncscapi/generated/` and OpenAPI spec files, result retrieval commands, acquire/vol packaging, GUI layers, and any automatic replacement analysis creation after SAS expiry.
- Inference: generated hash fields are pointers but lack `omitempty` (`internal/ncscapi/generated/client.gen.go`), so nil may marshal as `"hash": null`; validate whether the API accepts null or whether wrapper/custom marshaling is needed before widening into generated code.

### Verification Plan

**Unit Tests**:

- Deferred `analysis create` accepts nonexistent filenames, returns/writes handoff JSON with `analysis_id`, `filename`, `aes_key`, and `sas_url`, and does not stat/open/read source files.
- Immediate `analysis create` behavior remains unchanged, including encryption, hash inclusion, checkpointing, resume, progress, and cleanup.
- Handoff JSON and checkpoints never persist OAuth secrets or AES keys; checkpoints may persist SAS URLs only with owner-only permissions.
- Handoff filename selection matches exact filename before basename and fails on ambiguous basename matches.

**Integration Tests**:

- Fake NCSC API create response returns per-file SAS URLs and receives deferred create requests without encrypted hashes.
- Fake Azure block blob endpoint verifies `analysis upload` encrypts plaintext with the supplied AES key, stages/resumes blocks, commits block lists, and reports SAS expiry without creating a replacement analysis.

**E2E / Manual Validation**:

- Run deferred create with `--output`, copy handoff to a separate temp directory with plaintext files, upload all files from handoff, then upload a selected file by exact and basename selection.

**Additional Checks (as applicable)**:

- Confirm handoff warning text explicitly says AES keys and SAS bearer URLs are secret-bearing.

## Validation update (2026-06-26 21:36)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed: `gofmt -w main.go cmd internal/config`, `gofmt -l internal/analysis internal/ncscapi` returned no files, `go test ./...`, `go vet ./...`, and `scripts/build-binary.sh /tmp/da-toolkit`.
* Focused checks passed: `/tmp/da-toolkit analysis upload --help`, `/tmp/da-toolkit analysis create --help`, and `go test -cover ./cmd ./internal/analysis ./internal/ncscapi` (`cmd` 52.8%, `internal/analysis` 75.7%, `internal/ncscapi` 79.5%).
* Touched-scope coverage: no material regression; changed command, analysis workflow, upload/checkpoint, encryption, and API request helper paths have focused unit/integration coverage.
* Security review: completed; AES keys are generated/used in memory, emitted only in secret-bearing handoff JSON, not persisted to checkpoints; SAS URLs remain checkpointed with owner-only permissions; no OAuth secrets/tokens are persisted; expired SAS handling remains explicit and does not create replacement analyses.
* Retained exploratory artifacts: none; no browser/UI validation required for this CLI/API-only story.
* Validated checklist items: deferred `analysis create` accepts nonexistent filenames without stat/open/read, creates API requests without hash fields, returns/writes handoff JSON with `analysis_id`, `filename`, `aes_key`, and `sas_url`, warns about AES keys and SAS bearer URLs, supports supported analysis types with metadata validation, preserves immediate create/upload behavior, registers `analysis upload`, supports explicit and handoff upload modes, selects handoff files exact-first then basename with ambiguity failure, uses the existing AES-256-GCM chunk format and Azure block upload path, reports progress, supports checkpointed resume and cleanup prompts, handles SAS expiry without automatic replacement analysis creation, and keeps checkpoints free of AES keys.
* Providers covered: local fake NCSC API creator and local fake Azure block blob server; no external provider credentials or live API used.
