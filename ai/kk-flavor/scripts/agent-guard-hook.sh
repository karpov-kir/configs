#!/usr/bin/env bash
# The PreToolUse hook command bootstrap registers on the Agent tool. It runs agent-guard.sh on the hook's
# JSON and passes the guard's own refusal through as exit 2. Every other failure runs the dispatch with a
# note on stderr. The resolver also exits 2 when it cannot build the guard, and a hook would read that as
# a refusal of every dispatch.
#
#   usage: agent-guard-hook.sh < <the hook's JSON>
#
# tested by: the Go suite in ai/tools/agent-guard/.

set -uo pipefail

here="$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || exit 0
said="$("$here/agent-guard.sh" 2>&1)"
code=$?
case "$code:$said" in
  "2:agent-guard refused this dispatch:"*)
    printf '%s\n' "$said" >&2
    exit 2
    ;;
  0:*) exit 0 ;;
esac
printf 'agent-guard did not run, so the dispatch runs unchecked: %s\n' "${said%%$'\n'*}" >&2
exit 0
