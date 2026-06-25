# Replace acquire poc with transparent embedded acquire command

## Type

feature

## Context

The embedded Python acquire proof of concept has validated that the toolkit can invoke the pinned Dissect/acquire package through the embedded Python runtime on `linux-arm64`. The user-facing command should now move from PoC shape to final acquisition launcher shape.

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
