#!/usr/bin/env bash
# Ecosystem size ledger — the numbers that decide whether a pass is worth running, measured rather
# than estimated. Owned by kk-reduce; it appends to ../stats.md, relative to this script's dir.
#   usage: stats.sh [<root>]                    print the current measurements
#          stats.sh --append "<note>" [<root>]  print them and append a dated row to stats.md
# The note is one argument — quote it, or its first word is read as <root>.
# <root> holds kk-flavor/ and skills/; defaults to . then ./ai, matching check.sh.
# Exits 0 on success, 2 when it could not measure — a measurement that did not run is not a zero.
#
# The measurements are Go, in `ai/tools/eco-stats/`; this reaches that binary and nothing else. That
# package shares its root resolution and its always-loaded figure with `ai/tools/eco-check/`, and the
# agreement cases in its own suite are what hold the two to one answer for one tree. A change there
# needs a case there, and `ai/tools/go-mutate` is what shows the case can fail.
#
# ../stats.md is found from argv[0], so this must stay in the skill's scripts/ directory.
#
# tested by: tool-stub-test.sh, and the resolver it calls by resolve-test.sh

set -euo pipefail

tool="eco-stats"

# --- shared:tool-stub ---
# Byte-identical in every stub that reaches a tool this way; the wiring check's shared-region scan is
# what holds it that way. Duplicated rather than sourced on purpose: sourcing a file is executing it,
# and these run from whatever repository the human is working in.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` consults it for a relative path and echoes where it landed, which would put a
# second line into the command substitution and corrupt every path built from it. `pwd -P` follows the
# symlink this skill is mounted by, so the tools directory is found from this file's own resolved
# location and never from cwd.
here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

resolver="$here/../../../tools/resolve.sh"
# The two cases are told apart because they have different fixes: a checkout that ships no ai/tools/
# needs a different install, and a resolver that lost its exec bit needs a chmod.
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver prints one path on stdout and names its own failures on stderr, so its exit is final
# here: anything short of a runnable binary is exit 2, never a run that quietly did nothing.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# argv[0] stays the path this was invoked by instead of becoming the binary's: the tools read their
# skill directory from it, the way the shell versions read it from $0, so a skill reached through its
# symlink mount still resolves its own ledger, template and sibling scripts.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
