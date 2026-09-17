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
# The title line is two slots — `[<repo abbrev>] <one imperative line: the work>` — and each is read on
# its own. Either one still holding its `<…>` placeholder is refused, and so is a bracketed opening
# word that is not what `repo-key.sh --abbrev` prints for <repo>. The prefix itself is optional: a
# title that does not open with a bracketed word passes, because the gate checks which abbreviation
# stands in that slot, not whether one stands there at all.
#
# Findings print one per line. Two other kinds print alongside them, and neither fails the draft: a
# `declared None:` line per slot the draft empties on purpose, and a `note:` when the repository is
# dirty. Exits 1 with findings, 0 when clean, and 2 when it could not run and said why on stderr. A 2
# prints no findings, so never read it as a clean draft.
#
# tested by: the Go suite beside the tool, `ai/tools/handoff-check/`; the shared stub region below by
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="handoff-check"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

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
