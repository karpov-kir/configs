#!/usr/bin/env bash
# Runs the comment lane's stages over a change set, one command per stage, so a run writes no script.
# It calls no model: the runner dispatches the writers.
#
#   usage: comment-run.sh seed --run-dir=<dir> --archive=<dir> --range=<base>..<head> [--heads=<sha,...>] [--contradictions=<tsv>]
#   usage: comment-run.sh prompts --run-dir=<dir> --workers=<n>
#   usage: comment-run.sh archive-written --run=<run> --archive=<dir> [--run-dir=<dir>] <writer return>...
#   usage: comment-run.sh carried --run=<run> --run-dir=<dir> --archive=<dir> <refactor return>...
#   usage: comment-run.sh taint --ledger=<file> <transcript>... [--ledger=<file> <transcript>...]
#   usage: comment-run.sh keep-test --archive=<dir> <path>...
#   usage: comment-run.sh loop --run-dir=<dir> --archive=<dir> --run=<run> [--contradict=<sentence>] <path>:<line> <review sentence>...
#
# Exit 0 is a clean run, 1 a run that reported findings, and 2 a stage that did not run.
#
# tested by: the Go suite in ai/tools/comment-run/. The shared stub region and the resolver it
# calls have their own cases in the Go suite in ai/tools/reach/.

set -euo pipefail

tool="comment-run"
# How far THIS file sits above the tools directory.
tools_offset="../../../.."

# --- shared:tool-stub ---
# Byte-identical in every stub, which the wiring check's shared-region scan enforces.
#
# Each stub carries its own copy. One shared file would be executed by the source call that read it,
# and a stub runs from whatever repository the human is standing in.

# What lives here is the part that cannot move: a stub has to find the resolver before the resolver
# can decide anything. ai/tools/resolve.sh owns the rest, argv[0] included. Its header says why each
# line here has the shape it has: the `cd -P`, the declared offset, the two guards, the exec.
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
