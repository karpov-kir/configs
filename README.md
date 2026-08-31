# Overview & installation

Config files for the tools I use:

`./bootstrap.sh` does the steps below on a fresh machine. Four are yours to run by hand, because they write outside what this repository owns: the two `git clone`s, `mise use --global`, and `rtk init -g`. `--dry-run` shows what it would change before it changes anything. It skips and reports any target you did not link yourself rather than replacing it, so the individual commands still matter when a step fails. Run it from a second checkout — a scratch clone, say — and it refuses before writing anything, because every link it makes would be repointed at that copy and deleting the copy would then leave you with no shell config; `--relocate` is how you say you mean it.

- [ZSH](https://zsh.org)
  - It's already installed by default on MacOS
  - [Prezto](https://github.com/sorin-ionescu/prezto)
    - `git clone --recursive https://github.com/sorin-ionescu/prezto.git "${ZDOTDIR:-$HOME}/.zprezto"`
    - `rm -f ~/.zpreztorc && ln -s ~/Documents/WP/configs/zsh/.zpreztorc ~/.zpreztorc`
  - `brew install zsh-autocomplete`
  - `mkdir -p ~/.zprezto-contrib`
  - `git clone --depth 1 https://github.com/marlonrichert/zsh-autocomplete ~/.zprezto-contrib/zsh-autocomplete`
  - `rm -f ~/.zshrc && ln -s ~/Documents/WP/configs/zsh/.zshrc ~/.zshrc`
- [Claude Code](https://code.claude.com)
  - `ln -s ~/Documents/WP/configs/ai/CLAUDE.md ~/.claude/CLAUDE.md`
  - Mount the kk-flavor bucket (standards, config, and templates the skills read): `ln -s ~/Documents/WP/configs/ai/kk-flavor ~/.kk-flavor`
  - Install the skills (each is a dir under `ai/skills/`): `mkdir -p ~/.claude/skills && for d in ~/Documents/WP/configs/ai/skills/*/; do ln -sfn "${d%/}" ~/.claude/skills/; done`
  - Install the Go tools the skills run (no Go needed; requires `gh`): `~/Documents/WP/configs/ai/tools/install.sh`. Re-run after a new release. Without it the skills build from source on first use, which needs Go.
  - MCP servers: `ai/mcp.jsonc` is the public source of truth; machine-private servers (internal hosts) live beside it in `ai/mcp.private.jsonc` — gitignored, same shape. Claude Code has no global MCP file to symlink, so sync both into the user scope (applies to all projects, CLI + IDE) with `~/Documents/WP/configs/ai/mcp-sync.sh`. Re-run after editing either file. Requires `jq` (`brew install jq`).
  - The `chrome-devtools` server drives the Chrome you already have open. Turn remote debugging on once at `chrome://inspect/#remote-debugging` (Chrome 144+). While it's on, any session can reach that profile, so untick it when you're done.
- [RTK](https://github.com/rtk-ai/rtk) — compresses CLI output before Claude Code reads it
  - `brew install rtk`
  - `rtk init -g`, then restart Claude Code
- [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) — a code graph, for the reachability questions `grep` answers a round at a time. `~/.kk-flavor/standards/code-navigation.md` is what an agent reads before reaching for it.
  - Download `codebase-memory-mcp-darwin-arm64.tar.gz` from a release, check it against `checksums.txt`, and verify provenance: `gh attestation verify <file> --repo DeusData/codebase-memory-mcp`
  - Unpack it to `~/.local/bin/codebase-memory-mcp` (~283 MB) and `chmod +x` it
  - **Do not run its `install` subcommand.** It wires itself into every agent client it can find; this machine reaches it by CLI only, deliberately — as an MCP server it costs ~6k tokens of tool schema in every session, for a tool worth reaching for on a minority of tasks
  - Confine it: `CBM_ALLOWED_ROOT=~/Documents/WP` makes it refuse a path outside that tree
  - Remove with `rm ~/.local/bin/codebase-memory-mcp && rm -rf ~/.cache/codebase-memory-mcp`
- [Git](https://git-scm.com)
  - It's already installed by default on MacOS
  - `ln -s ~/Documents/WP/configs/git/.gitconfig ~/.gitconfig`
- [Mise](https://github.com/jdx/mise)
  - `brew install mise`
  - `mise use --global node@lts`
  - `mise use --global go@latest`
- [HSTR](https://github.com/dvorka/hstr)
  - `brew install hstr`
- [Ghostty](https://ghostty.org)
  - `brew install --cask ghostty`
  - `rm -rf ~/.config/ghostty && ln -s ~/Documents/WP/configs/ghostty ~/.config/ghostty`
- [Lazygit](https://github.com/jesseduffield/lazygit)
  - `brew install lazygit`
  - `rm -rf ~/.config/lazygit && ln -s ~/Documents/WP/configs/lazygit ~/.config/lazygit`
- [Neovim](https://neovim.io)
  - `brew install neovim`
  - `rm -rf ~/.config/nvim && ln -s ~/Documents/WP/configs/nvim ~/.config/nvim`
- [Zellij](https://zellij.dev)
  - `brew install zellij`
  - `rm -rf ~/.config/zellij && ln -s ~/Documents/WP/configs/zellij ~/.config/zellij`
- [Starship](https://starship.rs) — prompt engine (replaces the Prezto `prompt` module)
  - `brew install starship`
  - `mkdir -p ~/.config`
  - `rm -f ~/.config/starship.toml && ln -s ~/Documents/WP/configs/starship/starship.toml ~/.config/starship.toml`
  - Default icon glyphs render via Ghostty's built-in Nerd Font fallback, so no font change is needed.

## Tests

`ai/run-tests.sh` runs every `*-test.sh` in the repo. It discovers them rather than listing them, so a suite is covered the day it is written. It exits non-zero if it finds none, because a runner that quietly matches nothing would report a clean tree it never read. GitHub Actions runs it on pushes to `main` and on pull requests, over Linux and macOS, alongside a `gofmt`/`vet`/`test` gate for the Go tools (`.github/workflows/gates.yml`). `release-tools.yml` gates those tools again at release time.
