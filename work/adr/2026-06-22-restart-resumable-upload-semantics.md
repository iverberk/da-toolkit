# Restart-resumable upload semantics

Date: 2026-06-22
Status: Accepted

## Context

The toolkit must support retry/resume for very large encrypted uploads. API credentials, OAuth tokens, and AES keys must stay in memory. The user accepts persisting checkpoint state such as data-analysis ID and SAS upload checkpoint metadata. The NCSC SAS upload flow is not assumed to support refresh after URL expiry, and the tool must not automatically create a new data-analysis if upload cannot finish before expiry. See `work/adr/2026-06-24-persist-sas-urls-for-restart-resume.md` for the later decision to classify SAS URLs as acceptable persisted operational secrets.

## Decision

Support restart-surviving upload resume only while the encrypted artifact remains on disk. Persist resume metadata, including data-analysis ID, encrypted artifact path, upload checkpoint data, file hash, selected analyses, phase status, SAS URL, and SAS expiry metadata when known. Do not persist client secrets, access tokens, or AES keys. If the encrypted artifact is removed before completion, the upload cannot resume and the user must restart the workflow manually. After successful upload, ask the user whether to delete the encrypted artifact. If the SAS URL expires before completion, fail clearly and do not automatically create a replacement data-analysis.

## Consequences

Restart-resume requires durable encrypted artifact and checkpoint retention until upload completes. The encrypted artifact and checkpoint become operationally sensitive even though AES keys are not persisted, so their location and cleanup must be visible to the user. The tool must distinguish retry within a running process from resume after restart. Stories touching upload behavior must validate checkpoint persistence, missing-artifact handling, explicit SAS-expiry failure, and post-upload cleanup prompts.
