# Persist SAS URLs for restart-resume

Date: 2026-06-24
Status: Accepted

## Context

Restart-resume after process exit needs the Azure Blob SAS URL returned when the data-analysis was created. The API flow is not assumed to support refreshing the SAS URL without creating a replacement data-analysis, and the toolkit must not automatically create a replacement after SAS expiry. Without persisting the SAS URL, a checkpoint can record uploaded blocks but cannot authorize a later process to continue the upload.

## Decision

Classify upload SAS URLs as acceptable persisted operational secrets for this workflow. Persist them in checkpoint files so restart-resume can continue incomplete block uploads while the SAS URL remains valid. Protect checkpoints with owner-only file permissions, document SAS URLs as bearer capability URLs, and continue to keep OAuth client secrets, OAuth tokens, and AES keys out of checkpoints.

## Consequences

Restart-resume can work from retained checkpoint and encrypted artifact state. Checkpoint files become sensitive and must be stored in protected workspaces, handled like short-lived credentials, and deleted or protected after use. Expired SAS URLs still fail clearly and must not be refreshed by automatically creating a replacement data-analysis.
