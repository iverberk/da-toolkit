# Embedded Python acquire PoC

## Type

spike

## Context

The toolkit must run Dissect/acquire exactly, but `work/adr/2026-06-22-embedded-python-acquire-packaging.md` remains Proposed until a proof of concept validates `go-embed-python` on the required platforms.

## Functional Requirements

- Validate that the Go binary can invoke a pinned Dissect/acquire installation through embedded Python.
- Validate behavior on `windows-amd64` and `linux-amd64`.
- Capture runtime extraction behavior and operator-facing failure modes.
- Document administrative/root requirements for live-system acquisition.

## Technical Requirements

- Use `go-embed-python` as the candidate mechanism under evaluation.
- Pin Python and acquire dependency versions for the PoC.
- Invoke acquire as a subprocess rather than via CPython bindings.
- Verify or document how extracted runtime files are controlled and checked before execution.
- Follow `work/guidelines/packaging-and-licensing.md`.

## Acceptance Criteria

- PoC evidence shows acquire invocation from Go on Linux amd64.
- PoC evidence shows acquire invocation from Go on Windows amd64, or records a blocking platform-specific issue.
- Runtime extraction location and cleanup behavior are documented.
- Required privileges for live acquisition are documented.
- Dependency versions and licensing observations are documented.
- The ADR status can be revisited based on PoC outcome.
