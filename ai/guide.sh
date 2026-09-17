#!/usr/bin/env bash
# Generate the field guide — the page that tells someone who just installed this what it does and
# which skill to reach for.
#
#   usage: guide.sh [--check | --graph | --cost <skill>] [<root>]
#          (no flag)      regenerate <root>/field-guide.html from the skills and the narrative template
#          --check        regenerate into memory and compare, failing when the committed page has drifted
#          --graph        print the workflow map — every priced row, what each dispatches, what each reads
#          --cost <skill> print one skill's per-run tier profile: its own row, what it dispatches, and
#                         what each contract it declares it extends dispatches in turn
#
# One of the three per run, never two: each writes something different to one stdout.
#
# <root> holds kk-flavor/ with skills/ inside it, and defaults to . then ./ai.
#
# `--graph` and `--cost` answer from the tree and models.json and write no file, so neither can go
# stale and neither is gated. They are the cost surface in a terminal: the page says what each
# dispatch buys, and these two say which dispatches one run actually reaches.
#
# Two halves, kept apart on purpose. The narrative is hand-written in
# `ai/tools/eco-guide/field-guide.template.html`; the two inventories are generated — one card per
# skill from its own frontmatter, one per worker from its brief and its model-policy row — because a
# hand-maintained catalogue of a tree this size drifts in silence.
# `--check` is what stops that drift going unnoticed: ai/gate.sh's `guide` unit runs it.
#
# The tool is Go, in `ai/tools/eco-guide/`.
#
# tested by: the Go suite beside the tool, `ai/tools/eco-guide/`; the shared stub region below by
# tool-stub-test.sh, and the resolver it calls by resolve-test.sh.

set -euo pipefail

tool="eco-guide"
# How far THIS file sits above the tools directory.
tools_offset="."

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

# The build about to answer, handed to the tool rather than printed: a stub's own output is a value
# callers parse. Empty when nothing stamped it. `ai/tools/resolve.sh` carries why.
export ECO_TOOL_BUILD="$(cat "$binary.stamp" 2>/dev/null || true)"

# The checkout that answered, which the build stamp does not name: the stamp hashes source, so it moves
# when the source does and says nothing about which commit the tree sits on. `ai/tools/resolve.sh`
# carries why the two are both needed.
export ECO_TOOL_TREE="$(git -C "${resolver%/*}" rev-parse HEAD 2>/dev/null || true)"

# `-a "$0"` keeps argv[0] as the path this was invoked by. The tools derive their skill directory from
# it, so a skill reached through its symlink mount still finds its own ledger, template and siblings.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
