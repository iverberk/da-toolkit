# API Upload Guidelines

## Principles
- Use OAuth client credentials for NCSC API authentication.
- Persist only non-secret resume metadata such as analysis ID, upload checkpoint, file path, and phase status.
- Require the encrypted artifact to remain on disk for restart-surviving upload resume.
- Fail explicitly when a SAS URL expires before upload completion.
- Make upload retry and resume behavior visible to the user.

## Pitfalls
- Do not store client secrets, access tokens, or AES keys.
- Do not assume an expired SAS URL can be refreshed.
- Do not hide partial-upload state from the operator.

## Decisions
- Do not automatically create a new data-analysis after SAS expiry.
- Ask the user whether to delete encrypted artifacts after successful upload.
