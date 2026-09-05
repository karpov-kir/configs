# env — shell, terminal, editor

`env/bootstrap.sh` does everything below on a fresh machine, and `--dry-run` prints what it would
change first. Three steps stay yours to run by hand, because they write outside what this repository
owns: the two `git clone`s and `mise use --global`.

Any target you didn't link yourself, it reports and skips rather than replacing, so the individual
commands below still matter when a step fails.

- [ZSH](https://zsh.org)
  - It's already installed by default on MacOS
  - [Prezto](https://github.com/sorin-ionescu/prezto)
    - `git clone --recursive https://github.com/sorin-ionescu/prezto.git "${ZDOTDIR:-$HOME}/.zprezto"`
    - `rm -f ~/.zpreztorc && ln -s ~/Documents/WP/configs/env/zsh/.zpreztorc ~/.zpreztorc`
  - `brew install zsh-autocomplete`
  - `mkdir -p ~/.zprezto-contrib`
  - `git clone --depth 1 https://github.com/marlonrichert/zsh-autocomplete ~/.zprezto-contrib/zsh-autocomplete`
  - `rm -f ~/.zshrc && ln -s ~/Documents/WP/configs/env/zsh/.zshrc ~/.zshrc`
- [Git](https://git-scm.com)
  - It's already installed by default on MacOS
  - `ln -s ~/Documents/WP/configs/env/git/.gitconfig ~/.gitconfig`
- [Mise](https://github.com/jdx/mise)
  - `brew install mise`
  - `mise use --global node@lts`
  - `mise use --global go@latest`
- [HSTR](https://github.com/dvorka/hstr)
  - `brew install hstr`
- [Ghostty](https://ghostty.org)
  - `brew install --cask ghostty`
  - `rm -rf ~/.config/ghostty && ln -s ~/Documents/WP/configs/env/ghostty ~/.config/ghostty`
- [Neovim](https://neovim.io)
  - `brew install neovim`
  - `rm -rf ~/.config/nvim && ln -s ~/Documents/WP/configs/env/nvim ~/.config/nvim`
  - [`nvim/README.md`](nvim/README.md) lists the binaries the plugins shell out to
- [Starship](https://starship.rs) — prompt engine (replaces the Prezto `prompt` module)
  - `brew install starship`
  - `mkdir -p ~/.config`
  - `rm -f ~/.config/starship.toml && ln -s ~/Documents/WP/configs/env/starship/starship.toml ~/.config/starship.toml`
  - Ghostty's built-in Nerd Font fallback renders the default icon glyphs, so there's no font to install.

## Removing it

Everything this half puts in your home is a symlink into this repository, so deleting the links is the
whole of it:

```sh
rm -f ~/.zpreztorc ~/.zshrc ~/.gitconfig ~/.config/starship.toml
rm -f ~/.config/ghostty ~/.config/nvim
```

You'll want a `.zshrc` of some kind before your next login. The formulae above stay installed until you
`brew uninstall` them, and Prezto, the zsh-autocomplete clone and neovim's plugin cache
(`~/.local/share/nvim`) are yours to delete separately.
