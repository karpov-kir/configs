#!/usr/bin/env bash
# Ecosystem size ledger — the numbers that decide whether a pass is worth running, measured rather
# than estimated. Owned by kk-reduce; it appends to ../stats.md, relative to this script's dir.
#   usage: stats.sh [<root>]                    print the current measurements
#          stats.sh --append "<note>" [<root>]  print them and append a dated row to stats.md
# The note is one argument — quote it, or its first word is read as <root>.
# <root> holds kk-flavor/ and skills/; defaults to . then ./ai, matching check.sh.
# Exits 0 on success, 2 when it could not measure — a measurement that did not run is not a zero.
# A change here needs a case in stats-test.sh beside it, which says what it covers and why, and
# stats-mutate.sh is what shows that case can fail rather than merely pass.
set -uo pipefail
export LC_ALL=C

note=""
case "${1:-}" in
  --append)
    shift
    note="${1:-upkeep}"
    shift 2>/dev/null || true
    # stats.md is a markdown table and the note is its last cell, so the note must not be able to
    # end the row. A newline in it writes a second line that reads as a row of its own, date and
    # counts included. A bare `|` splits the cell into extra columns. Either way a later pass reads
    # fabricated numbers as a measurement and skips a campaign that is owed.
    note=${note//[$'\n\r']/ }
    # Backslashes before pipes, never the other way round. Reversed, a note already carrying `\|`
    # comes out `\\|`: an escaped backslash followed by a live pipe.
    note=${note//\\/\\\\}
    note=${note//|/\\|}
    # A bar the writer cannot talk itself past. `kk-reduce` asks for what the campaign changed,
    # and prose asking for that politely has been ignored by every author of a long row here — including
    # the one who wrote the rule, three rows running. This file is re-read whole on every invocation that
    # reaches step 1, so the cost lands on every later run rather than on the author.
    note_words=$(printf '%s' "$note" | wc -w | tr -d ' ')
    if [ "$note_words" -gt 40 ]; then
      echo "stats.sh: the note is $note_words words; 40 is the most a row takes. Nothing was appended." >&2
      echo "stats.sh: keep what a later pass must act on — what ran, and what is open. The reasoning belongs in your reply to the human, which is read once." >&2
      exit 2
    fi
    ;;
esac
root="${1:-}"

# Resolve the same way check.sh does, so both tools always describe the same tree.
if [ -z "$root" ]; then
  if [ -d "./kk-flavor" ] && [ -d "./skills" ]; then
    root="."
  elif [ -d "./ai/kk-flavor" ] && [ -d "./ai/skills" ]; then
    root="./ai"
  fi
fi
[ -n "$root" ] && [ -d "$root/kk-flavor" ] && [ -d "$root/skills" ] || {
  echo "stats.sh: no root holding both kk-flavor/ and skills/" >&2
  echo "stats.sh: exit 2 — nothing was measured. Fix the invocation; do not read this as no change." >&2
  exit 2
}

# Words across every file matching one name pattern under a directory, or 0 when there are none.
words_in_tree() {
  find "$1" -name "$2" -type f -exec cat {} + 2>/dev/null | wc -w | tr -d ' '
}

# Anything attacker-chosen that reaches a message goes through this first — `check.sh` carries the same
# helper for the same reason. Every control byte, not only the two that end a line: these names come out of
# a tree someone else may have written, and an ESC sequence among them rewrites whatever read this.
oneline() {
  printf '%s' "$1" | LC_ALL=C tr '[:cntrl:]' ' '
}

# Words in one file. The `tr` drops the padding BSD `wc` writes, so the result is usable in arithmetic.
words_in_file() {
  wc -w <"$1" | tr -d ' '
}

# The directory a real path resolves to, symlinks followed. `readlink -f` is not portable to the
# bash 3.2 machines this runs on; `cd -P` is.
# --- shared:canonical-dir ---
canonical_dir() {
  [ -d "$1" ] || return 0
  (cd -P -- "$1" 2>/dev/null && pwd -P)
}
# --- end shared:canonical-dir ---

# True when a path's directory sits at or under the root. check.sh carries this guard and its helper
# verbatim, for the same reason. A `## Read always` target is attacker-authored wherever the tree under
# review is someone else's. Without this, `../../../` there pulls any file the invoking user can read
# into the budget, and the import scan below prints matched substrings, so it reaches whatever reads this
# output too. Both copies are fenced as a shared region, so check.sh fails the wiring check when they drift.
# A symlink is refused rather than resolved: `cd -P` canonicalises a *directory*, so it never sees
# the final component, and a link committed at a budget path would walk straight through a check
# that only tested its parent.
# --- shared:contained-in-root ---
root_canon="$(canonical_dir "$root")"
contained_in_root() {
  local dir
  [ -L "$1" ] && return 1
  # A regular file, or nothing. `-e` alone admits a FIFO or a device, which `cat` then blocks on
  # forever; a dangling symlink fails `-e` entirely, so callers enter on `-e || -L` and both become
  # a refusal, not a silent drop. It also refuses a path that does not exist, so no caller can pass
  # one through and inflate the file count.
  [ -f "$1" ] || return 1
  # Readable too, not just regular. `-f` passes on a mode-000 file, and the read behind the figure
  # then fails. Here that drops the file's words but still counts the file. In stats.sh an empty word
  # count makes the arithmetic a syntax error, so bash abandons the block and stats.sh prints 0.
  # Nothing records a refusal either. The guard that keeps a short figure out of the ledger never
  # fires, so `--append` writes that 0 as a measurement.
  [ -r "$1" ] || return 1
  dir="$(canonical_dir "$(dirname "$1")")"
  [ -n "$root_canon" ] && [ -n "$dir" ] || return 1
  [ "$dir" = "$root_canon" ] || [ "${dir#"$root_canon"/}" != "$dir" ]
}
# --- end shared:contained-in-root ---
# The refused file's content never reaches the message, but its name is attacker-chosen wherever the
# tree is not the user's own, so the name is truncated. check.sh bounds the same string for the same
# reason. A refusal makes the always-loaded figure wrong, so it is counted and acted on below.
# Bounded in count as well as in name length, the way check.sh bounds the identical string. 400 symlinked
# Read-always targets otherwise put 60 KB of attacker-chosen names on stderr; the count below stays exact,
# and it alone decides whether a row is withheld, so suppressing the naming costs nothing.
budget_refusals=0
refuse_budget_file() {
  budget_refusals=$((budget_refusals + 1))
  if [ "$budget_refusals" -le 5 ]; then
    echo "stats.sh: budget file refused (symlink, unreadable, or resolves outside $root) — not read, not counted: $(oneline "$1" | cut -c1-80)" >&2
  elif [ "$budget_refusals" -eq 6 ]; then
    echo "stats.sh: further budget-file refusals suppressed; the count in the exit message is the total" >&2
  fi
}

prose=$(words_in_tree "$root" '*.md')
prose_files=$(find "$root" -name '*.md' -type f | wc -l | tr -d ' ')

# The ledger is a record, not an instruction, and it is reported on its own line rather than inside
# `prose`. Counted together, the number that decides whether a reduction is owed rises every time a
# reduction records that it ran — at its worst, more than half the growth since the last campaign was
# this file describing that growth. Its own line keeps the cost visible, because step 1 reads it whole
# on every invocation and nothing else in the tier is read that way.
ledger="$root/skills/kk-reduce/stats.md"
ledger_words=0
# Guarded like a budget file, not with a bare `-f`: `prose` is measured with `find -type f`, which does
# not walk a symlink, while `-f` follows one. A symlink here would take words out of a total that never
# held them, and a large enough target drives `prose` negative into the "scan did not work" exit.
if contained_in_root "$ledger"; then
  ledger_words=$(words_in_file "$ledger")
  prose=$((prose - ledger_words))
  prose_files=$((prose_files - 1))
fi
scripts=$(words_in_tree "$root" '*.sh')
skills=$(find "$root/skills" -maxdepth 2 -name SKILL.md -type f | wc -l | tr -d ' ')

# The always-loaded tier, in two parts: the router's own "Read always" targets, and every skill
# description — which the harness keeps in context for any skill without disable-model-invocation.
# CLAUDE.md is seeded first: it is symlinked to ~/.claude/CLAUDE.md, sits in every system prompt, and
# loads inject.md at all. check.sh counts it too, and the two must report the same number.
# Read the target list a line at a time — a `for` over unquoted `$(…)` splits a path containing a
# space into fragments that each fail the -f test, dropping the file from the budget in silence.
# Links are extracted with check.sh's own expression, which puts every one on its own line: a `sed`
# taking the last `(…)` per line is greedy, so a bullet naming two docs yields only the second and
# the two scripts report different budgets for the same tree.
inject="$root/kk-flavor/inject.md"
always_loaded_words=0
budget_files=()
if [ -e "$root/CLAUDE.md" ] || [ -L "$root/CLAUDE.md" ]; then
  if contained_in_root "$root/CLAUDE.md"; then
    always_loaded_words=$(words_in_file "$root/CLAUDE.md")
    budget_files+=("$root/CLAUDE.md")
  else
    refuse_budget_file "$root/CLAUDE.md"
  fi
fi
# `-e`, not `-f`: a symlink to a FIFO or a device fails `-f`, and testing only that lets it fall
# through both branches — inject.md and every target it lists dropped from the budget in silence.
if { [ -e "$inject" ] || [ -L "$inject" ]; } && ! contained_in_root "$inject"; then
  refuse_budget_file "$inject"
elif [ -f "$inject" ]; then
  while IFS= read -r target; do
    [ -n "$target" ] || continue
    file="$root/kk-flavor/$target"
    if [ ! -e "$file" ] && [ ! -L "$file" ]; then
      echo "stats.sh: inject.md lists '$target' under Read always, but $file does not exist" >&2
    elif ! contained_in_root "$file"; then
      refuse_budget_file "inject.md Read-always target $target"
    else
      always_loaded_words=$((always_loaded_words + $(words_in_file "$file")))
      budget_files+=("$file")
    fi
  done <<EOF
$(awk '/^## Read always/{f=1;next} /^## /{f=0} f' "$inject" | grep -oE '\]\([^)#]+\)' | sed 's/^](//; s/)$//')
EOF
  always_loaded_words=$((always_loaded_words + $(words_in_file "$inject")))
  budget_files+=("$inject")
fi

# An `@path` import inside a budget file loads with it, so one resolved at the installed mount is
# counted into this tier below. check.sh owns the reasoning: why the scan exists, why it runs field by
# field, its two bounds, and every refusal. Both scripts report this tier and must not disagree.
# A function rather than an inline pass, because one named file's imports are needed below as well as
# the pool across all of them.
# --- shared:import-scan ---
imports_in() {
  awk 'FNR == 1 { fence = 0 }
       /^```/ { fence = !fence; next }
       fence { next }
       { line = $0
         gsub(/`[^`]*`/, " ", line)
         n = split(line, field, /[[:space:]]+/)
         for (i = 1; i <= n; i++) {
           if (length(field[i]) > 4096) continue
           tok = " " field[i]
           hits = 0
           while (match(tok, /[^A-Za-z0-9_]@[~A-Za-z0-9._\/-]+\.[A-Za-z0-9]+/) && ++hits <= 64) {
             print substr(tok, RSTART + 2, RLENGTH - 2)
             tok = substr(tok, RSTART + RLENGTH)
           }
         } }' "$@" | sort -u
}
# --- end shared:import-scan ---
# The array test is required: under `set -u`, bash 3.2 errors on "${arr[@]}" when arr is empty, and
# a repo with no CLAUDE.md and no inject.md reaches this line with exactly that.
budget_imports=""
if [ ${#budget_files[@]} -gt 0 ]; then
  budget_imports="$(imports_in "${budget_files[@]}")"
fi

# --- shared:import-at-mount ---
# An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in `CLAUDE.md`
# is `~/.claude/RTK.md`. That file is **not** a tracked file this repo forgot: the rtk installer puts it
# there and verifies it, so moving it into the tree fights the installer instead of versioning anything.
# One pass tried and had to revert. The reason it looks like an oversight is that `~/.claude/CLAUDE.md` is
# a symlink into this repo, so the import resolves beside the symlink rather than beside its source. Only `CLAUDE.md`'s own imports resolve here. The scan above pools every
# budget file, and `sort -u` drops the carrier. An `inject.md` import loads from `~/.kk-flavor/`, so
# resolving one of those here would count whatever file happens to share the name.
# That set is read once, with the scan's own rules. Don't swap in a substring search: it sees the
# fenced and backticked mentions the scan skips on purpose. A name this ecosystem's prose merely
# *discusses* then passes as one `CLAUDE.md` imports. The search also costs a pass over the whole
# file per name. A committed `CLAUDE.md` naming tens of thousands of them ran this check past a
# review agent's timeout, killing it before it printed anything about the branch.
# Depth 1: a resolved file joins the budget after the scan ran, so an import nested inside one is
# neither counted nor named. Nothing imports at that depth today, and here is where a rescan would go.
# Each refusal below has a precedent above. Bare filenames only: the name comes out of a budget file,
# and `@../../.ssh/id_rsa` is the traversal this script has already had to close once. Resolution also
# needs this checkout to be the installed one. Otherwise a branch someone else wrote names files in
# the invoking user's real `~/.claude/` and folds their sizes into a number it also authored. The test
# for that is the flavor mount, and it needs both halves. `cd -P` follows a symlinked *directory*, so
# a branch committing `kk-flavor` as a symlink to the real install makes both sides agree and opens
# the gate. Refusing a symlinked `$root/kk-flavor` is what closes it. At the target, a symlink or a
# non-regular file is refused exactly as `contained_in_root` refuses one: a FIFO or a device would
# block the counting `cat` forever. Unreadable goes too, since `-f` passes on a mode-000 file and the
# read that then fails leaves the tier short, with nothing on stdout saying so.
import_mount_is_installed=0
if [ -n "${HOME:-}" ] && [ ! -L "$root/kk-flavor" ] && [ -n "$(canonical_dir "$root/kk-flavor")" ] &&
   [ "$(canonical_dir "${HOME}/.kk-flavor")" = "$(canonical_dir "$root/kk-flavor")" ]; then
  import_mount_is_installed=1
fi
claude_imports=""
if [ ! -L "$root/CLAUDE.md" ] && [ -f "$root/CLAUDE.md" ] && [ -r "$root/CLAUDE.md" ]; then
  claude_imports="$(imports_in "$root/CLAUDE.md")"
fi
import_newline='
'
# Sets `import_target` rather than printing it. A command substitution per name means a fork per name,
# the same attacker-scaled cost as the read it replaced. The membership test below runs in the shell.
import_target=""
# What the gate deliberately does not defend: on the installed checkout it trusts the tree, so a name in a
# budget file there reveals whether a file of that name exists under `~/.claude/` and roughly its size.
# Accepted, not overlooked. Both hardenings that would close it — demand a clean tree, or the tracking
# branch — refuse the normal case, because every quality pass runs dirty on a feature branch. Reaching it
# needs a real import line committed in `CLAUDE.md` itself.
# `import_refusal` carries a reason only for the shapes nothing legitimate produces. An import absent
# from the mount, or a checkout that isn't the installed one, is the ordinary case anywhere. Those
# stay quiet names in the note, and so does a plain subdirectory import, which is a legitimate form this
# resolver simply does not handle. A traversal, a symlink planted at the mount path, or a file present and
# deliberately unreadable are probes. This file's contract is that every refusal
# reports itself, instead of blending into the drift a healthy run also produces.
import_refusal=""
resolve_import_at_mount() {
  import_target=""
  import_refusal=""
  [ "$import_mount_is_installed" -eq 1 ] || return 1
  # Only a traversal earns a reported reason. `@dir/file.md` is a legitimate import form, so a plain
  # subdirectory name is refused here — this resolves bare names only — but quietly, as an uncounted
  # name. Reporting it would put a probe's finding on honest content and take the run to exit 1.
  case "$1" in
    '') return 1 ;;
    '~'*|/*|../*|*/../*|*/..) import_refusal="a traversal, not a bare filename"; return 1 ;;
    */*) return 1 ;;
  esac
  case "$import_newline$claude_imports$import_newline" in
    *"$import_newline$1$import_newline"*) ;;
    *) return 1 ;;
  esac
  [ -L "${HOME}/.claude/$1" ] && { import_refusal="a symlink at the mount"; return 1; }
  [ -f "${HOME}/.claude/$1" ] || return 1
  [ -r "${HOME}/.claude/$1" ] || { import_refusal="unreadable at the mount"; return 1; }
  import_target="${HOME}/.claude/$1"
}
# --- end shared:import-at-mount ---

# These two steps are where the scripts differ, so they stay out of the shared region below. This one
# sums words per file as it builds the budget, so a resolved import is counted the moment it joins. A
# refusal goes to stderr rather than a findings file, and does not count as a budget refusal: a
# probe-shaped name was never a member of the tier, so the figure is not short and no row is withheld.
import_resolved_words=0
add_import_to_budget() {
  local words
  words=$(words_in_file "$1")
  always_loaded_words=$((always_loaded_words + words))
  import_resolved_words=$((import_resolved_words + words))
  budget_files+=("$1")
}

report_import_refusal() {
  echo "stats.sh: import refused ($2), named but not counted: $(oneline "$1" | cut -c1-80)" >&2
}

# Resolved imports join the budget and its word count; the rest stay named in the note. Reassigning
# `budget_imports` to the leftovers keeps the capping region below byte-identical in both scripts.
# Attempts are capped, and past the cap every remaining name goes to the note instead. A committed
# file naming thousands of imports then costs a bounded number of stat calls, and what was skipped
# stays visible instead of dropping out of the figure silently.
# --- shared:import-resolution ---
# Leftover names accumulate in a file, never in a shell string. `s="$s$name"` re-copies everything
# gathered so far on every name, which is quadratic in a count the attacker picks: ~90k `@aNNN.md`
# tokens in a committed `CLAUDE.md` took 235s that way, enough to run this past a review agent's
# timeout so it reports nothing at all about the branch. Appending costs the same at any size.
budget_uncounted=""
budget_uncounted_file="$(mktemp)" || {
  echo "budget scan: mktemp gave no scratch file — exit 2, the import list cannot be bounded." >&2
  exit 2
}
if [ -n "$budget_imports" ]; then
  import_attempts=0
  while IFS= read -r budget_import; do
    [ -n "$budget_import" ] || continue
    # Both, and here rather than only inside the resolver: past the cap the resolver is not called, so
    # its own reset never runs and the last examined name's reason would be reported against a name
    # nothing looked at — a refusal claimed for a file that would have counted.
    import_target=""
    import_refusal=""
    if [ "$import_attempts" -lt 64 ]; then
      import_attempts=$((import_attempts + 1))
      resolve_import_at_mount "$budget_import"
    fi
    if [ -n "$import_target" ]; then
      add_import_to_budget "$import_target"
    else
      [ -n "$import_refusal" ] && report_import_refusal "$budget_import" "$import_refusal"
      printf '%s\n' "$budget_import" >>"$budget_uncounted_file"
    fi
  done <<EOF
$budget_imports
EOF
  # Read back once. The capping region below wants one newline-separated string with no trailing
  # newline, which is what the string form produced; `$(cat)` strips it the same way.
  budget_imports="$(cat "$budget_uncounted_file")"
fi
rm -f "$budget_uncounted_file"
# --- end shared:import-resolution ---

# Frontmatter is read exactly as kk-ecosystem's check.sh reads it, because that script reports this
# same budget and the two must never disagree: anchored to line 1 (a `---` rule in the body does not
# open frontmatter), closing fence matched with trailing space allowed, and only the first
# `description:`. Reading it any other way makes one of the two silently count body text.
# --- shared:frontmatter-description ---
frontmatter_description() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$1"
}
# --- end shared:frontmatter-description ---

# True when the skill opted out of model invocation, so its description never enters a context
# window and costs nothing until the human types `/<name>`.
# --- shared:opted-out-of-model-invocation ---
opted_out_of_model_invocation() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       tolower($0) ~ /^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$/ { found = 1; exit }
       END { exit !found }' "$1"
}
# --- end shared:opted-out-of-model-invocation ---

description_words=0
routed_skills=0
for skill in "$root"/skills/*/SKILL.md; do
  [ -f "$skill" ] || continue
  opted_out_of_model_invocation "$skill" && continue
  routed_skills=$((routed_skills + 1))
  description_words=$((description_words + $(frontmatter_description "$skill" | wc -w | tr -d ' ')))
done

[ "$prose" -gt 0 ] || {
  echo "stats.sh: measured 0 words of prose under $root — the scan did not work" >&2
  exit 2
}

# A refused budget file makes the always-loaded figure wrong by an unknown amount. Appending the row
# anyway writes a number a later pass compares against and cannot tell is short — the one failure
# the ledger exists to prevent — so refuse the whole run instead.
[ "$budget_refusals" -eq 0 ] || {
  echo "stats.sh: $budget_refusals budget file(s) refused above — exit 2, the always-loaded figure is short by an unknown amount and no row was appended." >&2
  exit 2
}

printf 'prose:        %6s words (%s .md files, ledger excluded)\n' "$prose" "$prose_files"
printf 'scripts:      %6s words\n' "$scripts"
printf 'ledger:       %6s words  (stats.md — a record, not instructions; step 1 reads it in full, so it costs context like the always-loaded tier)\n' "$ledger_words"
# A `+` on the ledger's figure, because that column is what a later pass compares against and a
# number that silently excludes an import teaches it the tier held still while it grew. The note
# goes to the printed line only — a newline or a `|` in the table cell would forge a row.
budget_mark=""
budget_note=""
if [ -n "$budget_imports" ]; then
  budget_mark="+"
  # Capped in bytes as well as entries, count always exact — check.sh caps identically and must print
  # the same figure. Ten crafted names alone reach a megabyte, so an entry cap on its own bounds
  # nothing while the printed "10" makes the volume look bounded.
# --- shared:import-cap ---
  import_count=$(printf '%s\n' "$budget_imports" | wc -l | tr -d ' ')
  import_names=$(printf '%s\n' "$budget_imports" | head -10 | cut -c1-60 | tr '\n' ' ' | cut -c1-200 |
    sed 's/ $//')
  [ "$import_count" -gt 10 ] && import_names="$import_names … and $((import_count - 10)) more"
# --- end shared:import-cap ---
  budget_note="  (+ $import_count uncounted import(s): $import_names)"
fi
# Skills mounted at `~/.claude/skills` from outside this tree — `kk-foreman` routes them as tool skills,
# so their descriptions cost the same tier and no pass here can shrink them. Reported apart for that
# reason. Skills the harness bundles or a plugin supplies load too and are not on disk at all, so this
# figure is a floor for what sits outside the tree, not the total.
# Only when this tree is the installed one, which `import_mount_is_installed` already decided. Anywhere
# else — a clone, or the worktree a PR review runs in — the mounts resolve to the *installed* checkout, so
# the exclusion below matches nothing and every mounted skill counts as outside: measured, 889 words across
# 21 skills instead of 43 across 2, which publishes the size of the reviewer's own local skill inventory
# into a figure an agent may quote in a comment. A number that means something else elsewhere is not a
# number to print elsewhere.
outside_words=0
outside_skills=0
[ "$import_mount_is_installed" -eq 1 ] && for mounted in "${HOME:-}"/.claude/skills/*/SKILL.md; do
  [ -f "$mounted" ] || continue
  case "$(canonical_dir "$(dirname "$mounted")")" in
    "$root_canon"/*) continue ;;
  esac
  opted_out_of_model_invocation "$mounted" && continue
  outside_skills=$((outside_skills + 1))
  outside_words=$((outside_words + $(frontmatter_description "$mounted" | wc -w | tr -d ' ')))
done
[ "$outside_skills" -eq 0 ] ||
  printf 'mounted outside:%4s words  (%s skill(s) this tree cannot shrink; bundled and plugin skills are unmeasurable)\n' \
    "$outside_words" "$outside_skills"
printf 'always-loaded:%6s words  = %s router + %s descriptions across %s of %s skills%s\n' \
  "$((always_loaded_words + description_words))" "$always_loaded_words" "$description_words" \
  "$routed_skills" "$skills" "$budget_note"

[ -n "$note" ] || exit 0

# The row states how much of its always-loaded figure came from imports this script resolved. Without
# that, a reader comparing two rows can't tell a tier that grew from one the scripts merely started
# seeing. Asking the caller for it in prose would put a number the script already holds into a note
# that gets written from memory sooner or later. Appended after the sanitising above, and safe there:
# fixed text and a digit string carry no newline or pipe to forge a row with.
[ "$import_resolved_words" -eq 0 ] ||
  note="$note [of the always-loaded figure, $import_resolved_words words are imports this run resolved]"

# Every write below is guarded. Unguarded, an unwritable history file still prints "appended to …"
# and exits 0, and the next pass reads a row that never landed as what happened.
history_dir=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd) || history_dir=""
[ -n "$history_dir" ] || {
  echo "stats.sh: could not resolve kk-reduce's own directory — the row was NOT appended." >&2
  exit 2
}
history="$history_dir/stats.md"
# Refused as a symlink on write, the way `contained_in_root` refuses it on read. Following one appends the
# row to whatever it points at — and a dangling one creates that file outright. The installed mount points
# into the human's own tree, so a branch checked out there reaches this the moment a campaign records.
[ -L "$history" ] && {
  echo "stats.sh: $(oneline "$history") is a symlink — exit 2, no row was appended." >&2
  exit 2
}
[ -f "$history" ] || {
  {
    # Header kept to the table and a pointer. Every line of prose here would be a second home for
    # what the real `stats.md` already says, hand-synced across a `.md`/`.sh` pair that the shared
    # region check cannot cover — it scans `*.sh` only. This block runs solely when the ledger has
    # been deleted, so the copy it would carry is the one nobody would ever reread.
    echo "# Ecosystem size"
    echo
    echo "Appended by \`scripts/stats.sh --append <note>\`, one row before a campaign and one after."
    echo "Not a per-pass log — git holds the history, this holds the measurements."
    echo "What the columns mean, and what a \`+\` on the always-loaded figure means: \`kk-foreman\`'s"
    echo "SKILL.md, step 1."
    echo
    echo "| date | prose | scripts | always-loaded | skills | what ran |"
    echo "|---|---|---|---|---|---|"
  } >"$history" || {
    echo "stats.sh: could not create $history — the row was NOT appended." >&2
    exit 2
  }
}
printf '| %s | %s | %s | %s | %s | %s |\n' \
  "$(date +%Y-%m-%d)" "$prose" "$scripts" "$((always_loaded_words + description_words))$budget_mark" \
  "$skills" "$note" >>"$history" || {
  echo "stats.sh: could not append to $history — the row was NOT recorded; the ledger still shows the previous pass." >&2
  exit 2
}
echo "appended to $history"
