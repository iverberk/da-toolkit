# Packaging and Licensing Guidelines

## Principles
- Treat single-binary distribution as one user-facing executable per OS/architecture.
- Embed React GUI assets into the Go binary when GUI work begins.
- Use pinned Python and acquire dependencies when bundling Dissect/acquire.
- Verify extracted embedded runtimes before execution.
- Preserve third-party license notices and source availability for releases.

## Pitfalls
- Do not promise that no runtime files are ever extracted.
- Do not bundle AGPL components without release compliance checks.
- Do not upgrade embedded Python implicitly through dependency automation.

## Decisions
- Evaluate go-embed-python as the default embedded Python/acquire mechanism.
- License toolkit source under EUPL unless superseded by an ADR.
- Treat Dissect/acquire as AGPL-3.0 third-party software.
