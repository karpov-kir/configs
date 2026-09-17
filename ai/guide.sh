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
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="eco-guide"
# How far THIS file sits above the tools directory.
tools_offset="."

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
