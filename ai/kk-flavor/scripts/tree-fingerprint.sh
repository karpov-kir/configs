#!/usr/bin/env bash
# Print a fingerprint of the working tree (tracked and untracked content, ignored paths excluded) so a
# ledger can name the tree it was written against (`~/.kk-flavor/standards/skill-protocol.md` →
# **Queue**).
#
# usage: tree-fingerprint.sh [<repo path>]. Prints the tree hash, or exits 2 with a reason.
#
# There is no exit 1: a fingerprint either is the tree's hash or it is nothing, and a caller that read
# a refusal as a hash would write a ledger head no later run can match.
#
# The recipe is Go, in `ai/tools/tree-fingerprint/`, and the Go callers import it rather than coming
# through here — `eco-report` ran this about 110 times a suite, and each run cost a bash and an mktemp
# on top of the git calls that do the work.
#
# Two properties that recipe holds, and which a second copy of it would lose: untracked content goes to
# a THROWAWAY object store, because `add -A` would otherwise leave the caller's working files
# recoverable from `.git/objects` for good; and the throwaway index is seeded from HEAD, because git
# applies ignore rules only to paths the index does not hold, so an unseeded walk drops a tracked file
# matching an ignore rule and a rewrite of it becomes invisible to every ledger.
#
# tested by: the Go suite beside the tool, `ai/tools/tree-fingerprint/fingerprint_test.go`, which drives
# every case in one process; the shared stub region below by tool-stub-test.sh, and the resolver it
# calls by resolve-test.sh.

set -euo pipefail

tool="tree-fingerprint"

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
