# CLI-first v1 product scope

Date: 2026-06-22
Status: Accepted

## Context

The data-analysis toolkit must provide both CLI and GUI capabilities over time, but the first development slice needs a stable and testable foundation. The core product value is local forensic acquisition and NCSC Data Analysis API interaction. The NCSC API supports creating data-analyses, retrieving statuses and results, and returning SAS URLs for file upload. Dissect/acquire supports live-system acquisition and is required by the product direction.

## Decision

Build v1 CLI-first. Treat the CLI as the canonical workflow surface. Target `windows-amd64` and `linux-amd64` for v1. Support live-system acquisition. Run Dissect/acquire exactly for acquisition workflows. Keep the future GUI as a local webserver convenience wrapper over the same workflow semantics rather than an independent product path.

## Consequences

The first stories should focus on end-to-end CLI workflow behavior before GUI implementation. GUI work must not define separate business rules from the CLI. Development and validation must account for administrative/root requirements for live acquisition on supported platforms. Platform scope is intentionally limited to Windows amd64 and Linux amd64 until a later decision expands it.
