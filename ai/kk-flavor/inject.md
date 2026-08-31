# kk-flavor — inject

Read a doc only when its trigger below matches what you're doing. A project's own `PROJECT_CODE_STYLE.md` / `CLAUDE.md` layers on top and wins on conflict.

## Read always (any task)

- [standards/core-principles.md](standards/core-principles.md)
- [standards/writing.md](standards/writing.md)

## Read on trigger

| When you are… | Read |
| --- | --- |
| writing or refactoring code | [standards/code-style.md](standards/code-style.md) |
| designing modules, layers or boundaries; wiring dependencies | [standards/architecture/core.md](standards/architecture/core.md) |
| tracing what calls what, or what a change reaches, past the first grep | [standards/code-navigation.md](standards/code-navigation.md) |
| writing or reviewing tests, **or changing behaviour that should have one** | [standards/testing.md](standards/testing.md) |
| writing outward text — anything a person reads as communication | [standards/human-writing.md](standards/human-writing.md) |
| setting up env, scripts, local dev / Docker; **writing a schema migration; adding or upgrading a dependency** | [standards/project.md](standards/project.md) |
| committing, pushing, opening a PR | [standards/git.md](standards/git.md) |
| editing a skill, standard, prompt, template or `CLAUDE.md` | [standards/ecosystem.md](standards/ecosystem.md) |
| invoking another skill, or orchestrating a run of them | [standards/skill-protocol.md](standards/skill-protocol.md) |
| running a multi-stage quality pass over one change set | [standards/quality-pipeline.md](standards/quality-pipeline.md) |
| appending to a record kept across runs | [standards/records.md](standards/records.md) |
| driving a browser | [standards/browser.md](standards/browser.md) |
| touching a running system — a deploy, live data, an external write API, infrastructure | [standards/live-systems.md](standards/live-systems.md) |
