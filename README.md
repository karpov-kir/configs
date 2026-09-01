# Overview & installation

Config files for the tools I use:

`./bootstrap.sh` does the steps below on a fresh machine, and `--dry-run` prints what it would change first. Four steps stay yours to run by hand, because they write outside what this repository owns: the two `git clone`s, `mise use --global`, and `rtk init -g`.

Any target you didn't link yourself, it reports and skips rather than replacing — so the individual commands below still matter when a step fails. It won't repoint a machine that's already set up from another checkout, either. Run it from a scratch clone and it refuses before writing anything. Otherwise every link would swing over to the clone, and deleting the clone would leave you with no shell config. `--relocate` is how you say you mean it.

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
  - Install the Go tools the skills run (needs `gh`, not Go): `~/Documents/WP/configs/ai/tools/install.sh`. Re-run after a new release. Skip it and the skills build from source on first use, which does need Go.
  - MCP servers: `ai/mcp.jsonc` is the public source of truth. Machine-private servers for internal hosts sit beside it in `ai/mcp.private.jsonc`, gitignored and the same shape. Claude Code has no global MCP file to symlink, so `~/Documents/WP/configs/ai/mcp-sync.sh` syncs both into the user scope. That covers every project, in the CLI and the IDE. Re-run it after editing either file. Needs `jq` (`brew install jq`).
  - The `chrome-devtools` server drives the Chrome you already have open. Turn remote debugging on once at `chrome://inspect/#remote-debugging` (Chrome 144+). While it's on, any session can reach that profile, so untick it when you're done.
- [RTK](https://github.com/rtk-ai/rtk) — compresses CLI output before Claude Code reads it
  - `brew install rtk`
  - `rtk init -g`, then restart Claude Code
- [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) — a code graph, for the reachability questions `grep` answers a round at a time
  - Download `codebase-memory-mcp-darwin-arm64.tar.gz` from a release, check it against `checksums.txt`, and verify provenance: `gh attestation verify <file> --repo DeusData/codebase-memory-mcp`
  - Unpack it to `~/.local/bin/codebase-memory-mcp` (~283 MB) and `chmod +x` it
  - **Do not run its `install` subcommand.** It wires itself into every agent client it can find. This machine reaches it by CLI only, on purpose: as an MCP server its tool schema costs ~6k tokens in every session
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
  - Ghostty's built-in Nerd Font fallback renders the default icon glyphs, so there's no font to install.

## Tests

`ai/run-tests.sh` runs every `*-test.sh` in the repo. It finds them rather than listing them, so a suite is covered the day it's written, and it exits non-zero if it finds none. GitHub Actions runs it on pushes to `main` and on pull requests, over Linux and macOS, alongside the repo's other gates in `.github/workflows/gates.yml`.
