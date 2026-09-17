#!/usr/bin/env bash
# The judge: what a named reader would delete from an outward text, decided by a model that sees only
# what that reader sees.
#
#   usage: bloat-judge.sh [--config <policy.json>] [--numbers] [--changed[=<revisions>]] <kind> [<path>]   # no path reads stdin
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
# `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/bloat-judge.conf` retunes the bound on this machine
# with a `roll-timeout <seconds>` line; `ai/tools/bloat-judge/deadline.go` holds the figure it replaces.
#
# What the model may do, and why it is safe, is the package doc in `ai/tools/bloat-judge/judge.go`.
#
# tested by: the Go suite beside the tool, `ai/tools/bloat-judge/`; the shared stub region below by
# the Go suite in ai/tools/reach/, which covers the resolver it calls too.

set -euo pipefail

tool="bloat-judge"
# How far THIS file sits above the tools directory.
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
