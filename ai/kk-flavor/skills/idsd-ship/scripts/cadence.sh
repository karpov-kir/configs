#!/usr/bin/env bash
# Offer cadence — idsd-ship offers a periodic pass at most once per interval; the tool this reaches
# owns the interval and where its date is kept.
#
#   usage: cadence.sh audit {due|asked}
#          due    0 = offer one, 1 = not yet, 2 = undetermined (never "not due")
#          asked  record that the offer was made today, whatever the human answered
#
# The audit date goes under `.git/`, never in `.idsd/`: `report.sh discard` can take an external
# `.idsd/` down to nothing, and a cadence one discard deletes can never come due.
#
# Exit 2 is "nothing was determined" and is never a "not due": both end in "no offer made", so a
# caller that reads one as the other suppresses the pass for as long as the bad record sits there.
#
# tested by: the Go suite beside the tool, `ai/tools/cadence/`; the shared stub region below by
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="cadence"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, held so by the wiring check's shared-region scan. Copied rather than
# sourced because sourcing a file is executing it, and these run from whatever repo the human is in —
# so only what cannot move is here: a stub has to find the resolver before the resolver can decide
# anything. `ai/tools/resolve.sh` owns the rest, argv[0] included, and its header states why each line
# below is the shape it is — the `cd -P`, the one declared offset, the two guards, the exec.
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
