#!/usr/bin/env bash
# Ecosystem wiring check, the mechanical half of kk-ecosystem. It checks that every reference an
# agent could follow resolves to something that exists, and that every script still parses.
#   usage: check.sh [--gate] [<root>]   # <root> holds kk-flavor/ and skills/; defaults to . then ./ai
# Prints one line per finding, plus two always-loaded budgets: the router's files, and every skill's
# `description:`. Exits 1 with findings, 0 when clean, and 2 when it could not run at all. A check
# that did not run is not a clean one.
#
# --gate drops every gitignored path from the walk, so two checkouts of one commit cannot answer
# differently. The filter is on ignored and never on untracked, so a skill just written and not yet
# staged is still judged.
#
# The scans are Go, in `ai/tools/eco-check/`. This file reaches that binary and nothing else. After a
# change there run `go test -count=1 -timeout 30m ./...` and then `./bin/go-mutate`, both from
# `ai/tools`. Keep the timeout: the eco-report package alone runs past the 10m default on a loaded
# machine, and an overrun dumps every goroutine, which reads as a hang.
#
# tested by: tool-stub-test.sh, and the resolver it calls by resolve-test.sh

set -euo pipefail

tool="eco-check"

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
