# Embedded Python acquire packaging

Date: 2026-06-22
Status: Proposed

## Context

The toolkit should be distributed as a single user-facing binary while running Dissect/acquire exactly. Dissect/acquire is Python-based and available from PyPI. The `go-embed-python` project embeds standalone Python distributions into Go binaries using `//go:embed`, supports `windows-amd64` and `linux-amd64`, extracts the runtime at startup when needed, and runs Python in a separate process without requiring host Python or CGO.

## Decision

Evaluate `go-embed-python` as the default packaging mechanism for the embedded Python runtime and pinned Dissect/acquire dependencies. Define the distribution promise as one user-facing executable per OS/architecture, not as a guarantee that no runtime files are ever extracted. The Go binary may extract a verified embedded Python/acquire runtime to an app-owned runtime directory and invoke acquire as a subprocess.

## Consequences

The product can preserve a simple launch experience without requiring users to install Python. Runtime extraction, endpoint protection behavior, runtime verification, and cleanup must be handled explicitly. Python, acquire, and transitive dependencies must be pinned and upgraded deliberately. This ADR remains Proposed until a proof-of-concept validates acquire execution on Windows amd64 and Linux amd64, including live-system acquisition prerequisites.
