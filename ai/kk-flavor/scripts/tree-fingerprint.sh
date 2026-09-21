#!/usr/bin/env bash
# Print a fingerprint of the working tree (tracked and untracked content, ignored paths and untracked
# nested repositories excluded) so a ledger can name the tree it was written against
# (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).
#
#   usage: tree-fingerprint.sh [<repo path>]   # <repo path> defaults to .
#
# Prints the tree hash, or exits 2 with a reason. A second path is refused rather than dropped: the
# hash of the first one reads exactly like an answer about the pair.
#
# The recipe is Go, in `ai/tools/tree-fingerprint/`, and the Go callers import it rather than coming
# through here.
#
# Don't reimplement it. Untracked content goes to a THROWAWAY object store, because `add -A` would
# otherwise leave the caller's working files recoverable from `.git/objects` for good. And the
# throwaway index is seeded from HEAD, because git applies ignore rules only to paths the index does
# not hold, so an unseeded walk drops a tracked file matching an ignore rule and a rewrite of it
# becomes invisible to every ledger. And a repository sitting inside this one is held out of the walk,
# because `add -A` records it as its HEAD — a hash that moves when a session working in there commits,
# and not when anything here does.
#
# tested by: the Go suite in ai/tools/tree-fingerprint/. The shared stub region and the resolver it calls
# are covered by the Go suite in ai/tools/reach/.

set -euo pipefail

tool="tree-fingerprint"
# How far THIS file sits above the tools directory.
tools_offset="../.."

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
