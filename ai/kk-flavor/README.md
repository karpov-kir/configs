# kk-flavor

An opinionated bundle the kk skills read at runtime: the **standards** they enforce and the **config** that wires them. Mount it at a fixed path so any skill — Claude or Codex, symlinked or copied — resolves it the same way.

## Contents

- `inject.md` — the entry point: routes you to the rule doc for the current activity (read it first; see [How skills use it](#how-skills-use-it)).
- `standards/` — the rule docs.
- `templates/` — project-setup starter configs (Taskfile, Docker, env, `.editorconfig`, README), referenced by the project-setup standard.
- `config.yaml` — role mapping (which skill fills a cross-referenced role) and idsd-ship stage toggles.

## Install

1. **Mount the bucket** at the fixed path skills expect:
   ```sh
   ln -s /path/to/ai/kk-flavor ~/.kk-flavor
   ```
2. **Install the skills** you want into your agent's skills directory (e.g. `~/.claude/skills/`) — symlink or copy. Skills resolve standards and cross-referenced skills through `~/.kk-flavor`, so they work regardless of how they were installed.

## How skills use it

- **Standards**: skills read `~/.kk-flavor/inject.md`, which routes them to the rule doc(s) under `standards/` for the activity at hand.
- **Cross-referenced skills**: a skill that defers to a role (e.g. `refactor`) resolves the real skill name via `~/.kk-flavor/config.yaml` → `roles`. Repoint a role to substitute your own skill.
- **Pipeline toggles**: idsd-ship runs only the stages whose `config.yaml` → `pipeline` flag is true.

The shipped config reproduces the flavor's built-in behavior; edit it to retune.

## Tailored skills

The quality skills carry a `kk-` prefix (`kk-code-review`, `kk-refactor`, `kk-security-review`, `kk-tighten`) to avoid colliding with generic skill names. Cross-references use the unprefixed role name and resolve through `config.yaml`.
