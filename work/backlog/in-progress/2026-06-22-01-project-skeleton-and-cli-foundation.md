# Project skeleton and CLI foundation

## Type

chore

## Context

The toolkit is a new CLI-first Go project. `work/adr/2026-06-22-cli-first-v1-product-scope.md` makes the CLI the canonical v1 workflow surface and keeps the future GUI aligned with CLI workflow semantics.

## Functional Requirements

- Provide a Go project skeleton for the data-analysis toolkit.
- Provide a CLI entrypoint with stable help output and command grouping for future analysis, upload, result, acquisition, and GUI commands.
- Provide configuration/input conventions for non-secret settings.
- Provide a visible pattern for user-facing errors and command output.

## Technical Requirements

- Use Go as the primary implementation language.
- Do not introduce GUI implementation in this story.
- Do not persist API credentials, OAuth tokens, or AES keys.
- Define repo build, test, lint, and validation commands in `work/project-config.md` once the source layout exists.
- Follow `work/guidelines/cli-workflows.md`.

## Acceptance Criteria

- A basic CLI can be built and invoked on the development platform.
- CLI help describes the command groups without requiring network access or credentials.
- The source layout is documented in `work/project-config.md`.
- Validation commands are documented in `work/project-config.md`.
- No secret persistence mechanism is introduced.
