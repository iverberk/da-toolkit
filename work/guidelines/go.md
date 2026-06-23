# Go Guidelines

## Principles
- Keep Cobra CLI commands in the generated `cmd/` package.
- Add new CLI commands with `cobra-cli add <command>` before editing command behavior.
- Preserve Cobra scaffold conventions for `rootCmd`, command `init()`, help, flags, and completion.
- Keep non-command application logic in `internal/` packages.
- Run `gofmt`, `go test ./...`, and `go vet ./...` after Go changes.

## Pitfalls
- Do not hand-create Cobra command files when `cobra-cli add` can generate them.
- Do not replace Cobra routing, help, flag, or completion behavior with custom parsers.

## Decisions
- Use Cobra as the CLI framework for the Go application.
- Treat `cobra-cli` output as the baseline layout for command files.
