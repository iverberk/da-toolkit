# Align retrieval command references with analysis

## Type

refactor

## Context

The old top-level retrieval route was removed and replaced by `analysis` subcommands. The worktree still contained command file names, helper names, tests, README text, and backlog notes that referred to the removed command surface. Those stale references made the current `analysis` command hierarchy harder to follow and could suggest that the removed retrieval route still existed.

## Functional Requirements

- Remove remaining references to the removed top-level retrieval command surface from source, tests, and documentation.
- Preserve the existing `analysis create`, `analysis list`, `analysis read`, `analysis archive`, and `analysis file` behavior.
- Keep references to result artifacts and API result files where they describe downloaded analysis outputs rather than the removed command name.

## Technical Requirements

- Rename the command implementation and test files from result-oriented names to analysis-oriented names.
- Rename command wiring helpers, reset helpers, fake API fixtures, and test constants from result-oriented identifiers to analysis-oriented identifiers.
- Update upload command credential fallback wiring to use the renamed analysis command flags.
- Update README and backlog wording so retrieval is described as `analysis` retrieval subcommands rather than a separate command group.
- Do not change generated NCSC API bindings or result artifact API methods, because those names still represent API payloads and download outputs.

## Acceptance Criteria

- No remaining repository references describe the removed top-level retrieval command or its implementation file.
- Command tests continue to execute retrieval through `analysis` subcommands.
- Build, tests, vetting, and formatting complete successfully with the renamed command code.
- Existing result artifact download behavior and overwrite protection remain unchanged.
