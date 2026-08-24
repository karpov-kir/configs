#!/usr/bin/env bash
# Print a fingerprint of the working tree (tracked and untracked content, ignored paths excluded) so a
# ledger can name the tree it was written against (`~/.kk-flavor/standards/skill-protocol.md` →
# **Queue**).
#
# Usage: tree-fingerprint.sh [<repo path>]. Prints the tree hash, or exits 2 with a reason.
#
# Both git calls below need GIT_INDEX_FILE set. Drop it from the `add` and this stages the caller's whole
# tree against their real index, no prompt first; drop it from `write-tree` and the hash follows that
# index instead of the tree.
#
# A change here needs a case in `~/.kk-flavor/scripts/tree-fingerprint-test.sh`.
set -u

root=${1:-.}

git -C "$root" rev-parse --show-toplevel >/dev/null 2>&1 || {
  echo "error: not a git repository: $root" >&2
  exit 2
}

# Private scratch for everything this run writes. The index goes here rather than in a bare `mktemp`
# file because git recreates that file at 0644, and on a shared /tmp that hands every path in the tree
# to any local user.
scratch=$(mktemp -d) || {
  echo "error: could not create a temporary directory" >&2
  exit 2
}
trap 'rm -rf "$scratch"' EXIT

# `add -A` hashes every untracked, un-ignored file into the object store. No ref points at those blobs,
# so nothing ever collects them: fingerprinting a tree would leave the caller's working files, a live
# credential among them, recoverable from `.git/objects` for good. They go to a throwaway store instead.
# Both git calls only write objects, so the real store needs no alternate and the hash is the same.
#
# Git won't create GIT_OBJECT_DIRECTORY, and a missing one fails repository discovery itself ("fatal: not
# a git repository"), so don't drop this mkdir.
mkdir -p "$scratch/objects" || {
  echo "error: could not create the throwaway object store" >&2
  exit 2
}

# Nothing creates the index path: git rejects an existing 0-byte file ("index file smaller than
# expected"), so it must not be pre-made.
#
# git's stderr is captured because a caller may read the hash through `$(tree-fingerprint.sh 2>&1)`, and
# inherited stderr would put git's warnings and hints on that same channel. A warning that didn't stop
# the walk doesn't change the answer, so the success path drops it.
git_stderr="$scratch/git-stderr"
tree=$(
  GIT_INDEX_FILE="$scratch/index" GIT_OBJECT_DIRECTORY="$scratch/objects" \
    git -C "$root" add -A 2>"$git_stderr" &&
    GIT_INDEX_FILE="$scratch/index" GIT_OBJECT_DIRECTORY="$scratch/objects" \
      git -C "$root" write-tree 2>>"$git_stderr"
)
status=$?

[ "$status" = 0 ] && [ -n "$tree" ] || {
  [ ! -s "$git_stderr" ] || cat "$git_stderr" >&2
  echo "error: could not fingerprint the tree in $root" >&2
  exit 2
}

printf '%s\n' "$tree"
