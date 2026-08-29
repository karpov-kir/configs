#!/usr/bin/env bash
#
# Set this machine up from this repository: link every config into place, install what the links need,
# restore the generated files this repository owns the contents of, register the MCP servers, then
# verify the result by running the repository's own suites.
#
#   usage: bootstrap.sh [--dry-run] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-verify]
#
# Safe to re-run: every step checks the state it wants before changing anything, so a second run over
# a finished machine reports "ok" throughout and writes nothing.
#
# It refuses rather than deletes. README.md's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`,
# which is fine when a human types it having just looked at the directory, and is data loss when a
# script does it unattended on a machine that already had a real config there. A target this does not
# already own is reported and skipped, and the run exits non-zero with the list. The one exception is
# `~/.claude/RTK.md`, which holds generated text another tool rewrites unasked — the rtk step below
# carries why that path is owned rather than refused.
#
# It reaches other scripts rather than reimplementing them: `ai/tools/install.sh` for the Go tool
# binaries, `ai/mcp-sync.sh` for the MCP registry, `ai/run-tests.sh` to verify. Each of those owns its
# own contract and has its own suite; a second copy here would drift from all three.
#
# $HOME is read from the environment and never assumed, which is what lets the suite run the real
# linking logic against a throwaway home instead of faking it.
#
# tested by: bootstrap-test.sh
# untested: brew, gh and claude are external commands. Faking them would only assert the fake, so the
# suite covers the linking and the refusals and drives the three external steps behind --skip flags.
set -uo pipefail

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so `repo` comes back two
# lines long and every source path built from it resolves nowhere.
repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

dry_run=false
skip_brew=false
skip_tools=false
skip_mcp=false
skip_verify=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --skip-brew) skip_brew=true ;;
    --skip-tools) skip_tools=true ;;
    --skip-mcp) skip_mcp=true ;;
    --skip-verify) skip_verify=true ;;
    -h | --help)
      sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      printf 'bootstrap.sh: unknown option %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

# Refusals are collected rather than fatal: a machine missing one cask should still get every link,
# and a human fixing three named problems in one pass beats discovering them one run at a time.
refusals=()

say() { printf '%s\n' "$1"; }
refuse() {
  refusals+=("$1")
  printf '  REFUSED  %s\n' "$1"
}

# --- links ---------------------------------------------------------------------------------------

# Link `target` at `source`, and report which of the four states it found. The only state that writes
# over something is a symlink, which carries no data of its own.
link() {
  local source="$1" target="$2" parent current
  if [ ! -e "$source" ]; then
    refuse "$source is missing from the repository, so $target was left alone"
    return 1
  fi
  if [ -L "$target" ]; then
    # Trailing slashes stripped from both sides before comparing. README.md's skills loop linked with
    # `ln -sfn "$d"` for as long as the skills existed, where `$d` came from a `*/` glob, so every
    # machine set up from it holds links that read back as `…/kk-tighten/` while this script computes
    # `…/kk-tighten`. Same directory, different string: comparing raw reports each one as stale and
    # rewrites a correct link on every run, which is idempotence lost to a cosmetic difference.
    #
    # The README now documents `${d%/}`, so new machines will not have them. This stays for the ones
    # already set up: the readback is a property of links made years ago, not of the current README.
    current="$(readlink "$target")"
    if [ "${current%/}" = "${source%/}" ]; then
      say "  ok       $target"
      return 0
    fi
    if $dry_run; then
      say "  would repoint $target -> $source"
      return 0
    fi
    ln -sfn "$source" "$target" || {
      refuse "could not repoint $target at $source"
      return 1
    }
    say "  repointed $target"
    return 0
  fi
  if [ -e "$target" ]; then
    # The whole reason this script is not the README's `rm -rf`.
    refuse "$target exists and is not a symlink — move it aside, then re-run"
    return 1
  fi
  parent="$(dirname -- "$target")"
  if $dry_run; then
    say "  would link $target -> $source"
    return 0
  fi
  mkdir -p -- "$parent" || {
    refuse "could not create $parent"
    return 1
  }
  ln -s "$source" "$target" || {
    refuse "could not link $target at $source"
    return 1
  }
  say "  linked   $target"
}

say "links"
link "$repo/zsh/.zpreztorc" "$HOME/.zpreztorc"
link "$repo/zsh/.zshrc" "$HOME/.zshrc"
link "$repo/git/.gitconfig" "$HOME/.gitconfig"
link "$repo/ai/CLAUDE.md" "$HOME/.claude/CLAUDE.md"
link "$repo/ai/kk-flavor" "$HOME/.kk-flavor"
link "$repo/ghostty" "$HOME/.config/ghostty"
link "$repo/lazygit" "$HOME/.config/lazygit"
link "$repo/nvim" "$HOME/.config/nvim"
link "$repo/zellij" "$HOME/.config/zellij"
link "$repo/starship/starship.toml" "$HOME/.config/starship.toml"

# Discovery, not a list: a skill added tomorrow is mounted without anyone editing this file. The cost
# of discovery is that finding none would silently mount nothing, so that is a refusal.
say "skills"
skill_count=0
for dir in "$repo"/ai/skills/*/; do
  [ -d "$dir" ] || continue
  link "${dir%/}" "$HOME/.claude/skills/$(basename -- "${dir%/}")"
  skill_count=$((skill_count + 1))
done
[ "$skill_count" -gt 0 ] || refuse "no skill directories under $repo/ai/skills/ — nothing was mounted"

# --- packages ------------------------------------------------------------------------------------

if $skip_brew; then
  say "brew (skipped)"
elif ! command -v brew >/dev/null 2>&1; then
  refuse "brew is not installed, so no formula or cask was installed"
else
  say "brew"
  # Installed-first rather than `brew install` unconditionally: the latter is slow, noisy, and exits
  # non-zero on an already-installed cask, which would make a finished machine look broken.
  for formula in zsh-autocomplete rtk mise hstr lazygit neovim zellij starship jq; do
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

# --- rtk ------------------------------------------------------------------------------------------

# `~/.claude/RTK.md` is not a config this machine happens to hold: `@RTK.md` in ai/CLAUDE.md imports
# it into every session, so its contents are always-loaded context, paid for on every task in every
# project. `rtk init -g` writes its own template there — five times the lines — and rewrites it on
# every re-run, so the trimmed version is not something a machine keeps by having once been given it.
# This restores it, and a re-run after `rtk init -g` is what makes that self-healing.
#
# Copied rather than linked, unlike every target above. The always-loaded budget resolves that import
# at the mount and refuses a symlink found there, leaving the file named but uncounted — so a link
# would cost the measurement that the file exists to be measured by.
#
# Written over whatever is there, which is the one place this script overwrites data. That path holds
# generated text with exactly two possible authors, rtk and this repository, and this repository is
# now the one that owns it: the version to edit is ai/RTK.md, and a hand-edit at the mount is a change
# the next `rtk init -g` would discard anyway.
#
# Unconditional, rather than gated on rtk being on PATH. The `@RTK.md` import is committed in
# ai/CLAUDE.md whether or not this machine has rtk yet, so a probe would leave the import naming a
# file this repository chose not to put there — and would make what every session loads depend on an
# external command, which is the one thing here the suite could then no longer check.
say "rtk"
rtk_md="$HOME/.claude/RTK.md"
if [ ! -f "$repo/ai/RTK.md" ]; then
  refuse "$repo/ai/RTK.md is missing from the repository, so $rtk_md was left alone"
elif [ -L "$rtk_md" ]; then
  refuse "$rtk_md is a symlink — an import's mount must be a real file or it goes uncounted; move it aside, then re-run"
elif cmp -s "$repo/ai/RTK.md" "$rtk_md"; then
  say "  ok       $rtk_md"
elif $dry_run; then
  say "  would write $rtk_md from ai/RTK.md"
elif mkdir -p -- "$HOME/.claude" && cp -- "$repo/ai/RTK.md" "$rtk_md"; then
  say "  wrote    $rtk_md"
else
  refuse "could not write $rtk_md from $repo/ai/RTK.md"
fi

# --- the repository's own tools ------------------------------------------------------------------

# `ai/tools/install.sh` downloads the prebuilt Go tools and verifies each against the release's own
# SHA256SUMS. Reached, never reimplemented: it owns that contract and has its own suite.
if $skip_tools; then
  say "tools (skipped)"
elif $dry_run; then
  say "tools: would run ai/tools/install.sh"
elif ! command -v gh >/dev/null 2>&1; then
  refuse "gh is not installed, so ai/tools/install.sh could not fetch the tool binaries"
else
  say "tools"
  "$repo/ai/tools/install.sh" || refuse "ai/tools/install.sh failed"
fi

# --- MCP registry --------------------------------------------------------------------------------

# `ai/mcp-sync.sh` owns the registry, including the JSONC comment stripping and the private-file
# layering. CLAUDE_CONFIG_DIR is what lets a test point the `claude` CLI somewhere harmless.
if $skip_mcp; then
  say "mcp (skipped)"
elif $dry_run; then
  say "mcp: would run ai/mcp-sync.sh"
elif ! command -v claude >/dev/null 2>&1; then
  refuse "the claude CLI is not on PATH, so the MCP servers were not registered"
else
  say "mcp"
  "$repo/ai/mcp-sync.sh" || refuse "ai/mcp-sync.sh failed"
fi

# --- verify --------------------------------------------------------------------------------------

# A setup script that reports success without checking anything is the failure this repository has
# spent the day removing, so the last step is the repository's own suites over what was just linked.
#
# The re-entry guard is structural, not tidiness. `ai/run-tests.sh` discovers every `*-test.sh`,
# `bootstrap-test.sh` is one of them, and it runs this script — so verify reaches a suite that reaches
# verify. It terminates today only because every case in that suite remembers `--skip-verify`, which
# is a loop held open by a convention. The marker closes it whatever any caller passes.
if [ -n "${BOOTSTRAP_VERIFYING:-}" ]; then
  say "verify (skipped: already inside a verify run)"
elif $skip_verify; then
  say "verify (skipped)"
elif [ ! -x "$repo/ai/run-tests.sh" ]; then
  # A missing runner must not be reported as a failing suite. Without this the call exits 127 and the
  # arm below blames the suites for a file that was never there — a false diagnosis pointing at code
  # that is fine, which costs more than the silence would. Checked in the dry run too: a dry run that
  # says "ok" over a repository where the real run cannot work is the same lie one step earlier.
  refuse "ai/run-tests.sh is not in this checkout — the suites were not run, which is not the same as passing"
elif $dry_run; then
  say "verify: would run ai/run-tests.sh"
else
  say "verify"
  BOOTSTRAP_VERIFYING=1 "$repo/ai/run-tests.sh" >/dev/null ||
    refuse "ai/run-tests.sh reported a failing suite"
fi

# --- result --------------------------------------------------------------------------------------

echo
if [ "${#refusals[@]}" -eq 0 ]; then
  say "bootstrap: ok"
  exit 0
fi
printf 'bootstrap: %s thing(s) need you:\n' "${#refusals[@]}"
for item in "${refusals[@]}"; do
  printf '  - %s\n' "$item"
done
exit 1
