# Overview & installation

Config files for the tools I use:

`./bootstrap.sh` does the steps below on a fresh machine, bar those that write files this repository does not own — `rtk init -g` stays yours to run. `--dry-run` shows what it would change before it changes anything. It skips and reports any target you did not link yourself rather than replacing it, so the individual commands still matter when a step fails.

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
  - `cp ~/Documents/WP/configs/ai/RTK.md ~/.claude/RTK.md`, after `rtk init -g` — which writes its own longer template there and rewrites it on every run. `ai/CLAUDE.md` imports that path into every session, so this repository owns what it says. Copied rather than symlinked, unlike every other mount here: the always-loaded budget refuses a symlink at an import's mount and stops counting the file. `./bootstrap.sh` does this step too, so re-run it after any later `rtk init -g`.
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
  - Default icon glyphs render via Ghostty's built-in Nerd Font fallback — no font change needed.

## Tests

`ai/run-tests.sh` runs every `*-test.sh` in the repo. It discovers them rather than listing them, so a suite is covered the day it is written, and it exits non-zero if it finds none — a runner that quietly matches nothing would report a clean tree it never read. GitHub Actions runs it on pushes to `main` and on pull requests, over Linux and macOS, alongside a `gofmt`/`vet`/`test` gate for the Go tools — `.github/workflows/gates.yml`. `release-tools.yml` gates those tools again at release time, since a release must not attach binaries built from unchecked code.
