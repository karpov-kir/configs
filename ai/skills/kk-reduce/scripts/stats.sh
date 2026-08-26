#!/usr/bin/env bash
# Ecosystem size ledger — the numbers that decide whether a pass is worth running, measured rather
# than estimated. Owned by kk-reduce; it appends to ../stats.md, relative to this script's dir.
#   usage: stats.sh [<root>]                    print the current measurements
#          stats.sh --append "<note>" [<root>]  print them and append a dated row to stats.md
# The note is one argument — quote it, or its first word is read as <root>.
# <root> holds kk-flavor/ and skills/; defaults to . then ./ai, matching check.sh.
# Exits 0 on success, 2 when it could not measure — a measurement that did not run is not a zero.
# A change here needs a case in stats-test.sh beside it, and stats-mutate.sh is what shows that case
# can fail.
set -uo pipefail
export LC_ALL=C

note=""
case "${1:-}" in
  --append)
    shift
    note="${1:-upkeep}"
    shift 2>/dev/null || true
    # The note is stats.md's last table cell: a newline in it forges a whole row, a bare `|` forges
    # extra columns, and a later pass reads either as a measurement.
    note=${note//[$'\n\r']/ }
    # Backslashes before pipes, never the other way round.
    note=${note//\\/\\\\}
    note=${note//|/\\|}
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

# Anything attacker-chosen that reaches a message goes through this first. Every control byte, not only
# the two that end a line: an ESC sequence among them rewrites whatever read this.
oneline() {
  printf '%s' "$1" | LC_ALL=C tr '[:cntrl:]' ' '
}

# Words in one file. The `tr` drops the padding BSD `wc` writes.
words_in_file() {
  wc -w <"$1" | tr -d ' '
}

# The directory a real path resolves to, symlinks followed.
# --- shared:canonical-dir ---
canonical_dir() {
  [ -d "$1" ] || return 0
  (cd -P -- "$1" 2>/dev/null && pwd -P)
}
# --- end shared:canonical-dir ---

# True when a path's directory sits at or under the root; `check.sh` owns the reasoning.
# --- shared:contained-in-root ---
root_canon="$(canonical_dir "$root")"
contained_in_root() {
  local dir
  [ -L "$1" ] && return 1
  # A regular file, or nothing. `-e` alone admits a FIFO or a device, which `cat` then blocks on
  # forever; a dangling symlink fails `-e` entirely, so callers enter on `-e || -L` and both become
  # a refusal, not a silent drop.
  [ -f "$1" ] || return 1
  # Readable too, not just regular: `-f` passes on a mode-000 file, and the read behind the figure
  # then fails, leaving a file counted whose words are not.
  [ -r "$1" ] || return 1
  dir="$(canonical_dir "$(dirname "$1")")"
  [ -n "$root_canon" ] && [ -n "$dir" ] || return 1
  [ "$dir" = "$root_canon" ] || [ "${dir#"$root_canon"/}" != "$dir" ]
}
# --- end shared:contained-in-root ---
# The refused file's name is attacker-chosen, so it is bounded in length and in count. The count stays
# exact, and it alone decides whether a row is withheld.
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

# The ledger is a record, not an instruction, and is reported on its own line rather than inside
# `prose`: counted together, the number that decides whether a reduction is owed rises every time a
# reduction records that it ran.
ledger="$root/skills/kk-reduce/stats.md"
ledger_words=0
# Guarded like a budget file, not with a bare `-f`: `prose` is measured with `find -type f`, which does
# not walk a symlink, while `-f` follows one — so a symlink here subtracts words the total never held.
if contained_in_root "$ledger"; then
  ledger_words=$(words_in_file "$ledger")
  prose=$((prose - ledger_words))
  prose_files=$((prose_files - 1))
fi
scripts=$(words_in_tree "$root" '*.sh')
skills=$(find "$root/skills" -maxdepth 2 -name SKILL.md -type f | wc -l | tr -d ' ')

# The always-loaded tier, in two parts: the router's own "Read always" targets, and every skill
# description the harness keeps in context. `check.sh` counts the same tier and the two must agree, so
# the target list is read a line at a time and the links are extracted with its own expression.
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
# `-e`, not `-f`: a symlink to a FIFO fails `-f` and would fall through both branches, dropping
# inject.md and every target it lists from the budget in silence.
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
# counted into this tier below. `check.sh` owns the reasoning; both scripts report this tier and must
# not disagree.
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
# The array test is required: under `set -u`, bash 3.2 errors on "${arr[@]}" when arr is empty.
budget_imports=""
if [ ${#budget_files[@]} -gt 0 ]; then
  budget_imports="$(imports_in "${budget_files[@]}")"
fi

# --- shared:import-at-mount ---
# An import loads from beside the *installed* copy of the file carrying it, so `@RTK.md` in `CLAUDE.md`
# is `~/.claude/RTK.md`. That file is **not** one this repo forgot: the rtk installer puts it there and
# verifies it, so moving it into the tree fights the installer.
# Only `CLAUDE.md`'s own imports resolve here — an `inject.md` import loads from `~/.kk-flavor/`, so
# resolving one here would count whatever file shares the name. Don't swap the scan for a substring
# search: it sees the fenced and backticked mentions the scan skips on purpose.
# Depth 1: an import nested inside a resolved file is neither counted nor named.
# Bare filenames only — `@../../.ssh/id_rsa` must not resolve.
# Resolution also needs this checkout to be the installed one, or a branch someone else wrote names
# files in the invoking user's real `~/.claude/` and folds their sizes into a number it also authored.
# `cd -P` follows a symlinked *directory*, so refusing a symlinked `$root/kk-flavor` is what stops a
# branch committing one to the real install and opening that gate.
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
# Sets `import_target` rather than printing it: a command substitution per name is a fork per name.
import_target=""
# `import_refusal` carries a reason only for the shapes nothing legitimate produces — a traversal, a
# symlink planted at the mount path, a file present and deliberately unreadable. An import simply absent
# from the mount, a checkout that isn't the installed one, and a subdirectory import this resolver does
# not handle are the ordinary cases: they stay quiet names in the note.
import_refusal=""
resolve_import_at_mount() {
  import_target=""
  import_refusal=""
  [ "$import_mount_is_installed" -eq 1 ] || return 1
  # `@dir/file.md` is a legitimate import form, so a plain subdirectory name is refused here — this
  # resolves bare names only — but quietly: reporting it would take an honest run to exit 1.
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

# An import refusal does not count as a budget refusal: a probe-shaped name was never a member of the
# tier, so the figure is not short and no row is withheld.
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

# Resolved imports join the budget and its word count; the rest stay named in the note. Attempts are
# capped, and past the cap every remaining name goes to the note rather than dropping out silently.
# --- shared:import-resolution ---
# Leftover names accumulate in a file, never in a shell string: `s="$s$name"` re-copies everything
# gathered so far on every name, which is quadratic in a count the attacker picks.
budget_uncounted_file="$(mktemp)" || {
  echo "budget scan: mktemp gave no scratch file — exit 2, the import list cannot be bounded." >&2
  exit 2
}
if [ -n "$budget_imports" ]; then
  import_attempts=0
  while IFS= read -r budget_import; do
    [ -n "$budget_import" ] || continue
    # Reset here, not only inside the resolver: past the cap the resolver is not called, so its own
    # reset never runs and the last examined name's reason would be reported against a name nothing
    # looked at.
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
  # The capping region below wants one newline-separated string with no trailing newline.
  budget_imports="$(cat "$budget_uncounted_file")"
fi
rm -f "$budget_uncounted_file"
# --- end shared:import-resolution ---

# Frontmatter is read exactly as `check.sh` reads it, because that script reports this same budget and
# the two must never disagree.
# --- shared:frontmatter-description ---
frontmatter_description() {
  awk 'NR == 1 && !/^---[[:space:]]*$/ { exit }
       NR > 1 && /^---[[:space:]]*$/ { exit }
       /^description:[[:space:]]*/ { sub(/^description:[[:space:]]*/, ""); print; exit }' "$1"
}
# --- end shared:frontmatter-description ---

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

[ "$budget_refusals" -eq 0 ] || {
  echo "stats.sh: $budget_refusals budget file(s) refused above — exit 2, the always-loaded figure is short by an unknown amount and no row was appended." >&2
  exit 2
}

printf 'prose:        %6s words (%s .md files, ledger excluded)\n' "$prose" "$prose_files"
printf 'scripts:      %6s words\n' "$scripts"
printf 'ledger:       %6s words  (stats.md — a record, not instructions; step 1 reads it in full, so it costs context like the always-loaded tier)\n' "$ledger_words"
# A `+` on the ledger's figure marks it a lower bound: a number that silently excludes an import
# teaches a later pass the tier held still while it grew. The note goes to the printed line only.
budget_mark=""
budget_note=""
if [ -n "$budget_imports" ]; then
  budget_mark="+"
  # Capped in bytes as well as entries, count always exact — check.sh caps identically and must print
  # the same figure.
# --- shared:import-cap ---
  import_count=$(printf '%s\n' "$budget_imports" | wc -l | tr -d ' ')
  import_names=$(printf '%s\n' "$budget_imports" | head -10 | cut -c1-60 | tr '\n' ' ' | cut -c1-200 |
    sed 's/ $//')
  [ "$import_count" -gt 10 ] && import_names="$import_names … and $((import_count - 10)) more"
# --- end shared:import-cap ---
  budget_note="  (+ $import_count uncounted import(s): $import_names)"
fi
# Skills mounted at `~/.claude/skills` from outside this tree cost the same tier and no pass here can
# shrink them, so they are reported apart.
# Only when this tree is the installed one: anywhere else — a clone, or a PR review's worktree — the
# mounts resolve to the *installed* checkout, the exclusion below matches nothing, and the figure
# publishes the reviewer's own local skill inventory into something an agent may quote.
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

# The row states how much of its always-loaded figure came from imports this script resolved, or a
# reader comparing two rows cannot tell a tier that grew from one the scripts merely started seeing.
# Appended after the sanitising above, and safe there: fixed text and a digit string forge no row.
[ "$import_resolved_words" -eq 0 ] ||
  note="$note [of the always-loaded figure, $import_resolved_words words are imports this run resolved]"

# Every write below is guarded: unguarded, an unwritable history file still prints "appended to …"
# and exits 0, and the next pass reads a row that never landed as what happened.
history_dir=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd) || history_dir=""
[ -n "$history_dir" ] || {
  echo "stats.sh: could not resolve kk-reduce's own directory — the row was NOT appended." >&2
  exit 2
}
history="$history_dir/stats.md"
# Refused as a symlink on write, the way `contained_in_root` refuses it on read: following one appends
# the row to whatever it points at, and a dangling one creates that file outright.
[ -L "$history" ] && {
  echo "stats.sh: $(oneline "$history") is a symlink — exit 2, no row was appended." >&2
  exit 2
}
[ -f "$history" ] || {
  {
    # `stats.md` owns these rules — `kk-reduce`'s SKILL.md says so, and its reader arrives at the file,
    # not at the skill — so a fresh one has to carry them or it begins life with none of the protection
    # the ledger exists to have. This block had already drifted out of all three: no `, start`, no
    # campaign-cut-versus-drift, no never-edited absolute. It is a `.md`/`.sh` pair the shared-region
    # scan cannot cover, so `stats-test.sh` compares this output against the live file instead.
    echo "# Ecosystem size"
    echo
    echo "Appended by \`kk-reduce\` alone, via \`scripts/stats.sh --append <note>\` beside it: one row before a"
    echo "campaign, whose note ends \`, start\`, and one after. **A delta across that pair is the campaign's own"
    echo "cut, not drift** — drift is measured from a closing row forward."
    echo
    echo "\`kk-reduce\`'s own SKILL.md defines what each row's note carries. **A column is a measurement and is"
    echo "never edited — however that edit is authorised**"
    echo "(\`~/.kk-flavor/standards/skill-protocol.md\` → **Caller**): every delta is read off the rows below it,"
    echo "so one corrected figure silently restates every campaign since."
    echo
    echo "**A \`+\` on a row's always-loaded figure makes it a lower bound**: \`stats.sh\` named an \`@import\` it"
    echo "could not resolve and left it uncounted. Read the delta between two marked rows as \"at least this"
    echo "much\". From a marked row to an unmarked one, part of the rise is \`stats.sh\` resolving more rather"
    echo "than the tree growing. The unmarked row's note says how much."
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
