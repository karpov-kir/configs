#!/usr/bin/env bash
# Qualify report tool — the deterministic gates the skills must not execute by hand. The mechanism
# lives here; the contract it serves (repo modes, what goes in the report, never commit it) is
# `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**. idsd-ship calls it too (gate/state/promote/discard).
# One report per intent, at .idsd/qualify-reports/<intent>-qualify-report.md, so two ships never share a
# file. Portable: bash + git + awk/sed, no project runtime — plus one sibling script,
# `~/.kk-flavor/scripts/tree-fingerprint.sh`, which owns the tree-fingerprint recipe this reads the
# freshness gate from. Both installs symlink into the same repo, so they ship together or not at all.
# A change here needs a case in `~/.claude/skills/idsd-qualify/scripts/report-test.sh`.
# Subcommands:
#   init "<intent>" [--force]  scaffold .idsd/ + the report from the template, stamping its intent
#                    line. Refuses over an existing report unless --force, which first prints the open
#                    `- [ ]` it is about to discard. Refuses a symlink either way
#   repo-mode        print committed|throwaway — is .idsd/ tracked in git?
#   invalidate       clear reviewed-tree/reviewed-stages and drop the stage markers at pass start, so no
#                    stamp outlives its tree; stamp refuses until this pass has run it
#   stage-returned <stage>  mark a stage returned, recording the report as it then stood; stamp refuses until
#                    the report has changed since, so a stage's items cannot be left unrecorded. One stage at
#                    a time — refused while another stage's mark still has nothing recorded against it
#   no-items <stage> mark a stage already marked returned as having surfaced nothing, the one way to clear
#                    its marker without editing the report
#   stamp "<stages>" compute the tree fingerprint (throwaway index) and record reviewed-tree +
#                    reviewed-stages, one entry per pipeline stage. Any `(fast)` marks the pass
#                    not-full. Run `stamp` bare for the grammar; that usage text is the authority on it
#   gate             done-blocker: stale tree OR turnaround-trimmed stages (both human-overridable)
#                    OR any open `- [ ]` (never overridable) → non-zero + reasons
#   carry            print prior open `- [ ]` (with their section) so re-qualify loses none
#   check-ignore     keep qualify-reports/ out of the fingerprint, by the mechanism that fits the repo mode
#   promote          throwaway → committed: stop excluding .idsd/, ignore qualify-reports/ via .gitignore, stage
#   discard          throwaway only: remove this ship's local scratch (report, intent file, stage
#                    markers), and the whole .idsd/ + its local exclusion when nothing else remains.
#                    Another intent, or an authored charter/constitution/language/playbook, is
#                    "something" — those are the human's, not this ship's scratch
#   state            print the `continue` routing token:
#                    no-report|resume|re-qualify|decide|finalize|ready|done
#   list             one line per open ship, `<intent><TAB><state>`, for routing with several in flight
#   close [--force]  retire one landed ship's report and stage markers. Refuses while an open `- [ ]`
#                    stands, since nothing else keeps a copy
# Every subcommand that reads a report takes the intent as its LAST argument, optional while only one
# report is open. Several open and none named is refused, never guessed: resolving to the wrong report
# stamps one intent's review onto another's, and the stamp is what the merge gate trusts.
set -uo pipefail
# Byte-oriented, like every sibling script: macOS awk aborts on the first non-UTF-8 byte under a
# UTF-8 locale, and the report can carry one.
export LC_ALL=C

# exit 2 = "this did not run", never a result. Every path that stops halfway leaves by here.
# Call anything that can refuse from the caller's own shell, never inside `$( )`: there the exit kills
# the subshell alone, and the caller runs on with an empty substitution and the error already printed.
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

# Set by set_report_paths once the intent is known. Empty until then, and `set -u` won't catch that (an
# initialized variable is set), so a subcommand that reads a report resolves it first, through
# require_report or resolve_report.
reports_dir="$root/.idsd/qualify-reports"
report_suffix="-qualify-report.md"
report=""
stage_returns_dir=""

# The report's filename stem for an intent frontmatter value. Empty means unusable, and the caller
# refuses: the value reaches a path here, so the slug charset is what stops a `../` escaping
# qualify-reports/. A standalone review has no slug and shares the one `review` stem.
report_name_for() {
  local value="$1" slug
  # Leading whitespace goes BEFORE the truncation. Without that, a value starting with a space
  # truncates to nothing and takes the `review` arm, while intent_slug's sed strips the same
  # whitespace and recovers the real slug — so the filename and the frontmatter name different ships.
  value=${value#"${value%%[![:space:]]*}"}
  slug=${value%%[[:space:]]*}
  case "$slug" in
    review:* | "") echo review ;;
    # A leading dot is refused outright, not merely made path-safe: the glob in report_names cannot
    # match one, so `..-qualify-report.md` would sit in the directory addressable by its own name and
    # invisible to every discovery path — a ship whose report stands open while `state` answers
    # `no-report` and `idsd-ship continue` starts a fresh one over it.
    .* | *[!0-9A-Za-z._-]*) ;;
    *) echo "$slug" ;;
  esac
}

set_report_paths() {
  report="$reports_dir/$1$report_suffix"
  # Per-pass bookkeeping, in the git dir: no commit and no `git add -A` reaches it, and it is
  # per-worktree so ships cannot collide. Under .idsd/, committed mode would commit them. Keyed by
  # stem, or `invalidate` on one intent would clear another's stage markers and free its stamp.
  stage_returns_dir=$(git_path "idsd-stage-returns/$1") || exit 2
}

stem_of_report_path() {
  local name=${1##*/}
  printf '%s\n' "${name%"$report_suffix"}"
}

# Every report present, one filename stem per line.
report_names() {
  local path
  for path in "$reports_dir"/*"$report_suffix"; do
    [ -f "$path" ] || continue
    stem_of_report_path "$path"
  done
}

# Resolve which report this invocation acts on: the named one, or the only one present. 0 = resolved,
# 1 = none present, 3 = several and none named. Never guesses; the header says what a wrong guess costs.
ambiguous_names=""
resolve_report() {
  local names count name
  if [ -n "${1:-}" ]; then
    name=$(report_name_for "$1")
    [ -n "$name" ] ||
      refuse "error: '$1' names no report — a report file is named after the intent, so it must be a slug ([0-9A-Za-z._-]) or a \"review: <description>\""
    set_report_paths "$name"
    return 0
  fi
  names=$(report_names)
  [ -n "$names" ] || return 1
  count=$(printf '%s\n' "$names" | wc -l | tr -d ' ')
  [ "$count" -eq 1 ] || {
    ambiguous_names="$names"
    return 3
  }
  set_report_paths "$names"
}

# resolve_report's 3, refused with the names listed. $1 is what the caller must do instead: `state` sends
# the human to `list`, the rest only want the name. Refuses, so call it in your own shell (see refuse).
refuse_ambiguous() {
  refuse "error: several qualify reports are open — $1:" \
    "$(printf '%s\n' "$ambiguous_names" | sed 's/^/  /')"
}

skill_dir="$(cd "$(dirname "$0")/.." && pwd)"
template="$skill_dir/templates/qualify-report-template.md"
todo_gate="$(cd "$(dirname "$0")" && pwd)/todo-gate.sh"
NO_ITEMS="no-items"

# Every path a report has ever lived at, and none of them is this one. These are literal history, so
# they never move with a rename: a repo whose ship was in flight across either change has its report
# here, where nothing looks. The harm is silence — `state` answers no-report and a fresh ship starts
# over live work — so every path that reports finding none says these exist, on stderr, leaving `state`
# printing exactly one token. `promote` is the exception: it refuses for want of anything durable, not
# of a report.
legacy_paths() {
  printf '%s\n' "$root/.idsd/ship-report.md" "$root/.idsd/ship-reports"
}

legacy_note() {
  local path found=""
  while read -r path; do
    [ -e "$path" ] || continue
    found="$found $path"
  done <<EOF
$(legacy_paths)
EOF
  [ -n "$found" ] || return 0
  printf '%s\n' \
    "note: nothing reads these any more, and a report left at one is a ship nothing will resume:$found" \
    "  Move what is still live to $reports_dir/<intent>-qualify-report.md, or delete it." >&2
}

report_checksum() {
  cksum <"$report"
}

# Every pipeline stage, in pipeline order. One list, read by the vocabulary check below, by the usage
# line it prints, and by `stamp`'s required-set check — so a stage added to the pipeline cannot be
# accepted by one of the three and missed by the others.
stage_names="code-review security-review tighten refactor retro"

# Refuses, so call it in your own shell (see refuse). Through `$( )` with the status unrelayed, the
# stage name comes back empty and a downstream guard refuses in its place, misnaming the cause.
assert_valid_stage() {
  case " $stage_names " in
    *" $1 "*) return 0 ;;
  esac
  refuse "usage: report.sh $2 <$(printf '%s' "$stage_names" | tr ' ' '|')>"
}

# A marker never written while the caller prints "recorded" is a stage the stamp waves through.
write_stage_marker() {
  local stage="$1" value="$2"
  [ -n "$value" ] || {
    echo "error: the report could not be checksummed — $stage is NOT marked" >&2
    return 1
  }
  mkdir -p "$stage_returns_dir" && printf '%s\n' "$value" >"$stage_returns_dir/$stage" || {
    echo "error: could not write $stage_returns_dir/$stage — $stage is NOT marked" >&2
    return 1
  }
}

# The stage marked returned against the report as it now stands — items still unwritten. Only one may
# stand at a time (see `stage-returned`); a `no-items` marker holds a word, never a checksum.
outstanding_stage() {
  local marker current
  current=$(report_checksum)
  [ -n "$current" ] || return 1
  for marker in "$stage_returns_dir"/*; do
    [ -f "$marker" ] || continue
    [ "$(cat "$marker")" = "$current" ] || continue
    basename "$marker"
    return 0
  done
  return 1
}

stage_was_marked_returned() {
  [ -f "$stage_returns_dir/$1" ]
}

stage_block_reason() {
  local recorded
  stage_was_marked_returned "$1" || {
    printf 'ran but was never marked returned (report.sh stage-returned %s)' "$1"
    return
  }
  recorded=$(cat "$stage_returns_dir/$1")

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

# The two answers are not equally safe: `discard`'s guard is `committed`, so a failed read falling
# through to `throwaway` deletes a tracked .idsd/. That is why this is a separate assertion rather than
# a refusal inside `repo_mode`, which every call site reads as `$(repo_mode)` (see refuse). Run it in
# your own shell before the mode is read.
assert_repo_mode_readable() {
  git -C "$root" ls-files .idsd >/dev/null 2>&1 ||
    refuse "error: could not read the index (git ls-files .idsd) — the repo mode is unknown, and it decides whether .idsd/ is the durable record or scratch to delete"
}

# Nothing of the named ship present means there is no ship to discard, whatever the argument says.
# Without this, `discard <any-legal-slug>` deletes at exit 0 and reports "zero traces" — a whole .idsd/
# in a repo that never used idsd, or one holding only decisions.md. A slug that names a real ship still
# discards it, and must, since that is how a closed ship gets torn down.
assert_ship_exists() {
  local slug="$1"
  [ ! -f "$report" ] || return 0
  # `review` is the one stem with no intent file, so after `close` nothing identifies it, and refusing
  # would leave an empty .idsd/ and its exclusion standing in the mode whose contract is zero traces. Safe to
  # let through because it is a fixed literal: unlike a slug, it cannot be a typo of another ship.
  [ "$slug" != review ] || return 0
  [ ! -f "$root/.idsd/intents/$slug.md" ] || return 0
  [ ! -f "$root/.idsd/archive/$slug.md" ] || return 0
  refuse "error: nothing here belongs to '$slug' — nothing was discarded." \
    "  Looked for $report," \
    "  .idsd/intents/$slug.md and .idsd/archive/$slug.md, and found none of them." \
    "  Check the name against report.sh list; a standalone review is discarded before its report is closed, not after."
}

# What is left under .idsd/ that is not this ship's scratch, as a printable list — empty means `discard`
# may take the whole directory. What counts as remaining is named, never "the .idsd/ root is non-empty",
# so a stray dotfile cannot keep the dir alive. `decisions.md` is deliberately NOT on the list —
# `~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log** makes it throwaway scratch by design.
# Read after this ship's own files are gone, so every count it takes is of what survives. No refusal
# inside, so `$( )` at the call site loses nothing.
surviving_content() {
  local kept="" durable reports_left intents_left
  for durable in charter.md constitution.md language.md playbook.md; do
    [ -e "$root/.idsd/$durable" ] && kept="$kept $durable"
  done
  # A parallel ship's report is another human's work in flight, so it keeps .idsd/ standing. Counted by
  # re-globbing qualify-reports/ once this ship's report is gone — the caller's rmdir only tidies the
  # directory when it empties, and its status is discarded.
  reports_left=$(report_names | grep -c . )
  [ "$reports_left" -eq 0 ] || kept="$kept $reports_left other qualify report(s)"
  # Anything at all under intents/ or archive/ keeps .idsd/ alive, but the label counts what is actually
  # there — "other intents" for a stray `.DS_Store` tells the human something untrue.
  if [ -n "$(ls -A "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null)" ]; then
    intents_left=$(find "$root/.idsd/intents" "$root/.idsd/archive" -maxdepth 1 -name '*.md' -type f 2>/dev/null | wc -l | tr -d ' ')
    if [ "$intents_left" -gt 0 ]; then
      kept="$kept $intents_left other intent(s)"
    else
      kept="$kept unrecognised content under intents/ or archive/"
    fi
  fi
  printf '%s' "$kept"
}

# What must never be committed or fingerprinted, one path per line relative to $root. `promote`
# writes a .gitignore entry per line and `check-ignore` verifies one per line, so the two cannot
# disagree. The durable record is deliberately absent — committed mode keeps it tracked.
# The whole directory, never a path per report: the next intent's report does not exist when
# `promote` runs, so an entry per file would leave it tracked.
ignore_surface() {
  printf '%s/\n' "${reports_dir#"$root"/}"
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

# `promote` drops the exclusion before it writes .gitignore, so every refusal past that point puts it
# back — left off, the next `git add -A` stages the whole scratch dir.
restore_local_exclusion() {
  add_local_exclusion ||
    echo "error: could not restore the local .idsd/ exclusion — .idsd/ is now visible to 'git add -A'" >&2
}

# Every refusal past that point, with the restore attached, so the next one added to `promote` cannot be
# written without it. Refuses, so call it in your own shell (see refuse).
refuse_unpromoted() {
  restore_local_exclusion
  refuse "$@"
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

# Every frontmatter reader greps with `2>/dev/null`, and an empty answer is in is_unstamped()'s set, so
# an unreadable report answers `resume` with an unstamped tree and a clean stage record, all from a file
# nothing ever opened. Takes the consequence as $1, like assert_write_paths_are_real: each caller's is
# different, and a refusal naming the wrong one sends the human to the wrong place. Refuses, so call it
# in your own shell (see refuse).
assert_report_is_readable() {
  [ -r "$report" ] || refuse "error: $report cannot be read — $1"
}

# Resolve, or refuse naming what the caller must pass. Every subcommand that reads an existing report
# opens with this, and the optional stem is always its last argument.
require_report() {
  resolve_report "${1:-}"
  case $? in
    1)
      legacy_note
      refuse "error: no qualify report under $reports_dir — run report.sh init \"<intent>\" first"
      ;;
    3) refuse_ambiguous "name which as the last argument" ;;
  esac
  [ -f "$report" ] || {
    legacy_note
    refuse "error: no qualify report for that intent ($report)"
  }
  assert_report_is_readable "its state is unknown (permissions?)"
}

# The report's open `- [ ]`, for every caller that must refuse rather than read a failed scan as "nothing
# open": todo-gate.sh exits 0 with none and 1 with them printed, so anything above that is the scan not
# running, and $1 says what is then unknown. Sets open_todos; refuses, so call it in your own shell
# (see refuse).
open_todos=""
read_open_todos() {
  local status
  open_todos=$("$todo_gate" "$report")
  status=$?
  [ "$status" -le 1 ] ||
    refuse "error: the open-item scan did not run — todo-gate.sh exited $status; $1"
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

# The one script that fingerprints a tree. Never recompute the recipe here: get it half right, with a
# throwaway index but no throwaway object store, and every untracked file's content lands in the human's
# own .git/objects for good, referenced by no ref and so collected by nothing.
tree_fingerprint="$HOME/.kk-flavor/scripts/tree-fingerprint.sh"

# The fingerprint the freshness gate compares. Fails loudly rather than printing an empty tree, which
# would match the next equally-failed reading and report "tree fresh" — and loudly rather than falling
# back to a local recipe.
current_tree() {
  local tree status
  [ -x "$tree_fingerprint" ] || {
    echo "error: $tree_fingerprint is missing or not executable — the tree could not be fingerprinted." >&2
    echo "  It owns the fingerprint recipe; there is deliberately no local fallback, because a second copy is what put untracked working files in .git/objects for good." >&2
    return 1
  }
  # Its stderr is inherited, so git's own account of a failed walk reaches the caller.
  tree=$("$tree_fingerprint" "$root")
  status=$?
  [ "$status" -eq 0 ] && [ -n "$tree" ] || {
    echo "error: $tree_fingerprint exited $status without a tree — the tree could not be fingerprinted" >&2
    return 1
  }
  printf '%s\n' "$tree"
}

reviewed_tree() {
  grep -m1 '^reviewed-tree:' "$report" 2>/dev/null | sed 's/^reviewed-tree:[[:space:]]*//'
}

# Every frontmatter value meaning "no stamp stands here". One set, used whole by every reader — a
# reader knowing only some accepts what the others reject. `init` holds the template to this set.
is_unstamped() {
  case "$1" in
    "" | pending | "<hash>" | "<stages>") return 0 ;;
  esac
  return 1
}

# Empty when never stamped with stages — callers treat that as not-full.
reviewed_stages() {
  grep -m1 '^reviewed-stages:' "$report" 2>/dev/null | sed 's/^reviewed-stages:[[:space:]]*//'
}

# Entries the last stamp marked `(fast)` — stages trimmed for turnaround. Any of them means the pass
# was not a full one.
fast_trims() {
  reviewed_stages | tr ',' '\n' | grep '(fast)' | tr '\n' ' ' | sed 's/ $//'
}

# One fingerprint per invocation. `list` scores every report against the same working tree, so the
# walk `current_tree` does is the same walk each time.
cached_tree=""
current_tree_cached() {
  [ -n "$cached_tree" ] || cached_tree=$(current_tree) || return 1
  printf '%s\n' "$cached_tree"
}

# The `continue` routing token for the report the caller resolved. Every path either prints a token or
# leaves through `refuse`: a token it cannot stand behind would route `continue` past a live gate.
state_token() {
  local slug reviewed current
  # A built intent's file has moved to archive/ (a standalone `review:` has no slug, so this is
  # skipped).
  slug=$(intent_slug)
  if [ -n "$slug" ] && [ -f "$root/.idsd/archive/$slug.md" ]; then
    echo "done"
    return 0
  fi
  reviewed=$(reviewed_tree)
  if is_unstamped "$reviewed"; then # never stamped, or invalidated mid-pass → quality stages haven't completed
    echo "resume"
    return 0
  fi
  current=$(current_tree_cached) || exit 2
  if [ "$current" != "$reviewed" ]; then
    echo "re-qualify" # reviewed once, tree moved since
    return 0
  fi
  read_open_todos "the state is unknown."
  if [ -n "$open_todos" ]; then
    echo "decide" # quality done, tree fresh, open `- [ ]` remain
    return 0
  fi
  if is_unstamped "$(reviewed_stages)" || [ -n "$(fast_trims)" ]; then
    echo "finalize" # stages trimmed (or unrecorded) and fresh, nothing open — a full qualify remains
    return 0
  fi
  echo "ready" # full-reviewed, tree fresh, nothing open → merge-ready
}

# The template carries the frontmatter every later reader greps, and gate and state never read the
# template itself — so a missing field or a drifted placeholder can only be caught here, before a report
# is scaffolded from it. A drifted placeholder stamps every new report as already reviewed.
assert_template_stampable() {
  local field placeholder
  [ ! -L "$template" ] ||
    refuse "error: template $template is a symlink — refusing to read the template through one; the report was NOT initialized"
  [ -f "$template" ] || refuse "error: template not found ($template)"
  grep -q '^intent:' "$template" ||
    refuse "error: template $template has no 'intent:' line to stamp — the report was NOT initialized"
  for field in reviewed-tree reviewed-stages; do
    grep -q "^$field:" "$template" ||
      refuse "error: template $template has no '$field:' line — gate and state read it; the report was NOT initialized"
    placeholder=$(grep -m1 "^$field:" "$template" | sed "s/^$field:[[:space:]]*//")
    is_unstamped "$placeholder" ||
      refuse "error: template $template writes '$field: $placeholder', which gate and state read as a completed review — every new report would gate clean. Restore a placeholder is_unstamped() knows, or add this one to it. The report was NOT initialized."
  done
}

# `-L` tests only the *final* component, so every directory `init` writes through needs its own check: a
# symlinked `.idsd` slips past the report's test, and every write then lands wherever it points.
# `check-ignore` is the documented first step, and this is the assertion that it happened. A report
# written where git does not ignore it sits inside the tree it fingerprints, so `state` answers
# `re-qualify` straight after a complete five-stage stamp and `gate` blocks on freshness with nothing
# that can clear it.
# Ignored has to mean ignored by something that travels with the repository. `core.excludesFile` is one
# machine's, so it answers the plain `-q` question while ignoring nothing on anybody else's clone, and
# the next `git add -A` there stages the report. Prints the source it read, so a caller can name it.
# One predicate for every caller, or `check-ignore` asks a weaker question than `init` enforces and the
# remedy `init` names cannot satisfy it.
# Arm order is load-bearing. `info/exclude` is repo-relative in an ordinary repo and ABSOLUTE in a
# linked worktree, so it is matched before absolute paths are rejected; every other in-repo source is
# repo-relative, so rejecting the rest of the absolutes is what excludes `core.excludesFile`. Match
# `*/.gitignore` first instead and `core.excludesFile=~/.gitignore`, the common global setup, passes.
# Which file git read to ignore a path, or empty when nothing ignores it. `-v` because the answer is the
# whole question here: `core.excludesFile` and `.git/info/exclude` satisfy the plain `-q` form too, and
# each caller below accepts a different set of sources.
ignore_source_of() {
  local source_file
  source_file=$(git -C "$root" check-ignore -v "$1" 2>/dev/null | head -1)
  printf '%s' "${source_file%%:*}"
}

ignored_source_travels() {
  local source_file
  source_file=$(ignore_source_of "$1")
  printf '%s' "$source_file"
  case "$source_file" in
    .git/info/exclude | */.git/info/exclude) return 0 ;;
    /*) return 1 ;;
    .gitignore | */.gitignore) return 0 ;;
  esac
  return 1
}

assert_reports_dir_is_ignored() {
  local source_file
  source_file=$(ignored_source_travels "$reports_dir/") && return 0
  # Built outside the refusal, not as a `${var:+…}` inside it: an apostrophe within that expansion ends
  # the enclosing quote and the whole file stops parsing.
  local read_note=""
  [ -z "$source_file" ] ||
    read_note="  A global core.excludesFile does not count — it belongs to this machine alone, so a clone would commit the report. Source read: $source_file"
  refuse "error: nothing in this repository ignores $reports_dir — the report was NOT initialized." \
    "  Run report.sh check-ignore first; it is what excludes the scratch, by the mechanism that fits the repo mode." \
    "  Written here, the report would sit inside its own fingerprint, so every stamp would be stale on arrival." \
    "$read_note"
}

# Takes what did not happen, because both callers reach it: `init` writes and `discard` deletes, and a
# refusal naming the wrong one tells the human to look in the wrong place.
assert_write_paths_are_real() {
  local write_dir outcome="${1:-the report was NOT initialized}"
  for write_dir in "$root/.idsd" "$reports_dir"; do
    if [ -L "$write_dir" ]; then
      refuse "error: $write_dir is a symlink -> $(readlink "$write_dir") — $outcome." \
        "  both .idsd/ and its qualify-reports/ are always real directories inside the repo. Remove the link, then re-run."
    fi
  done
  # The report is never legitimately a symlink, and `--force` does not override this. The write is a
  # staged `cp` then `mv`, which replaces a link rather than following it — so what this catches is
  # `--force` destroying whatever link the human left there, on a dangling one `-e` cannot even see.
  if [ -L "$report" ]; then
    refuse "error: $report is a symlink -> $(readlink "$report") — $outcome." \
      "  the report is always a regular file. Remove the link, then re-run."
  fi
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
    # Trimmed once, before the emptiness guard: a whitespace-only value would otherwise pass the guard
    # and scaffold a report whose blank `intent:` every reader treats as a standalone review. Every later
    # reader uses the result, so the stem report_name_for derives and the value the frontmatter records
    # cannot come apart and name different ships. The collapse below maps whitespace to whitespace, so
    # nothing needs re-trimming.
    intent_value=${intent_value#"${intent_value%%[![:space:]]*}"}
    [ -n "$intent_value" ] || refuse "usage: report.sh init \"<intent frontmatter value>\" [--force]"
    # Frontmatter is single-line: collapse CR/LF so a value seeded from a fetched ticket can't inject
    # extra frontmatter lines (a forged reviewed-tree).
    intent_value=${intent_value//[$'\n\r']/ }
    report_name=$(report_name_for "$intent_value")
    [ -n "$report_name" ] ||
      refuse "error: '$intent_value' cannot name a report file — the intent must be a slug ([0-9A-Za-z._-]) or a \"review: <description>\". The report was NOT initialized."
    set_report_paths "$report_name"
    assert_template_stampable
    assert_write_paths_are_real
    assert_reports_dir_is_ignored
    # Emitted here too, and not only where a report is missing: this is the first command a pass runs,
    # so it is the moment a legacy report is worth knowing about — after this one exists, a note saying
    # an older one is lying around reads as noise about the report just created.
    legacy_note
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
    fi
    mkdir -p "$(dirname "$report")" ||
      refuse "error: could not create $(dirname "$report") — the report was NOT initialized"
    # Staged beside the report and renamed over it, never copied onto it: a write that dies partway
    # leaves a truncated report, and --force has already discarded the only other copy of those items.
    # The temp name is deliberately not `*-qualify-report.md`, so a leftover joins no listing.
    staged_report="$report.new"
    # Removed before the copy, never guarded by a symlink refusal: this path is ours and transient, so
    # a link planted there is hostile (a committed one reaches us through someone else's branch) and a
    # regular file there is a crashed `init`'s leftover. Refusing would wedge `init` on the second case;
    # `rm -f` unlinks the link itself, so the `cp` below lands on a fresh regular file either way.
    rm -f "$staged_report" || refuse "error: could not clear $staged_report — the report was NOT initialized"
    cp "$template" "$staged_report" && mv "$staged_report" "$report" || {
      rm -f "$staged_report"
      refuse "error: could not write $report from $template — the report was NOT initialized${replacing:+; the previous one is untouched and still holds $replacing}"
    }
    # Printed at the moment of the act, because it is the only record of what --force just discarded.
    [ -z "$replacing" ] || {
      {
        echo "warning: --force discarded the previous report, which held $replacing"
        [ -z "$carried" ] || printf '%s\n' "$carried" | sed 's/^/    /'
        echo "  nothing above is kept anywhere — route it now if it still matters."
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
    require_report "${3:-}"
    stage="${2:-}"
    assert_valid_stage "$stage" stage-returned
    outstanding=$(outstanding_stage)
    # Its own name is the one exception. A streamed stage returns, gets resumed with what landed, and
    # returns again with nothing recorded in between — the report has not moved, so it is itself the
    # outstanding stage. Re-marking it rewrites the same checksum, so this is idempotent; what the guard
    # exists to stop is a *second* stage being waved through on the first's unrecorded return.
    [ -z "$outstanding" ] || [ "$outstanding" = "$stage" ] ||
      refuse "error: $outstanding is marked returned and the report has not moved since — record its items, or run report.sh no-items $outstanding, before marking a stage returned."
    write_stage_marker "$stage" "$(report_checksum)" || exit 2
    echo "recorded return of $stage — record its items, or report.sh no-items $stage, before taking the next stage's return"
    ;;

  # The escape hatch for a stage that genuinely surfaced nothing. It demands a marker: without that,
  # a pass that ran nothing can declare every stage empty and stamp.
  no-items)
    require_report "${3:-}"
    stage="${2:-}"
    assert_valid_stage "$stage" no-items
    stage_was_marked_returned "$stage" ||
      refuse "error: $stage was never marked returned — run report.sh stage-returned $stage when it returns, then report.sh no-items $stage once you have read its findings."
    write_stage_marker "$stage" "$NO_ITEMS" || exit 2
    echo "recorded $stage as having surfaced nothing for the report"
    ;;

  stamp)
    require_report "${3:-}"
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
    validation_errors=$(printf '%s\n' "$entries" | tr ',' '\n' | stage_names="$stage_names" awk '
      /^code-review$/ { seen["code-review"]++; next }
      /^refactor(:partial\((fast|cap)\))?$/ { seen["refactor"]++; next }
      /^(security-review|tighten|retro)(:skipped\((fast|not-applicable)\))?$/ { name = $0; sub(/:.*/, "", name); seen[name]++; next }
      { print "malformed entry: " $0 }
      END {
        split(ENVIRON["stage_names"], required, " ")
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
      # Leading `(` on the pattern: bash 3.2 (still macOS's /bin/bash) ends the enclosing `$( … )` at a
      # bare case-arm `)`.
      case "$entry" in
        (*:skipped\(*) continue ;;
      esac
      stage_name=${entry%%:*}
      reason=$(stage_block_reason "$stage_name")
      [ -z "$reason" ] || printf '  %s: %s\n' "$stage_name" "$reason"
    done)
    [ -z "$block_reasons" ] || refuse "error: these stages are recorded as having run, but:" "$block_reasons"
    tree=$(current_tree) || exit 2
    # Frontmatter only, never a body line that happens to quote a field.
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
    require_report "${2:-}"
    blocked=0
    current=$(current_tree) || exit 2
    reviewed=$(reviewed_tree)
    if [ "$current" != "$reviewed" ]; then
      echo "BLOCK (freshness): tree changed since last qualify (current $current != reviewed ${reviewed:-<unstamped>}). Re-qualify, or the human may explicitly override this one." >&2
      blocked=1
    fi
    stages=$(reviewed_stages)
    trims=$(fast_trims)
    if is_unstamped "$stages"; then
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
    require_report "${2:-}"
    read_open_todos "prior open items are unknown."
    [ -z "$open_todos" ] || printf '%s\n' "$open_todos"
    ;;

  invalidate)
    require_report "${2:-}"
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
    rm -rf "$stage_returns_dir"
    echo "invalidated reviewed-tree — restamp when the pass completes"
    ;;

  check-ignore)
    # Runs before anything else (init included, so it never requires the report to exist), and before
    # any fingerprinting `git add -A`, so nothing scratch is ever staged.
    assert_repo_mode_readable
    if [ "$(repo_mode)" = committed ]; then
      # A path already tracked also answers "not ignored" here — the case most worth the warning. Asked
      # through the same predicate `init` enforces, or this prints ok where init then refuses and sends
      # the human back to this command.
      unignored=$(ignore_surface | while read -r entry; do
        ignored_source_travels "$root/$entry" >/dev/null || printf " '%s'" "$entry"
      done)
      if [ -z "$unignored" ]; then
        echo "ok: qualify-reports/ is gitignored (committed idsd repo)"
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
    # Promotion is about the whole .idsd/, so it names no single report — it only needs one to exist,
    # as the evidence that a ship happened here.
    [ -n "$(report_names)" ] ||
      refuse "error: no qualify report under $reports_dir — nothing to promote"
    assert_repo_mode_readable
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
    [ -z "$unwritten" ] ||
      refuse_unpromoted "error: could not add$unwritten to $gitignore — not promoted." \
        "  Nothing was staged, so the report is not on its way into a commit."
    # Writing an entry is not the same as it taking effect — ask git, and with -v, because
    # `core.excludesFile` and `.git/info/exclude` answer the plain question too and are this machine's
    # alone. Only root `.gitignore` is the shared answer this subcommand claims to have written.
    unignored=$(ignore_surface | while read -r entry; do
      [ "$(ignore_source_of "$root/$entry")" = ".gitignore" ] || printf " '%s'" "$entry"
    done)
    [ -z "$unignored" ] ||
      refuse_unpromoted "error: the entries are in $gitignore, but git still does not ignore:$unignored — not promoted." \
        "  git ignores nothing it cannot read; a symlinked or unreadable .gitignore does exactly this." \
        "  Nothing was staged, so the report is not on its way into a commit."
    git -C "$root" add .idsd .gitignore ||
      refuse_unpromoted "error: could not stage .idsd/ and .gitignore — not promoted"
    # `git add` on a directory whose every file is ignored stages nothing and still exits 0, and
    # qualify-reports/ is ignored by the entry just written — so with nothing else under .idsd/, the add
    # is a no-op. Success is read from the mode for that reason, never from the add's exit: unpromoted,
    # the next check-ignore re-excludes .idsd/ and the whole promotion silently un-happens.
    [ "$(repo_mode)" = committed ] ||
      refuse_unpromoted "error: nothing under .idsd/ could be staged, so it is still a throwaway — not promoted." \
        "  Every file there is ignored. A durable .idsd/ needs something that is not: an intent, a charter, a constitution." \
        "  The .gitignore entry stays (it is wanted in both modes); the local exclusion is back."
    echo "promoted: .idsd/ staged, qualify-reports/ ignored via .gitignore — commit when ready (not committed here)"
    ;;

  # Named, this runs with no report at all, so `done` can `close` first and still `discard` after. That
  # is the one order that composes, and the one `idsd-ship` → `done` uses; reversed, `close` has no
  # report left to read and refuses.
  discard)
    resolve_report "${2:-}"
    case $? in
      1)
        legacy_note
        refuse "error: nothing to discard — no qualify report under $reports_dir, and no intent named" \
          "  Name the intent to discard a ship whose report is already closed."
        ;;
      3) refuse_ambiguous "name which as the last argument" ;;
    esac
    assert_repo_mode_readable
    if [ "$(repo_mode)" = committed ]; then
      refuse "committed idsd repo — .idsd/ is the durable record; nothing to discard"
    fi
    # Without this, a symlinked `.idsd` lets every deletion below reach through to a target outside the
    # repo. `init` has carried the same guard since a link there could steer a write out.
    assert_write_paths_are_real "nothing was discarded"
    # The filename is the ship's name here — it came from the caller, or from being the only report
    # open. The frontmatter is read only to cross-check it, and only when there is a report left to
    # read: a closed ship has none, and nothing about the deletion below needed it.
    stem=$(stem_of_report_path "$report")
    assert_ship_exists "$stem"
    slug=$stem
    if [ -f "$report" ]; then
      # A report that is present must be readable. Falling back to the filename for one we cannot open
      # would skip the cross-check below on the single path that deletes another ship's intent file.
      assert_report_is_readable "nothing was discarded, because its intent cannot be cross-checked (permissions?)"
      slug=$(intent_slug)
    fi
    # The two must name the same ship before anything is deleted. Out of step, what gets deleted is
    # another ship's in-flight intent file, which throwaway mode keeps no copy of anywhere.
    [ -z "$slug" ] || [ "$slug" = "$stem" ] ||
      refuse "error: $report is named for '$stem' but records 'intent: $slug' — nothing was discarded." \
        "  Those are different ships, and discard deletes the intent file the frontmatter names." \
        "  Nothing writes that line after init, so a hand-edit or a bug is what put them out of step." \
        "  Reconcile the two by hand, then re-run."
    [ -n "$slug" ] && rm -f "$root/.idsd/intents/$slug.md" "$root/.idsd/archive/$slug.md"
    rm -f "$report"
    # The stage markers sit in the git dir, which the .idsd/ removal below never reaches.
    rm -rf "$stage_returns_dir"
    rmdir "$reports_dir" "$root/.idsd/intents" "$root/.idsd/archive" 2>/dev/null || true
    kept=$(surviving_content)
    if [ -z "$kept" ]; then
      rm -rf "$root/.idsd"
      # .git/info/exclude is shared across worktrees, and a parallel throwaway ship's .idsd/ must stay
      # excluded. Drop it only from the last worktree.
      # "Removed" holds only because assert_ship_exists ran: nothing reaches here without a report or an
      # intent file to remove. Without it, a second run or any wrong slug claims this having deleted nothing.
      if [ "$(git -C "$root" worktree list --porcelain 2>/dev/null | grep -c '^worktree ')" -gt 1 ]; then
        echo "discarded: removed .idsd/ scratch; kept the shared exclusion (other worktrees exist)"
      else
        # Read the return, don't just call it: `drop_local_exclusion` fails when it cannot read or
        # replace the exclude file, and "zero traces" over a surviving entry is the one claim here a
        # human acts on without checking.
        if drop_local_exclusion; then
          echo "discarded: removed .idsd/ scratch and its local exclusion (throwaway, zero traces)"
        else
          echo "discarded: removed .idsd/ scratch, but the '.idsd/' entry in $(git_path info/exclude) could not be removed — it is still excluded" >&2
          exit 2
        fi
      fi
    else
      # `close` may already have taken the report, and a ship can have no intent file, so
      # assert_ship_exists guarantees only that one of the two was there.
      echo "discarded: removed what remained of this ship; kept .idsd/ (still holds:$kept)"
    fi
    ;;

  state)
    resolve_report "${2:-}"
    resolved=$?
    # Several open ships have no single state, and `continue` must not act on one of them picked at
    # random. `list` is what answers here, and the message names it.
    [ "$resolved" -ne 3 ] ||
      refuse_ambiguous "no single state. Run report.sh list, then report.sh state <intent>"
    # The archive is read BEFORE the report's absence decides, because `close` retires a landed ship's
    # report and the archived intent file is then the only record that it landed. Absence alone routes
    # `continue <intent>` to "start ship <intent>" — a rebuild of work already merged.
    if [ "$resolved" -eq 0 ]; then
      archived_slug=$(stem_of_report_path "$report")
      if [ -f "$root/.idsd/archive/$archived_slug.md" ]; then
        echo "done"
        exit 0
      fi
    fi
    if [ "$resolved" -ne 0 ] || [ ! -f "$report" ]; then
      echo "no-report"
      legacy_note
      exit 0
    fi
    # `state` resolves for itself rather than through require_report, because a missing report is a
    # token here and a refusal there. Unreadable is neither: it is a report whose state is unknown.
    assert_report_is_readable "its state is unknown (permissions?), and 'resume' is what an unread report looks like"
    state_token
    ;;

  # One line per open ship, so `continue` can route with several in flight.
  list)
    names=$(report_names)
    [ -n "$names" ] || {
      echo "no reports"
      legacy_note
      exit 0
    }
    # Primed HERE, in this shell: `state_token` runs inside a command substitution below, so a cache it
    # filled there would die with that subshell and two ships could be scored against different trees.
    # Failing to prime is not fatal, though — an unstamped ship answers without the tree at all, and
    # refusing here would let one unreadable file anywhere in the repo silence the whole listing. Each
    # `state_token` that does need the tree retries and refuses on its own behalf.
    current_tree_cached >/dev/null 2>&1 || true
    # Built whole, then printed. Printing as it goes puts the ships already reached on stdout before a
    # later one refuses, and a truncated listing reads exactly like a complete one.
    listing=""
    # The body runs in this shell, never a pipeline's subshell: a refusal inside it exits the script,
    # where a pipe would leave the outer shell printing an empty listing and exiting 0.
    while read -r name; do
      set_report_paths "$name"
      assert_report_is_readable "nothing was printed, this listing included"
      # The token is checked, never just interpolated: `$(state_token)` is its own subshell, so a refusal
      # or an `exit 2` in there kills only that subshell and leaves this loop printing a blank token at 0.
      token=$(state_token) && [ -n "$token" ] ||
        refuse "error: no state could be read for $name — nothing was printed rather than a listing with a blank state"
      listing="$listing$name	$token
"
    done <<<"$names"
    printf '%s' "$listing"
    ;;

  # Retire one ship's scratch once it has landed. `done` calls this after the commit succeeds; the open
  # `- [ ]` refusal is what stops a hand-run from dropping a decision nobody routed.
  close)
    shift
    # Order-free, like `init`: `--force` read positionally resolves as an intent name (its whole charset
    # is legal in a slug) and closes a report that does not exist.
    is_forced=0
    close_name=""
    for arg in "$@"; do
      case "$arg" in
        --force) is_forced=1 ;;
        *) [ -n "$close_name" ] || close_name="$arg" ;;
      esac
    done
    require_report "$close_name"
    if [ "$is_forced" -eq 0 ]; then
      read_open_todos "nothing was closed."
      [ -z "$open_todos" ] ||
        refuse "error: $report still holds open '- [ ]' — nothing was closed." \
          "$(printf '%s\n' "$open_todos" | sed 's/^/  /')" \
          "  Resolve or route each, or re-run with --force to discard them."
    fi
    rm -f "$report"
    # The stage markers are in the git dir, so removing the report leaves them behind, and the next
    # ship for this intent would inherit a completed stage record and stamp for free.
    rm -rf "$stage_returns_dir"
    rmdir "$reports_dir" 2>/dev/null || true
    echo "closed ${report##*/} — its stage markers are gone; decisions.md is untouched"
    ;;

  *)
    refuse "usage: report.sh {init <intent>|repo-mode|invalidate|stage-returned <stage>|no-items <stage>|stamp \"<stages>\"|gate|carry|check-ignore|promote|discard|close|state|list} [<intent>]" \
      "  every subcommand that reads a report takes the intent as its last argument; omit it when only one is open"
    ;;
esac
