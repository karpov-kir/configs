# Override `ghosty-term` or `kitty-term` so that all keys work over SSH
export TERM=xterm-256color
export VISUAL="nvim"
export EDITOR="nvim"

export PATH="/Users/kirill/.bun/bin:$PATH"

# E.g. Claude Code native installation is here
export PATH="$HOME/.local/bin:$PATH"

# Shared local dev infra (one Traefik + Postgres for every service); task start:dev routes + creates its DB through it
export DEV_INFRA="$HOME/Documents/WP/dev-infra"

# https://stackoverflow.com/a/64351976
# zmodload zsh/zprof

if [[ -f "/opt/homebrew/bin/brew" ]]; then
  eval "$(/opt/homebrew/bin/brew shellenv)"
  # Add Google Cloud SDK to PATH
  source "$(brew --prefix)/share/google-cloud-sdk/path.zsh.inc"
  source "$(brew --prefix)/share/google-cloud-sdk/completion.zsh.inc"
fi

eval "$(mise activate zsh)"

# Source Prezto
if [[ -s "${ZDOTDIR:-$HOME}/.zprezto/init.zsh" ]]; then
  source "${ZDOTDIR:-$HOME}/.zprezto/init.zsh"
fi

# Starship prompt (must init after Prezto so its precmd hook wins)
eval "$(starship init zsh)"

# Use menu-complete for tab completion, and cycle through options with Tab and Shift-Tab
bindkey              '^I' menu-select
bindkey "$terminfo[kcbt]" menu-select
bindkey -M menuselect  '^[[D' .backward-char  '^[OD' .backward-char
bindkey -M menuselect  '^[[C'  .forward-char  '^[OC'  .forward-char

# Go access to private repos
export GOPRIVATE=github.com

# Add Go binaries to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# HSTR configuration
alias hh=hstr
# Skip cmds w/ leading space from history
setopt histignorespace
# Get more colors
export HSTR_CONFIG=hicolor
# Bind hstr to Ctrl-r (for Vi mode check doc)
bindkey -s "\C-r" "\C-a hstr -- \C-j"
export HSTR_TIOCSTI=y
