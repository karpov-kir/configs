#!/usr/bin/env bash
# Build and run the handoff-prompt gate over a drafted handoff prompt, from any working directory.
#
#   usage: handoff-check.sh <draft.md> [<repo>]   # <repo> resolves the base commit and is the path
#                                                 # the draft must name; defaults to .
#
# It refuses the drafts a fresh session cannot act on: a missing or barely filled slot, a slot still
# holding its template comment, a base commit that resolves to nothing, a repository the draft never
# names, a licence that was paraphrased instead of quoted, or a sentence pointing back at the
# conversation the receiver was never in.
#
# Findings print one per line. Two other kinds print alongside them, and neither fails the draft: a
# `declared None:` line per slot the draft empties on purpose, and a `note:` when the repository is
# dirty. Exits 1 with findings, 0 when clean, and 2 when it could not run and said why on stderr. A 2
# prints no findings, so never read it as a clean draft.
#
# The scan is Go, in `ai/tools/handoff-check/`; this reaches that binary and nothing else. Run both
# from `ai/tools` after a change there: `go test -count=1 -timeout 30m ./...`, then `./bin/go-mutate`.
# Keep the timeout: the eco-report package alone runs past the 10m default on a loaded machine, and
# overrunning it prints a goroutine dump that reads as a hang rather than as a slow pass.
#
# tested by: the shared stub region by tool-stub-test.sh, and the resolver it calls by resolve-test.sh

set -euo pipefail

tool="handoff-check"

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
