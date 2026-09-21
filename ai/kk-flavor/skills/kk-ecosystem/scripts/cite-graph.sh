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
# Every figure here is a finder, not a target. Each measures the tree through a proxy, so moving a
# number and improving what agents read are different acts: a door count rises when a restatement is
# correctly cut, and the tree got better as the metric got worse. Read a figure as a place to go and
# look, then act on what you find at that place.
#
# Exits 0 with the report, and 2 when the measurement did not run. There is no exit 1: this measures
# the tree and never refuses one, so 0 is a report to read, never a verdict that the tree is flat.
# A root holding no `.md` exits 2 for the same reason: reading nothing is not a flat tree.
#
# tested by: the Go suite in ai/tools/cite-graph/, which drives the tool in process. It covers each
# figure this header names, and each of the two ways a root can fail to name one tree. The shared
# stub region and the resolver it calls have their own cases in
# the Go suite in ai/tools/reach/.

set -euo pipefail

tool="cite-graph"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, which the wiring check's shared-region scan enforces.
#
# Each stub carries its own copy. One shared file would be executed by the source call that read it,
# and a stub runs from whatever repository the human is standing in.

# What lives here is the part that cannot move: a stub has to find the resolver before the resolver
# can decide anything. ai/tools/resolve.sh owns the rest, argv[0] included. Its header says why each
# line here has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

exec "$resolver" --run "$tool" "$0" "$@"
# --- end shared:tool-stub ---
