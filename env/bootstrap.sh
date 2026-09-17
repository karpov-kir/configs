#!/usr/bin/env bash
#
# Set this machine's shell and editor environment up from this repository: link every config in env/
# into place and install what those links need.
#
#   usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]
#
# Safe to re-run: every step checks the state it wants before changing anything, so a second run over
# a finished machine reports "ok" throughout and writes nothing. It refuses rather than deletes, and
# it will not move a machine whose configuration is mounted from a different checkout.
#
# The recipe is Go, in `ai/tools/env-bootstrap/`. env/ and ai/ install together for that reason: this
# reaches the resolver next door, and a checkout carrying only env/ has nothing to run.
#
# tested by: the Go suite in ai/tools/env-bootstrap/; stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="env-bootstrap"
# How far THIS file sits above the tools directory.
tools_offset="../ai"

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
