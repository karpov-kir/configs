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

prose=$(words_in_tree "$root" '*.md')
prose_files=$(find "$root" -name '*.md' -type f | wc -l | tr -d ' ')
scripts=$(words_in_tree "$root" '*.sh')
skills=$(find "$root/skills" -maxdepth 2 -name SKILL.md -type f | wc -l | tr -d ' ')

# The always-loaded tier, in two parts: the router's own "Read always" targets, and every skill
# description — which the harness keeps in context for any skill without disable-model-invocation.
# CLAUDE.md is seeded first: it is symlinked to ~/.claude/CLAUDE.md, sits in every system prompt, and
# is what loads inject.md at all. check.sh counts it too, and the two must report the same number.
# Read the target list a line at a time — a `for` over unquoted `$(…)` splits a path containing a
# space into fragments that each fail the -f test, dropping the file from the budget in silence.
# Links are extracted with check.sh's own expression, which puts every one on its own line: a `sed`
# taking the last `(…)` per line is greedy, so a bullet naming two docs yields only the second and
# the two scripts report different budgets for the same tree.
inject="$root/kk-flavor/inject.md"
always_loaded_words=0
[ -f "$root/CLAUDE.md" ] && always_loaded_words=$(words_in_file "$root/CLAUDE.md")
if [ -f "$inject" ]; then
  while IFS= read -r target; do
    [ -n "$target" ] || continue
    file="$root/kk-flavor/$target"
    if [ -f "$file" ]; then
      always_loaded_words=$((always_loaded_words + $(words_in_file "$file")))
    else
      echo "stats.sh: inject.md lists '$target' under Read always, but $file does not exist" >&2
    fi
  done <<EOF
$(awk '/^## Read always/{f=1;next} /^## /{f=0} f' "$inject" | grep -oE '\]\([^)#]+\)' | sed 's/^](//; s/)$//')
EOF
  always_loaded_words=$((always_loaded_words + $(words_in_file "$inject")))
fi

description_words=0
routed_skills=0
# Frontmatter is read exactly as kk-ecosystem's check.sh reads it, because that script reports this
# same budget and the two must never disagree: anchored to line 1 (a `---` rule in the body does not
# open frontmatter), closing fence matched with trailing space allowed, and only the first
# `description:`. Reading it any other way makes one of the two silently count body text.
# Copied rather than shared on purpose, and re-examined each pass. Two sites is inside the
# tolerance, and the alternative is a script in one skill depending on a script in another. Each
# side would then have to guard for the other's absence, to report the number this copy gets free.
# Each predicate is one awk, never `awk | grep -q`: grep -q exits on the first match, and under
# `pipefail` awk's resulting SIGPIPE (141) turns a match into a miss.
for skill in "$root"/skills/*/SKILL.md; do
  [ -f "$skill" ] || continue
  # A skill opted out of model invocation costs no context until it is typed.
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       tolower($0) ~ /^disable-model-invocation:[[:space:]]*(true|yes|on|1)[[:space:]]*$/ { found = 1; exit }
       END { exit !found }' "$skill" && continue
  routed_skills=$((routed_skills + 1))
  description_words=$((description_words + $(awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$skill" | wc -w | tr -d ' ')))
done

[ "$prose" -gt 0 ] || {
  echo "stats.sh: measured 0 words of prose under $root — the scan did not work" >&2
  exit 2
}

printf 'prose:        %6s words (%s .md files)\n' "$prose" "$prose_files"
printf 'scripts:      %6s words\n' "$scripts"
printf 'always-loaded:%6s words  = %s router + %s descriptions across %s of %s skills\n' \
  "$((always_loaded_words + description_words))" "$always_loaded_words" "$description_words" \
  "$routed_skills" "$skills"

[ -n "$note" ] || exit 0

# Every write below is guarded. The ledger exists so a later pass reads what happened instead of
# guessing it, which a row that did not land but reported as one would defeat. Unguarded, an
# unwritable history file still prints "appended to …" and exits 0.
history_dir=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd) || history_dir=""
[ -n "$history_dir" ] || {
  echo "stats.sh: could not resolve kk-foreman's own directory — the row was NOT appended." >&2
  exit 2
}
history="$history_dir/history.md"
[ -f "$history" ] || {
  {
    echo "# Ecosystem size history"
    echo
    echo "Appended by \`scripts/stats.sh --append <note>\`. One row per pass that changed the tree."
    echo "Read it to answer *when did this last shrink, and has it grown since* — the question that"
    echo "decides whether a scoped \`kk-ecosystem\` pass is enough or a \`kk-reduce\` campaign is owed."
    echo
    echo "| date | prose | scripts | always-loaded | skills | what ran |"
    echo "|---|---|---|---|---|---|"
  } >"$history" || {
    echo "stats.sh: could not create $history — the row was NOT appended." >&2
    exit 2
  }
}
printf '| %s | %s | %s | %s | %s | %s |\n' \
  "$(date +%Y-%m-%d)" "$prose" "$scripts" "$((always_loaded_words + description_words))" \
  "$skills" "$note" >>"$history" || {
  echo "stats.sh: could not append to $history — the row was NOT recorded; the ledger still shows the previous pass." >&2
  exit 2
}
echo "appended to $history"
