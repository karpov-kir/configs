#!/usr/bin/env bash
# The judge: what a named reader would delete from an outward text, decided by a model that sees only
# what that reader sees.
#
#   usage: reader-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]   # no path reads stdin
#
# Set JUDGE_PROVIDER=codex or JUDGE_PROVIDER=claude explicitly. Missing, unknown or unavailable
# providers fail with exit 2; no auto-selection or fallback. models.json selects the judge model.
# A legacy JUDGE_MODEL setting is refused.
#
# Prints the artifact with the judged units deleted, or with --numbers the 1-based line each deleted
# unit starts on. Exit 0 when nothing went, 1 when something did, 2 when it did not run — an unknown
# kind, an unreadable path, a provider that refused that model name, a model that did not answer
# inside its deadline, or an answer that was not numbers.
#
# Every attempted roll is bounded. Cancellation supplies no verdict.
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/reader-judge.conf` retunes the bound on this machine
# with a `roll-timeout <seconds>` line. `ai/tools/reader-judge/deadline.go` holds the figure it replaces.
#
# What the model may do, and why it is safe, is the package doc in `ai/tools/reader-judge/judge.go`.
#
# tested by: the Go suite in ai/tools/reader-judge/. The shared stub region and the resolver it
# calls have their own cases in the Go suite in ai/tools/reach/.

set -euo pipefail

tool="reader-judge"
# How far THIS file sits above the tools directory.
tools_offset="../.."

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
