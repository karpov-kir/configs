#!/usr/bin/env bash
# Build and run the citation-graph tool over an ecosystem root, from any working directory.
#
#   usage: cite-graph.sh <root>
#
# <root> is the directory holding the `.md` tree to measure — exactly one, and every `.md` under it
# is read. The report is four sections: DEPTH (the longest path through the graph, a coupling
# measure and not hops any one consumer walks), FAN-OUT (per file, door citers against precision
# citers), UNENTERED (sections nothing cites), and CYCLES.
#
# Every figure here is a finder, and none of them is a target. Each measures the tree through a
# proxy, so moving a number and improving what agents read are different acts — a door count rises
# when a restatement is correctly cut, and the tree got better as the metric got worse. Read a figure
# as a place to go and look, then act on what you find at that place, never on the figure.
#
# Exits 0 with the report, and 2 when the measurement did not run. There is no exit 1: this measures
# the tree and never refuses one, so 0 is a report to read, never a verdict that the tree is flat.
# A root holding no `.md` exits 2 for the same reason — reading nothing is not a flat tree, and the
# two must never arrive at a caller as the same number.
#
# tested by: cite-graph-test.sh, and the shared stub region by tool-stub-test.sh, the resolver it
# calls by resolve-test.sh

set -euo pipefail

tool="cite-graph"

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
