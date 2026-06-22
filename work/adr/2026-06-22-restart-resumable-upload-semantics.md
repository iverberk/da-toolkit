# Restart-resumable upload semantics

Date: 2026-06-22
Status: Accepted

## Context

The toolkit must support retry/resume for very large encrypted uploads. API credentials, OAuth tokens, and AES keys must stay in memory. The user accepts persisting non-secret state such as data-analysis ID and SAS upload checkpoint metadata. The NCSC SAS upload flow is not assumed to support refresh after URL expiry, and the tool must not automatically create a new data-analysis if upload cannot finish before expiry.

## Decision

Support restart-surviving upload resume only while the encrypted artifact remains on disk. Persist only non-secret resume metadata, including data-analysis ID, encrypted artifact path, upload checkpoint data, file hash, selected analyses, phase status, and SAS expiry metadata when known. Do not persist client secrets, access tokens, or AES keys. If the encrypted artifact is removed before completion, the upload cannot resume and the user must restart the workflow manually. After successful upload, ask the user whether to delete the encrypted artifact. If the SAS URL expires before completion, fail clearly and do not automatically create a replacement data-analysis.

## Consequences

Restart-resume requires durable encrypted artifact retention until upload completes. The encrypted artifact becomes operationally sensitive even though the AES key is not persisted, so its location and cleanup must be visible to the user. The tool must distinguish retry within a running process from resume after restart. Stories touching upload behavior must validate checkpoint persistence, missing-artifact handling, explicit SAS-expiry failure, and post-upload cleanup prompts.
