#!/usr/bin/env bash
# Repeated-long-literal detector — byte-identical long strings appearing 2+ times among the diff's
# ADDED lines: copy-pasted tokens, keys, fixtures. Run by the refactor worker's setup, and by a pipeline
# orchestrator before the refactor stage.
#
#   usage: dup-literals.sh [<git-diff revisions>] [-- <paths>]   # revisions default to HEAD (all
#          uncommitted changes); a bare path argument is refused with exit 2, never scanned, and paths
#          after `--` narrow the scan to them
#   env:   DUP_MIN_LEN — minimum literal length in chars (default 100)
#          DUP_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
#
# Prints each duplicate (count, length, 60-char prefix). Exits 1 when any found, 0 when clean, 2 when
# the scan did not run — a path where a revision belongs, git rejecting the arguments, or a threshold
# that is no number.
#
# Because it echoes 60 bytes of every duplicate, the untracked scan skips secret-bearing names rather
# than print what is in them: two `.env` files sharing one API token is the ordinary case, and the
# token is over the length floor and appears twice.
#
# Every run ends with its denominator on stderr — files reached, duplicates, files skipped unread,
# binary lines ignored. An empty report at exit 0 means "nothing repeated" only when the first number
# is above zero, and "nothing was read" when it is not.
#
# The scanner is Go, in `ai/tools/dup-literals/`. The half it shares with voice-check is
# `ai/tools/diffscan/`: which arguments are refused, the git flags that pin the diff's shape, and the
# anchor that stops a file's own content forging a header.
#
# tested by: the Go suite beside the tool, `ai/tools/dup-literals/`; the shared stub region below by
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="dup-literals"
# How far THIS file sits above the tools directory.
tools_offset="../../.."

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
