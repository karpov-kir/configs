#!/usr/bin/env bash
#
# Set this machine up from this repository: link every config into place, install what the links need,
# clear the one file it used to write into your home and no longer owns, register the MCP servers, then
# verify the result by running the repository's own suites.
#
#   usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew] [--skip-tools] [--skip-mcp] [--skip-verify]
#
# Safe to re-run: every step checks the state it wants before changing anything, so a second run over
# a finished machine reports "ok" throughout and writes nothing.
#
# It will not move a machine that is already mounted from somewhere else. Run from a second checkout —
# a scratch clone, a colleague's copy — every link this script writes would be repointed at the copy,
# and deleting the copy afterwards leaves the human with no shell config, no git config and no agent
# instructions. That is refused before anything is written; `--relocate` is how you say you mean it.
#
# It refuses rather than deletes. README.md's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`,
# which is fine when a human types it having just looked at the directory, and is data loss when a
# script does it unattended on a machine that already had a real config there. A target this does not
# already own is reported and skipped, and the run exits non-zero with the list. The one thing it does
# remove is `~/.claude/RTK.md`, a file this repository used to write and nothing reads any more — the
# rtk step below carries why that removal belongs in this script rather than in a human's hands.
#
# It reaches other scripts rather than reimplementing them: `ai/tools/install.sh` for the Go tool
# binaries, `ai/mcp-sync.sh` for the MCP registry, `ai/run-tests.sh` to verify. Each of those owns its
# own contract and has its own suite.
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
# What a second copy of this repository is recognised by, below. Read from the running file rather than
# written down, so a rename cannot leave the guard looking for a name nothing has.
script_name="$(basename -- "${BASH_SOURCE[0]}")"

dry_run=false
relocate=false
skip_brew=false
skip_tools=false
skip_mcp=false
skip_verify=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --relocate) relocate=true ;;
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

# The end of every path through this script, including the guard below that stops before the first
# write. A refusal that ended the run its own way would report through something this has no say over,
# and the exit code and the collected list are the whole contract with a caller.
report_and_exit() {
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
}

# --- the mount table, and the guard over it -------------------------------------------------------

# Link `target` at `source`, and report which of the four states it found. The only state that writes
# over something is a symlink, which carries no data of its own — but see the guard below, which is
# the case where that reasoning holds for the link and not for what the link is part of.
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
  if $dry_run; then
    say "  would link $target -> $source"
    return 0
  fi
  # `%/` first, or `${target%/*}` names a trailing-slash target itself: `mkdir -p` then creates it
  # and `ln -s` links inside it. An empty `$HOME` leaves the expansion empty, hence the `/`.
  parent="${target%/}"
  parent="${parent%/*}"
  [ -n "$parent" ] || parent="/"
  [ -d "$parent" ] || mkdir -p -- "$parent" || {
    refuse "could not create $parent"
    return 1
  }
  ln -s "$source" "$target" || {
    refuse "could not link $target at $source"
    return 1
  }
  say "  linked   $target"
}

# The mounts, as data rather than as a run of calls, because the guard below has to survey every one
# of them before the first is written. A second list would be a list that drifts: a target added to
# the linking half and missed in the survey is a mount the guard silently stops covering.
cfg_sources=()
cfg_targets=()
add_cfg() {
  cfg_sources+=("$1")
  cfg_targets+=("$2")
}
add_cfg "$repo/zsh/.zpreztorc" "$HOME/.zpreztorc"
add_cfg "$repo/zsh/.zshrc" "$HOME/.zshrc"
add_cfg "$repo/git/.gitconfig" "$HOME/.gitconfig"
add_cfg "$repo/ai/CLAUDE.md" "$HOME/.claude/CLAUDE.md"
add_cfg "$repo/ai/kk-flavor" "$HOME/.kk-flavor"
add_cfg "$repo/ghostty" "$HOME/.config/ghostty"
add_cfg "$repo/lazygit" "$HOME/.config/lazygit"
add_cfg "$repo/nvim" "$HOME/.config/nvim"
add_cfg "$repo/zellij" "$HOME/.config/zellij"
add_cfg "$repo/starship/starship.toml" "$HOME/.config/starship.toml"

# Discovery, not a list: a skill added tomorrow is mounted without anyone editing this file. The cost
# of discovery is that finding none would silently mount nothing, so that is a refusal.
skill_sources=()
skill_targets=()
for dir in "$repo"/ai/skills/*/; do
  [ -d "$dir" ] || continue
  # `%/` first: `##*/` on a path ending in `/` returns nothing, pointing every skill at one target.
  skill_dir="${dir%/}"
  skill_sources+=("$skill_dir")
  skill_targets+=("$HOME/.claude/skills/${skill_dir##*/}")
done

# --- is this machine already mounted somewhere else? ----------------------------------------------

# `link()` above treats an existing symlink as safe to write over, on the grounds that a symlink
# carries no data of its own. True of the link. The damage this guards is to the *mount*: when the
# checkout a live link names is real, and is where this machine's config actually lives, that link is
# not stale — this checkout is the stranger, and repointing it moves the human's whole setup here.
# Delete the clone afterwards, which is the entire point of a scratch clone, and their next login has
# no .zshrc, no .gitconfig and no agent instructions. The script whose header promises it refuses
# rather than deletes would have done it by reporting "repointed" thirty-odd times.
#
# Narrow on purpose, so the rule `link()` states keeps working. A mount counts as belonging to another
# checkout on two conditions: its link value ends in the same relative path — `…/zsh/.zshrc` for
# `~/.zshrc` — and the root left over when that path is stripped holds a copy of this script. Together
# those make it the same file in a second copy of this repository, rather than an unrelated config the
# stale-symlink rule is right to repoint. A README-era link with a trailing slash still compares equal
# to this checkout and stays the compatibility case it was.
#
# A root that no longer resolves does not count either. That is the aftermath of this very bug, or of
# a checkout moved on purpose, and repointing a dangling link is the repair rather than the damage.
mount_foreign_root() {
  local source="$1" target="$2" rel cur root
  [ -L "$target" ] || return 1
  cur="$(readlink "$target")"
  cur="${cur%/}"
  # Absolute only. A relative link value resolves against the link's own directory, not this script's
  # working directory, so naming a root from it would name the wrong one. Every link written here is
  # absolute, so a relative one was not written by this script and is not one of its mounts.
  [ "${cur#/}" != "$cur" ] || return 1
  rel="${source#"$repo"/}"
  [ "${cur%"/$rel"}" != "$cur" ] || return 1
  root="${cur%"/$rel"}"
  root="$(CDPATH= cd -P -- "$root" 2>/dev/null && pwd -P)" || return 1
  [ "$root" != "$repo" ] || return 1
  # And the root has to hold a copy of this script, not merely end in a matching path component. Four
  # of the sources above are one component long — `nvim`, `zellij`, `ghostty`, `lazygit` — so the tail
  # comparison alone reads `~/.config/nvim -> ~/.dotfiles/nvim` as a second checkout, refuses the whole
  # run, and tells the human their configuration is mounted from a directory that has never held it.
  # That link is an ordinary stale mount and `link()` is right to repoint it.
  [ -f "$root/$script_name" ] || return 1
  printf '%s' "$root"
}

foreign_roots=()
note_foreign_root() {
  local candidate="$1" k
  for ((k = 0; k < ${#foreign_roots[@]}; k++)); do
    [ "${foreign_roots[k]}" != "$candidate" ] || return 0
  done
  foreign_roots+=("$candidate")
}

say "mounts"
cfg_foreign=()
skill_foreign=()
foreign_total=0
for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
  found="$(mount_foreign_root "${cfg_sources[i]}" "${cfg_targets[i]}")" || found=""
  cfg_foreign[i]="$found"
  [ -z "$found" ] || {
    foreign_total=$((foreign_total + 1))
    note_foreign_root "$found"
  }
done
for ((i = 0; i < ${#skill_targets[@]}; i++)); do
  found="$(mount_foreign_root "${skill_sources[i]}" "${skill_targets[i]}")" || found=""
  skill_foreign[i]="$found"
  [ -z "$found" ] || {
    foreign_total=$((foreign_total + 1))
    note_foreign_root "$found"
  }
done

if [ "$foreign_total" -eq 0 ]; then
  # Said out loud on the way past. A guard that prints nothing when it passes reads exactly like a
  # guard that was never reached, and this one runs on every machine that is already set up.
  say "  ok       no mount on this machine comes from another checkout"
elif $relocate; then
  say "  --relocate: moving $foreign_total mount(s) to this checkout, from:"
  for ((r = 0; r < ${#foreign_roots[@]}; r++)); do
    say "    ${foreign_roots[r]}"
  done
else
  # The count leads, before any list. Every line of the list carries the same two roots, so the list
  # is one fact repeated; the scale is the fact a reader cannot reconstruct, and a reader who takes in
  # the named configs and stops has not learned that every one of their skills moves too. The configs
  # are named because they are individually consequential — a shell, a git identity, the instructions
  # every agent session loads. The skills are homogeneous, so their count says all a list would.
  say ""
  say "  This checkout is not where this machine's configuration is mounted."
  for ((r = 0; r < ${#foreign_roots[@]}; r++)); do
    root="${foreign_roots[r]}"
    named=()
    n_skill=0
    for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
      [ "${cfg_foreign[i]}" = "$root" ] && named+=("${cfg_targets[i]}")
    done
    for ((i = 0; i < ${#skill_targets[@]}; i++)); do
      [ "${skill_foreign[i]}" = "$root" ] && n_skill=$((n_skill + 1))
    done
    say ""
    say "  $((${#named[@]} + n_skill)) mounts (${#named[@]} configs and $n_skill skills) currently resolve to"
    say "    $root"
    say "  and running from here would move every one of them to"
    say "    $repo"
    say ""
    for ((i = 0; i < ${#named[@]}; i++)); do
      say "    ${named[i]}"
    done
    [ "$n_skill" -eq 0 ] || say "    ...and all $n_skill skills under $HOME/.claude/skills/"
    say ""
    refuse "$((${#named[@]} + n_skill)) mounts (${#named[@]} configs and $n_skill skills) resolve to $root, not to this checkout — nothing was written; re-run with --relocate to move them here"
  done
  report_and_exit
fi

# --- links ----------------------------------------------------------------------------------------

say "links"
for ((i = 0; i < ${#cfg_targets[@]}; i++)); do
  link "${cfg_sources[i]}" "${cfg_targets[i]}"
done

say "skills"
for ((i = 0; i < ${#skill_targets[@]}; i++)); do
  link "${skill_sources[i]}" "${skill_targets[i]}"
done
[ "${#skill_targets[@]}" -gt 0 ] || refuse "no skill directories under $repo/ai/skills/ — nothing was mounted"

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

# `~/.claude/RTK.md` is a leftover, and this step is what clears it. ai/CLAUDE.md used to import that
# path with `@RTK.md`, which resolves relative to `~/.claude/`, and this script copied ai/RTK.md there
# so the import had something to resolve to. The import is gone and its two surviving sentences are
# inline in ai/CLAUDE.md, so nothing writes that file any more and nothing reads it.
#
# A step here rather than a deletion done by hand, because the file sits in the human's home rather
# than in this repository: putting it in the script is what makes the removal something they run
# knowingly, and the only thing that carries it to their other machines. It stays rather than being a
# one-off, too — `rtk init -g` still writes its own template at that path, so a re-run after the next
# one clears it again.
#
# A directory there is refused rather than removed. This script only ever wrote a file at that path,
# so a directory holds something else, and `rm -rf` over it is the data loss the header promises it is
# not. A symlink is removed like the file: a link carries no data of its own.
say "rtk"
rtk_md="$HOME/.claude/RTK.md"
if [ ! -e "$rtk_md" ] && [ ! -L "$rtk_md" ]; then
  say "  ok       no leftover $rtk_md"
elif [ ! -L "$rtk_md" ] && [ -d "$rtk_md" ]; then
  refuse "$rtk_md is a directory, and this script only ever wrote a file there — move it aside yourself"
elif $dry_run; then
  say "  would remove the leftover $rtk_md"
elif rm -f -- "$rtk_md"; then
  say "  removed  $rtk_md"
else
  refuse "could not remove the leftover $rtk_md"
fi

# --- the repository's own tools ------------------------------------------------------------------

# `ai/tools/install.sh` downloads the prebuilt Go tools and verifies each against the release's own
# SHA256SUMS.
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

# A setup script that reports success without checking anything has reported nothing, so the last step
# is the repository's own suites over what was just linked.
#
# The re-entry guard is load-bearing. `ai/run-tests.sh` discovers every `*-test.sh`,
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
  BOOTSTRAP_VERIFYING=1 "$repo/ai/run-tests.sh" >/dev/null
  verify_status=$?
  # The runner's non-zero codes send a reader to different places: 1 to the code, 2 to this machine, 3
  # to whatever else was writing in this checkout while the suites ran. Collapsing any of them into the
  # failing-suite arm blames the suites for something they did not do.
  #
  # Its own account of a moved checkout — the before/after diff — goes to its stdout, which the call
  # above discards, so the refusal below has to stand on its own and say where to look.
  if [ "$verify_status" -eq 2 ]; then
    refuse "ai/run-tests.sh could not measure every suite — unproven is not disproven, and it is not passing either"
  elif [ "$verify_status" -eq 3 ]; then
    refuse "ai/run-tests.sh ran the suites, but the checkout changed while they ran — what it measured is not this tree. Re-run once nothing else is writing here"
  elif [ "$verify_status" -ne 0 ]; then
    refuse "ai/run-tests.sh reported a failing suite"
  fi
fi

# --- result --------------------------------------------------------------------------------------

report_and_exit
