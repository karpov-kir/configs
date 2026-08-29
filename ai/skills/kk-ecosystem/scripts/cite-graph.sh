#!/usr/bin/env bash
# Build and run the citation-graph tool over an ecosystem root, from any working directory.
#
#   usage: cite-graph.sh <root>
#
# <root> is the directory holding the `.md` tree to measure — exactly one, and every `.md` under it
# is read. The report is four sections: DEPTH (the longest chain a consumer walks), FAN-OUT (per
# file, door citers against precision citers), UNENTERED (sections nothing cites), and CYCLES.
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

resolver="$here/../../../tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver names its own failures on stderr, so its exit passes straight through. Anything short
# of a runnable binary exits 2, never a run that quietly did nothing.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
