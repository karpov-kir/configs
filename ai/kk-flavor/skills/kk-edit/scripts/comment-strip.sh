#!/usr/bin/env bash
# Removes a source file's comment blocks and records what each one said, so a writer reads the code
# with the old block gone. It runs without a model.
#
#   usage: comment-strip.sh --facts=<dir> [--archive=<dir>] [--lines=<n,...>] <path>
#
# `--lines` strips only the blocks on those lines, for a code-review finding sent back to its site.
# `--archive=<dir> --contradict=<run> <path> <claim> <sentence>` records a claim code review found
# false, and the strip offers that claim with a `contradicted:` line under it.
#
# Every comment block in the file is removed. A file the change touches is the unit, so a block from
# before the change goes with the rest. The file is rewritten in place. Each removed block is written to `<dir>/<n>.facts` under the site it
# sat on, as the STRIPPED file numbers it, which is the file the writer reads. Stdout lists the same
# sites. Exit 1 removed something, 0 removed none, 2 did not run.
#
# tested by: the Go suite in ai/tools/comment-strip/. The shared stub region and the resolver it
# calls have their own cases in the Go suite in ai/tools/reach/.

set -euo pipefail

tool="comment-strip"
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
