#!/usr/bin/env bash
# Build and run the rule-echo tool over an ecosystem root, from any working directory.
#
#   usage: ruleecho.sh <root> [file ...]
#
# Exits 0 with no pairs, 1 with pairs, 2 when the scan did not run. Every failure exits 2 and says
# what did not happen: this is the only cross-file restatement detector we have, and a scan that
# could not run must never be mistaken for one that found nothing.
#
# tested by: the Go suite in ai/tools/rule-echo/, which drives the tool in process — the matcher, and
# all three exit codes above. The shared stub region below, and the resolver it calls, by the Go
# suite in ai/tools/reach/.

set -euo pipefail

tool="rule-echo"
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
