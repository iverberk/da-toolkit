# Replace acquire poc with transparent embedded acquire command

## Type

feature

## Context

The embedded Python acquire proof of concept has validated that the toolkit can invoke the pinned Dissect/acquire package through the embedded Python runtime on `linux-arm64`. The user-facing command should now move from PoC shape to final acquire launcher shape.

Today the CLI exposes `da-toolkit acquire poc` with PoC-specific flags and diagnostics. Operators should instead be able to run Dissect/acquire through the toolkit as if they were running the installed Python `acquire` console script directly.

## Functional Requirements

- Replace the public `da-toolkit acquire poc` command with direct `da-toolkit acquire [acquire args/options...]` behavior.
- Remove the `poc` subcommand immediately from the public CLI surface.
- Forward all arguments and options after `da-toolkit acquire` transparently to Dissect/acquire.
- Make `da-toolkit acquire --help` show Dissect/acquire help directly.
- Support all Dissect/acquire commands and options transparently; do not intentionally filter or wrap acquire modes in this story.
- Keep the supported runtime platform limited to `linux-arm64` for now.
- Store all artifacts created by embedded runtime and dependency extraction in a hidden directory relative to the process current working directory by default: `.da-toolkit/acquire`.
- Do not use `/tmp` or any other global path for default runtime, dependency, or extraction artifacts.
- Allow the default acquire home to be overridden with `DA_TOOLKIT_ACQUIRE_HOME`.
- Enable setup diagnostics only when `DA_TOOLKIT_ACQUIRE_DEBUG=1` is set.
- Reuse the extracted Python runtime and acquire dependencies between runs for performance.

## Technical Requirements

- Preserve the proven embedded Python subprocess execution model from the PoC.
- Invoke Dissect/acquire using the installed console script entry point semantics: `acquire = acquire.acquire:main`.
- Ensure `sys.argv[0]` and forwarded arguments behave like the installed `acquire` console script as closely as possible.
- Automatically prepare the pinned embedded Python runtime and pinned acquire dependencies before invocation when they are missing.
- Verify the reused extracted runtime and dependencies before execution.
- Keep Python and acquire dependency versions pinned; do not implicitly upgrade embedded Python, acquire, or transitive dependencies.
- Keep normal stdout and stderr behavior as transparent passthrough from Dissect/acquire.
- Suppress PoC diagnostic output during normal execution.
- When `DA_TOOLKIT_ACQUIRE_DEBUG=1`, diagnostics may describe setup/extraction/verification paths and decisions without changing acquire argument behavior.
- Treat `DA_TOOLKIT_ACQUIRE_HOME` as the root for all embedded acquire runtime and dependency artifacts when set; otherwise resolve `.da-toolkit/acquire` relative to the process current working directory.
- Remove or hide public PoC-only flags such as `--runtime-dir`, `--site-packages`, `--bootstrap`, `--cleanup`, and `--acquire-arg` from the production command surface.
- Remove README references to `acquire poc` and document the direct `acquire` command behavior.
- Rejected alternative: continue using repeated `--acquire-arg` flags for argument forwarding.
- Rejected alternative: keep `/tmp` or OS-global temp directories as the default extraction location.
- Rejected alternative: keep runtime/bootstrap/extraction controls as normal user-facing CLI flags.
- Rejected alternative: keep `acquire poc` as a visible compatibility alias.

## Acceptance Criteria

- `da-toolkit acquire --help` prints Dissect/acquire help directly.
- `da-toolkit acquire <args...>` forwards the provided args to Dissect/acquire without requiring `--acquire-arg`.
- `da-toolkit acquire poc` is no longer available as a public subcommand.
- A default run creates or reuses extraction artifacts only under `.da-toolkit/acquire` relative to the process current working directory.
- A run with `DA_TOOLKIT_ACQUIRE_HOME=<path>` creates or reuses extraction artifacts only under the configured acquire home.
- A default run does not create acquire runtime or site-packages artifacts under `/tmp` or another global temp path.
- Repeated runs reuse the extracted runtime/dependencies after verification rather than forcing cleanup after every invocation.
- Normal command output does not include PoC status lines such as embedded Python version, runtime dir, site-packages dir, or synthetic invocation text.
- `DA_TOOLKIT_ACQUIRE_DEBUG=1 da-toolkit acquire --help` includes setup diagnostics while still showing Dissect/acquire help.
- Unsupported platforms fail clearly with a `linux-arm64` scope message.
- README command reference describes `da-toolkit acquire [args...]` instead of `acquire poc`.

## Analysis

### Likely Impact

- Primary implementation lane: `cmd/acquire.go` Cobra command surface -> `internal/acquire/acquire.go` embedded Python/acquire runner -> `README.md` command reference/tests.
- `cmd/acquire.go` - currently defines `acquire` as `Args: cobra.NoArgs` and exposes only the `poc` subcommand plus PoC flags (`--runtime-dir`, `--site-packages`, `--bootstrap`, `--cleanup`, `--acquire-arg`); this is the first layer to replace with direct `da-toolkit acquire [args...]` forwarding.
- `internal/acquire/acquire.go` - contains the proven execution model: linux-arm64 guard, `go-embed-python` runtime setup, embedded site-packages extraction, Python subprocess invocation, and `sys.argv = ['acquire'] + sys.argv[1:]`; refactor this from PoC options/diagnostics into production acquire launch behavior.
- `cmd/root_test.go` and `internal/acquire/acquire_test.go` - current tests assert PoC help/flags and temp-dir defaults, so they will need replacement coverage for direct acquire help/arg forwarding, hidden CWD-relative acquire home, env override, and debug-gated diagnostics.
- `README.md` - still documents `acquire poc`; update to the direct command shape.

### Possible Adjacent Touchpoints

- `internal/acquire/embedded/site-packages/linux-arm64/bin/acquire` - useful reference for installed console-script semantics, but likely not a file to edit.
- `work/adr/2026-06-22-embedded-python-acquire-packaging.md` - confirms the intended model is one user-facing binary that may extract a verified embedded Python/acquire runtime to an app-owned directory and invoke acquire as a subprocess; implementation should stay within that decision rather than changing packaging approach.
- `cmd/root_test.go` test helper `resetAcquireCommandFlags()` - likely needs cleanup once PoC flags/subcommand state is removed or replaced.

### Existing Patterns / Prior Art

- `internal/acquire/acquire.go` - closest prior art for runtime preparation, embedded site-packages extraction from `embed.FS`, Python path setup, subprocess stdout/stderr passthrough, and platform failure wording.
- `internal/acquire/acquire_test.go` - closest unit-test pattern for validating default paths, Python invocation argument preservation, and embedded dependency extraction.
- `cmd/root_test.go` - existing Cobra command-surface tests should guide assertions that `acquire poc` is unavailable and public PoC flags are absent.

### Layer Boundaries

- Touch first: Cobra acquire command definition in `cmd/`, acquire runner logic in `internal/acquire/`, command/docs/tests that directly describe the acquire command.
- Avoid unless evidence emerges: upload/result/API/config packages, generated NCSC API client, dependency/version upgrades, embedded site-packages contents, and GUI-related surfaces.
- Inference: no new Cobra subcommand generation is needed because the existing top-level `acquire` command already exists; this story changes its behavior and removes/hides the child `poc` command.

### Verification Plan

Repo-configured command checks are handled by implementation/validation via `work/project-config.md`.

**Unit Tests**:

- Cover direct Cobra parsing/forwarding for `da-toolkit acquire --help` and arbitrary acquire args without `--acquire-arg`.
- Cover absence of the public `poc` subcommand and PoC-only flags from help/command surface.
- Cover default acquire home resolution to `.da-toolkit/acquire` under process CWD and override via `DA_TOOLKIT_ACQUIRE_HOME`.
- Cover diagnostics emitted only when `DA_TOOLKIT_ACQUIRE_DEBUG=1`.

**Integration Tests**:

- On native `linux-arm64`, run `da-toolkit acquire --help` and confirm Dissect/acquire help is shown through the embedded runtime with normal PoC diagnostics suppressed.
- Run twice against the same acquire home and confirm extracted runtime/dependencies are reused after verification rather than forcibly cleaned/recreated.

**E2E / Manual Validation**:

- From a scratch working directory, confirm default artifacts are created only below `.da-toolkit/acquire` and not under `/tmp` or another global temp path.
- With `DA_TOOLKIT_ACQUIRE_HOME=<path>`, confirm all runtime/dependency artifacts stay under that configured path.

**Additional Checks (as applicable)**:

- Confirm unsupported platforms fail with a clear message preserving the current `linux-arm64` scope.

## Implementation update (2026-06-25 20:31)

- Addressed: replaced public `acquire poc` Cobra surface with direct `acquire [args...]` forwarding; moved default acquire artifacts to CWD-relative `.da-toolkit/acquire` with `DA_TOOLKIT_ACQUIRE_HOME` override; made setup diagnostics debug-gated; added site-packages verification/reuse; updated README and unit coverage.
- Verified: native `linux-arm64` `acquire --help`, default scratch artifact location, home override, debug diagnostics, and reuse status.
- Not addressed: none.
- Status: done

## Validation update (2026-06-25 20:38)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `go test ./...`, `go build -o /tmp/da-toolkit .`, `go vet ./...`, configured `gofmt -w main.go cmd internal/config` plus `gofmt -l main.go cmd internal/config internal/acquire`.
* Touched-scope coverage: no material regression; unit coverage now exercises direct Cobra forwarding, hidden PoC surface/flags, default and override acquire home resolution, debug gating, entrypoint argument preservation, embedded site-packages extraction, and reuse of verified extraction.
* Security review: completed for local path/extraction/input-forwarding scope; no credential, token, redirect, network upload, or privilege-escalation handling changed. `DA_TOOLKIT_ACQUIRE_HOME` remains an explicit local filesystem override, and default artifacts are CWD-relative.
* Retained exploratory artifacts: `/tmp/opencode/da-validator-acquire-default-help.txt`, `/tmp/opencode/da-validator-acquire-default-stderr.txt`, `/tmp/opencode/da-validator-acquire-default-debug-help.txt`, `/tmp/opencode/da-validator-acquire-default-debug-stderr.txt`, `/tmp/opencode/da-validator-acquire-default/.da-toolkit/acquire/`, `/tmp/opencode/da-validator-acquire-override-help.txt`, `/tmp/opencode/da-validator-acquire-override-stderr.txt`, `/tmp/opencode/da-validator-acquire-override/home/`.
* Validated checklist items: direct `da-toolkit acquire --help` shows Dissect/acquire help; arbitrary args are forwarded without `--acquire-arg`; `poc` is not a registered public subcommand; PoC-only flags are absent; default artifacts stay under `.da-toolkit/acquire` relative to CWD; `DA_TOOLKIT_ACQUIRE_HOME` confines artifacts to the configured home; normal output suppresses PoC status lines; debug mode emits setup diagnostics while preserving acquire help; repeated native runs reuse verified site-packages; unsupported-platform guard contains the required `linux-arm64` scope message; README documents `da-toolkit acquire [acquire args/options...]` instead of the public `acquire poc` reference.
* Providers covered: native `linux-arm64` CLI/acquire runtime; no UI/browser provider applicable.
