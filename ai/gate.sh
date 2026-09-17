#!/usr/bin/env bash
# The pre-commit gate: every check this repository gates on, run from cold, every time.
#
#   usage: gate.sh [--full]
#          (no flag)  run every check, letting Go's own test cache answer where it can
#          --full     defeat that cache too, which is what the time budget is measured against
#
# Five checks — gofmt, vet, the Go suite, the wiring check, the field guide — run at once and printed
# in that order.
#
# It may never report a pass for a check it did not run, finish over budget and exit 0, or skip
# anything quietly. `ai/kk-flavor/standards/testing.md` rule 6 is the bound and `ai/tools/gate/` is
# where it is enforced.
#
# tested by: the Go suite in ai/tools/gate/.

set -euo pipefail

tool="gate"
# How far THIS file sits above the tools directory.
tools_offset="."

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
