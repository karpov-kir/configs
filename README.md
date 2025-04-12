# Overview & installation

Config files for the tools I use:

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
- [ZSH](https://zsh.org)
  - It's already installed by default on MacOS
  - [Prezto](https://github.com/sorin-ionescu/prezto)
    - `git clone --recursive https://github.com/sorin-ionescu/prezto.git "${ZDOTDIR:-$HOME}/.zprezto"`
    - `rm -f ~/.zpreztorc && ln -s ~/Documents/WP/configs/zsh/.zpreztorc ~/.zpreztorc`
  - `rm -f ~/.zshrc && ln -s ~/Documents/WP/configs/zsh/.zshrc ~/.zshrc`
