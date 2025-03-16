# Override `ghosty-term` or `kitty-term` so that all keys work over SSH
export TERM=xterm-256color
export VISUAL="nvim"
export EDITOR="nvim"

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

# Add WebOS binaries to PATH
# export PATH=$PATH:~/Documents/Other/webOS_TV_SDK/CLI/bin

# Add Tizen binaries to PATH
# export PATH=$PATH:~/Documents/Other/tizen-studio/tools:~/Documents/Other/tizen-studio/tools/ide/bin:~/Documents/Other/tizen-studio/package-manager

# Go access to private repos
export GOPRIVATE=github.com

# Add custom binaries to PATH
# export PATH=$PATH:~/Documents/Other/bin

# Add AWS MFA bash snippet alias
# https://bitmovin.atlassian.net/wiki/spaces/DEVOPS/pages/1607696505/AWS+MFA+Setup
if [[ -r "$HOME/Documents/Other/bin/aws-mfa-cli.sh" ]]; 
then
  alias aws-mfa-cli="source ~/Documents/Other/bin/aws-mfa-cli.sh"
fi

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
