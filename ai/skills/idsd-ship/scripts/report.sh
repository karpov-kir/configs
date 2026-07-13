#!/usr/bin/env bash
# idsd-ship report tool — the deterministic gates the skill must not execute by hand.
# Operates on the repo-root idsd-ship-report.md and git. Portable: bash + git + awk/sed,
# no project runtime. Subcommands:
#   stamp          compute the tree fingerprint and write it to reviewed-tree
#   gate           done-blocker: stale tree OR any open `- [ ]` → non-zero + reasons
#   carry          print prior open `- [ ]` (with their section) so re-review loses none
#   check-ignore   assert the report is gitignored
#   state          print the `continue` routing token: no-report|resume|re-review|decide|ready|done
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
    # Must run before any fingerprinting `git add -A` (stamp/gate) so the report is never staged.
    if git -C "$root" check-ignore -q "$report"; then
      echo "ok: idsd-ship-report.md is already ignored"
      exit 0
    fi
    if [ -d "$root/.idsd" ]; then
      # idsd repo: the ignore entry is shared and committed with the rest of the idsd setup.
      echo "WARN: idsd-ship-report.md is NOT gitignored — add it to .gitignore" >&2
      exit 1
    fi
    # Non-idsd repo: ignore locally so `.gitignore` stays untouched and no ignore diff enters the change set.
    exclude=$(git -C "$root" rev-parse --git-path info/exclude)
    case "$exclude" in /*) ;; *) exclude="$root/$exclude" ;; esac
    mkdir -p "$(dirname "$exclude")"
    printf 'idsd-ship-report.md\n' >>"$exclude"
    echo "ok: idsd-ship-report.md excluded locally via .git/info/exclude (.gitignore untouched — no .idsd)"
    ;;

  state)
    # Routing token for `idsd-ship continue` — one word to stdout. Derives where the change set
    # stands from the same signals `gate` uses (reviewed-tree stamp, tree freshness, open TODOs)
    # plus the intent's archive location.
    if [ ! -f "$report" ]; then
      echo "no-report"
      exit 0
    fi
    intent=$(grep -m1 '^intent:' "$report" 2>/dev/null | sed -e 's/^intent:[[:space:]]*//' -e 's/^"//' -e 's/"$//')
    case "$intent" in
      review:*) ;; # a standalone review has no merge/archive target — never reaches `done`
      *)
        slug=${intent%%[[:space:]]*}
        if [ -n "$slug" ] && [ -f "$root/.idsd/archive/$slug.md" ]; then
          echo "done"
          exit 0
        fi
        ;;
    esac
    reviewed=$(reviewed_tree)
    case "$reviewed" in
      "" | "<hash>") # never stamped → quality stages haven't completed
        echo "resume"
        exit 0
        ;;
    esac
    if [ "$(current_tree)" != "$reviewed" ]; then
      echo "re-review" # reviewed once, tree moved since
      exit 0
    fi
    if [ -n "$("$todo_gate" "$report")" ]; then
      echo "decide" # quality done, tree fresh, open `- [ ]` remain
      exit 0
    fi
    echo "ready" # quality done, tree fresh, nothing open → merge-ready
    ;;

  *)
    echo "usage: report.sh {stamp|gate|carry|check-ignore|state}" >&2
    exit 2
    ;;
esac
