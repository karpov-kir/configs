#!/usr/bin/env bash
#
# Set this machine's shell and editor environment up from this repository: link every config in env/
# into place and install what those links need.
#
#   usage: env/bootstrap.sh [--dry-run] [--relocate] [--skip-brew]
#
# Safe to re-run: every step checks the state it wants before changing anything, so a second run over
# a finished machine reports "ok" throughout and writes nothing.
#
# It will not move a machine that is already mounted from somewhere else. Run from a second checkout —
# a scratch clone, a colleague's copy — every link this script writes would be repointed at the copy,
# and deleting the copy afterwards leaves the human with no shell config and no git config. That is
# refused before anything is written; `--relocate` is how you say you mean it.
#
# It refuses rather than deletes. env/README.md's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`,
# which is fine when a human types it having just looked at the directory, and is data loss when a
# script does it unattended on a machine that already had a real config there. A target this does not
# already own is reported and skipped, and the run exits non-zero with the list.
#
# Independent of ai/bootstrap.sh in both directions: neither reads the other's mounts, and either half
# can be installed on a machine that never gets the other.
#
# tested by: bootstrap-test.sh
# untested: brew is an external command. Faking it would only assert the fake, so the suite covers the
# linking and the refusals and drives the brew step behind --skip-brew.
set -uo pipefail

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so `repo` comes back two
# lines long and every source path built from it resolves nowhere.
repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# What a second copy of this repository is recognised by. Read from the running file rather than
# written down, so a rename cannot leave the guard looking for a name nothing has.
script_name="$(basename -- "${BASH_SOURCE[0]}")"
label="env bootstrap"

dry_run=false
relocate=false
skip_brew=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --relocate) relocate=true ;;
    --skip-brew) skip_brew=true ;;
    -h | --help)
      sed -n '3,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      printf 'env/bootstrap.sh: unknown option %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

# Refused by name rather than left to `.` failing. Without `set -e` a missing library carries on into
# `add_cfg: command not found` six times over and exits 127, naming neither the file that is gone nor
# what to do about it — the same false diagnosis the verify step in ai/bootstrap.sh guards against.
# env/ is copied out of this repository on its own, so a checkout without lib/ is a real one.
[ -r "$repo/../lib/mount.sh" ] || {
  printf 'env/bootstrap.sh: lib/mount.sh is missing from this checkout — env/ and lib/ install together, and nothing was linked\n' >&2
  exit 2
}
# shellcheck source=../lib/mount.sh
. "$repo/../lib/mount.sh"

# --- the mount table ------------------------------------------------------------------------------

add_cfg "$repo/zsh/.zpreztorc" "$HOME/.zpreztorc"
add_cfg "$repo/zsh/.zshrc" "$HOME/.zshrc"
add_cfg "$repo/git/.gitconfig" "$HOME/.gitconfig"
add_cfg "$repo/ghostty" "$HOME/.config/ghostty"
add_cfg "$repo/nvim" "$HOME/.config/nvim"
add_cfg "$repo/starship/starship.toml" "$HOME/.config/starship.toml"

mount_run

# --- packages ------------------------------------------------------------------------------------

if $skip_brew; then
  say "brew (skipped)"
elif ! command -v brew >/dev/null 2>&1; then
  refuse "brew is not installed, so no formula or cask was installed"
else
  say "brew"
  # Installed-first rather than `brew install` unconditionally: the latter is slow, noisy, and exits
  # non-zero on an already-installed cask, which would make a finished machine look broken.
  for formula in zsh-autocomplete mise hstr neovim starship; do
    if brew list --formula "$formula" >/dev/null 2>&1; then
      say "  ok       $formula"
    elif $dry_run; then
      say "  would install $formula"
    else
      brew install "$formula" >/dev/null || refuse "brew install $formula failed"
    fi
  done
  for cask in ghostty; do
    if brew list --cask "$cask" >/dev/null 2>&1; then
      say "  ok       $cask"
    elif $dry_run; then
      say "  would install --cask $cask"
    else
      brew install --cask "$cask" >/dev/null || refuse "brew install --cask $cask failed"
    fi
  done
fi

# --- result --------------------------------------------------------------------------------------

report_and_exit
