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
#   gate             done-blocker: stale tree OR turnaround-trimmed stages (both human-overridable)
#                    OR any open `- [ ]` (never overridable) → non-zero + reasons
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
set -uo pipefail
# Byte-oriented, like every sibling script: macOS awk aborts on the first non-UTF-8 byte under a
# UTF-8 locale, and the report can carry one.
export LC_ALL=C

# exit 2 = "this did not run", never a result. Every path that stops halfway leaves by here.
refuse() {
  printf '%s\n' "$@" >&2
  exit 2
}

root=$(git rev-parse --show-toplevel 2>/dev/null) || refuse "error: not a git repo"

# Absolute path to <name> in this worktree's git dir. --git-path answers relative for an ordinary
# repo, absolute in a linked worktree; an empty answer would build a bare "$root/" in the tree.
git_path() {
  local path
  path=$(git -C "$root" rev-parse --git-path "$1") && [ -n "$path" ] ||
    refuse "error: could not resolve '$1' inside the git dir (git rev-parse --git-path)"
  case "$path" in /*) echo "$path" ;; *) echo "$root/$path" ;; esac
}

report="$root/.idsd/ship-report.md"
superseded="$root/.idsd/ship-report.superseded.md"
# Per-pass bookkeeping, in the git dir: no commit and no `git add -A` reaches it, and it is
# per-worktree so ships cannot collide. Under .idsd/, committed mode would commit them.
stage_returns=$(git_path idsd-stage-returns) || exit 2
skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
template="$skill_dir/templates/ship-report-template.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"
NO_ITEMS="no-items"

report_checksum() {
  cksum <"$report"
}

valid_stage() {
  case "$1" in
    code-review | security-review | tighten | refactor | retro) printf '%s' "$1" ;;
    *) refuse "usage: report.sh $2 <code-review|security-review|tighten|refactor|retro>" ;;
  esac
}

# A marker never written while the caller prints "recorded" is a stage the stamp waves through.
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

# The stage marked returned against the report as it now stands — items still unwritten. Only one may
# stand at a time (see `stage-returned`); a `no-items` marker holds a word, never a checksum.
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

stage_was_marked_returned() {
  [ -f "$stage_returns/$1" ]
}

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

repo_mode() {
  if [ -n "$(git -C "$root" ls-files .idsd 2>/dev/null)" ]; then
    echo committed
  else
    echo throwaway
  fi
}

# What must never be committed or fingerprinted, one path per line relative to $root. `promote`
# writes a .gitignore entry per line and `check-ignore` verifies one per line, so the two cannot
# disagree. The durable record is deliberately absent — committed mode keeps it tracked.
ignore_surface() {
  printf '%s\n' "${report#"$root"/}" "${superseded#"$root"/}"
}

# The trailing-newline check is the point: appending to a file whose last line has none fuses the
# two, and then neither the human's own rule nor the entry just added matches anything.
append_line() {
  local file="$1" entry="$2"
  grep -qxF "$entry" "$file" 2>/dev/null && return 0
  if [ -s "$file" ] && [ -n "$(tail -c 1 "$file")" ]; then
    printf '\n' >>"$file" || return 1
  fi
  printf '%s\n' "$entry" >>"$file"
}

# Guarded to the slug charset: nothing for a standalone `review:`, or for any char outside the set
# (notably `/`), so a slug can never `../`-escape a path it indexes.
intent_slug() {
  local intent slug
  intent=$(grep -m1 '^intent:' "$report" 2>/dev/null | sed -e 's/^intent:[[:space:]]*//' -e 's/^"//' -e 's/"$//')
  slug=${intent%%[[:space:]]*}
  case "$slug" in
    review:* | "" | *[!0-9A-Za-z._-]*) ;;
    *) echo "$slug" ;;
  esac
}

add_local_exclusion() {
  local exclude
  exclude=$(git_path info/exclude) || return 1
  mkdir -p "$(dirname "$exclude")" || return 1
  append_line "$exclude" '.idsd/'
}

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
  # grep exits 1 when it prints nothing — the file held only that entry. 2+ means it could not read
  # the file, and moving the empty temp over it would wipe every other exclusion.
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

# Atomic: on any failure the report is left exactly as it was. The two failure messages ride the
# environment — `no_temp=<what a failed mktemp leaves standing> no_write=<what a failed rewrite
# leaves standing>` — as do awk's own values, never `-v`, which processes C escapes.
rewrite_report() {
  local program="$1" tmp
  # Refuse rather than let `set -u` kill the shell: that exits 1, and 1 is a result to every caller.
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

# The fingerprint the freshness gate compares, against a throwaway index so the human's staging area
# is untouched. Fails loudly rather than printing an empty tree, which would match the next
# equally-failed reading and report "tree fresh".
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

# Every frontmatter value meaning "no stamp stands here". One set, used whole by every reader — a
# reader knowing only some accepts what the others reject. `init` holds the template to this set.
unstamped() {
  case "$1" in
    "" | pending | "<hash>" | "<stages>") return 0 ;;
  esac
  return 1
}

# Empty when never stamped with stages — callers treat that as not-full.
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
    # Frontmatter is single-line: collapse CR/LF so a value seeded from a fetched ticket can't inject
    # extra frontmatter lines (a forged reviewed-tree).
    intent_value=${intent_value//[$'\n\r']/ }
    [ ! -L "$template" ] ||
      refuse "error: template $template is a symlink — refusing to read the template through one; the report was NOT initialized"
    [ -f "$template" ] || refuse "error: template not found ($template)"
    # gate and state never see the template, so a placeholder drifting out of unstamped()'s set can
    # only be caught here: it would stamp every new report as already reviewed.
    grep -q '^intent:' "$template" ||
      refuse "error: template $template has no 'intent:' line to stamp — the report was NOT initialized"
    for field in reviewed-tree reviewed-stages; do
      grep -q "^$field:" "$template" ||
        refuse "error: template $template has no '$field:' line — gate and state read it; the report was NOT initialized"
      placeholder=$(grep -m1 "^$field:" "$template" | sed "s/^$field:[[:space:]]*//")
      unstamped "$placeholder" ||
        refuse "error: template $template writes '$field: $placeholder', which gate and state read as a completed review — every new report would gate clean. Restore a placeholder unstamped() knows, or add this one to it. The report was NOT initialized."
    done
    # `-L` tests only the *final* component, so a `.idsd` that is itself a symlink needs its own
    # check: it slips past the report test below, and every write here then lands wherever it points.
    if [ -L "$root/.idsd" ]; then
      refuse "error: $root/.idsd is a symlink -> $(readlink "$root/.idsd") — the report was NOT initialized." \
        "  .idsd/ is always a real directory inside the repo. Remove the link, then re-run."
    fi
    # The report is never legitimately a symlink, and `--force` does not override this. A dangling
    # link reads as absent to `-e`, so only `-L` catches it before `cp` follows it out of the repo.
    if [ -L "$report" ]; then
      refuse "error: $report is a symlink -> $(readlink "$report") — the report was NOT initialized." \
        "  the report is always a regular file. Remove the link, then re-run."
    fi
    if { [ -e "$report" ] || [ -L "$report" ]; } && [ "$is_forced" -eq 0 ]; then
      existing_intent=$(grep -m1 '^intent:' "$report" 2>/dev/null)
      refuse "error: $report already exists — the report was NOT initialized." \
        "  it records ${existing_intent:-(no intent: line)}" \
        "  a new report starts empty; this one holds $(open_items_phrase)." \
        "  Keep it and re-qualify (report.sh carry lists those items), or re-run with --force to start over it."
    fi
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
    # The copy lands before the intent is stamped, or the rewrite writes the new intent onto the
    # previous report's tree and open items.
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

  # A marker holds the report's checksum as that stage returned, and the stamp demands the report
  # have moved since. Serialising the marks is what makes that per-stage: the round's stages return
  # together, so back-to-back marks share one checksum and one edit would clear them all.
  stage-returned)
    require_report
    # `|| exit 2`: valid_stage's refusal ends the `$( )` subshell, not this script. Unchecked, an
    # invalid stage leaves `stage` empty and still prints "recorded return of".
    stage=$(valid_stage "${2:-}" stage-returned) || exit 2
    outstanding=$(outstanding_stage)
    [ -z "$outstanding" ] ||
      refuse "error: $outstanding is marked returned and the report has not moved since — record its items, or run report.sh no-items $outstanding, before marking a stage returned."
    write_stage_marker "$stage" "$(report_checksum)" || exit 2
    echo "recorded return of $stage — record its items, or report.sh no-items $stage, before taking the next stage's return"
    ;;

  # The escape hatch for a stage that genuinely surfaced nothing. It demands a marker: without that,
  # a pass that ran nothing can declare every stage empty and stamp.
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
    # Only `fast` reaches fast_trims, so any other word for a turnaround trim stamps a trimmed pass
    # as full.
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
    # `invalidate` is what separates one pass from the next, so it comes before the per-stage check
    # below, whose markers mean nothing until it is known which pass made them.
    stamped=$(reviewed_tree)
    [ "$stamped" = pending ] ||
      refuse "error: this pass never invalidated — reviewed-tree still reads '${stamped:-<empty>}', not 'pending'. Run report.sh invalidate first, or the stamp and the stage markers standing here are the previous pass's, not this one's."
    # `refactor,retro` is legally shaped whether or not either ran, hence the per-stage check.
    block_reasons=$(printf '%s\n' "$entries" | tr ',' '\n' | while read -r entry; do
      # Leading `(` on the pattern: bash 3.2 — still macOS's /bin/bash — ends the enclosing `$( … )`
      # at a bare case-arm `)`.
      case "$entry" in
        (*:skipped\(*) continue ;;
      esac
      stage_name=${entry%%:*}
      reason=$(stage_block_reason "$stage_name")
      [ -z "$reason" ] || printf '  %s: %s\n' "$stage_name" "$reason"
    done)
    [ -z "$block_reasons" ] || refuse "error: these stages are recorded as having run, but:" "$block_reasons"
    tree=$(current_tree) || exit 2
    # Frontmatter only, never a body line quoting a field: refresh reviewed-tree, write
    # reviewed-stages beside it, retire any pre-stages reviewed-mode line.
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
    # 0 = nothing open, 1 = items printed. Anything else yields empty output, which the -n test below
    # would read as "no open TODOs" and pass the merge gate on a scan that never ran.
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
    # As in gate: a failed scan prints nothing, and nothing reads as "no prior items".
    [ "$carry_status" -le 1 ] ||
      refuse "error: the carry scan did not run — todo-gate.sh exited $carry_status; prior open items are unknown."
    ;;

  invalidate)
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
    # Last pass's stage returns would otherwise satisfy this pass's stamp for free.
    rm -rf "$stage_returns"
    echo "invalidated reviewed-tree — restamp when the pass completes"
    ;;

  check-ignore)
    # Runs before anything else — init included, so it never requires the report to exist — and
    # before any fingerprinting `git add -A`, so nothing scratch is ever staged.
    if [ "$(repo_mode)" = committed ]; then
      # A path already tracked also answers "not ignored" here — the case most worth the warning.
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
    add_local_exclusion ||
      refuse "error: could not add '.idsd/' to $(git_path info/exclude) — the scratch dir is NOT excluded"
    echo "ok: throwaway run — .idsd/ excluded locally via .git/info/exclude (.gitignore untouched)"
    ;;

  promote)
    require_report
    if [ "$(repo_mode)" = committed ]; then
      echo "already committed — .idsd/ is tracked; nothing to promote"
      exit 0
    fi
    gitignore="$root/.gitignore"
    # Refuse a symlinked .gitignore before anything is written: afterwards the append has already
    # landed in whatever the link points at, and no refusal below undoes that write.
    if [ -L "$gitignore" ]; then
      refuse "error: $gitignore is a symlink -> $(readlink "$gitignore") — not promoted, and nothing was written." \
        "  git ignores nothing it cannot read, so entries added through the link would take effect nowhere" \
        "  while this reported success and staged the report. Replace it with a regular file, then re-run."
    fi
    drop_local_exclusion || exit 2
    # Staging waits until every entry is written, or a promotion that reports success with one
    # missing lets that file reach a commit.
    unwritten=$(ignore_surface | while read -r entry; do
      append_line "$gitignore" "$entry" || printf " '%s'" "$entry"
    done)
    [ -z "$unwritten" ] || {
      # The exclusion is already dropped by the time a write can fail, so put it back.
      add_local_exclusion ||
        echo "error: could not restore the local .idsd/ exclusion — .idsd/ is now visible to 'git add -A'" >&2
      refuse "error: could not add$unwritten to $gitignore — not promoted." \
        "  Nothing was staged, so the report is not on its way into a commit."
    }
    # Writing an entry is not the same as it taking effect — ask git, and with -v, because
    # `core.excludesFile` and `.git/info/exclude` answer the plain question too and are this machine's
    # alone. Only root `.gitignore` is the shared answer this subcommand claims to have written.
    unignored=$(ignore_surface | while read -r entry; do
      source_file=$(git -C "$root" check-ignore -v "$root/$entry" 2>/dev/null | head -1)
      source_file=${source_file%%:*}
      [ "$source_file" = ".gitignore" ] || printf " '%s'" "$entry"
    done)
    [ -z "$unignored" ] || {
      # Put the exclusion back before refusing, as above.
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
    if [ "$(repo_mode)" = committed ]; then
      refuse "committed idsd repo — .idsd/ is the durable record; nothing to discard"
    fi
    # Only this ship's intent file (pre- and post-archive); a standalone `review:` has no slug.
    slug=$(intent_slug)
    [ -n "$slug" ] && rm -f "$root/.idsd/intents/$slug.md" "$root/.idsd/archive/$slug.md"
    rm -f "$report" "$superseded"
    # The stage markers are in the git dir, so removing .idsd/ below never reaches them.
    rm -rf "$stage_returns"
    # What counts as remaining is named, never "the .idsd/ root is non-empty", so a stray dotfile
    # cannot keep the dir alive. `decisions.md` is deliberately NOT on the list —
    # `idsd-qualify/SKILL.md` → **The decision log** makes it throwaway scratch by design.
    rmdir "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null || true
    kept=""
    for durable in charter.md constitution.md language.md playbook.md; do
      [ -e "$root/.idsd/$durable" ] && kept="$kept $durable"
    done
    # Anything at all under intents/ or archive/ keeps .idsd/ alive, but the label counts what is
    # actually there — "other intents" for a stray `.DS_Store` tells the human something untrue.
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
      # .git/info/exclude is shared across worktrees, and a parallel throwaway ship's .idsd/ must stay
      # excluded. Drop it only from the last worktree.
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
    if [ ! -f "$report" ]; then
      echo "no-report"
      exit 0
    fi
    # A built intent's file has moved to archive/ (a standalone `review:` has no slug, so this is
    # skipped).
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
    # As in gate: a scan that could not run prints nothing, and nothing reads as "no open items".
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
