# Overview & installation

Config files for the tools I use:

- [Git](https://git-scm.com)
  - It's already installed by default on MacOS
  - `ln -s ~/Documents/WP/configs/git/.gitconfig ~/.gitconfig`
- [Mise](https://github.com/jdx/mise)
  - `brew install mise`
  - `mise use --global node@lts`
  - `mise use --global go@latest`
- [ZSH](https://zsh.org)
  - It's already installed by default on MacOS
  - [Prezto](https://github.com/sorin-ionescu/prezto)
    - `git clone --recursive https://github.com/sorin-ionescu/prezto.git "${ZDOTDIR:-$HOME}/.zprezto"`
    - `rm -f ~/.zpreztorc && ln -s ~/Documents/WP/configs/zsh/.zpreztorc ~/.zpreztorc`
  - `rm -f ~/.zshrc && ln -s ~/Documents/WP/configs/zsh/.zshrc ~/.zshrc`
- [HSTR](https://github.com/dvorka/hstr)
  - `brew install hstr`
- [Ghostty](https://ghostty.org)
  - Download and install from https://ghostty.org
  - `rm -rf ~/.config/ghostty && ln -s ~/Documents/WP/configs/ghostty ~/.config/ghostty`
- [Lazygit](https://github.com/jesseduffield/lazygit)
  - `brew install lazygit`
  - `rm -rf ~/.config/lazygit && ln -s ~/Documents/WP/configs/lazygit ~/.config/lazygit`
- [Neovim](https://neovim.io/)
  - `brew install neovim`
  - `rm -rf ~/.config/nvim && ln -s ~/Documents/WP/configs/nvim ~/.config/nvim`
- [Tmux](https://github.com/tmux/tmux)
  - `brew install tmux`
  - `rm -rf ~/.config/tmux && ln -s ~/Documents/WP/configs/tmux ~/.config/tmux`
  - [TPM](https://github.com/tmux-plugins/tpm)
    - `git clone https://github.com/tmux-plugins/tpm ~/.tmux/plugins/tpm`
