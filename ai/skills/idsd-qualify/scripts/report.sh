#!/usr/bin/env bash
# Qualify report tool — the deterministic gates the skills must not execute by hand. The mechanism
# lives in `ai/tools/eco-report/`; the contract it serves (repo modes, what goes in the report, never
# commit it) is `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**. idsd-ship calls it too
# (gate/state/promote/discard). One report per intent, at
# .idsd/qualify-reports/<intent>-qualify-report.md, so two ships never share a file.
#   usage: report.sh {init <intent>|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|
#                     stamp "<stages>"|gate|carry|check-ignore|promote|discard|close|state|list} [<intent>]
#
# Two sibling files are found from argv[0] and one from $HOME, so this must stay in the skill's
# scripts/ directory: ./todo-gate.sh, ../templates/qualify-report-template.md, and
# `~/.kk-flavor/scripts/tree-fingerprint.sh`, which owns the tree-fingerprint recipe the freshness
# gate reads. Both installs symlink into the same repo, so they ship together or not at all.
#
# A change to a gate needs a case in that package's suite: `cd ai/tools && go test -count=1 ./...`.
#
# tested by: tool-stub-test.sh, and the resolver it calls by resolve-test.sh

set -euo pipefail

tool="eco-report"

# --- shared:tool-stub ---
# Byte-identical in every stub that reaches a tool this way; the wiring check's shared-region scan is
# what holds it that way. Duplicated rather than sourced on purpose: sourcing a file is executing it,
# and these run from whatever repository the human is working in.
die() {
  printf '%s: %s\n' "${0##*/}" "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` consults it for a relative path and echoes where it landed, which would put a
# second line into the command substitution and corrupt every path built from it. `pwd -P` follows the
# symlink this skill is mounted by, so the tools directory is found from this file's own resolved
# location and never from cwd.
here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so $tool could not be located"

resolver="$here/../../../tools/resolve.sh"
# The two cases are told apart because they have different fixes: a checkout that ships no ai/tools/
# needs a different install, and a resolver that lost its exec bit needs a chmod.
[ -e "$resolver" ] ||
  die "no resolver at $resolver — this skill is mounted from a checkout that does not ship ai/tools/, and $tool did NOT run"
[ -x "$resolver" ] ||
  die "$resolver is not executable, so $tool did NOT run — chmod +x it"

# The resolver prints one path on stdout and names its own failures on stderr, so its exit is final
# here: anything short of a runnable binary is exit 2, never a run that quietly did nothing.
binary="$("$resolver" "$tool")" || exit 2
[ -n "$binary" ] && [ -x "$binary" ] ||
  die "the resolver named no runnable binary for $tool, so it did NOT run"

# argv[0] stays the path this was invoked by instead of becoming the binary's: the tools read their
# skill directory from it, the way the shell versions read it from $0, so a skill reached through its
# symlink mount still resolves its own ledger, template and sibling scripts.
exec -a "$0" "$binary" "$@"
# --- end shared:tool-stub ---
