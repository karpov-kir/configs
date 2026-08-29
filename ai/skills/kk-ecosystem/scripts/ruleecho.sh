#!/usr/bin/env bash
# Build and run the rule-echo tool over an ecosystem root, from any working directory.
#
#   usage: ruleecho.sh <root> [file ...]
#
# Exits 0 with no pairs, 1 with pairs, 2 when the scan did not run. Every failure exits 2 and says
# what did not happen: this is the only cross-file restatement detector we have, and a scan that
# could not run must never be mistaken for one that found nothing.
#
# tested by: ruleecho-test.sh and tool-stub-test.sh, and the resolver it calls by resolve-test.sh

set -euo pipefail

tool="rule-echo"

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
