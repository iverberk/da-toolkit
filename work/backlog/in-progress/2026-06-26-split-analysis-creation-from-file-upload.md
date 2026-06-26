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
