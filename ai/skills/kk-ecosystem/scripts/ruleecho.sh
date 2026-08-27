#!/usr/bin/env bash
# Build and run the rule-echo tool over an ecosystem root, from any working directory.
#
# Usage: ruleecho.sh <root> [file ...]
#
# Exits 0 with no pairs, 1 with pairs, 2 when the scan did not run.
#
# The step that calls this used to build the tool with a path relative to the caller's cwd, which
# resolves only inside the repo that ships this skill. Run the lane against any other ecosystem root
# and the build failed, the step was skipped, and the only cross-file restatement detector we have
# printed nothing — which reads exactly like a clean tree. So every failure below exits 2 and says
# what did not happen: a scan that could not run must never be mistaken for one that found nothing.
#
# The tool is located from this script's own resolved path, never from cwd. The skill is reached
# through a symlink at `~/.claude/skills/kk-ecosystem`, so the resolution has to follow it — `pwd -P`
# does, and lands in the checkout that actually ships the tool.
#
# tested by: ruleecho-test.sh
set -euo pipefail

die() {
  printf 'ruleecho.sh: %s\n' "$1" >&2
  exit 2
}

# `CDPATH=` because `cd` consults it for a relative path and echoes where it landed, which would put a
# second line into the command substitution and corrupt every path built from it.
here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" ||
  die "cannot resolve my own directory, so the tool cannot be located"
tools="$here/../../../tools"

[ $# -ge 1 ] || die "usage: ruleecho.sh <root> [file ...]"
[ -d "$tools/ruleecho" ] ||
  die "no rule-echo source at $tools/ruleecho — this skill is mounted from a checkout that does not ship it, and NO restatement scan ran"
command -v go >/dev/null 2>&1 ||
  die "go is not installed, so the restatement scan did NOT run — it is not clean, it is unchecked"

mkdir -p "$tools/bin" || die "cannot create $tools/bin"
(CDPATH= cd "$tools" && go build -o bin/ruleecho ./ruleecho/) ||
  die "the rule-echo tool did not build, so NO restatement scan ran"

exec "$tools/bin/ruleecho" "$@"
