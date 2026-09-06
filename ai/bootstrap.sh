#!/usr/bin/env bash
#
# Set the agent side of this machine up from this repository: link the instructions, the kk-flavor
# bucket and every skill into place, install the tools they run, register the MCP servers, then verify
# the result by running the repository's own suites.
#
#   usage: ai/bootstrap.sh [--dry-run] [--relocate] [--skip-brew] [--skip-tools] [--skip-mcp]
#                          [--skip-verify] [--skip-maintainer-skills]
#
# Safe to re-run: every step checks the state it wants before changing anything, so a second run over
# a finished machine reports "ok" throughout and writes nothing.
#
# It will not move a machine that is already mounted from somewhere else. Run from a second checkout —
# a scratch clone, a colleague's copy — every link this script writes would be repointed at the copy,
# and deleting the copy afterwards leaves the human with no agent instructions and no skills. That is
# refused before anything is written; `--relocate` is how you say you mean it.
#
# It refuses rather than deletes. A target it does not already own is reported and skipped, and the
# run exits non-zero with the list. The one thing it does remove is `~/.claude/RTK.md`, a file this
# repository used to write and nothing reads any more — the rtk step below carries why that removal
# belongs in this script rather than in a human's hands.
#
# It reaches other scripts rather than reimplementing them: `tools/install.sh` for the Go tool
# binaries, `mcp-sync.sh` for the MCP registry, `run-tests.sh` to verify. Each of those owns its own
# contract and has its own suite.
#
# Independent of env/bootstrap.sh in both directions: neither reads the other's mounts, so this is the
# whole of what a machine needs to run these agents, on a machine whose shell setup is its own.
#
# tested by: bootstrap-test.sh
# untested: brew, gh and claude are external commands. Faking them would only assert the fake, so the
# suite covers the linking and the refusals and drives the three external steps behind --skip flags.
set -uo pipefail

# `CDPATH=`: set in the environment, `cd` echoes the directory it landed on, so `repo` comes back two
# lines long and every source path built from it resolves nowhere.
repo="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# What a second copy of this repository is recognised by. Read from the running file rather than
# written down, so a rename cannot leave the guard looking for a name nothing has.
script_name="$(basename -- "${BASH_SOURCE[0]}")"
label="ai bootstrap"

dry_run=false
relocate=false
skip_brew=false
skip_tools=false
skip_mcp=false
skip_verify=false
skip_maintainer_skills=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=true ;;
    --relocate) relocate=true ;;
    --skip-brew) skip_brew=true ;;
    --skip-tools) skip_tools=true ;;
    --skip-mcp) skip_mcp=true ;;
    --skip-verify) skip_verify=true ;;
    --skip-maintainer-skills) skip_maintainer_skills=true ;;
    -h | --help)
      sed -n '3,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      printf 'ai/bootstrap.sh: unknown option %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

bulk_label="skills"
# Refused by name rather than left to `.` failing. Without `set -e` a missing library carries on into
# `add_cfg: command not found` and exits 127, naming neither the file that is gone nor what to do
# about it — the same false diagnosis the verify step below guards against. ai/ is copied out of this
# repository on its own, so a checkout without lib/ is a real one.
[ -r "$repo/../lib/mount.sh" ] || {
  printf 'ai/bootstrap.sh: lib/mount.sh is missing from this checkout — ai/ and lib/ install together, and nothing was linked\n' >&2
  exit 2
}
# shellcheck source=../lib/mount.sh
. "$repo/../lib/mount.sh"

# --- the mount table ------------------------------------------------------------------------------

add_cfg "$repo/CLAUDE.md" "$HOME/.claude/CLAUDE.md"
add_cfg "$repo/kk-flavor" "$HOME/.kk-flavor"

# Whether a skill exists to maintain this instruction tree rather than to work in any repository. The
# audience is declared in the skill's own frontmatter, so discovery below stays discovery: a
# maintainer-only skill added tomorrow is excluded without anyone editing this file, and a list of
# three names here would be wrong the day a fourth is marked.
#
# The block opens on line 1 and has to close before any line in it counts, which is the rule the Go
# reader states in ai/tools/shell/markdown.go — an `audience:` line in the prose is prose, and an
# unterminated block is not frontmatter. The pattern below is that reader's, character for character,
# because eco-check's mount scan asks the same question of the same files and the two answers cannot
# be allowed to differ.
is_maintainer_only() { # <SKILL.md>
  [ -r "$1" ] || return 1
  awk '
    NR == 1 { if ($0 !~ /^---[[:space:]]*$/) exit; next }
    /^---[[:space:]]*$/ { closed = 1; exit }
    tolower($0) ~ /^audience:[[:space:]]*maintainer[[:space:]]*$/ { found = 1 }
    END { if (closed && found) exit 0; exit 1 }
  ' "$1"
}

# Discovery, not a list: a skill added tomorrow is mounted without anyone editing this file. The cost
# of discovery is that finding none would silently mount nothing, so that is a refusal below.
skills_found=0
skipped_count=0
skipped_names=""
for dir in "$repo"/skills/*/; do
  [ -d "$dir" ] || continue
  # `%/` first: `##*/` on a path ending in `/` returns nothing, pointing every skill at one target.
  skill_dir="${dir%/}"
  skills_found=$((skills_found + 1))
  if $skip_maintainer_skills && is_maintainer_only "$skill_dir/SKILL.md"; then
    skipped_count=$((skipped_count + 1))
    skipped_names="$skipped_names ${skill_dir##*/}"
    continue
  fi
  add_bulk "$skill_dir" "$HOME/.claude/skills/${skill_dir##*/}"
done

mount_run

# Said out loud, and after the mounts so it reads beside them. A flag that quietly leaves skills out is
# indistinguishable from a discovery loop that stopped finding them: the machine ends up short of
# skills with nothing in the run saying why. The zero case is the same claim about work that did not
# happen — a flag passed to a tree carrying no marked skill has to say it excluded nothing.
if $skip_maintainer_skills && [ "$skipped_count" -eq 0 ]; then
  say "  ok       no maintainer-only skill was there to exclude"
elif $skip_maintainer_skills; then
  say "  skipped  $skipped_count maintainer-only skill(s):$skipped_names"
fi

# Two ways to mount no skill, and they send a reader to different places: a skills directory with
# nothing in it is a broken checkout, while a flag that excluded every skill it found is a flag doing
# exactly what it says on a tree that has nothing else. The exit code is the same for both, so the
# wording is the only thing telling them apart.
if [ "${#bulk_targets[@]}" -gt 0 ]; then
  :
elif [ "$skills_found" -gt 0 ]; then
  refuse "every skill under $repo/skills/ is maintainer-only, and --skip-maintainer-skills excluded all $skills_found — nothing was mounted"
else
  refuse "no skill directories under $repo/skills/ — nothing was mounted"
fi

# --- packages ------------------------------------------------------------------------------------

if $skip_brew; then
  say "brew (skipped)"
elif ! command -v brew >/dev/null 2>&1; then
  refuse "brew is not installed, so no formula was installed"
else
  say "brew"
  # Installed-first rather than `brew install` unconditionally: the latter is slow, noisy, and exits
  # non-zero on an already-installed formula, which would make a finished machine look broken.
  for formula in rtk jq; do
    if brew list --formula "$formula" >/dev/null 2>&1; then
      say "  ok       $formula"
    elif $dry_run; then
      say "  would install $formula"
    else
      brew install "$formula" >/dev/null || refuse "brew install $formula failed"
    fi
  done
fi

# --- rtk ------------------------------------------------------------------------------------------

# `~/.claude/RTK.md` is a leftover, and this step is what clears it. ai/CLAUDE.md used to import that
# path with `@RTK.md`, which resolves relative to `~/.claude/`, and this script copied ai/RTK.md there
# so the import had something to resolve to. The import is gone and its two surviving sentences are
# inline in ai/CLAUDE.md, so nothing writes that file any more and nothing reads it.
#
# The step is here rather than in the human's hands because the file sits in their home, not in this
# repository, and a step is what carries the removal to their other machines. It stays for good:
# `rtk init -g` writes its own template at that path, so the next one puts the file back.
#
# `! -L` in the directory arm, so a symlink to a directory goes the way the file does — a link carries
# no data of its own.
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

if $skip_tools; then
  say "tools (skipped)"
elif $dry_run; then
  say "tools: would run ai/tools/install.sh"
elif ! command -v gh >/dev/null 2>&1; then
  refuse "gh is not installed, so ai/tools/install.sh could not fetch the tool binaries"
else
  say "tools"
  "$repo/tools/install.sh" || refuse "ai/tools/install.sh failed"
fi

# --- MCP registry --------------------------------------------------------------------------------

if $skip_mcp; then
  say "mcp (skipped)"
elif $dry_run; then
  say "mcp: would run ai/mcp-sync.sh"
elif ! command -v claude >/dev/null 2>&1; then
  refuse "the claude CLI is not on PATH, so the MCP servers were not registered"
else
  say "mcp"
  "$repo/mcp-sync.sh" || refuse "ai/mcp-sync.sh failed"
fi

# --- verify --------------------------------------------------------------------------------------

# A setup script that reports success without checking anything has reported nothing, so the last step
# is the repository's own suites over what was just linked.
#
# The re-entry guard is load-bearing. `run-tests.sh` discovers every `*-test.sh`, `bootstrap-test.sh`
# is one of them, and it runs this script — so verify reaches a suite that reaches verify. It
# terminates today only because every case in that suite remembers `--skip-verify`, which is a loop
# held open by a convention. The marker closes it whatever any caller passes.
if [ -n "${BOOTSTRAP_VERIFYING:-}" ]; then
  say "verify (skipped: already inside a verify run)"
elif $skip_verify; then
  say "verify (skipped)"
elif [ ! -x "$repo/run-tests.sh" ]; then
  # A missing runner must not be reported as a failing suite. Without this the call exits 127 and the
  # arm below blames the suites for a file that was never there — a false diagnosis pointing at code
  # that is fine, which costs more than the silence would. Checked in the dry run too: a dry run that
  # says "ok" over a repository where the real run cannot work is the same lie one step earlier.
  refuse "ai/run-tests.sh is not in this checkout — the suites were not run, which is not the same as passing"
elif $dry_run; then
  say "verify: would run ai/run-tests.sh"
else
  say "verify"
  BOOTSTRAP_VERIFYING=1 "$repo/run-tests.sh" >/dev/null
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
