# Rename acquisition command to acquire

## Type

chore

## Context

The CLI exposed the embedded Python acquire proof-of-concept under the top-level `acquisition` command even though the underlying tool and operator-facing workflow are named acquire. The command surface needed a small naming cleanup so users invoke the workflow as `da-toolkit acquire poc`.

## Functional Requirements

- Rename the top-level acquisition workflow command from `acquisition` to `acquire`.
- Keep the existing `poc` subcommand behavior and flags unchanged.
- Ensure root help advertises `acquire` and no longer advertises `acquisition`.
- Ensure the old `acquisition` top-level command is unavailable.
- Update README command reference text to use `acquire poc`.

## Technical Requirements

- Preserve Cobra command structure in `cmd/` and continue registering the acquisition workflow as a root child command.
- Limit implementation to the user-facing command name, internal command variable name, tests, and README reference.
- Do not alter embedded Python/acquire execution, bootstrap, cleanup, or argument forwarding behavior.
- Validate with the repository-configured Go formatting, test, vet, build, and diff checks.

## Acceptance Criteria

- `da-toolkit --help` lists `acquire` among available commands.
- `da-toolkit acquire poc --help` documents the existing PoC flags and offline bootstrap behavior.
- `da-toolkit acquisition` returns an unknown-command error.
- README references the PoC command as `acquire poc`.
- Verification passes with `gofmt -w main.go cmd internal/config`, `go test ./...`, `go vet ./...`, `go build -o /tmp/da-toolkit .`, and `git diff --check`.
