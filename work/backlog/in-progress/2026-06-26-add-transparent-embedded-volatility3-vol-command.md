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
