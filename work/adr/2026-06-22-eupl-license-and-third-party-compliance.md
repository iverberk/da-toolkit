# EUPL license and third-party compliance

Date: 2026-06-22
Status: Accepted

## Context

The toolkit is intended to be open-source and licensed under the European Union Public Licence. Dissect/acquire is AGPL-3.0 software. EUPL-1.2 explicitly lists AGPL v3 as a compatible licence. Volatility 3 is published under the Volatility Software License (VSL). `go-embed-python` is Apache-2.0. Bundling acquire or Volatility 3 into a user-facing binary may create distribution obligations beyond licensing the toolkit's own source files.

## Decision

License the toolkit source under EUPL-1.2 unless a later ADR supersedes this decision. Treat Dissect/acquire as AGPL-3.0 third-party software and Volatility 3 as VSL third-party software, and preserve their licence obligations in source and binary releases. Preserve notices and licence texts for bundled third-party components, including all generated embedded `volatility3[full]` transitive dependencies. Release artifacts that bundle acquire or Volatility 3 must provide clear source availability, build information, dependency versions, and third-party notices. Obtain legal review before the first public binary release that bundles acquire or Volatility 3.

## Consequences

The repository can use EUPL while still acknowledging AGPL obligations for bundled acquire and VSL obligations for bundled Volatility 3. Release engineering must include licence and notice validation, not just build/test validation. For Volatility 3 releases, the dependency inventory must be produced from the pinned embedded `volatility3[full]` payload and reviewed with the VSL terms before packaging. If legal review finds the bundled model unacceptable, the project may need to switch to a stronger separation model where acquire or Volatility 3 is installed separately or revise the overall licensing/distribution strategy.
