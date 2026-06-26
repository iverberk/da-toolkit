# Remove stale upload and acquisition paths

## Type

chore

## Context

The command hierarchy and internal packages had already moved toward `analysis create` and `acquire`, but the worktree still contained old-name paths: `cmd/upload.go`, `internal/upload/`, and a duplicate `internal/acquisition/` PoC tree. Those names conflicted with the current terminology where acquisition is `acquire` and upload workflow code belongs under analysis.

## Functional Requirements

- Remove or rename stale code paths that still use the old upload/acquisition naming.
- Keep the existing `analysis create` behavior available after the path/package cleanup.
- Keep the existing `acquire` implementation as the only acquire runtime package.
- Avoid retaining duplicate `internal/acquisition` embedded artifacts when `internal/acquire` provides the current implementation.

## Technical Requirements

- Rename `cmd/upload.go` to `cmd/analysis_create.go` so the file name matches the Cobra command it defines.
- Rename `internal/upload/` to `internal/analysis/` and update the Go package name from `upload` to `analysis`.
- Update command wiring, tests, imports, helper names, and default workspace/checkpoint names to use analysis terminology.
- Remove the obsolete `internal/acquisition/` tree instead of keeping a second acquire implementation and duplicate embedded assets.
- Preserve memory-only secret handling and restart-resume behavior while changing package and path names only.

## Acceptance Criteria

- No Go source references remain to `cmd/upload.go`, `internal/upload`, `internal/acquisition`, `package upload`, or `package acquisition`.
- The repository contains `cmd/analysis_create.go` and `internal/analysis/` for analysis creation/upload workflow code.
- The repository no longer contains `internal/acquisition/`.
- Required validation passes: formatting, binary build, Go tests, and Go vet.
