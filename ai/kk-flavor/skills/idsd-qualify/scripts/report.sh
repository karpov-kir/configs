#!/usr/bin/env bash
# Qualify report tool — the deterministic gates the skills must not execute by hand. idsd-ship calls
# it too. What the gates do, and what each subcommand means, is `ai/tools/eco-report/`.
#   usage: report.sh {init <intent>|root|layout check|layout migrate --dry-run|layout migrate --apply|repo-mode|invalidate|stage-result <json-file>|result-context|decisions-reviewed|scope <base-ref>|stamp "<stages>"|gate|intent-ready <NNN-slug>|carry|check-ignore|promote|discard|finalize|merge-slot|close|state|list|record [--intent <NNN-slug>] <op> <record> "<text>"} [<intent>]
# One line because the tool refuses with this one, byte for byte, and ai/tools/stub_usage_test.go holds
# the two against each other. Every subcommand that reads a report takes the intent last; omit it when
# only one is open.
#
# The report template is found from argv[0], so this must stay in the skill's scripts/ directory:
# ../templates/qualify-report-template.md. `~/.kk-flavor/scripts/tree-fingerprint.sh` is found from
# $HOME. Both installs symlink into the same repo, so they ship together or not at all.
#
# tested by: the Go suite in ai/tools/eco-report/. The shared stub region and the resolver it calls
# are covered by the Go suite in ai/tools/reach/.

set -euo pipefail

tool="eco-report"
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
