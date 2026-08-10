#!/usr/bin/env bash
# Ecosystem size ledger — the numbers that decide whether a pass is worth running, measured rather
# than estimated. Owned by kk-foreman; it appends to ../history.md, relative to this script's dir.
#   usage: stats.sh [<root>]                    print the current measurements
#          stats.sh --append "<note>" [<root>]  print them and append a dated row to history.md
# The note is one argument — quote it, or its first word is read as <root>.
# <root> holds kk-flavor/ and skills/; defaults to . then ./ai, matching check.sh.
# Exits 0 on success, 2 when it could not measure — a measurement that did not run is not a zero.
set -uo pipefail
export LC_ALL=C

note=""
case "${1:-}" in
  --append)
    shift
    note="${1:-upkeep}"
    shift 2>/dev/null || true
    # history.md is a markdown table and the note is its last cell, so the note must not be able to
    # end the row. A newline in it writes a second line that reads as a row of its own, date and
    # counts included. A bare `|` splits the cell into extra columns. Either way a later pass reads
    # fabricated numbers as a measurement and skips a campaign that is owed.
    note=${note//[$'\n\r']/ }
    # Backslashes before pipes, never the other way round. Reversed, a note already carrying `\|`
    # comes out `\\|`: an escaped backslash followed by a live pipe.
    note=${note//\\/\\\\}
    note=${note//|/\\|}
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
# verbatim, for the same reason: a `## Read always` target is attacker-authored wherever the tree
# under review is someone else's, and `../../../` there otherwise pulls a file the invoking user can
# read into the budget — and, since the import scan below prints matched substrings, into whatever
# reads this output. Both copies are fenced as a shared region, so check.sh fails the wiring check
# when they drift.
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
  dir="$(canonical_dir "$(dirname "$1")")"
  [ -n "$root_canon" ] && [ -n "$dir" ] || return 1
  [ "$dir" = "$root_canon" ] || [ "${dir#"$root_canon"/}" != "$dir" ]
}
# --- end shared:contained-in-root ---
# The refused file's content never reaches the message, but its name is attacker-chosen where the
# tree is not the user's own, so the name is truncated — check.sh bounds the same string for the same
# reason. A refusal makes the always-loaded figure wrong, so it is counted and acted on below.
budget_refusals=0
refuse_budget_file() {
  budget_refusals=$((budget_refusals + 1))
  echo "stats.sh: budget file refused (symlink, or resolves outside $root) — not read, not counted: $(printf '%s' "$1" | cut -c1-80)" >&2
}

prose=$(words_in_tree "$root" '*.md')
prose_files=$(find "$root" -name '*.md' -type f | wc -l | tr -d ' ')
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

# An `@path` import inside a budget file loads with it and is not counted here — the reasons, and
# the detection down to the field-by-field scan and its two bounds, are check.sh's, which reports
# this same tier and must not disagree.
# The array test is required: under `set -u`, bash 3.2 errors on "${arr[@]}" when arr is empty, and
# a repo with no CLAUDE.md and no inject.md reaches this line with exactly that.
budget_imports=""
if [ ${#budget_files[@]} -gt 0 ]; then
# --- shared:import-scan ---
  budget_imports=$(awk 'FNR == 1 { fence = 0 }
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
                          } }' "${budget_files[@]}" | sort -u)
# --- end shared:import-scan ---
fi

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

printf 'prose:        %6s words (%s .md files)\n' "$prose" "$prose_files"
printf 'scripts:      %6s words\n' "$scripts"
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
printf 'always-loaded:%6s words  = %s router + %s descriptions across %s of %s skills%s\n' \
  "$((always_loaded_words + description_words))" "$always_loaded_words" "$description_words" \
  "$routed_skills" "$skills" "$budget_note"

[ -n "$note" ] || exit 0

# Every write below is guarded. Unguarded, an unwritable history file still prints "appended to …"
# and exits 0, and the next pass reads a row that never landed as what happened.
history_dir=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd) || history_dir=""
[ -n "$history_dir" ] || {
  echo "stats.sh: could not resolve kk-foreman's own directory — the row was NOT appended." >&2
  exit 2
}
history="$history_dir/history.md"
[ -f "$history" ] || {
  {
    # Header kept to the table and a pointer. Every line of prose here would be a second home for
    # what the real `history.md` already says, hand-synced across a `.md`/`.sh` pair that the shared
    # region check cannot cover — it scans `*.sh` only. This block runs solely when the ledger has
    # been deleted, so the copy it would carry is the one nobody would ever reread.
    echo "# Ecosystem size history"
    echo
    echo "Appended by \`scripts/stats.sh --append <note>\`. One row per pass that changed the tree."
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
