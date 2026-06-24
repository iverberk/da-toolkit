# da-toolkit

`da-toolkit` is a CLI-first toolkit for NCSC Data Analysis API workflows. The current implemented workflow creates an `other` data-analysis from one or more existing local files, encrypts those files locally, uploads the encrypted artifacts through SAS URLs returned by the API, and stops after upload.

The tool is designed to keep secrets in memory only. Do not put API credentials, OAuth tokens, or AES keys in config files or checkpoint files.

## Build

```sh
go build -o /tmp/da-toolkit .
```

Use `/tmp/da-toolkit` in the examples below, or replace it with the path to your built binary.

## Upload workflow overview

The `upload` command is for pre-existing local files. It does not run acquisition tooling and it does not retrieve analysis results.

For each run, the tool:

1. Reads one or more local file paths from the command line.
2. Creates a workspace directory for encrypted artifacts and checkpoint metadata.
3. Generates one in-memory AES-256-GCM key per file.
4. Encrypts each file into an on-disk encrypted artifact.
5. Hashes each encrypted artifact.
6. Creates one `other` data-analysis with `manual_analysis_allowed=true`.
7. Sends the base64 AES keys and encrypted-file hashes to the API request.
8. Receives one SAS upload URL per file.
9. Uploads encrypted artifacts as Azure Block Blob blocks with the Azure Blob SDK using those SAS URLs.
10. Writes per-file status and uploaded block progress to the checkpoint after progress.
11. Commits the Azure block list only after all blocks for a file are uploaded.
12. Prints one end-of-run summary.
13. Prompts once to delete encrypted artifacts for successfully uploaded files.

The command does **not** retrieve results. Use future/result commands separately when result retrieval is implemented for your workflow.

## Required information

You need:

- One or more local files to upload.
- OAuth token URL for the NCSC API.
- OAuth client ID.
- OAuth client secret.
- Optional API base URL. If omitted, the default NCSC Data Analysis API base URL is used.
- Optional workspace directory. If omitted, a timestamped `.da-toolkit/upload-...` directory is created.
- Optional checkpoint path. If omitted, `upload-checkpoint.json` is created inside the workspace.

Credentials can be supplied by flags or environment variables:

| Value | Flag | Environment variable |
| --- | --- | --- |
| API base URL | `--base-url` | `DA_TOOLKIT_API_BASE_URL` |
| OAuth token URL | `--token-url` | `DA_TOOLKIT_TOKEN_URL` |
| OAuth client ID | `--client-id` | `DA_TOOLKIT_CLIENT_ID` |
| OAuth client secret | `--client-secret` | `DA_TOOLKIT_CLIENT_SECRET` |

Prefer environment variables or a secret manager for credentials. The toolkit does not persist the client secret, OAuth token, or AES keys.

## First upload

Example with environment variables:

```sh
export DA_TOOLKIT_TOKEN_URL="https://example.identity.provider/oauth/token"
export DA_TOOLKIT_CLIENT_ID="your-client-id"
export DA_TOOLKIT_CLIENT_SECRET="your-client-secret"

/tmp/da-toolkit upload \
  --workspace ./upload-workspace/case-001 \
  ./evidence/file-a.bin \
  ./evidence/file-b.bin
```

Example with flags:

```sh
/tmp/da-toolkit upload \
  --base-url "https://api.ncsc.nl/data-analysis" \
  --token-url "https://example.identity.provider/oauth/token" \
  --client-id "your-client-id" \
  --client-secret "your-client-secret" \
  --workspace ./upload-workspace/case-001 \
  ./evidence/file-a.bin \
  ./evidence/file-b.bin
```

After the command finishes, review the printed summary. It lists each source file, status, encrypted artifact path, and whether the file remains resumable.

If at least one file uploaded successfully, the tool asks:

```text
Delete encrypted artifacts for successfully uploaded files? [y/N]:
```

Answer `y` to delete only encrypted artifacts for successfully uploaded files. Failed or incomplete artifacts are kept so they can be resumed.

Use `--yes` to skip the prompt and delete successfully uploaded encrypted artifacts automatically:

```sh
/tmp/da-toolkit upload --yes --workspace ./upload-workspace/case-001 ./evidence/file-a.bin
```

## Resume an incomplete upload

Resume uses the checkpoint and encrypted artifacts left on disk. It does not create a replacement data-analysis, does not re-upload files already marked complete, and does not re-stage Azure blocks already recorded as uploaded for an incomplete file.

Use the same workspace or checkpoint path from the original run:

```sh
/tmp/da-toolkit upload \
  --resume \
  --workspace ./upload-workspace/case-001
```

Or resume with an explicit checkpoint file:

```sh
/tmp/da-toolkit upload \
  --resume \
  --checkpoint ./upload-workspace/case-001/upload-checkpoint.json
```

Resume behavior:

- Files marked uploaded are skipped.
- Files that still have encrypted artifacts and a valid checkpointed SAS URL resume from their checkpointed uploaded Azure block indexes.
- The Azure block list is committed only after all blocks for the file are staged.
- If an encrypted artifact is missing, only that file fails with a remediation message.
- If a SAS URL is expired, the affected file fails clearly.
- The tool does not automatically create a new data-analysis after SAS expiry.

If a file is non-resumable because its artifact is missing or its SAS URL expired, start a new upload workflow manually when appropriate.

## Checkpoint and artifact handling

The checkpoint contains operational metadata, including SAS URLs that are classified as acceptable persisted operational secrets for restart-resume:

- Analysis ID.
- Source file path.
- Encrypted artifact path.
- Encrypted-file hash.
- Azure upload block size, total blocks, uploaded block indexes, and commit state.
- SAS URL and expiry metadata when available.
- Per-file phase/status messages.

The checkpoint must not contain:

- OAuth client secrets.
- OAuth access tokens.
- AES keys.

Checkpoint files contain SAS URLs, which are bearer capability URLs. Treat checkpoints as sensitive operational-secret files, keep the workspace protected, and remove checkpoints when resume is no longer needed.

Encrypted artifacts are operationally sensitive. Keep the workspace protected and delete successful artifacts when you no longer need resume support.

## Command reference

```text
da-toolkit upload [flags] <file> [file...]

Flags:
  --base-url string       NCSC Data Analysis API base URL (or DA_TOOLKIT_API_BASE_URL)
  --token-url string      OAuth token URL (or DA_TOOLKIT_TOKEN_URL)
  --client-id string      OAuth client ID (or DA_TOOLKIT_CLIENT_ID)
  --client-secret string  OAuth client secret (or DA_TOOLKIT_CLIENT_SECRET; memory-only)
  --workspace string      Directory for encrypted artifacts and checkpoint metadata
  --checkpoint string     Checkpoint file for restart-resume metadata
  --resume                Resume from the checkpoint without creating a replacement data-analysis
  --yes                   Delete successfully uploaded encrypted artifacts without prompting
```

Other command groups exist in the CLI (`analysis`, `result`, `acquisition`, `gui`) but are currently placeholders unless implemented by later stories.
