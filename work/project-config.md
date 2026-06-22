# Project Config

This file is the canonical, human-readable source of truth for repo structure, guideline loading, review triggers, and repo-level command rules.

If another repo document conflicts with this file, follow this file.

## Purpose

- Use this file as rule, not background reading.
- Humans read it to understand expected agent behavior.
- Agents read it to decide which guidelines, reviews, and commands apply.

## Repo Structure

- `work/` is the durable AiFact source of truth for project context, ideas, stories, ADRs, guidelines, and release notes.
- `work/ideas/ideas/` stores brainstormed concepts before they become scoped stories.
- `work/backlog/planned/`, `work/backlog/in-progress/`, and `work/backlog/done/` store story lifecycle state.
- `work/adr/` stores architecture and product decisions that should survive chat sessions.
- `work/guidelines/` stores compact project-specific guidance loaded by agents for matching work.
- Application source layout is not created yet; define it through stories/ADRs before implementation begins.

## Output Rules

- Separate verified repo facts, proposals, and open questions.
- Cite repo artifact paths for durable decisions and constraints.
- Keep brainstorm output concise and record durable decisions in `work/` artifacts.
- Do not treat chat-only decisions as durable until recorded in `work/`.

## Domain Rules

- CLI workflows: load `work/guidelines/cli-workflows.md` for CLI commands, workflow orchestration, acquisition flow, GUI parity, cleanup, and user prompts.
- API upload: load `work/guidelines/api-upload.md` for NCSC API, OAuth, SAS upload, encryption, hashing, resume, retry, checkpoints, and result retrieval.
- Packaging/licensing: load `work/guidelines/packaging-and-licensing.md` for single-binary distribution, embedded Python, Dissect/acquire, React asset embedding, EUPL, AGPL, notices, and release packaging.

## Technology Rules

- Go is the intended primary implementation language; record detailed Go project standards before implementation.
- Dissect/acquire must run exactly for acquisition workflows; use packaging/licensing guidance before changing acquisition integration.
- go-embed-python is the current candidate for embedded Python/acquire packaging; record an ADR before committing to it.
- React/shadcn is the intended GUI stack; GUI work should remain a local webserver convenience wrapper over CLI workflow semantics.
- NCSC Data Analysis API integration must use OAuth client credentials and preserve memory-only handling for secrets.

## Review Rules

- Review CLI workflow changes against `work/guidelines/cli-workflows.md`.
- Review API, encryption, upload, resume, or result retrieval changes against `work/guidelines/api-upload.md`.
- Review packaging, dependency embedding, release, or license changes against `work/guidelines/packaging-and-licensing.md`.
- Require ADR discussion before changing licensing posture, embedded Python approach, acquisition execution model, or restart-resume semantics.

## Loading Rules

### Always Load

- `work/project-config.md`

### Analysis

- Load guidelines matching the story domain keywords and affected files.
- Prefer recorded ideas, ADRs, and stories over chat history.
- If no guideline exists for a touched domain, identify the gap before implementation planning.

### Implementation

- Load `work/guidelines/cli-workflows.md` when adding or changing CLI/user workflow behavior.
- Load `work/guidelines/api-upload.md` when adding or changing NCSC API, OAuth, encryption, hashing, upload, retry, resume, or result retrieval behavior.
- Load `work/guidelines/packaging-and-licensing.md` when adding or changing embedded Python, Dissect/acquire packaging, GUI asset embedding, release, or license behavior.

### Validation

- Validate behavior against all guidelines loaded during analysis/implementation.
- Treat memory-only secret handling, restart-resume semantics, SAS expiry behavior, and license notice preservation as explicit validation concerns when relevant.
- Record validation gaps in the story artifact instead of relying on chat memory.

## Command Rules

- Repo-level validation commands are not defined yet because application source does not exist.
- Define build, test, lint, codegen, and release commands in this file once the Go module and UI workspace are introduced.

## Agent Usage Rule

- Read this file first when you need repo structure, guideline loading rules, review triggers, or repo-level command rules.
- Load only the guideline files that match the current task.
- Do not invent rules outside this file, `AGENTS.md`, and the loaded guideline files.
