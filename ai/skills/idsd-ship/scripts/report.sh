#!/usr/bin/env bash
# idsd-ship report tool — the deterministic gates the skill must not execute by hand.
# The report always lives at .idsd/ship-report.md. Portable: bash + git + awk/sed,
# no project runtime. Subcommands:
#   init "<intent>"  scaffold .idsd/ + the report from the template, stamping its intent line
#   mode             print committed|throwaway — is .idsd/ tracked in git?
#   stamp            compute the tree fingerprint and write it to reviewed-tree
#   gate             done-blocker: stale tree OR any open `- [ ]` → non-zero + reasons
#   carry            print prior open `- [ ]` (with their section) so re-qualify loses none
#   check-ignore     keep the report out of the fingerprint by the mechanism that fits the mode
#   promote          throwaway → committed: stop excluding .idsd/, ignore report via .gitignore, stage
#   state            print the `continue` routing token: no-report|resume|re-qualify|decide|ready|done
#
# committed vs throwaway (the mode): a repo that has *committed* .idsd/ content durably uses idsd —
# the report is ignored via a shared, tracked .gitignore entry and .idsd/ changes get committed. A repo
# with no .idsd/, or an untracked one a single-shot run created, is throwaway — the whole .idsd/ (intents
# + report) is excluded locally via .git/info/exclude, leaving zero traces and never touching .gitignore.
#
# Open-`- [ ]` scanning is a separate concern, owned by sibling `todo-gate.sh` (shared with
# idsd-build); gate and carry delegate to it. This script only preserves and gates;
# deciding a finding is resolved stays human/agent judgment.
set -uo pipefail

root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "error: not a git repo" >&2
  exit 2
}
report="$root/.idsd/ship-report.md"
skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
template="$skill_dir/templates/ship-report-template.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"

# committed when .idsd/ has any tracked file; throwaway otherwise (absent, or untracked scratch).
mode() {
  if [ -n "$(git -C "$root" ls-files .idsd 2>/dev/null)" ]; then
    echo committed
  else
    echo throwaway
  fi
}

# Absolute path to .git/info/exclude (git-path is repo-relative for a normal repo).
exclude_path() {
  local p
  p=$(git -C "$root" rev-parse --git-path info/exclude)
  case "$p" in /*) echo "$p" ;; *) echo "$root/$p" ;; esac
}

require_report() {
  [ -f "$report" ] || {
    echo "error: no ship-report.md ($report)" >&2
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
  init)
    intent_val="${2:-}"
    [ -n "$intent_val" ] || {
      echo "usage: report.sh init \"<intent frontmatter value>\"" >&2
      exit 2
    }
    # Frontmatter is single-line: collapse any CR/LF so the value can't inject extra frontmatter
    # lines (e.g. a forged reviewed-tree) — matters now the intent may be seeded from a fetched ticket.
    intent_val=${intent_val//[$'\n\r']/ }
    [ -f "$template" ] || {
      echo "error: template not found ($template)" >&2
      exit 2
    }
    mkdir -p "$(dirname "$report")"
    cp "$template" "$report"
    tmp=$(mktemp)
    # Pass via ENVIRON, not -v: awk's -v processes C escapes, so a backslash in the value would be mangled.
    intent_val="$intent_val" awk '!done_i && /^intent:/ { print "intent: " ENVIRON["intent_val"]; done_i = 1; next } { print }' "$report" >"$tmp" && mv "$tmp" "$report"
    echo "initialized $report (mode: $(mode), intent: $intent_val)"
    ;;

  mode)
    mode
    ;;

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
      echo "BLOCK (freshness): tree changed since last qualify (current $current != reviewed ${reviewed:-<unstamped>}). Re-qualify, or the human may explicitly override this one." >&2
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
    # Must run before any fingerprinting `git add -A` (stamp/gate) so nothing scratch is ever staged.
    if [ "$(mode)" = committed ]; then
      # committed idsd repo: the report ignore is shared and committed with the rest of the idsd setup.
      if git -C "$root" check-ignore -q "$report"; then
        echo "ok: report is gitignored (committed idsd repo)"
        exit 0
      fi
      echo "WARN: report is NOT gitignored — add '.idsd/ship-report.md' to .gitignore (shared idsd setup)" >&2
      exit 1
    fi
    # throwaway: exclude the *whole* .idsd/ locally so intents + report leave zero traces —
    # .gitignore is never touched and `git add -A` skips the dir entirely.
    exclude=$(exclude_path)
    mkdir -p "$(dirname "$exclude")"
    grep -qxF '.idsd/' "$exclude" 2>/dev/null || printf '.idsd/\n' >>"$exclude"
    echo "ok: throwaway run — .idsd/ excluded locally via .git/info/exclude (.gitignore untouched)"
    ;;

  promote)
    require_report
    # throwaway → committed: keep .idsd/ durably. Drop the local exclusion, share the report ignore via
    # a tracked .gitignore entry, and stage .idsd/. Never commits — the human/idsd-build owns that.
    if [ "$(mode)" = committed ]; then
      echo "already committed — .idsd/ is tracked; nothing to promote"
      exit 0
    fi
    exclude=$(exclude_path)
    if [ -f "$exclude" ]; then
      tmp=$(mktemp)
      grep -vxF '.idsd/' "$exclude" >"$tmp" 2>/dev/null || true
      mv "$tmp" "$exclude"
    fi
    gitignore="$root/.gitignore"
    grep -qxF '.idsd/ship-report.md' "$gitignore" 2>/dev/null || printf '.idsd/ship-report.md\n' >>"$gitignore"
    git -C "$root" add .idsd .gitignore
    echo "promoted: .idsd/ staged, report ignored via .gitignore — commit when ready (not committed here)"
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
        # Guard the slug before it indexes a path: reject empty or any char outside the slug set
        # (notably `/`), so an attacker-influenced intent can't `../`-escape the archive probe.
        case "$slug" in
          "" | *[!0-9A-Za-z._-]*) ;;
          *)
            if [ -f "$root/.idsd/archive/$slug.md" ]; then
              echo "done"
              exit 0
            fi
            ;;
        esac
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
      echo "re-qualify" # reviewed once, tree moved since
      exit 0
    fi
    if [ -n "$("$todo_gate" "$report")" ]; then
      echo "decide" # quality done, tree fresh, open `- [ ]` remain
      exit 0
    fi
    echo "ready" # quality done, tree fresh, nothing open → merge-ready
    ;;

  *)
    echo "usage: report.sh {init <intent>|mode|stamp|gate|carry|check-ignore|promote|state}" >&2
    exit 2
    ;;
esac
