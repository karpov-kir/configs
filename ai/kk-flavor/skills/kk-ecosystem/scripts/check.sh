#!/usr/bin/env bash
# Ecosystem wiring check, the mechanical half of kk-ecosystem. It checks that every reference an
# agent could follow resolves to something that exists, that every script still parses, and that the
# standards stay layered — each declaring its layer, and no cycle of citations crossing one.
#
#   usage: check.sh --agent=claude|codex [--gate] [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
#
# Prints one line per finding, plus two always-loaded budgets: the router's files, and every skill's
# `description:`. Outside the install it prints `mounts: skipped`; no such line means the mount scan
# ran. Exits 1 with findings, 0 when clean, and 2 when it could not run: no resolvable root, a scan
# that could not run, or a check that never started.
#
# --gate drops every gitignored path from the walk, so two checkouts of one commit cannot answer
# differently. The filter is on ignored and never on untracked, so a skill just written and not yet
# staged is still judged.
#
# tested by: the Go suite beside the tool, `ai/tools/eco-check/`; the shared stub region below by
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="eco-check"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub. The wiring check's shared-region scan holds it so.
#
# Each stub carries a copy instead of sourcing one file. Sourcing a file executes it, and a stub runs
# from whatever repository the human is standing in. So the only part that lives here is the part that
# cannot move: a stub has to find the resolver before the resolver can decide anything.
#
# ai/tools/resolve.sh owns everything after that, argv[0] included. Its header says why each line
# below has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
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
