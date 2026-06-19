#!/usr/bin/env bash
# idsd-ship report tool — the deterministic gates the skill must not execute by hand.
# Operates on the repo-root idsd-ship-report.md and git. Portable: bash + git + awk/sed,
# no project runtime. Subcommands:
#   stamp          compute the tree fingerprint and write it to reviewed-tree
#   gate           done-blocker: stale tree OR any open `- [ ]` → non-zero + reasons
#   carry          print prior open `- [ ]` (with their section) so re-review loses none
#   check-ignore   assert the report is gitignored
#
# Open-`- [ ]` scanning is a separate concern, owned by sibling `todo-gate.sh` (shared with
# idsd-build); gate and carry delegate to it. This script only preserves and gates;
# deciding a finding is resolved stays human/agent judgment.
set -uo pipefail

root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "error: not a git repo" >&2
  exit 2
}
report="$root/idsd-ship-report.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"

require_report() {
  [ -f "$report" ] || {
    echo "error: no idsd-ship-report.md at repo root ($report)" >&2
    exit 2
  }
}

# git add -A && git write-tree — the fingerprint the freshness gate compares. Matches the
# index to the working tree as a side effect (same as the skill's prior manual step).
current_tree() {
  git -C "$root" add -A
  git -C "$root" write-tree
}

reviewed_tree() {
  grep -m1 '^reviewed-tree:' "$report" 2>/dev/null | sed 's/^reviewed-tree:[[:space:]]*//'
}

case "${1:-}" in
  stamp)
    require_report
    grep -q '^reviewed-tree:' "$report" || {
      echo "error: no 'reviewed-tree:' line in frontmatter" >&2
      exit 2
    }
    tree=$(current_tree)
    tmp=$(mktemp)
    # First match only — the frontmatter line, never a body line that quotes the field.
    awk -v tree="$tree" '!stamped && /^reviewed-tree:/ { print "reviewed-tree: " tree; stamped = 1; next } { print }' "$report" >"$tmp" && mv "$tmp" "$report"
    echo "stamped reviewed-tree: $tree"
    ;;

  gate)
    require_report
    blocked=0
    current=$(current_tree)
    reviewed=$(reviewed_tree)
    if [ "$current" != "$reviewed" ]; then
      echo "BLOCK (freshness): tree changed since last review (current $current != reviewed ${reviewed:-<unstamped>}). Re-review, or the human may explicitly override this one." >&2
      blocked=1
    fi
    todos=$("$todo_gate" "$report")
    if [ -n "$todos" ]; then
      echo "BLOCK (open TODOs): clear each before merge — no override." >&2
      echo "$todos" >&2
      blocked=1
    fi
    [ "$blocked" -eq 0 ] && echo "gate clean: tree fresh, no open TODOs"
    exit "$blocked"
    ;;

  carry)
    require_report
    "$todo_gate" "$report" || true
    ;;

  check-ignore)
    require_report
    if git -C "$root" check-ignore -q "$report"; then
      echo "ok: idsd-ship-report.md is gitignored"
    else
      echo "WARN: idsd-ship-report.md is NOT gitignored — add it to .gitignore" >&2
      exit 1
    fi
    ;;

  *)
    echo "usage: report.sh {stamp|gate|carry|check-ignore}" >&2
    exit 2
    ;;
esac
