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
# tested by: cite-graph-test.sh and the Go suite beside the tool, `ai/tools/cite-graph/`; the shared
# stub region below by tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="cite-graph"
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
