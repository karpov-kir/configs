#!/usr/bin/env bash
# Comment-density detector. By default it flags changed source files whose ADDED lines are
# comment-heavy; with `--bar` it holds the whole change set to the host repo's own comment rate.
#
#   usage: comment-density.sh [--bar] [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
#          COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
#          DENSITY_MAX_FILE_BYTES — skip a file larger than this unread: only untracked files in the
#          default mode, every file under --bar (default 262144)
#
# Exits 1 with findings, 0 when clean, 2 when the scan did not run — git rejecting the arguments, a
# path passed where a revision belongs, or a threshold that is no number. Prose/data files (md, txt,
# json, lockfiles) don't count. With no diff args, untracked text files are scanned too; the index is
# never touched.
#
# The default mode prints each outlier with its counts and ends with its denominator on stderr — files
# reached, files with countable added lines, outliers, untracked files skipped unread. Read it: an empty
# report at exit 0 means "nothing was comment-heavy" only when that first number is above zero, and
# "nothing was read" when it is not. It is a targeting aid, not a bar. It counts ADDED lines, so
# rewording a comment the base already carried moves it into the added set, and the ratio can rise
# across a pass that cut comments.
#
# `--bar` counts each changed file as it will land, against the rate the repo's untouched files run at,
# and reports what has to go. Two runs over one tree print one verdict, so re-run it after every cut.
# Files are read as they sit in the working tree; revisions only choose which files. Only a file new
# since the diff's base is held to the per-file ceiling: one the repo already carried has the repo's own
# density, and its added lines are the default mode's to flag. The two COMMENT_* thresholds do not
# apply to it, and it exits 2 as well when no file outside the change set carries countable lines.
#
# tested by: the Go suite beside the tool, `ai/tools/comment-density/`; the shared stub region below
# by tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="comment-density"
# How far THIS file sits above the tools directory. The shared region below resolves exactly this one
# path and consults nothing else, so a stub can only ever reach the directory it names.
tools_offset="../../.."

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
