# Add transparent embedded volatility3 vol command

## Type

feature

## Context

The CLI already exposes `da-toolkit acquire [acquire args/options...]` as a transparent launcher for Dissect/acquire through the embedded Python runtime. Operators should be able to run Volatility 3 the same way, using a `vol` command that behaves like the upstream volatility3 `vol` console script while remaining fully embedded in the toolkit binary distribution model.

Volatility 3 is published as `volatility3` and exposes the upstream console script `vol = volatility3.cli:main`. The requested dependency scope is `volatility3[full]`. The command should preserve volatility3's normal symbol and cache behavior rather than introducing toolkit-managed symbol packs or custom symbol workflows.

## Functional Requirements

- Add a top-level `da-toolkit vol [volatility3 args/options...]` command.
- Forward all arguments and options after `da-toolkit vol` transparently to volatility3.
- Make `da-toolkit vol --help` show volatility3 help directly.
- Run volatility3 from embedded Python and embedded pinned `volatility3[full]` dependencies; do not require host Python or network bootstrap at runtime.
- Preserve normal stdout and stderr behavior from volatility3.
- Preserve volatility3's normal symbol and cache behavior; do not bundle symbol packs and do not add toolkit-specific symbol management in this story.
- Expose only `vol`; do not expose `volshell`.
- Store embedded runtime and dependency artifacts in a hidden current-working-directory-relative location by default.
- Allow the volatility artifact home to be overridden with `DA_TOOLKIT_VOL_HOME`.

## Technical Requirements

- Mirror the existing transparent `acquire` command shape: disable Cobra flag parsing and pass the raw argument vector to the volatility runner.
- Invoke the upstream console-script equivalent using `volatility3.cli:main` and set `sys.argv` so invocation behaves as closely as possible to installed `vol`.
- Embed `volatility3[full]` and its transitive dependencies as pinned, platform-specific site-packages payloads.
- Keep embedded Python, volatility3, and transitive dependency versions pinned; do not implicitly upgrade them through dependency automation.
- Use `DA_TOOLKIT_VOL_HOME` as the root for volatility runtime and dependency artifacts when set.
- Use `.da-toolkit/vol` relative to the process current working directory as the default volatility home unless implementation evidence identifies a stronger repository-consistent default.
- Reuse verified extracted runtime and dependency artifacts between runs for performance.
- Gate setup/extraction diagnostics behind a volatility-specific debug environment variable if diagnostics are added; diagnostics must not alter forwarded volatility arguments.
- Keep `volshell` out of scope and do not register it as a public command or alias.
- Preserve volatility3-native symbol lookup/cache behavior and avoid adding bundled symbol packs, automatic symbol downloads, or toolkit-managed symbol configuration in this story.
- Validate third-party licensing and notice obligations for volatility3's Volatility Software License and all `volatility3[full]` dependencies before release packaging.
- Reuse implementation patterns from the embedded acquire runner where appropriate, but keep volatility-specific package verification, paths, environment variables, and dependency payloads separate from acquire.
- Rejected alternative: bundle base `volatility3` without the `full` extras.
- Rejected alternative: expose `volshell`.
- Rejected alternative: add toolkit-managed or bundled volatility symbol packs.
- Rejected alternative: use a command name other than `vol` or add aliases in this story.

## Acceptance Criteria

- `da-toolkit vol --help` prints volatility3 help from the embedded runtime.
- `da-toolkit vol <args...>` forwards the provided arguments to volatility3 without requiring wrapper-specific forwarding flags.
- `da-toolkit vol` uses embedded pinned `volatility3[full]` dependencies and does not require host Python or runtime Internet access for dependency installation.
- Normal volatility3 stdout and stderr are passed through without toolkit-specific status output unless debug diagnostics are explicitly enabled.
- A default run creates or reuses volatility runtime/dependency artifacts only under `.da-toolkit/vol` relative to the process current working directory, unless a different default is explicitly justified during implementation.
- A run with `DA_TOOLKIT_VOL_HOME=<path>` creates or reuses volatility artifacts only under the configured home.
- Repeated runs against the same volatility home reuse verified extracted artifacts rather than forcibly recreating them.
- No public `volshell` command or alias is registered.
- Volatility3 symbol/cache behavior remains native to volatility3; no toolkit-specific symbol pack bundling or management is introduced.
- Unsupported build or runtime platforms fail with a clear supported-platform message consistent with the embedded runtime support policy.
- Tests cover command argument forwarding, help behavior, default home resolution, `DA_TOOLKIT_VOL_HOME` override, absence of `volshell`, and dependency verification/reuse behavior where practical.
- README or command reference documentation describes `da-toolkit vol [volatility3 args/options...]`, `DA_TOOLKIT_VOL_HOME`, and the fact that volatility symbols/cache behavior remains volatility3-native.
- Licensing/notice documentation captures volatility3 VSL and `volatility3[full]` dependency obligations before release packaging.

## Analysis

### Likely Impact

- Primary implementation lane: new `cmd/vol.go` Cobra command surface -> new `internal/vol` embedded Python/volatility runner -> build/docs/tests for separate volatility dependency payloads.
- `cmd/acquire.go` shows the transparent launcher shape to mirror: `DisableFlagParsing: true`, raw `args` forwarding, and stdout/stderr passthrough into the runner options.
- `internal/acquire/acquire.go` is the closest downstream model for default home resolution, platform gating, embedded Python runtime setup, platform-specific embedded site-packages extraction/reuse, `ep.AddPythonPath`, and Python subprocess invocation.
- `scripts/build-binary.sh` currently prepares only acquire payloads under `internal/acquire/embedded/site-packages/<platform>` with `acquire==3.22`; it will likely need a parallel volatility payload path and pinned `volatility3[full]` install step without merging volatility artifacts into acquire paths.
- `README.md` currently documents `acquire` only as the embedded-tool command; command reference needs a sibling `vol` entry, `DA_TOOLKIT_VOL_HOME`, and native volatility symbol/cache behavior notes.

### Possible Adjacent Touchpoints

- `cmd/root_test.go` - existing command-surface tests reset and stub `runAcquire`; likely needs a `runVol` stub/reset and assertions for `vol --help`, arbitrary forwarding, and no `volshell` command.
- `internal/vol/vol_test.go` - likely parallel to `internal/acquire/acquire_test.go` for default `.da-toolkit/vol`, `DA_TOOLKIT_VOL_HOME`, invocation args, embedded path selection, verification, and reuse behavior.
- `.gitignore` - currently ignores generated platform payloads only under `internal/acquire/embedded/site-packages/...`; likely needs equivalent ignore/placeholder rules for the volatility embedded payload directory.
- `go.mod`/`go.sum` - `go-embed-python` is already present in `go.mod`; no new Go dependency is obvious unless implementation factors shared helpers.
- Licensing/notice artifacts - `work/adr/2026-06-22-eupl-license-and-third-party-compliance.md` covers acquire AGPL obligations only; volatility3 VSL/full-extra obligations need release-package notice capture before release.

### Existing Patterns / Prior Art

- `cmd/acquire.go` - direct prior art for a transparent top-level command that disables Cobra flag parsing and sends raw args to an embedded runner.
- `internal/acquire/acquire.go` - direct prior art for hidden CWD-relative home (`.da-toolkit/acquire`), env override (`DA_TOOLKIT_ACQUIRE_HOME`), runtime dir/site-packages layout, debug-gated setup diagnostics, platform guard, `embed.FS` extraction, verification/reuse, and subprocess stdout/stderr passthrough.
- `internal/acquire/acquire_test.go` - direct prior art for path resolution, env override, Python invocation preservation, embedded payload extraction checks, and verified extraction reuse tests.
- `scripts/build-binary.sh` - prior art for pinned Python `3.12.8`, `uv` cross-platform binary-only installation, target platform mapping, placeholder cleanup, and Go build invocation.
- `work/backlog/done/2026-06-25-replace-acquire-poc-with-transparent-embedded-acquire-command.md` - story-level prior art for the same transparent embedded-console-script transition and validation expectations.

### Layer Boundaries

- Touch first: Cobra command wiring in `cmd/`, a new volatility-specific internal runner/package, build-time embedded dependency preparation, README/command reference, and tests directly covering the new `vol` surface and runner behavior.
- Avoid unless evidence emerges: NCSC API/upload/analysis packages, `internal/config`, acquire behavior and payload semantics beyond reusable pattern reference, GUI surfaces, volatility symbol-pack management, custom cache/symbol workflows, and public `volshell` exposure.
- Inference: keep volatility-specific constants, environment variables, embedded FS, verification, and paths separate from `internal/acquire` to avoid coupling acquire and volatility dependency lifecycles.

### Verification Plan

Repo-configured command checks are handled by implementation/validation via `work/project-config.md`.

**Unit Tests**:

- Cover `da-toolkit vol --help` forwarding to the volatility runner despite Cobra flag parsing being disabled.
- Cover arbitrary arg forwarding and `sys.argv` construction for `volatility3.cli:main` with `sys.argv[0]` behaving like `vol`.
- Cover default home resolution to process-CWD `.da-toolkit/vol` and `DA_TOOLKIT_VOL_HOME` override.
- Cover absence of a public `volshell` command/alias.
- Cover volatility site-packages verification and reuse behavior where embedded placeholders allow practical testing.

**Integration Tests**:

- Run `da-toolkit vol --help` from a built binary and confirm upstream volatility3 help appears without toolkit status output in normal mode.
- Run twice against the same volatility home and confirm verified runtime/dependency artifacts are reused.

**E2E / Manual Validation**:

- From a scratch working directory, confirm default artifacts are created only below `.da-toolkit/vol`.
- With `DA_TOOLKIT_VOL_HOME=<path>`, confirm runtime/dependency artifacts stay under that configured home.
- Confirm normal volatility3 symbol/cache behavior remains native and no toolkit symbol-pack directories or downloads are introduced.

**Additional Checks (as applicable)**:

- Validate `volatility3[full]` and transitive dependency license/notice obligations, including Volatility Software License, before release packaging.
- Validate unsupported build/runtime platforms fail with supported-platform wording consistent with the acquire embedded-runtime policy.

## Implementation notes (2026-06-26 12:19)

- Added top-level `vol` Cobra command with disabled flag parsing and raw argument forwarding to a new embedded Volatility runner.
- Added `internal/vol` runner using embedded Python, `DA_TOOLKIT_VOL_HOME`, default CWD-relative `.da-toolkit/vol`, pinned `volatility3[full]==2.26.2`, separate embedded payload paths, debug-gated setup diagnostics, and native `volatility3.cli:main` invocation with `sys.argv[0] == "vol"`.
- Added volatility-specific embedded placeholder directories, `.gitignore` entries, build-script payload preparation, command/runner tests, README command documentation, and VSL/release-notice obligations in the licensing ADR.
- No `volshell` command or alias was added; no toolkit-managed symbol or cache workflow was added.

## Implementation results (2026-06-26 12:19)

- Passed: `go test ./...`.
- Passed: `gofmt -w main.go cmd internal/config`.
- Passed: `go vet ./...`.
- Blocked: `scripts/build-binary.sh /tmp/da-toolkit` fails while preparing the pinned `volatility3[full]` payload because the `leechcorepyc` transitive dependency has no usable wheel for this environment and source build requires missing native `pkg-config`/`libusb` build inputs. This also blocks built-binary `da-toolkit vol --help` integration validation in this environment.
- Status: partial; Go implementation and unit checks are complete, but embedded full-payload binary packaging needs native build dependencies or a supported wheel strategy before release packaging can pass.

## Implementation update (2026-06-26 13:07)

- Addressed: reran the previously blocked build gate after native inputs were added; adjusted the build script to isolate native volatility Python extension builds from ambient Nix Python flags and pinned the dev shell Python to 3.12 to match the embedded runtime.
- Addressed: fixed the volatility invocation bootstrap so `sys.argv[0]` is set to `vol` before importing `volatility3.cli`, making built-binary help render `usage: vol ...`.
- Passed: `scripts/build-binary.sh /tmp/da-toolkit`.
- Passed: `gofmt -w main.go cmd internal/config internal/vol` and the repo-configured formatting command scope `gofmt -w main.go cmd internal/config`.
- Passed: `go test ./...`.
- Passed: `go vet ./...`.
- Passed: built-binary `DA_TOOLKIT_VOL_HOME=/tmp/da-toolkit-vol-home /tmp/da-toolkit vol --help` prints Volatility 3 Framework 2.26.2 help with `usage: vol ...`.
- Passed: repeated built-binary run with `DA_TOOLKIT_VOL_DEBUG=1` reports `site-packages-status=reuse` for the same volatility home.
- Passed: built-binary default run from `/tmp/da-toolkit-vol-cwd` creates artifacts under `.da-toolkit/vol` in that current working directory.
- Not addressed: full release-package license/notice file generation remains a release packaging follow-up; this story recorded volatility3/full-extra obligations in the licensing ADR.
- Status: done.

## Validation update (2026-06-26 13:11)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `gofmt -w main.go cmd internal/config`, `go test ./...`, `go vet ./...`, `scripts/build-binary.sh /tmp/da-toolkit`.
* Touched-scope coverage: no material regression observed; focused coverage command `go test ./cmd ./internal/vol -cover` passed.
* Security review: completed for embedded runtime extraction, environment override handling, argument forwarding, stdout/stderr passthrough, and absence of toolkit-managed symbol/cache workflows; no story-introduced security issue found.
* Retained exploratory artifacts: `/tmp/da-toolkit`, `/tmp/da-toolkit-vol-help.out`, `/tmp/da-toolkit-vol-debug2.err`, `/tmp/da-toolkit-vol-cwd/.da-toolkit/vol`, `/tmp/da-toolkit-vol-home`.
* Validated checklist items: `vol --help` forwards to Volatility 3 help with `usage: vol`; arbitrary args are forwarded by unit tests; embedded pinned `volatility3[full]==2.26.2` payload builds; normal help emits no stderr/status output without debug; default artifacts are under CWD-relative `.da-toolkit/vol`; `DA_TOOLKIT_VOL_HOME` override confines artifacts to the configured home; repeated runs reuse verified site-packages (`site-packages-status=reuse`); no public `volshell` command or alias is registered; native Volatility symbol/cache behavior is preserved by not adding toolkit symbol management; unsupported build platform reports supported platforms clearly; README and ADR document command usage, Volatility-native symbols/cache, and VSL/full-extra notice obligations.
* Providers covered: not applicable; CLI/binary behavior validated on linux-amd64.

## Implementation update (2026-06-26 13:38)

- Addressed: source `go run . vol --help` with placeholder-only embedded payloads now fails before extraction with a clear "embedded volatility3 site-packages payload is not prepared" message instead of extracting placeholders and reporting a missing extracted file; the build script can retain generated payloads for source runs with `DA_TOOLKIT_KEEP_EMBEDDED_PAYLOADS=1`.
- Addressed: applied the same missing-payload guard to the shared acquire pattern so `go run . acquire --help` fails clearly when acquire payloads are not embedded.
- Passed: `go run . vol --help` now reports the clear missing-payload/source-run message; built binary `/tmp/da-toolkit vol --help` still prints Volatility 3 Framework 2.26.2 help.
- Passed: acquire regression checks cover the new missing-payload message and built binary `/tmp/da-toolkit acquire --help` still prints upstream acquire help.
- Passed: `go test ./...`, `go vet ./...`, `gofmt -w main.go cmd internal/config`, and `scripts/build-binary.sh /tmp/da-toolkit`.
- Not addressed: source `go run` without prepared embedded payloads does not run Volatility; this remains a clear build/payload preparation error rather than a runtime network bootstrap.
- Status: done.

## Validation update (2026-06-26 13:28)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `gofmt -w main.go cmd internal/config`, `go test ./...`, `go vet ./...`, `scripts/build-binary.sh /tmp/da-toolkit-story-validator-196974/da-toolkit`.
* Touched-scope coverage: no material regression observed; focused coverage command `go test ./cmd ./internal/vol ./internal/acquire -cover` passed.
* Security review: completed for embedded payload verification before site-packages extraction, environment-home overrides, generated payload retention/cleanup behavior, argument forwarding, and acquire shared-pattern changes; no story-introduced security issue found.
* Retained exploratory artifacts: `/tmp/da-toolkit-story-validator-196974`, `/tmp/da-toolkit-story-validator-source-prepared-197520`, `/home/iverberk/.local/share/opencode/tool-output/tool_f03af42eb0015Bk2yHeBHd0dOk`.
* Validated checklist items: placeholder-only source `DA_TOOLKIT_VOL_HOME=<tmp> go run . vol --help` now fails with the clear `embedded volatility3 site-packages payload is not prepared` source-run guidance and does not extract placeholder site-packages; prepared-payload source `DA_TOOLKIT_KEEP_EMBEDDED_PAYLOADS=1 scripts/build-binary.sh <tmp>` followed by `go run . vol --help` prints Volatility 3 Framework 2.26.2 help with `usage: vol ...` and no stderr; built binary `vol --help` prints Volatility 3 Framework 2.26.2 help with `usage: vol ...` and no normal-mode setup output; repeated built-binary `vol --help` with `DA_TOOLKIT_VOL_DEBUG=1` reports `site-packages-status=reuse`; shared acquire guard returns clear `embedded acquire site-packages payload is not prepared` guidance for placeholder-only source runs and built-binary `acquire --help` still prints upstream acquire help; unit tests cover argument forwarding, help forwarding, no `volshell`, home resolution, environment override, and missing-payload guard behavior.
* Requirement-to-evidence summary: source missing-payload failure fix -> PASS via `/tmp/da-toolkit-story-validator-196974/src-vol.err`; source prepared-payload help -> PASS via `/tmp/da-toolkit-story-validator-source-prepared-197520/source-prepared-vol.out`; built-binary volatility help/stdout-stderr passthrough -> PASS via `/tmp/da-toolkit-story-validator-196974/bin-vol.out` and empty `bin-vol.err`; reuse behavior -> PASS via `/tmp/da-toolkit-story-validator-196974/bin-vol-debug.err`; acquire shared-pattern regression -> PASS via `/tmp/da-toolkit-story-validator-196974/src-acquire.err` and `bin-acquire.out`; tests/build/static analysis -> PASS via command output retained in the tool log.
* Providers covered: not applicable; CLI/source/binary behavior validated on linux-arm64.
