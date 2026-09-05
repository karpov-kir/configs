#!/usr/bin/env bash
# Print a fingerprint of the working tree (tracked and untracked content, ignored paths excluded) so a
# ledger can name the tree it was written against (`~/.kk-flavor/standards/skill-protocol.md` →
# **Queue**).
#
# usage: tree-fingerprint.sh [<repo path>]. Prints the tree hash, or exits 2 with a reason.
#
# There is no exit 1: a fingerprint either is the tree's hash or it is nothing, and a caller that read
# a refusal as a hash would write a ledger head no later run can match.
#
# The recipe is Go, in `ai/tools/tree-fingerprint/`, and the Go callers import it rather than coming
# through here.
#
# Don't reimplement it. Untracked content goes to a THROWAWAY object store, because `add -A` would
# otherwise leave the caller's working files recoverable from `.git/objects` for good. And the
# throwaway index is seeded from HEAD, because git applies ignore rules only to paths the index does
# not hold, so an unseeded walk drops a tracked file matching an ignore rule and a rewrite of it
# becomes invisible to every ledger.
#
# tested by: the Go suite beside the tool, `ai/tools/tree-fingerprint/`; the shared stub region below
# by tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="tree-fingerprint"
# How far THIS file sits above the tools directory.
tools_offset="../.."

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

# Exactly one path, named by the stub above rather than searched for here. The stubs sit at three
# depths, so anything that guesses between them is a stub reaching a directory it does not name: an
# upward walk leaves a checkout shipping no `ai/tools/` and execs the first `tools/resolve.sh` in any
# ancestor, and a list of relative candidates resolves outside the repository for the stubs one level
# above the tools directory. Either runs a stranger's binary at exit 0.
resolver="$here/$tools_offset/tools/resolve.sh"
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver names its own failures on stderr, so nothing is re-reported here. Its status is NOT
# passed through: the 2 below is deliberate rather than a copy of it. Every way a resolver can fail
# means the tool did not run, which is 2 in this repo's vocabulary, and 3 (ran, and refuses a result)
# must never reach a caller for a binary that never started. `ai/tools/resolve.sh` exits 2 for all of
# them today, so keep the literal 2 if it ever grows a code.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
