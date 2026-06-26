# Wait for Analysis and Download Results

## Type

feature

## Context

Users can currently read a data-analysis metadata payload once with `analysis read <analysis-id>`, download a completed analysis archive with `analysis archive`, or download a named result file with `analysis file`.

Operators need a way to wait for a data-analysis to complete while respecting API rate-limit guidance. When requested, available result files should be downloaded as soon as they appear in metadata instead of waiting for the whole analysis to complete and then downloading an archive.

## Functional Requirements

- Add `--wait` to `analysis read <analysis-id>`.
- `analysis read <analysis-id> --wait` must poll indefinitely until the data-analysis metadata reports `status: "completed"`, unless the user interrupts the command.
- Polling must respect the `Retry-After` response header from the metadata read API response when it is present.
- `analysis read <analysis-id> --wait` without `--download` must print the final metadata JSON after completion.
- Add `--download` to `analysis read <analysis-id>` for use only with `--wait`.
- `analysis read <analysis-id> --wait --download` must download newly observed result files while waiting continues.
- Download mode must use each result name as the local path exactly, including path separators, and create parent directories as needed.
- If a target result file already exists locally, skip it and count it as complete permanently for the current wait session.
- Failed result downloads must be reported and retried on later polls.
- Download mode must continue retrying observed failed downloads after the analysis reaches `completed` until every observed result has been downloaded or skipped, unless the user interrupts the command.
- Download mode output must use stdout and include progress while waiting plus a final summary only; it must not print the final metadata JSON.
- `--download` without `--wait` must fail with helpful guidance that `--download` is only available while waiting and that `analysis archive` and `analysis file` can download results from a completed analysis.
- Preserve the existing `analysis archive` and `analysis file` command behavior.

## Technical Requirements

- Keep generated NCSC API bindings behind toolkit-owned wrappers; do not expose generated response types directly from the command layer.
- Extend the toolkit API wrapper used by `analysis read` so successful metadata reads expose the response metadata needed for wait behavior, including `Retry-After`.
- Treat `Retry-After` as authoritative for the next metadata poll delay when present; define a conservative fallback interval for cases where the header is absent or unusable.
- Determine completion from the data-analysis-level `status` field, not from file-level statuses.
- Track per-session result state so successfully downloaded and skipped-existing results are not retried.
- Treat failed downloads as pending so they are retried on later polls.
- Use the existing named-result retrieval logic/API path for result downloads rather than archive retrieval.
- Create local parent directories before writing result files whose names include path separators.
- Preserve local overwrite protection by skipping existing files rather than replacing them.
- Keep OAuth client secrets and access tokens memory-only; do not persist credentials, tokens, or wait/download state for this feature.
- Rejected alternative: download the archive after completion; rejected because users should receive individual results as soon as they become available.
- Rejected alternative: wait for all file sections to report `status: "completed"`; rejected in favor of the data-analysis-level `status: "completed"` completion rule.
- Rejected alternative: abort waiting on a failed result download; rejected because transient download failures should be retried while waiting continues.
- Rejected alternative: sanitize result names down to basenames; rejected because result names may intentionally include subdirectories.
- Rejected alternative: print final metadata JSON in download mode; rejected because download mode should provide progress and a final summary only.

## Acceptance Criteria

- Running `analysis read <analysis-id> --wait` polls until the metadata reports `status: "completed"` and then prints the completed metadata JSON.
- Polling uses the `Retry-After` header from metadata read responses when present.
- Running `analysis read <analysis-id> --wait --download` downloads each newly observed result file before overall completion when the result appears in metadata.
- Downloaded files are written using result names as paths, including creation of parent directories.
- Existing local result paths are skipped, reported, and considered complete for the rest of the wait session.
- Failed result downloads are reported and retried on later polls.
- If the analysis reaches `completed` while observed result downloads are still failing, the command keeps retrying those downloads until they succeed or are skipped, unless interrupted.
- Download mode prints progress and a final summary to stdout and does not print metadata JSON.
- Running `analysis read <analysis-id> --download` without `--wait` fails with guidance mentioning `analysis archive` and `analysis file` for completed-analysis result retrieval.
- Existing `analysis read` without `--wait`, `analysis archive`, and `analysis file` behaviors remain unchanged.
