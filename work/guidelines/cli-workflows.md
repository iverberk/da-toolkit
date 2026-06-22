# CLI Workflows Guidelines

## Principles
- Keep the CLI as the canonical workflow surface for v1.
- Route GUI actions through the same workflow semantics as CLI commands.
- Require explicit user confirmation before destructive cleanup.
- Report resumable state, non-resumable failures, and SAS expiry clearly.
- Keep API credentials, OAuth tokens, and AES keys in memory only.

## Pitfalls
- Do not persist secrets to support resume.
- Do not create replacement data-analyses automatically after SAS expiry.
- Do not let GUI behavior diverge from CLI behavior.

## Decisions
- Target `windows-amd64` and `linux-amd64` for v1.
- Treat live-system acquisition as a v1 requirement.
- Ask before deleting encrypted artifacts after upload success.
