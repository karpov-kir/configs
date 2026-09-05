#!/usr/bin/env bash
# Apply `~/.kk-flavor/standards/writing.md` → **Score what survives**: hold the per-lane thresholds,
# and cut a scored list against one. It never produces a score — "how much this reader needs it" is a
# judgment, so the caller scores and this decides what that score buys.
#
# usage: score.sh threshold <lane>
#        score.sh cut [--kept-all <why>] <lane> <what a 10 is here>
#            reads `<score><TAB><label>` lines on stdin; exit 3 if nothing fell below the line
#
# Prints to stdout. Exit 2 means it did not run; exit 3 means it ran and refuses the result.
#
# Thresholds come from `../thresholds.conf`, which is tracked, overlaid per lane by an untracked
# machine-local file. An override in effect is always announced: a bar moved locally produces a
# verdict no other machine reproduces, and silence about it is the hazard.
#
# The tool is Go, in `ai/tools/score/`; this reaches that binary and nothing else. Its refusals are the
# enforcement, not convenience: read the comment at one before removing it. What `default` is, and why
# an unknown lane exits rather than landing on it, is stated at that lane in `../thresholds.conf`.
#
# tested by: the Go suite beside the tool, `ai/tools/score/`; the shared stub region below by
# tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="score"
# How far THIS file sits above the tools directory. The shared region below resolves exactly this one
# path and consults nothing else, so a stub can only ever reach the directory it names.
tools_offset="../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, held so by the wiring check's shared-region scan. Copied rather than
# sourced because sourcing a file is executing it, and these run from whatever repo the human is in.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` echoes where it landed when the path is relative, which would put a second
# line into this substitution and corrupt every path built from it. `pwd -P` resolves the symlink the
# skill is mounted by, so the tools directory is found from this file's real location, not from cwd.
here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

# Exactly one path, named by the stub above rather than searched for here. The stubs sit at three
# depths, and a shared region that guesses between them is a stub reaching a directory it does not
# name: first a walk upward, which where a checkout ships no `ai/tools/` climbed OUT of it and exec'd
# the first `tools/resolve.sh` in any ancestor, and then a list of three relative candidates, which is
# the same hole one step quieter — those offsets are applied to files at DIFFERENT depths, so two of
# them resolve outside the repository for a stub one level above the tools directory. Demonstrated
# both times, exit 0 with a stranger's binary run. One named path cannot do either.
resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver names its own failures on stderr, so nothing is re-reported here. Its status is NOT
# passed through, and the 2 below is deliberate rather than a copy of it: every way a resolver can fail
# means the tool did not run, which is 2 in this repo's vocabulary, and 3 — ran and refuses a result —
# must never reach a caller for a binary that never started. `ai/tools/resolve.sh` exits 2 for all of
# them today, so this collapses nothing now; it is what keeps the guarantee if it ever grows a code.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
