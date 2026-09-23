#!/usr/bin/env bash
# Holds a kk-build change back from a commit, a push or a PR. A kk-qualify pass over its tree or the
# human's skip releases it.
#
#   usage: build-gate.sh open <requirement> | stamp | skip <the human's words> | check | close
#
# `close` retires a landed build. Exits 0 on a pass or a written record, 1 when the gate refuses,
# and 2 when it could not run.
#
# tested by: the Go suite in ai/tools/build-gate/. The shared stub region and the resolver it calls
# are covered by the Go suite in ai/tools/reach/.

set -euo pipefail

tool="build-gate"
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
