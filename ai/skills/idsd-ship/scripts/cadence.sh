#!/usr/bin/env bash
# Offer cadence — idsd-ship offers a periodic pass at most once per interval; the tool this reaches
# owns the interval and where its date is kept.
#
#   usage: cadence.sh audit due     0 = offer one, 1 = not yet, 2 = undetermined (never "not due")
#          cadence.sh audit asked   record that the offer was made today, whatever the human answered
#
# The audit date goes under `.git/`, never in `.idsd/` — `report.sh discard` wipes a throwaway
# `.idsd/`, and a cadence the ship itself deletes can never come due.
#
# Exit 2 is "nothing was determined" and is never a "not due": both end in "no offer made", so a
# caller that reads one as the other suppresses the pass for as long as the bad record sits there.
#
# tested by: the Go suite beside the tool, `ai/tools/cadence/cadence_test.go`, which drives every case
# in one process; the shared stub region below by tool-stub-test.sh, and the resolver it calls by
# resolve-test.sh.

set -euo pipefail

tool="cadence"

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

# Walked up rather than counted. This region is byte-identical in every stub, and the stubs do not all
# sit at one depth — a skill's `scripts/` is three below `ai/`, `kk-flavor/scripts/` is two — so a fixed
# run of `..` lands on the tools directory from some of them and silently on a stranger from the rest.
# The walk stops at the first `tools/resolve.sh` above this file, which inside a checkout is always
# `ai/`'s. `${probe%/*}` rather than `dirname`, because this runs on every invocation of every stub.
resolver=""
probe="$here"
while [ -n "$probe" ]; do
  if [ -e "$probe/tools/resolve.sh" ]; then
    resolver="$probe/tools/resolve.sh"
    break
  fi
  probe="${probe%/*}"
done
[ -n "$resolver" ] ||
  die "no tools/resolve.sh above $here — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
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
