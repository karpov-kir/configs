#!/usr/bin/env bash
# Qualify report tool — the deterministic gates the skills must not execute by hand. The mechanism
# lives here; the contract it serves (repo modes, what goes in the report, never commit it) is
# `idsd-qualify/SKILL.md` → **Report**. idsd-ship calls it too (gate/state/promote/discard).
# The report always lives at .idsd/ship-report.md. Portable: bash + git + awk/sed,
# no project runtime. Subcommands:
#   init "<intent>" [--force]  scaffold .idsd/ + the report from the template, stamping its intent
#                    line. Refuses over an existing report unless --force, which prints its open
#                    `- [ ]` and keeps the file as ship-report.superseded.md. Refuses a symlink either way
#   repo-mode        print committed|throwaway — is .idsd/ tracked in git?
#   invalidate       clear reviewed-tree/reviewed-stages and drop the stage markers at pass start, so no
#                    stamp outlives its tree; stamp refuses until this pass has run it
#   stage-returned <stage>  mark a stage returned, recording the report as it then stood; stamp refuses until
#                    the report has changed since, so a stage's items cannot be left unrecorded. One stage at
#                    a time — refused while an earlier mark still has nothing recorded against it
#   no-items <stage> mark a stage already marked returned as having surfaced nothing, the one way to clear
#                    its marker without editing the report
#   stamp "<stages>" compute the tree fingerprint (throwaway index) and record reviewed-tree +
#                    reviewed-stages — every pipeline stage: code-review always bare, refactor bare
#                    or `:partial(fast|cap)`, the other three bare or `:skipped(fast|not-applicable)`;
#                    any `(fast)` marks the pass not-full. The `stamp` usage string is the authority
#   gate             done-blocker: stale tree OR turnaround-trimmed stages OR any open `- [ ]`
#                    → non-zero + reasons
#   carry            print prior open `- [ ]` (with their section) so re-qualify loses none
#   check-ignore     keep the report and its superseded copy out of the fingerprint, by the
#                    mechanism that fits the repo mode
#   promote          throwaway → committed: stop excluding .idsd/, ignore both via .gitignore, stage
#   discard          throwaway only: remove this ship's local scratch (reports, intent file, stage
#                    markers), and the whole .idsd/ + its local exclusion when nothing else remains.
#                    Another intent, or an authored charter/constitution/language/playbook, is
#                    "something" — those are the human's, not this ship's scratch
#   state            print the `continue` routing token:
#                    no-report|resume|re-qualify|decide|finalize|ready|done
#
# Repo mode, short version: committed = .idsd/ has tracked content (the report and its superseded
# copy ignored via shared .gitignore entries); throwaway = whole .idsd/ excluded via
# .git/info/exclude, zero traces.
#
# Open-`- [ ]` scanning is a separate concern, owned by sibling `todo-gate.sh` (shared with
# idsd-build); gate and carry delegate to it. This script only preserves and gates;
# deciding a finding is resolved stays human/agent judgment.
set -uo pipefail
# Byte-oriented, like every sibling script: macOS awk aborts on the first non-UTF-8 byte under a
# UTF-8 locale, and the report can carry one — it quotes code excerpts, and `init` may seed its
# intent line from fetched ticket text.
export LC_ALL=C

# Stop, saying why: one stderr line per argument, then exit 2. Every caller reads 2 as "this did not
# run", never as a result, so every path that stops halfway has to leave by here.
refuse() {
  printf '%s\n' "$@" >&2
  exit 2
}

root=$(git rev-parse --show-toplevel 2>/dev/null) || refuse "error: not a git repo"

# Absolute path to <name> inside this worktree's git dir. --git-path answers relative to the repo for
# an ordinary repo and absolute inside a linked worktree, so anchor the relative form at $root. An
# empty answer is refused rather than passed on: the bare "$root/" it would otherwise build puts
# whatever the caller writes into the working tree, where a commit can pick it up.
git_path() {
  local path
  path=$(git -C "$root" rev-parse --git-path "$1") && [ -n "$path" ] ||
    refuse "error: could not resolve '$1' inside the git dir (git rev-parse --git-path)"
  case "$path" in /*) echo "$path" ;; *) echo "$root/$path" ;; esac
}

report="$root/.idsd/ship-report.md"
# The copy `init --force` keeps of the report it replaced — a copy of the report, so it lives where
# the report lives and is handled as the report is: never committed, never in the fingerprint.
superseded="$root/.idsd/ship-report.superseded.md"
# Per-pass bookkeeping, held in the git dir because that is the one place no commit can reach and no
# `git add -A` can see. Under .idsd/ they would be committed in committed mode, and every later
# `invalidate` would hand the human deletions they never made. Per-worktree, so ships cannot collide.
stage_returns=$(git_path idsd-stage-returns) || exit 2
skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
template="$skill_dir/templates/ship-report-template.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"
# Written into a marker instead of a checksum when a stage returned with nothing the report should carry.
NO_ITEMS="no-items"

report_checksum() {
  cksum <"$report"
}

# The five pipeline stages, by name — the only values stage-returned and no-items accept.
valid_stage() {
  case "$1" in
    code-review | security-review | tighten | refactor | retro) printf '%s' "$1" ;;
    *) refuse "usage: report.sh $2 <code-review|security-review|tighten|refactor|retro>" ;;
  esac
}

# Write a stage's marker, or fail loudly: a marker that was never written while the caller still
# prints "recorded" is a stage the stamp then waves through as handled.
write_stage_marker() {
  local stage="$1" value="$2"
  [ -n "$value" ] || {
    echo "error: the report could not be checksummed — $stage is NOT marked" >&2
    return 1
  }
  mkdir -p "$stage_returns" && printf '%s\n' "$value" >"$stage_returns/$stage" || {
    echo "error: could not write $stage_returns/$stage — $stage is NOT marked" >&2
    return 1
  }
}

# The stage marked returned against the report as it now stands, meaning its items are still
# unwritten. Prints nothing when no stage is. See `stage-returned` for why only one may stand at a
# time. A `no-items` marker holds a word, never a checksum, so a stage settled that way never matches.
outstanding_stage() {
  local marker current
  current=$(report_checksum)
  [ -n "$current" ] || return 1
  for marker in "$stage_returns"/*; do
    [ -f "$marker" ] || continue
    [ "$(cat "$marker")" = "$current" ] || continue
    basename "$marker"
    return 0
  done
  return 1
}

# True once a stage has been marked returned. A stage with no marker never ran, which is what
# separates "returned having surfaced nothing" from "never happened" everywhere this is asked.
stage_was_marked_returned() {
  [ -f "$stage_returns/$1" ]
}

# Why a stage may not be stamped yet, or nothing if it may. A stage that ran must have been marked
# returned, and the report must have changed since that mark. Otherwise nobody wrote its findings down.
stage_block_reason() {
  local recorded
  stage_was_marked_returned "$1" || {
    printf 'ran but was never marked returned (report.sh stage-returned %s)' "$1"
    return
  }
  recorded=$(cat "$stage_returns/$1")

  [ "$recorded" != "$NO_ITEMS" ] || return

  [ "$recorded" != "$(report_checksum)" ] || {
    printf 'returned but the report is unchanged since — record its items, or report.sh no-items %s' "$1"
  }
}

# committed when .idsd/ has any tracked file; throwaway otherwise (absent, or untracked scratch).
repo_mode() {
  if [ -n "$(git -C "$root" ls-files .idsd 2>/dev/null)" ]; then
    echo committed
  else
    echo throwaway
  fi
}

# What this script writes under .idsd/ that must never be committed or reach the tree fingerprint,
# one path per line, relative to $root: the report, and the copy `init --force` keeps of the one it
# replaced. `promote` writes a .gitignore entry per line and `check-ignore` verifies one per line, so
# the two can never disagree. Deliberately absent: the durable record (intents/, archive/,
# charter.md, constitution.md, roadmap.md, decisions.md), which committed mode keeps tracked, and the
# stage markers, which are in the git dir and so are out of git's reach already.
ignore_surface() {
  printf '%s\n' "${report#"$root"/}" "${superseded#"$root"/}"
}

# Add <entry> as a line of <file>, unless it already is one. The trailing-newline check is the whole
# point: appending to a file whose last line has none fuses the two into one, and then neither the
# rule that was there nor the entry just added matches anything — a promotion that reports the report
# ignored while staging it, and a human's own ignore rule silently dropped.
append_line() {
  local file="$1" entry="$2"
  grep -qxF "$entry" "$file" 2>/dev/null && return 0
  if [ -s "$file" ] && [ -n "$(tail -c 1 "$file")" ]; then
    printf '\n' >>"$file" || return 1
  fi
  printf '%s\n' "$entry" >>"$file"
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

# Start excluding .idsd/ locally: add the whole-line entry to .git/info/exclude, unless it is already
# one. Used by `check-ignore` in throwaway mode, and by `promote` to put the exclusion back on every
# path where it refuses after having dropped it.
add_local_exclusion() {
  local exclude
  exclude=$(git_path info/exclude) || return 1
  mkdir -p "$(dirname "$exclude")" || return 1
  append_line "$exclude" '.idsd/'
}

# Stop excluding .idsd/ locally: drop the whole-line entry from .git/info/exclude (a no-op if absent).
# Shared by `promote` (going durable) and `discard` (removing the scratch dir).
drop_local_exclusion() {
  local exclude tmp status
  exclude=$(git_path info/exclude) || return 1
  [ -f "$exclude" ] || return 0
  tmp=$(mktemp) || {
    echo "error: mktemp failed — left the .idsd/ exclusion in $exclude" >&2
    return 1
  }
  grep -vxF '.idsd/' "$exclude" >"$tmp"
  status=$?
  # grep exits 1 when it prints nothing, which is the ordinary "the file held only that entry" case.
  # 2+ means it could not read the file, and moving the empty temp over it would wipe every other
  # local exclusion the repo has.
  [ "$status" -le 1 ] || {
    rm -f "$tmp"
    echo "error: could not read $exclude — left it untouched" >&2
    return 1
  }
  mv "$tmp" "$exclude"
}

require_report() {
  [ -f "$report" ] || refuse "error: no ship-report.md ($report)"
}

# How many open `- [ ]` the report holds, phrased for the human about to lose them. Never answers
# "none" from a scan that did not run: todo-gate.sh exits 2 printing nothing, and a reader that
# counts that as zero promises the human there is nothing to lose.
open_items_phrase() {
  local items status
  items=$("$todo_gate" "$report")
  status=$?
  case "$status" in
    0) echo "no open '- [ ]'" ;;
    1) printf "%s open '- [ ]'\n" "$(printf '%s\n' "$items" | wc -l | tr -d ' ')" ;;
    *) echo "an unknown number of open '- [ ]' — the scan did not run (todo-gate.sh exited $status)" ;;
  esac
}

# Run an awk program over the report and put the result back, atomically: on any failure the report
# is left exactly as it was, and the caller sees a non-zero return. The program is the one argument.
# The two failure messages are named, so that a call site reads without opening this function:
# `no_temp=<what a failed mktemp leaves standing> no_write=<what a failed rewrite leaves standing>
# rewrite_report '<program>'`. awk's own values ride the same environment
# (`v=x rewrite_report … 'ENVIRON["v"]'`), never `-v`: awk's -v processes C escapes, so a backslash
# in the value would be mangled.
rewrite_report() {
  local program="$1" tmp
  # Refuse rather than let `set -u` kill the shell on an unset message: that exits 1, and 1 is a
  # result to every caller of this script, not the "did not run" this is.
  [ -n "${no_temp:-}" ] && [ -n "${no_write:-}" ] ||
    refuse "error: rewrite_report was called without no_temp/no_write — $report was NOT rewritten"
  tmp=$(mktemp) || {
    echo "error: mktemp failed — $no_temp" >&2
    return 1
  }
  awk "$program" "$report" >"$tmp" && mv "$tmp" "$report" || {
    rm -f "$tmp"
    echo "error: $no_write" >&2
    return 1
  }
}

# git add -A && git write-tree against a throwaway index — the fingerprint the freshness gate
# compares. Never touches the human's staging area — a polluted real index carries whatever is
# staged into the stamp.
# Fails loudly rather than printing an empty tree: an empty fingerprint stamps as `reviewed-tree:`
# (empty) and then matches the next equally-failed reading, so the freshness gate reports "tree
# fresh" over a fingerprint that was never computed.
current_tree() {
  local tmp_index tree="" status
  tmp_index=$(mktemp) || {
    echo "error: no throwaway index (mktemp failed) — the tree could not be fingerprinted" >&2
    return 1
  }
  rm -f "$tmp_index"
  GIT_INDEX_FILE="$tmp_index" git -C "$root" add -A &&
    tree=$(GIT_INDEX_FILE="$tmp_index" git -C "$root" write-tree)
  status=$?
  rm -f "$tmp_index"
  [ "$status" -eq 0 ] && [ -n "$tree" ] || {
    echo "error: git add -A / write-tree failed — the tree could not be fingerprinted" >&2
    return 1
  }
  echo "$tree"
}

reviewed_tree() {
  grep -m1 '^reviewed-tree:' "$report" 2>/dev/null | sed 's/^reviewed-tree:[[:space:]]*//'
}

# Every frontmatter value that means "no stamp stands here": absent, `pending` from invalidate, and
# the template's own placeholders. One set, and every reader uses all of it. A reader that knows
# only some of these values accepts what the others reject: a report carrying a real tree beside the
# literal `<stages>` would then gate clean and route `ready` with no stage ever run. `init` holds
# the template to this set.
unstamped() {
  case "$1" in
    "" | pending | "<hash>" | "<stages>") return 0 ;;
  esac
  return 1
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
    shift
    intent_value=""
    is_forced=0
    for arg in "$@"; do
      case "$arg" in
        --force) is_forced=1 ;;
        *) [ -n "$intent_value" ] || intent_value="$arg" ;;
      esac
    done
    [ -n "$intent_value" ] || refuse "usage: report.sh init \"<intent frontmatter value>\" [--force]"
    # Frontmatter is single-line: collapse any CR/LF so the value can't inject extra frontmatter
    # lines (e.g. a forged reviewed-tree) — matters now the intent may be seeded from a fetched ticket.
    intent_value=${intent_value//[$'\n\r']/ }
    [ ! -L "$template" ] ||
      refuse "error: template $template is a symlink — refusing to read the template through one; the report was NOT initialized"
    [ -f "$template" ] || refuse "error: template not found ($template)"
    # gate and state never see the template, so a placeholder that drifts out of unstamped()'s set
    # can only be caught here: it would stamp every brand-new report as already reviewed.
    grep -q '^intent:' "$template" ||
      refuse "error: template $template has no 'intent:' line to stamp — the report was NOT initialized"
    for field in reviewed-tree reviewed-stages; do
      grep -q "^$field:" "$template" ||
        refuse "error: template $template has no '$field:' line — gate and state read it; the report was NOT initialized"
      placeholder=$(grep -m1 "^$field:" "$template" | sed "s/^$field:[[:space:]]*//")
      unstamped "$placeholder" ||
        refuse "error: template $template writes '$field: $placeholder', which gate and state read as a completed review — every new report would gate clean. Restore a placeholder unstamped() knows, or add this one to it. The report was NOT initialized."
    done
    # `-L` tests only the *final* component, so the directory check has to be its own: a `.idsd`
    # that is itself a symlink slips past the report test below, and then every write here lands
    # wherever it points — outside the repository, where check-ignore never applied and nothing
    # downstream can see that the report is not where it says it is.
    if [ -L "$root/.idsd" ]; then
      refuse "error: $root/.idsd is a symlink -> $(readlink "$root/.idsd") — the report was NOT initialized." \
        "  .idsd/ is always a real directory inside the repo. Remove the link, then re-run."
    fi
    # The report is never legitimately a symlink, and `--force` does not override this. A dangling
    # link reads as absent to `-e`, so this `-L` test is the only one that catches it: unguarded,
    # `cp` follows the link and writes wherever it points, and the `mv` below then replaces the link
    # with a real file — leaving nothing here to record that a write landed outside the repository.
    if [ -L "$report" ]; then
      refuse "error: $report is a symlink -> $(readlink "$report") — the report was NOT initialized." \
        "  the report is always a regular file. Remove the link, then re-run."
    fi
    # Overwriting is a real path (a report recording a different intent), but it takes every
    # unresolved `- [ ]` with it, so it has to be asked for.
    if { [ -e "$report" ] || [ -L "$report" ]; } && [ "$is_forced" -eq 0 ]; then
      existing_intent=$(grep -m1 '^intent:' "$report" 2>/dev/null)
      refuse "error: $report already exists — the report was NOT initialized." \
        "  it records ${existing_intent:-(no intent: line)}" \
        "  a new report starts empty; this one holds $(open_items_phrase)." \
        "  Keep it and re-qualify (report.sh carry lists those items), or re-run with --force to start over it."
    fi
    # `--force` supersedes rather than overwrites. What it drops are the open `- [ ]`, which is where
    # a finding nobody has resolved yet lives — a count on stderr, printed after the file is already
    # gone, is not something anyone can act on. So keep the file, and print the items themselves
    # while they still exist. A second `--force` replaces this copy, so route them, don't bank them.
    replacing=""
    carried=""
    if [ -e "$report" ] || [ -L "$report" ]; then
      replacing=$(open_items_phrase)
      carried=$("$todo_gate" "$report")
      mv "$report" "$superseded" ||
        refuse "error: could not move $report aside to $superseded — the report was NOT initialized"
    fi
    mkdir -p "$(dirname "$report")" ||
      refuse "error: could not create $(dirname "$report") — the report was NOT initialized"
    # The copy must land before the intent is stamped: a failed one leaves the previous report's body
    # standing, and the rewrite below then writes the new intent onto an old tree and old open items.
    cp "$template" "$report" ||
      refuse "error: could not copy $template to $report — the report was NOT initialized${replacing:+; the previous report is at $superseded}"
    [ -z "$replacing" ] || {
      {
        echo "warning: --force superseded the previous report, which held $replacing"
        [ -z "$carried" ] || printf '%s\n' "$carried" | sed 's/^/    /'
        echo "  it is kept at $superseded — route anything above before a further --force replaces it."
      } >&2
    }
    no_temp="$report still carries the template's placeholder intent" \
      no_write="could not write the intent line into $report" \
      intent_value="$intent_value" rewrite_report \
      '!replaced && /^intent:/ { print "intent: " ENVIRON["intent_value"]; replaced = 1; next } { print }' || exit 2
    echo "initialized $report (repo mode: $(repo_mode), intent: $intent_value)"
    ;;

  repo-mode)
    repo_mode
    ;;

  # Recording a stage's findings is prose discipline, so this makes the omission mechanical: a report
  # that lags the pass asserts stages ran with nothing to show. A marker holds the report's checksum
  # as it stood when that stage returned, and the stamp demands the report have moved since.
  # Checksums alone do not make that per-stage. The round's stages return together, so marks made
  # back to back all hold one checksum and a single edit clears them all. Serialising the marks is
  # what fixes it: only one stage may stand unrecorded, so the edit that clears it is that stage's.
  stage-returned)
    require_report
    # `|| exit 2`: valid_stage's refusal ends the `$( )` subshell, not this script, so the call site
    # has to check the status itself. Unchecked, an invalid stage leaves `stage` empty, aims the
    # marker write at the directory rather than a file, and still prints "recorded return of".
    stage=$(valid_stage "${2:-}" stage-returned) || exit 2
    outstanding=$(outstanding_stage)
    [ -z "$outstanding" ] ||
      refuse "error: $outstanding is marked returned and the report has not moved since — record its items, or run report.sh no-items $outstanding, before marking a stage returned."
    write_stage_marker "$stage" "$(report_checksum)" || exit 2
    echo "recorded return of $stage — record its items, or report.sh no-items $stage, before taking the next stage's return"
    ;;

  # The escape hatch for a stage that genuinely surfaced nothing. Without it the only way to clear a
  # marker is to edit the report, and the report's own contract bans the per-stage line that would
  # say "nothing to record". It clears a marker, so it demands one: "returned" and "surfaced nothing"
  # are two points in time, and a stage with no marker never ran. Drop that demand and a pass that
  # ran nothing can declare every stage empty and stamp.
  no-items)
    require_report
    stage=$(valid_stage "${2:-}" no-items) || exit 2
    stage_was_marked_returned "$stage" ||
      refuse "error: $stage was never marked returned — run report.sh stage-returned $stage when it returns, then report.sh no-items $stage once you have read its findings."
    write_stage_marker "$stage" "$NO_ITEMS" || exit 2
    echo "recorded $stage as having surfaced nothing for the report"
    ;;

  stamp)
    require_report
    entries="${2:-}"
    [ -n "$entries" ] || {
      cat >&2 <<'USAGE'
usage: report.sh stamp "<all five stages, comma-separated>"
  code-review                                   always runs, always bare
  refactor | refactor:partial(fast|cap)         partial = the loop ended non-compliant
  security-review|tighten|retro [:skipped(fast|not-applicable)]
    fast            = trimmed for turnaround; blocks the merge gate until a full pass
    not-applicable  = its condition was unmet
USAGE
      exit 2
    }
    entries=$(printf '%s' "$entries" | tr -d '[:space:]')
    # Validate: exactly the five pipeline stages, each once, against the closed reason vocabulary above.
    # Free text must not be accepted — only `fast` reaches fast_trims, so any other word for a turnaround
    # trim stamps a trimmed pass as full and walks it through the merge gate.
    validation_errors=$(printf '%s\n' "$entries" | tr ',' '\n' | awk '
      /^code-review$/ { seen["code-review"]++; next }
      /^refactor(:partial\((fast|cap)\))?$/ { seen["refactor"]++; next }
      /^(security-review|tighten|retro)(:skipped\((fast|not-applicable)\))?$/ { name = $0; sub(/:.*/, "", name); seen[name]++; next }
      { print "malformed entry: " $0 }
      END {
        split("code-review security-review tighten refactor retro", required, " ")
        for (i in required) {
          if (!(required[i] in seen)) print "missing stage: " required[i]
          else if (seen[required[i]] > 1) print "duplicate stage: " required[i]
        }
      }')
    [ -z "$validation_errors" ] || refuse "error: invalid stage record" "$validation_errors"
    grep -q '^reviewed-tree:' "$report" || refuse "error: no 'reviewed-tree:' line in frontmatter"
    # `invalidate` is what separates one pass from the next: it parks reviewed-tree at `pending` and
    # drops the stage markers. Skip it and nothing has to happen for this stamp to succeed, because
    # the previous pass's markers still validate against a report nothing has touched. An edited tree
    # then restamps clean with no stage having run. This check comes before the per-stage one below,
    # whose markers mean nothing until it is known which pass made them.
    stamped=$(reviewed_tree)
    [ "$stamped" = pending ] ||
      refuse "error: this pass never invalidated — reviewed-tree still reads '${stamped:-<empty>}', not 'pending'. Run report.sh invalidate first, or the stamp and the stage markers standing here are the previous pass's, not this one's."
    # Every stage this record says ran must also have been marked returned, with its items recorded
    # since. The entry string cannot carry that on its own: `refactor,retro` is legally shaped
    # whether or not either ran, and a stage that never marked itself looks exactly like one that
    # had nothing to say.
    block_reasons=$(printf '%s\n' "$entries" | tr ',' '\n' | while read -r entry; do
      # Leading `(` on the pattern: bash 3.2 — still macOS's /bin/bash — ends the enclosing `$( … )`
      # at a bare case-arm `)`, so without it the whole script fails to parse there.
      case "$entry" in
        (*:skipped\(*) continue ;;
      esac
      stage_name=${entry%%:*}
      reason=$(stage_block_reason "$stage_name")
      [ -z "$reason" ] || printf '  %s: %s\n' "$stage_name" "$reason"
    done)
    [ -z "$block_reasons" ] || refuse "error: these stages are recorded as having run, but:" "$block_reasons"
    tree=$(current_tree) || exit 2
    # Rewrite frontmatter only (never a body line quoting a field): refresh reviewed-tree, write
    # reviewed-stages next to it, retire any reviewed-mode line from before stages subsumed it.
    no_temp="nothing was stamped" \
      no_write="could not write the stamp into $report — reviewed-tree is unchanged" \
      entries="$entries" tree="$tree" rewrite_report \
      '
      NR > 1 && /^---[[:space:]]*$/ { in_frontmatter = 0 }
      in_frontmatter && /^reviewed-mode:/ { next }
      in_frontmatter && /^reviewed-stages:/ { next }
      in_frontmatter && /^reviewed-tree:/ { print "reviewed-tree: " ENVIRON["tree"]; print "reviewed-stages: " ENVIRON["entries"]; next }
      { print }
      NR == 1 { in_frontmatter = 1 }
    ' || exit 2
    echo "stamped reviewed-tree: $tree (stages: $entries)"
    ;;

  gate)
    require_report
    blocked=0
    current=$(current_tree) || exit 2
    reviewed=$(reviewed_tree)
    if [ "$current" != "$reviewed" ]; then
      echo "BLOCK (freshness): tree changed since last qualify (current $current != reviewed ${reviewed:-<unstamped>}). Re-qualify, or the human may explicitly override this one." >&2
      blocked=1
    fi
    stages=$(reviewed_stages)
    trims=$(fast_trims)
    if unstamped "$stages"; then
      echo "BLOCK (stages): no reviewed-stages record — run a full qualify (it stamps the stage set), or the human may explicitly override this one." >&2
      blocked=1
    elif [ -n "$trims" ]; then
      echo "BLOCK (stages): trimmed for turnaround ($trims) — run a full qualify before merge, or the human may explicitly override this one." >&2
      blocked=1
    fi
    todos=$("$todo_gate" "$report")
    todo_status=$?
    # 0 = nothing open, 1 = items printed. Anything else (missing script, lost exec bit, bad args)
    # yields empty output, which the -n test below would read as "no open TODOs" and pass the merge
    # gate on a scan that never ran.
    if [ "$todo_status" -gt 1 ]; then
      echo "BLOCK (open TODOs): the scan did not run — todo-gate.sh exited $todo_status. No override; fix the invocation." >&2
      blocked=1
    elif [ -n "$todos" ]; then
      echo "BLOCK (open TODOs): clear each before merge — no override." >&2
      echo "$todos" >&2
      blocked=1
    fi
    [ "$blocked" -eq 0 ] && echo "gate clean: tree fresh, full qualify, no open TODOs"
    exit "$blocked"
    ;;

  carry)
    require_report
    "$todo_gate" "$report"
    carry_status=$?
    # Same reasoning as gate: a failed scan prints nothing, and nothing reads as "no prior items" —
    # the one outcome that silently drops every carried `- [ ]`.
    [ "$carry_status" -le 1 ] ||
      refuse "error: the carry scan did not run — todo-gate.sh exited $carry_status; prior open items are unknown."
    ;;

  invalidate)
    # Clears the stamp at the start of a pass, so a stamp can never outlive the tree it describes —
    # the merge gate must not read a review state the pass under way has already invalidated.
    require_report
    no_temp="the stamp was NOT cleared; it still describes an older tree" \
      no_write="could not clear the stamp in $report — it still describes an older tree" \
      rewrite_report \
      '
      NR > 1 && /^---[[:space:]]*$/ { in_frontmatter = 0 }
      in_frontmatter && /^reviewed-tree:/ { print "reviewed-tree: pending"; next }
      in_frontmatter && /^reviewed-stages:/ { print "reviewed-stages: pending"; next }
      { print }
      NR == 1 { in_frontmatter = 1 }
    ' || exit 2
    # Last pass's stage returns are not this pass's; left behind they would satisfy its stamp for free.
    rm -rf "$stage_returns"
    echo "invalidated reviewed-tree — restamp when the pass completes"
    ;;

  check-ignore)
    # Runs before anything else — init included, so it never requires the report to exist — and
    # before any fingerprinting `git add -A` (stamp/gate), so nothing scratch is ever staged.
    if [ "$(repo_mode)" = committed ]; then
      # committed idsd repo: these ignores are shared, committed with the rest of the idsd setup.
      # A path already tracked also answers "not ignored" here, and that is the case most worth the
      # warning — the file is sitting in the commits the entry was meant to keep it out of.
      unignored=$(ignore_surface | while read -r entry; do
        git -C "$root" check-ignore -q "$root/$entry" || printf " '%s'" "$entry"
      done)
      if [ -z "$unignored" ]; then
        echo "ok: report and its superseded copy are gitignored (committed idsd repo)"
        exit 0
      fi
      echo "WARN: NOT gitignored:$unignored — add each to .gitignore (shared idsd setup)" >&2
      exit 1
    fi
    # throwaway: exclude the *whole* .idsd/ locally so intents + report leave zero traces —
    # .gitignore is never touched and `git add -A` skips the dir entirely.
    add_local_exclusion ||
      refuse "error: could not add '.idsd/' to $(git_path info/exclude) — the scratch dir is NOT excluded"
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
    gitignore="$root/.gitignore"
    # Refuse a symlinked .gitignore before anything is written, not after. Detection afterwards is
    # already too late: the append has landed in whatever the link points at — a tracked file
    # elsewhere in the repo, or a path outside it — and no refusal below undoes that write.
    if [ -L "$gitignore" ]; then
      refuse "error: $gitignore is a symlink -> $(readlink "$gitignore") — not promoted, and nothing was written." \
        "  git ignores nothing it cannot read, so entries added through the link would take effect nowhere" \
        "  while this reported success and staged the report. Replace it with a regular file, then re-run."
    fi
    drop_local_exclusion || exit 2
    # Each entry is guarded, and staging waits until every one of them is written. A promotion that
    # reports success with one still missing lets that file reach a commit, which the mode forbids.
    unwritten=$(ignore_surface | while read -r entry; do
      append_line "$gitignore" "$entry" || printf " '%s'" "$entry"
    done)
    [ -z "$unwritten" ] || {
      # The exclusion is already dropped by the time a write can fail, so this refusal has to put it
      # back for the same reason the one below does — otherwise an unwritable .gitignore hands the
      # human a repo where the next `git add -A` stages the report.
      add_local_exclusion ||
        echo "error: could not restore the local .idsd/ exclusion — .idsd/ is now visible to 'git add -A'" >&2
      refuse "error: could not add$unwritten to $gitignore — not promoted." \
        "  Nothing was staged, so the report is not on its way into a commit."
    }
    # Writing an entry is not the same as it taking effect, and only git can say which. So ask git,
    # and ask it *which file* answered, with -v, because the plain question has three other sources.
    # `core.excludesFile` and `.git/info/exclude` are this machine's alone: satisfied by either, the
    # promotion reports a shared ignore that no teammate cloning the repo will have, and the report
    # reaches a commit from their checkout. Only `.gitignore` at the root is the durable, shared
    # answer this subcommand claims to have written.
    unignored=$(ignore_surface | while read -r entry; do
      source_file=$(git -C "$root" check-ignore -v "$root/$entry" 2>/dev/null | head -1)
      source_file=${source_file%%:*}
      [ "$source_file" = ".gitignore" ] || printf " '%s'" "$entry"
    done)
    [ -z "$unignored" ] || {
      # Put the exclusion back before refusing. Without it the whole of .idsd/ is visible to the
      # human's next `git add -A`.
      add_local_exclusion ||
        echo "error: could not restore the local .idsd/ exclusion — .idsd/ is now visible to 'git add -A'" >&2
      refuse "error: the entries are in $gitignore, but git still does not ignore:$unignored — not promoted." \
        "  git ignores nothing it cannot read; a symlinked or unreadable .gitignore does exactly this." \
        "  Nothing was staged, so the report is not on its way into a commit."
    }
    git -C "$root" add .idsd .gitignore ||
      refuse "error: could not stage .idsd/ and .gitignore — not promoted"
    echo "promoted: .idsd/ staged, report + superseded copy ignored via .gitignore — commit when ready (not committed here)"
    ;;

  discard)
    require_report
    # throwaway-only cleanup, run by `done` after the code has landed: a throwaway run promises zero
    # traces, but `done` would otherwise leave the report + archived intent behind. Committed repos keep
    # .idsd/ as their durable record, so refuse there.
    if [ "$(repo_mode)" = committed ]; then
      refuse "committed idsd repo — .idsd/ is the durable record; nothing to discard"
    fi
    # Remove only this ship's intent file (both pre- and post-archive locations); a standalone
    # `review:` intent has no slug and no file to remove.
    slug=$(intent_slug)
    [ -n "$slug" ] && rm -f "$root/.idsd/intents/$slug.md" "$root/.idsd/archive/$slug.md"
    rm -f "$report" "$superseded"
    # The stage markers are in the git dir, so removing .idsd/ below never reaches them.
    rm -rf "$stage_returns"
    # If nothing remains, this ship was the last occupant: remove the whole scratch dir (regenerable
    # roadmap and OS junk with it) and stop excluding it, leaving the repo pristine. What counts as
    # remaining is named, never "the .idsd/ root is non-empty". A stray dotfile must not keep the dir
    # alive. The other way round, a throwaway charter, constitution, language or playbook is the
    # human's authored work rather than this ship's scratch, and no other copy of it exists, so
    # `rm -rf` must not take it with the report. `decisions.md` is deliberately NOT on that list:
    # `idsd-qualify/SKILL.md` → **The decision log** makes it throwaway scratch by design, and says
    # anything that must outlive the ship is routed out of it rather than into it.
    rmdir "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null || true
    kept=""
    for durable in charter.md constitution.md language.md playbook.md; do
      [ -e "$root/.idsd/$durable" ] && kept="$kept $durable"
    done
    # The guard stays conservative: anything at all under intents/ or archive/ keeps .idsd/ alive, so
    # a file nobody anticipated is never deleted. The label counts what is actually there, because a
    # stray `.DS_Store` reporting as "other intents" tells the human something untrue.
    if [ -n "$(ls -A "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null)" ]; then
      intents_left=$(find "$root/.idsd/intents" "$root/.idsd/archive" -maxdepth 1 -name '*.md' -type f 2>/dev/null | wc -l | tr -d ' ')
      if [ "$intents_left" -gt 0 ]; then
        kept="$kept $intents_left other intent(s)"
      else
        kept="$kept unrecognised content under intents/ or archive/"
      fi
    fi
    if [ -z "$kept" ]; then
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
      echo "discarded: removed this ship's report + intent; kept .idsd/ (still holds:$kept)"
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
    if unstamped "$reviewed"; then # never stamped, or invalidated mid-pass → quality stages haven't completed
      echo "resume"
      exit 0
    fi
    current=$(current_tree) || exit 2
    if [ "$current" != "$reviewed" ]; then
      echo "re-qualify" # reviewed once, tree moved since
      exit 0
    fi
    # The same status check gate and carry make: a scan that could not run prints nothing, and
    # nothing here reads as "no open items" — routing a change set with open `- [ ]` past `decide`.
    todos=$("$todo_gate" "$report")
    todo_status=$?
    [ "$todo_status" -le 1 ] ||
      refuse "error: the open-item scan did not run — todo-gate.sh exited $todo_status; the state is unknown."
    if [ -n "$todos" ]; then
      echo "decide" # quality done, tree fresh, open `- [ ]` remain
      exit 0
    fi
    if unstamped "$(reviewed_stages)" || [ -n "$(fast_trims)" ]; then
      echo "finalize" # stages trimmed (or unrecorded) and fresh, nothing open — a full qualify remains
      exit 0
    fi
    echo "ready" # full-reviewed, tree fresh, nothing open → merge-ready
    ;;

  *)
    refuse "usage: report.sh {init <intent>|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|stamp \"<stages>\"|gate|carry|check-ignore|promote|discard|state}"
    ;;
esac
