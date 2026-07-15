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
#   discard          throwaway only: remove this ship's local scratch (report + intent file), and the
#                    whole .idsd/ + its local exclusion when nothing else remains — for `done` cleanup
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

# This ship's intent slug — the first whitespace-delimited token of the report's `intent:` line,
# guarded to the slug charset. Prints nothing when the intent is a standalone `review:` (no slug) or
# holds any char outside the set (notably `/`), so a slug can never `../`-escape a path it indexes.
# Single source for both `state`'s archive probe and `discard`'s intent-file removal.
intent_slug() {
  local intent slug
  intent=$(grep -m1 '^intent:' "$report" 2>/dev/null | sed -e 's/^intent:[[:space:]]*//' -e 's/^"//' -e 's/"$//')
  slug=${intent%%[[:space:]]*}
  case "$slug" in
    review:* | "" | *[!0-9A-Za-z._-]*) ;;
    *) echo "$slug" ;;
  esac
}

# Stop excluding .idsd/ locally: drop the whole-line entry from .git/info/exclude (a no-op if absent).
# Shared by `promote` (going durable) and `discard` (removing the scratch dir).
drop_local_exclusion() {
  local exclude tmp
  exclude=$(exclude_path)
  [ -f "$exclude" ] || return 0
  tmp=$(mktemp)
  grep -vxF '.idsd/' "$exclude" >"$tmp" 2>/dev/null || true
  mv "$tmp" "$exclude"
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
    drop_local_exclusion
    gitignore="$root/.gitignore"
    grep -qxF '.idsd/ship-report.md' "$gitignore" 2>/dev/null || printf '.idsd/ship-report.md\n' >>"$gitignore"
    git -C "$root" add .idsd .gitignore
    echo "promoted: .idsd/ staged, report ignored via .gitignore — commit when ready (not committed here)"
    ;;

  discard)
    require_report
    # throwaway-only cleanup, run by `done` after the code has landed: a throwaway run promises zero
    # traces, but `done` would otherwise leave the report + archived intent behind. Committed repos keep
    # .idsd/ as their durable record, so refuse there.
    if [ "$(mode)" = committed ]; then
      echo "committed idsd repo — .idsd/ is the durable record; nothing to discard" >&2
      exit 2
    fi
    # Remove only this ship's intent file (both pre- and post-archive locations); a standalone
    # `review:` intent has no slug and no file to remove.
    slug=$(intent_slug)
    [ -n "$slug" ] && rm -f "$root/.idsd/intents/$slug.md" "$root/.idsd/archive/$slug.md"
    rm -f "$report"
    # If no intents remain in either location, this ship was the last occupant — remove the whole
    # scratch dir (regenerable roadmap and OS junk with it) and stop excluding it, leaving the repo
    # pristine. Checking the intent dirs, not the .idsd/ root, avoids a stray dotfile falsely keeping it.
    rmdir "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null || true
    if [ -z "$(ls -A "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null)" ]; then
      rm -rf "$root/.idsd"
      drop_local_exclusion
      echo "discarded: removed .idsd/ scratch and its local exclusion (throwaway, zero traces)"
    else
      echo "discarded: removed this ship's report + intent; kept .idsd/ (other intents remain)"
    fi
    ;;

  state)
    # Routing token for `idsd-ship continue` — one word to stdout. Derives where the change set
    # stands from the same signals `gate` uses (reviewed-tree stamp, tree freshness, open TODOs)
    # plus the intent's archive location.
    if [ ! -f "$report" ]; then
      echo "no-report"
      exit 0
    fi
    # A built intent's file has been moved to archive/ (a standalone `review:` has no slug, so
    # intent_slug is empty and this is skipped — it never reaches `done`).
    slug=$(intent_slug)
    if [ -n "$slug" ] && [ -f "$root/.idsd/archive/$slug.md" ]; then
      echo "done"
      exit 0
    fi
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
    echo "usage: report.sh {init <intent>|mode|stamp|gate|carry|check-ignore|promote|discard|state}" >&2
    exit 2
    ;;
esac
