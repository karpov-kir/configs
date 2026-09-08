# Agent ecosystem

Install the standards, skills and their Go tools from a permanent checkout. Skill symlinks point
back here; deleting the checkout breaks them. [The field guide](field-guide.html) describes the skills.

## Install

Choose `--agent=claude` or `--agent=codex` for every install and uninstall. There is no default.

```sh
ai/bootstrap.sh --agent=codex          # Codex, machine-wide
ai/bootstrap.sh --agent=claude         # Claude Code, machine-wide
ai/bootstrap.sh --agent=codex --maintainer  # include ecosystem maintenance skills
ai/bootstrap-owner.sh --agent=claude   # Claude owner instructions, maintainer skills and RTK
ai/bootstrap-owner.sh --agent=codex    # same owner instructions, maintainer skills and RTK
ai/install-project.sh --agent=codex ~/code/thing  # after bootstrap, for a single project
```

| Target | Machine skills | Machine instructions | Project skills | Project instructions |
| --- | --- | --- | --- | --- |
| `--agent=claude` | `~/.claude/skills` | `~/.claude/CLAUDE.md` | `.claude/skills` | `AGENTS.md`, imported by `CLAUDE.md` |
| `--agent=codex` | `~/.agents/skills` | `${CODEX_HOME:-~/.codex}/AGENTS.md` | `.agents/skills` | `AGENTS.md`, imported by `CLAUDE.md` |

Codex paths follow [OpenAI's skill discovery](https://learn.chatgpt.com/docs/build-skills) and
[instruction discovery](https://learn.chatgpt.com/docs/agent-configuration/agents-md). An existing
nonempty `AGENTS.override.md` shadows `AGENTS.md`; bootstrap reports that conflict. Restart Codex
after installation to load the new global instructions.

Both clients share `~/.kk-flavor` and the source tree, but can be installed and removed independently.
The shared bucket stays while another client has skill mounts from this checkout. Codex migrates old
`$CODEX_HOME/skills` links only after their replacements exist under `~/.agents/skills`. It leaves unrelated links alone.

The default tier excludes skills marked `audience: maintainer`. Add `--maintainer` to include them.
Normal installations add a shared fenced region to the client's instructions and keep its memory policy.

Owner installs copy [owner-instructions.md](owner-instructions.md) to the client's user instruction
file: `CLAUDE.md` for Claude or `AGENTS.md` for Codex. These are independent copies. The repository
stores only the template, so working here doesn't load owner instructions as project instructions.
Re-run bootstrap after changing the template. An adjacent receipt identifies unchanged copies for
upgrades and removal; local edits are preserved and reported. An older generated Codex instruction
file is backed up before replacement. If it contains added personal text, bootstrap refuses to replace it.

Both clients use `~/Document/AI/MEMORY.md` for owner memory. Bootstrap creates the file only when absent
and never removes it on uninstall.

For an agent performing an install: run bootstrap once, then the project installer for each named
project. Choose the requested client. Do not select the owner or maintainer tier unless requested.
Report `REFUSED` lines so the user can resolve targets the installer left untouched.

## Dependencies and controls

The Go tools need `gh` to download verified releases, or Go to build from source when no release exists.
Set `JUDGE_PROVIDER=codex` or `JUDGE_PROVIDER=claude` for each judge invocation and authenticate
the matching CLI yourself. Missing, invalid or unavailable providers fail with exit 2. There is no
`auto` mode or fallback.

```sh
JUDGE_PROVIDER=codex ~/.kk-flavor/scripts/bloat-judge.sh instruction instructions.md
```

Model assignments and usage sites live in [models.json](kk-flavor/models.json). The shipped policy
keeps agent work on the original task's model and preserves the judge's existing helper assignments.
Read [model policy](kk-flavor/standards/model-policy.md) before choosing an override or a cheaper
coordinator. The resolver prints requested settings and provenance; native or CLI dispatch still
must verify the effective selection. `JUDGE_PROVIDER` selects the client, not a fallback provider.
Change the central policy to tune the judge; a legacy `JUDGE_MODEL` setting is refused.

Reply editing and structured stage returns do not call the judge. Durable deletion disputes can use
it explicitly. Matching first and second votes avoid a third call; cache identity includes the model
policy and judging policy so a changed assignment cannot reuse an old verdict.

## Pipeline use

Use `kk-build` to implement a settled requirement, `kk-qualify` for its quality pass, and `idsd-ship`
for the intent lifecycle. Individual IDSD checkpoints remain available. These entries share one
coordinator, which dispatches bounded leaf workers and waits on completion. Independent correctness
and security reviews keep separate contexts; fixes reopen affected evidence.

`kk-edit` combines the former concision and humanization passes for prose and comments. It preserves
meaning and stops at an edited artifact. `kk-skillcraft` remains the focused skill-structure entry;
`kk-ecosystem` owns instruction semantics and applies its ordered checks within one worker. A full
ecosystem audit requires an explicit request. The editor never deletes agent obligations.

After landing an upgrade, rerun bootstrap and each recorded project's installer for every installed
client. They mount `kk-edit` and remove retired links owned by that checkout. During candidate
validation, use explicit worktree paths; do not point installed mounts at unfinished skills.

Qualification receipts use the `edit` stage. Existing receipts using the retired `tighten` stage
require requalification. A not-applicable skip needs a scope receipt for the exact review base and
candidate; unknown paths and security surfaces keep their reviews.

Both clients install `jq` (`brew install jq`) and sync `mcp.jsonc` plus the optional gitignored
`mcp.private.jsonc` with `mcp-sync.sh --agent=codex` or `--agent=claude`. Codex stdio servers retain
their command, arguments and environment; HTTP servers use Codex's streamable HTTP transport.
Authenticate servers that require OAuth with `codex mcp login <name>` after syncing.

The owner tier installs `rtk` (`brew install rtk`). Codex runs `rtk init --codex --global` in a
temporary profile, then copies `RTK.md` to `CODEX_HOME` if none exists there. The owner instruction
template stays untouched and supplies RTK guidance to both clients. Codex selects commands explicitly;
Claude uses its native hook.
Use `rtk proxy <command>` when exact output is needed, including every diff read for review.

- `--dry-run`: report changes without writing them.
- `--relocate`: authorize moving mounts from another checkout; otherwise the installer refuses before writing.
- `--skip-tools`, `--skip-brew`, `--skip-mcp`, `--skip-rtk`: skip the corresponding machine step.
- `--skip-verify`: skip the repository suites, which bootstrap runs last by default.

Re-runs preserve correct links and unchanged owned regions. A target owned by somebody else or a
modified fenced region is refused. Bootstrap also removes its stale skill links after a skill disappears.

Project installs add fenced ignore rules for skill symlinks and a shared instruction region in
`AGENTS.md`. A regular `CLAUDE.md` imports it with `@AGENTS.md`, using
[Claude's import syntax](https://code.claude.com/docs/en/memory). The installer replaces its old flavor
region in `CLAUDE.md` with that import and preserves other prose in both files. Commit both files
and the ignore regions.

Removing one client keeps shared instructions while the other still has mounts. Removing the last
removes only installer-owned regions. Each client has its own ignore region; a broad ignore rule
for the client's whole directory is reported and left unchanged.

The wiring check and statistics tool also require `--agent=claude|codex`:

```sh
~/.kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent=codex ai
~/.kk-flavor/skills/kk-reduce/scripts/stats.sh --agent=codex ai
```

In skill commands, set `ECO_AGENT` explicitly to supply that argument; set `JUDGE_PROVIDER`
separately for model calls. Reports name instructions excluded from measurement; audit those
installed files separately. Skills restricted to explicit invocation carry Claude frontmatter and
[Codex invocation policy](https://learn.chatgpt.com/docs/build-skills) in
`agents/openai.yaml`.

## By hand

- [Claude Code](https://code.claude.com)
  - Mount the shared standards, templates, scripts and skills: `ln -s ~/Documents/WP/configs/ai/kk-flavor ~/.kk-flavor`
  - Add this line to `~/.claude/CLAUDE.md`: ``Read `~/.kk-flavor/inject.md` now and follow it``. For owner instructions, use `ai/bootstrap-owner.sh --agent=claude`; it installs a regular copy and tracks it for upgrades.
  - Mount each skill under `ai/kk-flavor/skills/`: `mkdir -p ~/.claude/skills && for d in ~/Documents/WP/configs/ai/kk-flavor/skills/*/; do ln -sfn "${d%/}" ~/.claude/skills/; done`
  - Install the Go tools the skills run (needs `gh`, not Go): `~/Documents/WP/configs/ai/tools/install.sh`. Re-run after a new release. Skip it and the skills build from source on first use, which does need Go.
  - Sync the MCP files described above with `~/Documents/WP/configs/ai/mcp-sync.sh --agent=claude`. This needs `jq` (`brew install jq`) and registers servers for every project in the CLI and IDE. Re-run after editing either file. HTTP registration does not contact the server. If it shows `! Needs authentication`, run `/mcp` in an interactive session to log in.
  - The `chrome-devtools` server drives the Chrome you already have open. Turn remote debugging on once at `chrome://inspect/#remote-debugging` (Chrome 144+). While it's on, any session can reach that profile, so untick it when you're done.
- [RTK](https://github.com/rtk-ai/rtk) — compresses CLI output before the agent reads it
  - `brew install rtk`
  - `rtk init --agent claude --global --hook-only --auto-patch`, then restart Claude Code
- [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) provides a code graph for reachability questions.
  - Download `codebase-memory-mcp-darwin-arm64.tar.gz` from a release and verify provenance: `gh attestation verify <file> --repo DeusData/codebase-memory-mcp`. Read the signer workflow path from a release run and pin it with `--signer-workflow`, as `ai/tools/install.sh` does. The release's `checksums.txt` checks consistency, not provenance; `--repo` alone permits any signing workflow in that repository.
  - Unpack it to `~/.local/bin/codebase-memory-mcp` (~283 MB) and `chmod +x` it
  - Do not run its `install` subcommand: it registers with every agent client it finds. Use the CLI only; the MCP tool schema costs ~6k tokens per session.
  - Confine it: `CBM_ALLOWED_ROOT=~/Documents/WP` makes it refuse a path outside that tree
  - Remove with `rm ~/.local/bin/codebase-memory-mcp && rm -rf ~/.cache/codebase-memory-mcp`
  - Not part of `bootstrap.sh`: the download is a release asset you verify by hand

## Remove

Pass the same client selector used for installation:

```sh
ai/install-project.sh --agent=codex --uninstall ~/code/thing
ai/bootstrap.sh --agent=codex --uninstall
ai/bootstrap.sh --agent=claude --uninstall
```

Uninstall removes owned symlinks and fenced regions, preserving surrounding instructions. For an
owner installation, use `ai/bootstrap-owner.sh --agent=claude|codex --uninstall` to remove its instruction copy.
Bootstrap lists recorded projects still using the checkout; uninstall those before deleting it.

Tool binaries live in `ai/tools/bin/` and go with the checkout. Uninstall preserves client sessions,
settings, Brew dependencies, native RTK setup and MCP connections. List Claude connections with
`claude mcp list` and remove them with `claude mcp remove <name> -s user`.

Remove Codex RTK setup with `rtk init --codex --global --uninstall`; remove a Codex MCP connection
with `codex mcp remove <name>`. Owner memory is preserved.
