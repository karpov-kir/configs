#!/usr/bin/env bash
# Qualify report tool — the deterministic gates the skills must not execute by hand. Owned by
# idsd-qualify (which writes the report); idsd-ship calls it too (gate/state/promote/discard).
# The report always lives at .idsd/ship-report.md. Portable: bash + git + awk/sed,
# no project runtime. Subcommands:
#   init "<intent>"  scaffold .idsd/ + the report from the template, stamping its intent line
#   repo-mode        print committed|throwaway — is .idsd/ tracked in git?
#   invalidate       clear reviewed-tree/reviewed-stages at pass start, so no stamp outlives its tree
#   stage-returned <stage>  mark a stage returned, recording the report as it then stood; stamp refuses until
#                    the report has changed since, so a stage's items cannot be left unrecorded
#   no-items <stage> mark a stage as having surfaced nothing, the one way to clear its marker without
#                    editing the report
#   stamp "<stages>" compute the tree fingerprint (throwaway index) and record reviewed-tree +
#                    reviewed-stages — every pipeline stage, bare (ran) or `:skipped(reason)` /
#                    `refactor:partial(reason)`; any `(fast)` reason marks the pass not-full
#   gate             done-blocker: stale tree OR turnaround-trimmed stages OR any open `- [ ]`
#                    → non-zero + reasons
#   carry            print prior open `- [ ]` (with their section) so re-qualify loses none
#   check-ignore     keep the report out of the fingerprint by the mechanism that fits the repo mode
#   promote          throwaway → committed: stop excluding .idsd/, ignore report via .gitignore, stage
#   discard          throwaway only: remove this ship's local scratch (report + intent file), and the
#                    whole .idsd/ + its local exclusion when nothing else remains — for `done` cleanup
#   state            print the `continue` routing token:
#                    no-report|resume|re-qualify|decide|finalize|ready|done
#
# Repo mode, short version: committed = .idsd/ has tracked content (report ignored via a shared
# .gitignore entry); throwaway = whole .idsd/ excluded via .git/info/exclude, zero traces. The full
# contract lives in idsd-qualify's SKILL.md → Report — the owner; this header is just the map.
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
# Written into a marker instead of a checksum when a stage returned with nothing the report should carry.
NO_ITEMS="no-items"

report_checksum() {
  cksum <"$report"
}

# The five pipeline stages, by name — the only values stage-returned and no-items accept.
valid_stage() {
  case "$1" in
    code-review | security-review | tighten | refactor | retro) printf '%s' "$1" ;;
    *)
      echo "usage: report.sh $2 <code-review|security-review|tighten|refactor|retro>" >&2
      exit 2
      ;;
  esac
}

# Why a stage may not be stamped yet, or nothing if it may. A stage that ran must have been marked returned, and the
# report must have changed since that mark — otherwise its findings were never written down.
stage_block_reason() {
  marker="$root/.idsd/.stage-returns/$1"

  [ -f "$marker" ] || {
    printf 'ran but was never marked returned (report.sh stage-returned %s)' "$1"
    return
  }

  [ "$(cat "$marker")" != "$NO_ITEMS" ] || return

  [ "$(cat "$marker")" != "$(report_checksum)" ] || {
    printf 'returned but the report is unchanged since — record its items, or report.sh no-items %s' "$1"
  }
}
skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
template="$skill_dir/templates/ship-report-template.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"

# committed when .idsd/ has any tracked file; throwaway otherwise (absent, or untracked scratch).
repo_mode() {
  if [ -n "$(git -C "$root" ls-files .idsd 2>/dev/null)" ]; then
    echo committed
  else
    echo throwaway
  fi
}

# Absolute path to .git/info/exclude (git-path is repo-relative for a normal repo).
exclude_path() {
  local path
  path=$(git -C "$root" rev-parse --git-path info/exclude)
  case "$path" in /*) echo "$path" ;; *) echo "$root/$path" ;; esac
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

# git add -A && git write-tree against a throwaway index — the fingerprint the freshness gate
# compares. Never touches the human's staging area (a polluted real index once carried ~19k lines
# of stray browser debris into a stamp).
current_tree() {
  local tmp_index tree=""
  tmp_index=$(mktemp) && rm -f "$tmp_index"
  GIT_INDEX_FILE="$tmp_index" git -C "$root" add -A &&
    tree=$(GIT_INDEX_FILE="$tmp_index" git -C "$root" write-tree)
  rm -f "$tmp_index"
  echo "$tree"
}

reviewed_tree() {
  grep -m1 '^reviewed-tree:' "$report" 2>/dev/null | sed 's/^reviewed-tree:[[:space:]]*//'
}

# Stage entries of the last stamp (comma-separated). Empty when never stamped with stages —
# callers treat that as not-full (covers reports predating the stage record).
reviewed_stages() {
  grep -m1 '^reviewed-stages:' "$report" 2>/dev/null | sed 's/^reviewed-stages:[[:space:]]*//'
}

# Entries the last stamp marked `(fast)` — stages trimmed for turnaround. Non-empty ⇒ not a full pass.
fast_trims() {
  reviewed_stages | tr ',' '\n' | grep '(fast)' | tr '\n' ' ' | sed 's/ $//'
}

case "${1:-}" in
  init)
    intent_value="${2:-}"
    [ -n "$intent_value" ] || {
      echo "usage: report.sh init \"<intent frontmatter value>\"" >&2
      exit 2
    }
    # Frontmatter is single-line: collapse any CR/LF so the value can't inject extra frontmatter
    # lines (e.g. a forged reviewed-tree) — matters now the intent may be seeded from a fetched ticket.
    intent_value=${intent_value//[$'\n\r']/ }
    [ -f "$template" ] || {
      echo "error: template not found ($template)" >&2
      exit 2
    }
    mkdir -p "$(dirname "$report")"
    cp "$template" "$report"
    tmp=$(mktemp)
    # Pass via ENVIRON, not -v: awk's -v processes C escapes, so a backslash in the value would be mangled.
    intent_value="$intent_value" awk '!replaced && /^intent:/ { print "intent: " ENVIRON["intent_value"]; replaced = 1; next } { print }' "$report" >"$tmp" && mv "$tmp" "$report"
    echo "initialized $report (repo mode: $(repo_mode), intent: $intent_value)"
    ;;

  repo-mode)
    repo_mode
    ;;

  # Recording a stage's findings is prose discipline, and prose discipline failed twice: a report two hours behind
  # the pass asserted stages had never run. This makes the omission mechanical. A marker holds the report's checksum
  # as it stood when that stage returned, so "were this stage's items written down?" is answered per stage and
  # regardless of the order the markers were made in — an mtime comparison could do neither, because the round's
  # stages return concurrently and one report edit would clear every marker at once.
  stage-returned)
    require_report
    stage=$(valid_stage "${2:-}" stage-returned)
    mkdir -p "$root/.idsd/.stage-returns"
    report_checksum >"$root/.idsd/.stage-returns/$stage"
    echo "recorded return of $stage — record its items in the report before stamping"
    ;;

  # The escape hatch for a stage that genuinely surfaced nothing. Without it the only way to clear a marker is to
  # edit the report, and the report's own contract bans the per-stage line that would say "nothing to record".
  no-items)
    require_report
    stage=$(valid_stage "${2:-}" no-items)
    mkdir -p "$root/.idsd/.stage-returns"
    echo "$NO_ITEMS" >"$root/.idsd/.stage-returns/$stage"
    echo "recorded $stage as having surfaced nothing for the report"
    ;;

  stamp)
    require_report
    entries="${2:-}"
    [ -n "$entries" ] || {
      echo "usage: report.sh stamp \"<stage[,stage:skipped(reason),...]>\" — all of: code-review security-review tighten refactor retro" >&2
      exit 2
    }
    entries=$(printf '%s' "$entries" | tr -d '[:space:]')
    # Validate: exactly the five pipeline stages, each once. code-review always runs (bare);
    # refactor runs at least once (bare or :partial(reason)); the rest bare or :skipped(reason).
    invalid=$(printf '%s\n' "$entries" | tr ',' '\n' | awk '
      /^code-review$/ { seen["code-review"]++; next }
      /^refactor(:partial\([a-z0-9-]+\))?$/ { seen["refactor"]++; next }
      /^(security-review|tighten|retro)(:skipped\([a-z0-9-]+\))?$/ { name = $0; sub(/:.*/, "", name); seen[name]++; next }
      { print "malformed entry: " $0 }
      END {
        split("code-review security-review tighten refactor retro", required, " ")
        for (i in required) {
          if (!(required[i] in seen)) print "missing stage: " required[i]
          else if (seen[required[i]] > 1) print "duplicate stage: " required[i]
        }
      }')
    [ -z "$invalid" ] || {
      printf 'error: invalid stage record\n%s\n' "$invalid" >&2
      exit 2
    }
    # Every stage this record says ran must also have been marked returned, with its items recorded since. Validating
    # the entry string alone let a pass stamp `refactor,retro` while neither had run: the shape was legal, and a stage
    # that never marked itself looked identical to one that had nothing to say.
    blocked=$(printf '%s\n' "$entries" | tr ',' '\n' | while read -r entry; do
      case "$entry" in
        *:skipped\(*) continue ;;
      esac
      stage_name=${entry%%:*}
      reason=$(stage_block_reason "$stage_name")
      [ -z "$reason" ] || printf '  %s: %s\n' "$stage_name" "$reason"
    done)
    [ -z "$blocked" ] || {
      printf 'error: these stages are recorded as having run, but:\n%s\n' "$blocked" >&2
      exit 2
    }
    grep -q '^reviewed-tree:' "$report" || {
      echo "error: no 'reviewed-tree:' line in frontmatter" >&2
      exit 2
    }
    tree=$(current_tree)
    tmp=$(mktemp)
    # Rewrite frontmatter only (never a body line quoting a field): refresh reviewed-tree, write
    # reviewed-stages next to it, retire any reviewed-mode line from before stages subsumed it.
    entries="$entries" tree="$tree" awk '
      NR > 1 && /^---[[:space:]]*$/ { in_frontmatter = 0 }
      in_frontmatter && /^reviewed-mode:/ { next }
      in_frontmatter && /^reviewed-stages:/ { next }
      in_frontmatter && /^reviewed-tree:/ { print "reviewed-tree: " ENVIRON["tree"]; print "reviewed-stages: " ENVIRON["entries"]; next }
      { print }
      NR == 1 { in_frontmatter = 1 }
    ' "$report" >"$tmp" && mv "$tmp" "$report"
    echo "stamped reviewed-tree: $tree (stages: $entries)"
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
    stages=$(reviewed_stages)
    trims=$(fast_trims)
    if [ -z "$stages" ] || [ "$stages" = pending ]; then
      echo "BLOCK (stages): no reviewed-stages record — run a full qualify (it stamps the stage set), or the human may explicitly override this one." >&2
      blocked=1
    elif [ -n "$trims" ]; then
      echo "BLOCK (stages): trimmed for turnaround ($trims) — run a full qualify before merge, or the human may explicitly override this one." >&2
      blocked=1
    fi
    todos=$("$todo_gate" "$report")
    if [ -n "$todos" ]; then
      echo "BLOCK (open TODOs): clear each before merge — no override." >&2
      echo "$todos" >&2
      blocked=1
    fi
    [ "$blocked" -eq 0 ] && echo "gate clean: tree fresh, full qualify, no open TODOs"
    exit "$blocked"
    ;;

  carry)
    require_report
    "$todo_gate" "$report" || true
    ;;

  invalidate)
    # Clears the stamp at the start of a pass, so a stamp can never outlive the tree it describes —
    # the merge gate must not read a review state the pass under way has already invalidated.
    require_report
    tmp=$(mktemp)
    awk '
      NR > 1 && /^---[[:space:]]*$/ { in_frontmatter = 0 }
      in_frontmatter && /^reviewed-tree:/ { print "reviewed-tree: pending"; next }
      in_frontmatter && /^reviewed-stages:/ { print "reviewed-stages: pending"; next }
      { print }
      NR == 1 { in_frontmatter = 1 }
    ' "$report" >"$tmp" && mv "$tmp" "$report"
    # Last pass's stage returns are not this pass's; left behind they would block its stamp forever.
    rm -rf "$root/.idsd/.stage-returns"
    echo "invalidated reviewed-tree — restamp when the pass completes"
    ;;

  check-ignore)
    # Runs before anything else — init included, so it never requires the report to exist — and
    # before any fingerprinting `git add -A` (stamp/gate), so nothing scratch is ever staged.
    if [ "$(repo_mode)" = committed ]; then
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
    if [ "$(repo_mode)" = committed ]; then
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
    if [ "$(repo_mode)" = committed ]; then
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
      # The exclusion lives in the shared .git/info/exclude — every worktree reads it, and a
      # parallel throwaway ship's .idsd/ must stay excluded. Drop it only from the last worktree.
      if [ "$(git -C "$root" worktree list --porcelain 2>/dev/null | grep -c '^worktree ')" -gt 1 ]; then
        echo "discarded: removed .idsd/ scratch; kept the shared exclusion (other worktrees exist)"
      else
        drop_local_exclusion
        echo "discarded: removed .idsd/ scratch and its local exclusion (throwaway, zero traces)"
      fi
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
      "" | "<hash>" | pending) # never stamped, or invalidated mid-pass → quality stages haven't completed
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
    if [ -z "$(reviewed_stages)" ] || [ -n "$(fast_trims)" ]; then
      echo "finalize" # stages trimmed (or unrecorded) and fresh, nothing open — a full qualify remains
      exit 0
    fi
    echo "ready" # full-reviewed, tree fresh, nothing open → merge-ready
    ;;

  *)
    echo "usage: report.sh {init <intent>|repo-mode|invalidate|stamp \"<stages>\"|gate|carry|check-ignore|promote|discard|state}" >&2
    exit 2
    ;;
esac
