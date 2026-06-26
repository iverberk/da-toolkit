# Add multi-platform embedded acquire build targets

## Type

chore

## Context

The embedded acquire packaging path was scoped to `linux-arm64` only, while the project guidelines target `linux-amd64` and `windows-amd64` for v1 as well. The build script and embedded site-packages layout need to select the correct target platform without changing pinned Python or Dissect/acquire versions.

## Functional Requirements

- Keep `linux-arm64` embedded acquire support working.
- Allow embedded acquire builds for `linux-amd64` and `windows-amd64` targets.
- Fail clearly when a build or runtime platform is outside the supported embedded acquire targets.
- Keep generated embedded dependency payloads ignored while preserving platform placeholder files in git.

## Technical Requirements

- Resolve embedded site-packages from a platform-specific directory under `internal/acquire/embedded/site-packages/<goos>-<goarch>`.
- Embed the shared site-packages root so supported platform subdirectories can be selected at runtime.
- Derive build target selection from `GOOS`, `GOARCH`, and `CGO_ENABLED`, defaulting to the current Go environment when unset.
- Use `uv` with the matching Python platform and binary-only dependency resolution for cross-platform target artifact preparation.
- Keep the native `python3`/`pip` fallback limited to builds targeting the host platform.
- Keep Python `3.12.8`, go-embed-python `v0.0.0-3.12.8-20241219-1`, and acquire `3.22` pinned.

## Acceptance Criteria

- `.gitignore` ignores generated embedded site-packages payloads for `linux-arm64`, `linux-amd64`, and `windows-amd64` while allowing placeholders.
- `scripts/build-binary.sh` selects `linux-arm64`, `linux-amd64`, or `windows-amd64` embedded dependency targets from `GOOS` and `GOARCH`.
- Unsupported target platforms fail with a supported-platform message before building.
- Runtime platform validation allows `linux-arm64`, `linux-amd64`, and `windows-amd64` and reports unsupported runtime platforms clearly.
- Unit coverage verifies that the embedded site-packages path is derived from the runtime platform.
