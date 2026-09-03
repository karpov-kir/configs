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
# The scan is Go, in `ai/tools/handoff-check/`; this reaches that binary and nothing else.
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

# Two candidates, named, and NOT a walk upward. The stubs sit at exactly two depths — a skill's
# `scripts/` is three below the tools directory, `kk-flavor/scripts/` is two — so both are written out
# and nothing else is consulted.
#
# An unbounded walk was here and was a hole: where a checkout genuinely does not ship `ai/tools/`, it
# climbed out of the checkout and ran the first `tools/resolve.sh` it met in any ancestor, exec'ing a
# stranger's binary at exit 0 where the refusal below is the whole point. Bounding it by depth alone
# does not close that — an ancestor can sit inside the bound. Naming the two real candidates does.
resolver=""
for candidate in "$here/../../tools/resolve.sh" "$here/../../../tools/resolve.sh"; do
  if [ -e "$candidate" ]; then
    resolver="$candidate"
    break
  fi
done
[ -n "$resolver" ] ||
  die "no tools/resolve.sh at either depth above $here — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver names its own failures on stderr, so nothing is re-reported here. Its status is NOT
# passed through, and the 2 below is deliberate rather than a copy of it: every way a resolver can fail
# means the tool did not run, which is 2 in this repo's vocabulary, and 3 — ran and refuses a result —
# must never reach a caller for a binary that never started. `ai/tools/resolve.sh` exits 2 for all of
# them today, so this collapses nothing now; it is what keeps the guarantee if it ever grows a code.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
