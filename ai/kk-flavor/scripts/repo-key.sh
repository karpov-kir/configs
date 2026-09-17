#!/usr/bin/env bash
# Print a stable name for this clone — `<basename>-<digest>` — so the idsd scratch dir under a
# machine-local override root and the owner's worktree directory key one repository the same way.
#
#   usage: repo-key.sh [--abbrev] [<repo path>]   # <repo path> defaults to .
#
# `--abbrev` prints the clone's abbreviation instead — `GADK8s` for a clone in
# `github-action-deploy-k8s`. It is the same from every worktree of the clone, and it is what a session
# title's prefix takes. Two repositories can abbreviate alike, so a directory keys off the name above,
# never this.
#
# Exits 2 with a reason where it cannot name the clone.
#
# The recipe is Go, in `ai/tools/repo-key/`; Go callers import it rather than coming through here.
#
# tested by: the Go suite in ai/tools/repo-key/; stub region by the Go suite in ai/tools/reach/.
set -euo pipefail

tool="repo-key"
tools_offset="../.."

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
