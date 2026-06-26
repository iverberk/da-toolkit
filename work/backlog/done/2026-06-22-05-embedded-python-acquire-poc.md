# Embedded Python acquire PoC

## Type

spike

## Context

The toolkit must run Dissect/acquire exactly, but `work/adr/2026-06-22-embedded-python-acquire-packaging.md` remains Proposed until a proof of concept validates `go-embed-python` on the required platforms.

## Functional Requirements

- Validate that the Go binary can invoke a pinned Dissect/acquire installation through embedded Python.
- Validate behavior on `linux-arm64` for now; `windows-amd64` and `linux-amd64` evidence is deferred.
- Capture runtime extraction behavior and operator-facing failure modes.
- Document administrative/root requirements for live-system acquisition.

## Technical Requirements

- Use `go-embed-python` as the candidate mechanism under evaluation.
- Pin Python and acquire dependency versions for the PoC.
- Commit generated embedded artifacts for acquire and transitive Python dependencies so bootstrap does not require Internet access.
- Invoke acquire as a subprocess rather than via CPython bindings.
- Verify or document how extracted runtime files are controlled and checked before execution.
- Follow `work/guidelines/packaging-and-licensing.md`.

## Acceptance Criteria

- PoC evidence shows acquire invocation from Go on Linux arm64.
- Runtime extraction location and cleanup behavior are documented.
- Required privileges for live acquisition are documented.
- Dependency versions and licensing observations are documented.
- Bootstrap does not require Internet access because pinned acquire/transitive dependencies are available from committed embedded artifacts.
- The ADR status can be revisited based on PoC outcome.

## Analysis

### Likely Impact

- Primary implementation lane: `cmd/acquisition.go` entry point -> new/temporary acquisition PoC logic under `internal/` -> embedded Python runtime/dependency artifacts and PoC evidence documentation.
- `cmd/acquisition.go` - current acquisition command is only a placeholder, so any operator-facing PoC invocation should start here while preserving Cobra conventions from `work/guidelines/go.md`.
- New `internal/acquisition` or similarly named package - likely place for subprocess/runtime orchestration because repo guidance keeps non-command application logic in `internal/` and `cmd/upload.go` already delegates workflow behavior into `internal/upload`.
- `go.mod` / dependency metadata - likely in scope to add the `go-embed-python` dependency and keep pinned versions visible; current `go.mod` has no embedded Python or subprocess-related dependency.
- PoC evidence/documentation under `work/` - required to capture linux-arm64 results, runtime extraction/cleanup behavior, privilege requirements, and licensing observations before revisiting `work/adr/2026-06-22-embedded-python-acquire-packaging.md`; linux-amd64 and windows-amd64 evidence is deferred.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` - may need acquisition command test reset/expectation updates if the placeholder becomes a real PoC command or gains flags.
- Runtime/package asset locations - Inference: `go-embed-python` may introduce embedded runtime/dependency directories or build-tagged files; keep these limited to the acquisition PoC path and document platform-specific behavior.
- License/notice documentation - likely needed because `work/guidelines/packaging-and-licensing.md` treats Dissect/acquire as AGPL-3.0 third-party software and requires preserving third-party notices for releases.

### Existing Patterns / Prior Art

- `cmd/upload.go` -> `internal/upload/workflow.go` - closest repo pattern for a Cobra command validating flags, constructing options, and delegating workflow logic to an internal package.
- `cmd/root_test.go` - existing pattern for stable command help/error tests and placeholder expectations; use this if acquisition CLI behavior changes.
- `work/adr/2026-06-22-embedded-python-acquire-packaging.md` - closest recorded design context: single user-facing binary, extracted verified runtime, subprocess invocation, pinned Python/acquire dependencies, and linux/windows validation.
- No close prior art found in Go code for `os/exec`, `//go:embed`, Python runtime extraction, or acquire invocation; repo search found no existing subprocess/embed implementation.

### Layer Boundaries

- Touch first: acquisition CLI surface in `cmd/acquisition.go`, a narrow internal acquisition/embedded-runtime PoC package, dependency/version metadata, and `work/` evidence/docs needed for ADR follow-up.
- Avoid unless evidence emerges: NCSC API client/generated bindings, upload/encryption/checkpoint workflow, analysis retrieval subcommands, GUI paths, and persistent config/secrets handling.
- Avoid changing the acquisition execution model beyond the story: invoke acquire as a subprocess through embedded Python; do not introduce CPython bindings or host-Python requirements.

### Verification Plan

**Unit Tests**:

- If acquisition flags or command behavior are added, cover Cobra argument/flag validation and user-facing failure messages without requiring live acquisition.
- If runtime path verification/cleanup helpers are added, test deterministic extraction-location selection, integrity-check failure handling, and cleanup decisions with temp directories.

**Integration Tests**:

- Exercise the Go binary invoking the pinned embedded Python/acquire environment on `linux-arm64` and record command, versions, output, extraction location, and cleanup result.
- Defer `linux-amd64` and `windows-amd64` runtime evidence for now.

**E2E / Manual Validation**:

- Run or document live-system acquisition privilege behavior on Linux and Windows, including root/administrator requirements and operator-facing failure mode when privileges are insufficient.

**Additional Checks (as applicable)**:

- Confirm Python, acquire, and transitive dependency versions are pinned and not implicitly upgraded.
- Confirm extracted runtime files are controlled and checked before execution, and document endpoint-protection/runtime extraction observations.
- Confirm Dissect/acquire AGPL-3.0 and `go-embed-python` license/notice implications are captured for the PoC outcome.

## Implementation notes (2026-06-25 00:00)

- Added `acquisition poc` as the bounded Linux arm64 PoC entry point. It uses `github.com/kluctl/go-embed-python` pinned to `v0.0.0-3.12.8-20241219-1` / Python `3.12.8`, extracts runtime files into an app-owned temp subtree by default, optionally deletes the extracted runtime with `--cleanup`, and invokes acquire through a Python subprocess (`from acquire.acquire import main`).
- Added PoC bootstrap support for `acquire==3.22` into an app-owned site-packages directory. The bootstrap path now uses committed generated embedded artifacts for acquire and transitive Python dependencies, so it does not need Internet access.
- Linux amd64 and Windows amd64 runtime evidence has been deferred by story scope update; current target is Linux arm64.
- Linux arm64 runtime evidence: native `linux-arm64` host (`go env GOOS GOARCH` -> `linux arm64`, `uname -m` -> `aarch64`) ran `/tmp/da-toolkit acquisition poc --bootstrap --cleanup --runtime-dir /tmp/da-toolkit-poc-arm64-runtime --site-packages /tmp/da-toolkit-poc-arm64-site-packages` successfully. Output reported `embedded-python=v0.0.0-3.12.8-20241219-1`, `python=3.12.8`, `acquire=acquire==3.22`, `platform=linux-arm64`, `runtime-dir=/tmp/da-toolkit-poc-arm64-runtime-973d085229676e56`, and printed acquire help from the subprocess.
- Cleanup evidence: after the successful PoC run, `test ! -d /tmp/da-toolkit-poc-arm64-runtime-973d085229676e56 && test -d /tmp/da-toolkit-poc-arm64-site-packages` passed, confirming `--cleanup` removed the extracted Python runtime and preserved the app-owned acquire site-packages directory.
- Runtime extraction behavior: `go-embed-python` extracts under the configured runtime directory with a content-hash suffix and verifies/reuses existing files from its embedded file list before execution; `--cleanup` calls `Cleanup()` to delete the extracted runtime after the run. The acquire site-packages directory is separate and preserved unless `--bootstrap` resets it.
- Live acquisition privileges: Linux live-system collection should be run as root (or with equivalent read access to protected devices/files); Windows live-system collection should be run from an elevated Administrator process. Insufficient privileges are expected to surface as acquire subprocess errors and non-zero exit status.
- Licensing observations: `go-embed-python` is Apache-2.0; Dissect/acquire `3.22` metadata declares `AGPL-3.0-or-later`; release packaging still needs third-party notices/source-availability checks before the ADR can move beyond Proposed.

## Scope update (2026-06-25)

- Updated PoC requirement: acquire and all transitive Python dependencies must be committed as generated embedded artifacts for the scoped `linux-arm64` target.
- Bootstrap must not contact Python package indexes or require Internet access; it should extract the committed embedded site-packages artifact into the app-owned `--site-packages` directory.

## Implementation update (2026-06-25 10:51)

- Addressed: adjusted PoC runtime gate and evidence to the updated Linux arm64-only scope; verified native Linux arm64 acquire invocation and cleanup behavior.
- Not addressed: Linux amd64 and Windows amd64 runtime evidence remains deferred by story scope.
- Status: done

## Validation update (2026-06-25 10:54)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`, and `gofmt -w main.go cmd internal/config internal/acquisition`.
* Linux arm64 PoC proof passed on native `linux-arm64`: `/tmp/da-toolkit acquisition poc --bootstrap --cleanup --runtime-dir /tmp/da-toolkit-validator-arm64-runtime --site-packages /tmp/da-toolkit-validator-arm64-site-packages` printed `embedded-python=v0.0.0-3.12.8-20241219-1`, `python=3.12.8`, `acquire=acquire==3.22`, `platform=linux-arm64`, and acquire help from the Python subprocess.
* Runtime cleanup proof passed: `test ! -d /tmp/da-toolkit-validator-arm64-runtime-973d085229676e56 && test -d /tmp/da-toolkit-validator-arm64-site-packages`.
* Touched-scope coverage: no material regression; touched package tests cover CLI help/reset behavior plus runtime/site-package path selection and subprocess argument preservation, with runtime PoC covered by native integration command.
* Security review: completed for subprocess invocation, bootstrap network behavior, runtime extraction paths, cleanup, and privilege documentation; no secrets or credentials persisted.
* Retained exploratory artifacts: none.
* Validated checklist items: Go binary invokes pinned Dissect/acquire via embedded Python subprocess on linux-arm64; linux-amd64/windows-amd64 runtime evidence deferred by current story scope; extraction location and cleanup behavior documented and observed; live acquisition privilege requirements documented; dependency versions and licensing observations documented; ADR can be revisited with linux-arm64 PoC outcome while broader platform evidence remains deferred.
* Providers covered: linux-arm64.

## Implementation update (2026-06-25 offline bootstrap)

* Addressed: committed generated Linux arm64 site-packages artifacts under `internal/acquisition/embedded/site-packages/linux-arm64` and changed `--bootstrap` to extract those embedded artifacts instead of invoking pip or contacting package indexes.
* Offline bootstrap evidence: `/tmp/da-toolkit acquisition poc --bootstrap --cleanup --runtime-dir /tmp/da-toolkit-offline-runtime --site-packages /tmp/da-toolkit-offline-site-packages` passed on native `linux-arm64`, printed the pinned embedded Python/acquire versions, and printed acquire help from the Python subprocess.
* Cleanup evidence: `test ! -d /tmp/da-toolkit-offline-runtime-973d085229676e56 && test -d /tmp/da-toolkit-offline-site-packages` passed, confirming runtime cleanup and preserved extracted embedded acquire dependencies.
* Generated artifact note: the committed site-packages tree is scoped to `linux-arm64`/Python `3.12.8`/`acquire==3.22`; linux-amd64 and windows-amd64 generated artifacts remain deferred with their runtime evidence.
* Verification passed: `gofmt -w main.go cmd internal/config internal/acquisition`, `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`, and `git diff --check`.
