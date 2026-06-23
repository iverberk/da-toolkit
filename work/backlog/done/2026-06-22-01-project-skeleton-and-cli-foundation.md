# Project skeleton and CLI foundation

## Type

chore

## Context

The toolkit is a new CLI-first Go project. `work/adr/2026-06-22-cli-first-v1-product-scope.md` makes the CLI the canonical v1 workflow surface and keeps the future GUI aligned with CLI workflow semantics.

## Functional Requirements

- Provide a Go project skeleton for the data-analysis toolkit.
- Provide a CLI entrypoint built on the Cobra CLI framework, with stable help output and command grouping for future analysis, upload, result, acquisition, and GUI commands.
- Provide configuration/input conventions for non-secret settings.
- Provide a visible pattern for user-facing errors and command output.

## Technical Requirements

- Use Go as the primary implementation language.
- Use `github.com/spf13/cobra` for command routing, help rendering, and flag handling instead of a hand-rolled CLI parser.
- Scaffold the CLI with `cobra-cli init` and add top-level commands with `cobra-cli add <command>` to follow Cobra conventions.
- Do not introduce GUI implementation in this story.
- Do not persist API credentials, OAuth tokens, or AES keys.
- Define repo build, test, lint, and validation commands in `work/project-config.md` once the source layout exists.
- Follow `work/guidelines/cli-workflows.md`.

## Acceptance Criteria

- A basic CLI can be built and invoked on the development platform.
- CLI help describes the command groups without requiring network access or credentials.
- CLI command wiring, help, and flags are implemented through Cobra and follow the standard `cobra-cli` scaffold layout.
- The source layout is documented in `work/project-config.md`.
- Validation commands are documented in `work/project-config.md`.
- No secret persistence mechanism is introduced.

## Analysis

### Likely Impact

- Primary implementation lane: new Go module/project skeleton -> `cobra-cli init` generated entrypoint -> `cobra-cli add` command-group wiring/help output -> non-secret config/output/error conventions -> `work/project-config.md` source layout and command rules.
- Cobra scaffold entrypoint and `cmd/` package - likely first layer because the repo currently has no application source or Go module, and `work/project-config.md` states application source layout is not created yet.
- Cobra command definitions - likely in scope to expose stable top-level help and placeholder groups for analysis, upload, result, acquisition, and GUI without implementing those workflows.
- `work/project-config.md` - must be updated because command rules currently say repo-level validation commands are undefined until the Go module/source exists.

### Possible Adjacent Touchpoints

- Internal CLI/output package or command helper - may be useful to establish a visible user-facing error/output pattern while letting Cobra own command routing, help rendering, and flag parsing.
- Non-secret configuration convention docs/code - may need a small config-loading/defaults surface, but should avoid secret/token/key storage per `work/guidelines/cli-workflows.md`.
- Basic CLI tests - likely needed to lock help text, command grouping, and no-network/no-credential invocation behavior.

### Existing Patterns / Prior Art

- `work/adr/2026-06-22-cli-first-v1-product-scope.md` - primary product prior art: CLI is canonical v1 surface, GUI must later align to the same workflow semantics, and Windows/Linux amd64 are v1 targets.
- `work/guidelines/cli-workflows.md` - closest behavioral guidance: keep CLI canonical, keep secrets memory-only, report failures clearly, and avoid GUI divergence.
- No source-code prior art found quickly: repo currently contains only `.git/`, `AGENTS.md`, and `work/`, with no Go files or `go.mod`.

### Layer Boundaries

- Touch first: Go module metadata, CLI entrypoint/command tree, minimal output/error/config conventions, basic tests, and `work/project-config.md` command/source-layout documentation.
- Avoid unless evidence emerges: GUI implementation, NCSC API/OAuth/upload/encryption flows, Dissect/acquire execution, persistence/resume mechanics, and secret storage.

### Verification Plan

**Unit Tests**:

- Assert top-level help includes the intended future command groups and runs without network access or credentials.
- Assert placeholder commands return predictable user-facing output/errors and do not require persisted secrets.

**E2E / Manual Validation**:

- Build the CLI and invoke help on the development platform.
- Inspect the introduced config convention to confirm it covers only non-secret settings and does not persist API credentials, OAuth tokens, or AES keys.

## Validation update (2026-06-23 11:43)

* Validation passed with no regressions found.
* Gate result: PASS.
* Baseline checks passed or had no unrelated failures observed: `go build -o /tmp/da-toolkit .`, `go test ./...`, `go vet ./...`, `gofmt -w main.go cmd internal/config` with diff inspection, `/tmp/da-toolkit --help`, and placeholder command smoke checks.
* Touched-scope coverage: no material regression; new packages report `cmd` 78.6% and `internal/config` 75.0% with focused tests for help, placeholders, error pattern, and non-secret config handling.
* Security review: completed; no secret persistence mechanism, token storage, credential storage, external calls, privileged workflow, or GUI implementation found in touched code.
* Retained exploratory artifacts: none; UI/browser validation not applicable for CLI-only story.
* Validated checklist items: Go module/project skeleton; Cobra entrypoint and command groups for analysis, upload, result, acquisition, and GUI; stable offline help; placeholder user-facing output; unknown-command error pattern; non-secret config convention; source-layout and command-rule documentation in `work/project-config.md`; no API credential, OAuth token, or AES key persistence.
* Providers covered: not applicable.
