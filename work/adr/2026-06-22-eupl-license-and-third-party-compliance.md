# EUPL license and third-party compliance

Date: 2026-06-22
Status: Accepted

## Context

The toolkit is intended to be open-source and licensed under the European Union Public Licence. Dissect/acquire is AGPL-3.0 software. EUPL-1.2 explicitly lists AGPL v3 as a compatible licence. `go-embed-python` is Apache-2.0. Bundling acquire into a user-facing binary may create distribution obligations beyond licensing the toolkit's own source files.

## Decision

License the toolkit source under EUPL-1.2 unless a later ADR supersedes this decision. Treat Dissect/acquire as AGPL-3.0 third-party software and preserve its licence obligations in source and binary releases. Preserve notices and licence texts for bundled third-party components. Release artifacts that bundle acquire must provide clear source availability, build information, dependency versions, and third-party notices. Obtain legal review before the first public binary release that bundles acquire.

## Consequences

The repository can use EUPL while still acknowledging AGPL obligations for bundled acquire. Release engineering must include licence and notice validation, not just build/test validation. If legal review finds the bundled model unacceptable, the project may need to switch to a stronger separation model where acquire is installed separately or revise the overall licensing/distribution strategy.
