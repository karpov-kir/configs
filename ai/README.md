# ai — Claude Code, the standards, the skills

This file is how you install it. [`field-guide.html`](field-guide.html) is what it does and which
skill to reach for — open it in a browser. That page is generated from the skills themselves by
`ai/guide.sh`, so it cannot fall behind them; the prose around the list is hand-written, in
`ai/tools/eco-guide/field-guide.template.html`.

## Which install you want

There are three tiers and two scopes, and they are separate questions.

**The tiers.** By default you get the skills that work in any repository. `--maintainer` adds the few
that exist to maintain this instruction tree itself — they declare `audience: maintainer` in their own
frontmatter, and they do nothing for a repository that merely uses the tree. Leaving them out is the
default because every skill's `description:` is loaded in every session whether or not it is invoked,
so an unusable skill is a standing cost. `ai/bootstrap-owner.sh` is the third tier: `--maintainer`
plus `rtk` and this checkout's own `CLAUDE.md` as the machine's instruction file. That one is for the
person who owns this repository, not for anyone installing from it.

**The scopes.** `ai/bootstrap.sh` installs machine-wide: the skills land in `~/.claude/skills/` and
every session on the machine loads them. `ai/install-project.sh <project>` installs into one project
instead — the skills land in `<project>/.claude/skills/` and cost nothing in your other repositories.
The machine-wide half that a project install still needs (`~/.kk-flavor`, the Go tools, the MCP
servers, `jq`) comes from `ai/bootstrap.sh`, so run that once and the project script once per project.

Both are safe to re-run: a second run over a finished machine or project reports "ok" throughout and
writes nothing.

```sh
ai/bootstrap.sh                       # machine-wide, the any-repo skills
ai/bootstrap.sh --maintainer          # and the ones that maintain this tree
ai/bootstrap-owner.sh                 # the owner's machine: maintainer + rtk + this CLAUDE.md
ai/install-project.sh ~/code/thing    # one project, after ai/bootstrap.sh has run once
```

## Installing from an agent

If you are Claude, and someone asks you to install this into their projects, this is the whole of it.
Clone the repository somewhere permanent — **not** a scratch directory, because every mount points
back into it and deleting it breaks them all — then:

1. `ai/bootstrap.sh` once, from the clone. This is the machine-wide half and needs `brew`, `gh` and
   the `claude` CLI. Its `--skip-brew`, `--skip-tools` and `--skip-mcp` flags turn off the steps that
   reach the network if any of those is missing.
2. `ai/install-project.sh <project>` once per project named.

Do not pass `--maintainer` or run `ai/bootstrap-owner.sh` unless the person asked for them by name.
Report any line the run prints as `REFUSED` rather than working around it: each one names a target the
scripts will not take over, and each is a decision for the human whose machine it is.

## What a project install puts in the project

- A symlink per skill under `<project>/.claude/skills/`, pointing back at the clone. Mounted rather
  than copied, so one tree serves every project and an update reaches all of them at once.
- Ignore rules for those symlinks in `<project>/.gitignore`, fenced with `# kk-flavor:begin`. If the
  project already ignores `.claude/` wholesale, the run reports that and adds nothing — that rule
  covers the project's own settings too, so what to do about it is a human's call.
- A short region in `<project>/CLAUDE.md`, fenced with `<!-- kk-flavor:begin -->`, pointing at
  `~/.kk-flavor/inject.md`. The fences are how a re-run recognises its own work and how an uninstall
  finds it again; edit inside them and the next run refuses rather than overwriting you.

The `.gitignore` lines and the `CLAUDE.md` region are meant to be committed. A colleague who clones
the project without installing anything is unaffected: the region names a path they do not have, and
Claude Code skips an instruction file it cannot find.

**One known rough edge.** A symlinked skill loads and is invocable, but Claude Code has had issues
listing symlinked skills in `/` autocomplete. If a skill does not appear when you type `/`, invoke it
by name — it is there.

## The flags

The flags and the refusals both bootstrap scripts share are in the repository's root `README.md`; a
target this one reports and skips is still yours to link with the commands below. The last thing
`ai/bootstrap.sh` does is run the repository's own suites over what it just linked; `--skip-verify`
turns that off, and `--skip-brew`, `--skip-tools` and `--skip-mcp` turn off the steps that reach the
network.

Because the skills below are mounted by discovery, renaming or deleting one leaves its old link
behind. Each run removes those, and names what it removed — only a link it would have written itself:
an absolute symlink under `~/.claude/skills/`, pointing into this checkout's `ai/kk-flavor/skills/`,
whose directory is gone. Everything else it leaves. So if the wiring check
(`ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh`) keeps naming a mount no run drops, remove that
one by hand.

## By hand

- [Claude Code](https://code.claude.com)
  - Mount the kk-flavor bucket — standards, templates, scripts, and the skills themselves: `ln -s ~/Documents/WP/configs/ai/kk-flavor ~/.kk-flavor`
  - Point your `~/.claude/CLAUDE.md` at it, by adding a line reading ``Read `~/.kk-flavor/inject.md` now and follow it``. The owner's machine symlinks this checkout's own file there instead: `ln -s ~/Documents/WP/configs/ai/CLAUDE.md ~/.claude/CLAUDE.md`
  - Install the skills (each is a dir under `ai/kk-flavor/skills/`): `mkdir -p ~/.claude/skills && for d in ~/Documents/WP/configs/ai/kk-flavor/skills/*/; do ln -sfn "${d%/}" ~/.claude/skills/; done`
  - Install the Go tools the skills run (needs `gh`, not Go): `~/Documents/WP/configs/ai/tools/install.sh`. Re-run after a new release. Skip it and the skills build from source on first use, which does need Go.
  - MCP servers: `ai/mcp.jsonc` is the public source of truth. Machine-private servers for internal hosts sit beside it in `ai/mcp.private.jsonc`, gitignored and the same shape. Claude Code has no global MCP file to symlink, so `~/Documents/WP/configs/ai/mcp-sync.sh` syncs both into the user scope. That covers every project, in the CLI and the IDE. Re-run it after editing either file. Needs `jq` (`brew install jq`). An `http` server registers without ever being contacted, so it lands as `! Needs authentication`: run `/mcp` in an interactive session and complete its login once.
  - The `chrome-devtools` server drives the Chrome you already have open. Turn remote debugging on once at `chrome://inspect/#remote-debugging` (Chrome 144+). While it's on, any session can reach that profile, so untick it when you're done.
- [RTK](https://github.com/rtk-ai/rtk) — compresses CLI output before Claude Code reads it
  - `brew install rtk`
  - `rtk init -g`, then restart Claude Code
- [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) — a code graph, for the reachability questions `grep` answers a round at a time
  - Download `codebase-memory-mcp-darwin-arm64.tar.gz` from a release and verify provenance: `gh attestation verify <file> --repo DeusData/codebase-memory-mcp`. `checksums.txt` ships in that same release, so a matching hash only says the two files agree with each other — the attestation is the only thing saying where the binary came from, and `--repo` on its own is weaker than it reads: any workflow in that repository holding `id-token: write` can sign for it. Read the signer workflow's path off a release run and pin it with `--signer-workflow`, the way `ai/tools/install.sh` pins this repository's own
  - Unpack it to `~/.local/bin/codebase-memory-mcp` (~283 MB) and `chmod +x` it
  - **Do not run its `install` subcommand.** It wires itself into every agent client it can find. This machine reaches it by CLI only, on purpose: as an MCP server its tool schema costs ~6k tokens in every session
  - Confine it: `CBM_ALLOWED_ROOT=~/Documents/WP` makes it refuse a path outside that tree
  - Remove with `rm ~/.local/bin/codebase-memory-mcp && rm -rf ~/.cache/codebase-memory-mcp`
  - Not part of `bootstrap.sh`: the download is a release asset you verify by hand

## Removing it

Each install has a mode that takes itself back out, over the same table it went in by — so nothing is
left behind because an uninstall re-derived the list and got it wrong.

```sh
ai/install-project.sh --uninstall ~/code/thing   # one project
ai/bootstrap.sh --uninstall                      # the machine-wide half
```

A project uninstall removes the skill symlinks, the `.gitignore` rules and the `CLAUDE.md` region —
its own fenced lines only, never anything you wrote beside them. A machine-wide uninstall removes the
mounts and, unless you are the owner, the region it wrote in `~/.claude/CLAUDE.md`. Both remove a
symlink only when it resolves back into this checkout: anything else is reported and left, on the same
rule the install follows.

`ai/bootstrap.sh --uninstall` also names every project still holding mounts from this checkout, read
from `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/installs`. Those mounts would dangle the moment the
checkout goes, so uninstall each project before deleting it.

The tool binaries live in `ai/tools/bin/` inside this checkout, so they go with the checkout. What
outlives it:

- The MCP servers: `claude mcp list` to see what the sync registered, then
  `claude mcp remove <name> -s user` for each one.
- `jq` and `rtk` stay installed, and there is no suggestion here to remove them. A brew formula is
  shared and unrefcounted: nothing records whether this machine already had one or what else depends
  on it now, so uninstalling on a guess breaks unrelated tooling while leaving a small CLI in place
  costs nothing. Treat them as dependencies this repository may have installed, and decide yourself.

Nothing here touches `~/.claude/projects`, `~/.claude/settings.json` or anything else Claude Code
writes for itself.
