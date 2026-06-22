# Project Kickoff Ideas - data-analysis toolkit

Last updated: 2026-06-22

## Scope

- Target: Data-analysis toolkit project foundation
- Theme: CLI-first forensic acquisition and NCSC Data Analysis API integration
- Basis: `work/project-config.md`, `work/README.md`, NCSC Data Analysis OpenAPI, Dissect/acquire README, go-embed-python README, EUPL-1.2 text
- Existing idea check: `work/ideas/Inbox.md`, `work/ideas/ideas/`

## Ideas

### CLI-first end-to-end forensic workflow

- Why it matters: Proving acquisition, encryption, API creation, SAS upload, polling, and result retrieval validates the core product before GUI investment.
- Evidence: `work/project-config.md` has no implementation domains yet; NCSC API exposes create/read/result endpoints and returns per-file SAS URLs; Dissect/acquire supports live-system acquisition.
- Impact: Gives development a narrow first slice with high product value and clear validation boundaries.
- Recommended next step: Record a Story for a CLI workflow that acquires or accepts an acquired artifact, encrypts and hashes it, creates a forensic data-analysis, uploads via SAS URL, polls status, and retrieves results.
- Confidence: High

### Single user-facing binary with embedded Python/acquire runtime

- Why it matters: Users need minimal setup while the tool must run Dissect/acquire exactly.
- Evidence: go-embed-python embeds standalone Python distributions with `//go:embed`, supports `windows-amd64` and `linux-amd64`, extracts at runtime, and runs Python in a subprocess; Dissect/acquire requires Python.
- Impact: Preserves the single-entrypoint UX while accepting controlled runtime extraction and subprocess execution.
- Recommended next step: Record an ADR evaluating go-embed-python as the default packaging mechanism for pinned Python and acquire dependencies.
- Confidence: Medium

### Restart-surviving upload resume with encrypted artifact retention

- Why it matters: Very large uploads need retry/resume, but API credentials and AES keys must remain in memory only.
- Evidence: User decision allows persisted data-analysis IDs and SAS checkpoint metadata, but not persisted credentials or AES keys; user decision says no automatic new data-analysis after SAS expiry.
- Impact: Upload resume after process restart requires the encrypted artifact to remain on disk until upload completion.
- Recommended next step: Define resume semantics: persist non-secret checkpoint state, require durable encrypted artifact retention, ask the user whether to delete the artifact after successful upload, and fail clearly on SAS expiry.
- Confidence: High

### GUI as local webserver convenience wrapper over CLI workflows

- Why it matters: The GUI should improve usability without creating a second product path that diverges from CLI behavior.
- Evidence: User decision makes CLI-first mandatory and GUI later via local webserver; React/shadcn build artifacts can be embedded in Go.
- Impact: Keeps GUI scope aligned with proven CLI commands and reduces duplication of workflow/business rules.
- Recommended next step: Defer GUI implementation until CLI workflows stabilize, but reserve a local-server boundary in the project architecture.
- Confidence: Medium

### EUPL-licensed toolkit with explicit third-party license compliance

- Why it matters: The project should be open-source under EUPL while bundling AGPL-licensed acquire may impose additional distribution obligations.
- Evidence: EUPL-1.2 explicitly lists AGPL v3 as a compatible licence; Dissect/acquire is AGPL-3.0; go-embed-python is Apache-2.0.
- Impact: The source repository can be EUPL, but release artifacts must preserve third-party notices and provide corresponding source/build information for bundled components.
- Recommended next step: Record a licensing ADR before first public binary release and add release checklist items for EUPL, AGPL, notices, dependency versions, and source availability.
- Confidence: Medium
