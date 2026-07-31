# kk-flavor — inject

The entry point to the flavor's standards. **If this routing is not already in your context, you have not injected the flavor — read this file now**, then load docs per the triggers below.

Load lazily: read a doc only when its trigger matches what you're doing — unread rules you don't need dilute attention. A project's own `PROJECT_CODE_STYLE.md` / `CLAUDE.md`, when present, layers on top and wins on conflict.

## Read always (any task)

- [standards/core-principles.md](standards/core-principles.md) — how to approach any task.
- [standards/writing.md](standards/writing.md) — whenever you write prose: code comments, PR/commit descriptions, tickets, docs.

## Read on trigger

| When you are… | Read |
| --- | --- |
| writing or refactoring code | [standards/code-style.md](standards/code-style.md) — naming, params, comments, type safety, control flow, abstraction, classes vs functions, extraction, dependencies, tooling |
| designing modules / wiring dependencies | [standards/architecture/core.md](standards/architecture/core.md), then [backend.md](standards/architecture/backend.md) or [frontend.md](standards/architecture/frontend.md) |
| writing or reviewing tests | [standards/testing.md](standards/testing.md) — no mocks; test code is production code |
| writing outward text — anything a person reads as communication (PR/ticket text, chat, email, …) | [standards/human-writing.md](standards/human-writing.md) — the set it covers, natural voice, no AI tells, reader-action budget |
| setting up env, scripts, local dev / Docker | [standards/project.md](standards/project.md) |
| committing, pushing, opening a PR | [standards/git.md](standards/git.md) — print the command and get approval before any commit/push |

## Skill wiring

Cross-referenced skill names (e.g. `/refactor`, `/code-review`, `/tighten`) and quality-pipeline stage switches resolve through [config.yaml](config.yaml) — `roles` (name → skill) and `pipeline` (stage on/off). Consult it when a skill defers to another or runs a toggleable stage. The quality skills and their orchestrators also run under [standards/skill-protocol.md](standards/skill-protocol.md) — each skill loads it in its own setup, so no trigger row here.
