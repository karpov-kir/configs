# ai — Claude Code, the standards, the skills

`ai/bootstrap.sh` does everything below on a fresh machine, apart from the steps that write outside
what this repository owns — `rtk init -g` and the codebase-memory-mcp download among them.

The flags and the refusals both bootstrap scripts share are in the repository's root `README.md`; a
target this one reports and skips is still yours to link with the commands below. The last thing it
does is run the repository's own suites over what it just linked; `--skip-verify` turns that off, and
`--skip-brew`, `--skip-tools` and `--skip-mcp` turn off the steps that reach the network.

- [Claude Code](https://code.claude.com)
  - `ln -s ~/Documents/WP/configs/ai/CLAUDE.md ~/.claude/CLAUDE.md`
  - Mount the kk-flavor bucket (standards, config, and templates the skills read): `ln -s ~/Documents/WP/configs/ai/kk-flavor ~/.kk-flavor`
  - Install the skills (each is a dir under `ai/skills/`): `mkdir -p ~/.claude/skills && for d in ~/Documents/WP/configs/ai/skills/*/; do ln -sfn "${d%/}" ~/.claude/skills/; done`
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

The mounts are symlinks into this repository, so deleting the links is the whole of that half:

```sh
rm -f ~/.claude/CLAUDE.md ~/.kk-flavor
for link in ~/.claude/skills/*; do
  case "$(readlink "$link")" in */configs/ai/skills/*) rm -f "$link" ;; esac
done
```

The tool binaries live in `ai/tools/bin/` inside this checkout, so they go with the checkout. What
outlives it:

- The MCP servers: `claude mcp list` to see what the sync registered, then
  `claude mcp remove <name> -s user` for each one.
- `rtk` and `jq`, if nothing else on the machine wants them: `brew uninstall rtk jq`, plus
  `rm -f ~/.claude/RTK.md` for whatever `rtk init -g` left behind.

Nothing here touches `~/.claude/projects`, `~/.claude/settings.json` or anything else Claude Code
writes for itself.
