#!/usr/bin/env bash
# Repeated-long-literal detector — byte-identical long strings appearing 2+ times among the diff's
# ADDED lines: copy-pasted tokens, keys, fixtures. Run by kk-refactor's setup, and by a pipeline
# orchestrator before the refactor stage.
#
#   usage: dup-literals.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
#          a path argument is refused with exit 2, never scanned
#   env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
#          DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
#
# Prints each duplicate (count, length, 60-char prefix). Exits 1 when any found, 0 when clean, 2 when
# the scan did not run — a path where a revision belongs, git rejecting the arguments, or a threshold
# that is no number.
#
# Because it echoes 60 bytes of every duplicate, the untracked scan skips secret-bearing names rather
# than print what is in them: two `.env` files sharing one API token is the ordinary case, and the
# token is over the length floor and appears twice.
#
# Every run ends with its denominator on stderr — files reached, duplicates, files skipped unread,
# binary lines ignored. An empty report at exit 0 means "nothing repeated" only when the first number
# is above zero, and "nothing was read" when it is not.
#
# The scanner is Go, in `ai/tools/dup-literals/`, and the half it shares with comment-density — which
# arguments are refused, the git flags that pin the diff's shape, the anchor that stops a file's own
# content forging a header — is `ai/tools/diffscan/`.
#
# tested by: the Go suite beside the tool, `ai/tools/dup-literals/dup_test.go`, which drives every case
# in one process; the shared stub region below by tool-stub-test.sh, and the resolver by
# resolve-test.sh.

set -euo pipefail

tool="dup-literals"

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

# The candidates, named, and NOT a walk upward. The stubs sit at exactly three depths — `ai/` itself is
# one above the tools directory, `kk-flavor/scripts/` is two, a skill's `scripts/` is three — so all
# three are written out and nothing else is consulted. They cannot collide: from any one depth the
# other two name directories that do not exist.
#
# An unbounded walk was here and was a hole: where a checkout genuinely does not ship `ai/tools/`, it
# climbed out of the checkout and ran the first `tools/resolve.sh` it met in any ancestor, exec'ing a
# stranger's binary at exit 0 where the refusal below is the whole point. Bounding it by depth alone
# does not close that — an ancestor can sit inside the bound. Naming the two real candidates does.
resolver=""
for candidate in "$here/tools/resolve.sh" "$here/../../tools/resolve.sh" "$here/../../../tools/resolve.sh"; do
  if [ -e "$candidate" ]; then
    resolver="$candidate"
    break
  fi
done
[ -n "$resolver" ] ||
  die "no tools/resolve.sh at any of the three depths around $here — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
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
