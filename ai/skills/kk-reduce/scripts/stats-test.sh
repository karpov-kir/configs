#!/usr/bin/env bash
# Tests for stats.sh — the parts kk-ecosystem's check-test.sh cannot reach.
#   usage: stats-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# The two scripts share fenced regions that check.sh's wiring check keeps byte-identical, so those
# need no test here. What's untested is everything *around* them: each script has its own budget
# accumulator and its own consumer loop, and that is where they last disagreed. An unreadable file
# gave stats.sh an empty word count. That turned its arithmetic into a syntax error and printed a
# figure of 0 that `--append` would have written to the ledger as a measurement. So the first case is
# the invariant that matters: for one tree, both scripts report the same router figure.
#
# Self-contained rather than sharing helpers with check-test.sh. That file belongs to kk-ecosystem,
# and a test under one skill sourcing a file under another is the cross-skill dependency this tree
# refuses, for the same reason the shared regions are duplicated and drift-checked instead of sourced.
set -uo pipefail
export LC_ALL=C

here="$(cd "$(dirname "$0")" && pwd)"
stats="$here/stats.sh"
check="$here/../../kk-ecosystem/scripts/check.sh"
for required in "$stats" "$check"; do
  [ -x "$required" ] || { echo "stats-test: $required is not executable"; exit 1; }
done

base="$(mktemp -d)" || { echo "stats-test: mktemp gave no fixture dir — nothing was tested"; exit 1; }
# The chmod is for an interrupted run. Each case restores the mode it took away, so a completed run needs
# nothing; a run killed between the two chmods would otherwise leave a directory this trap cannot clear.
trap 'chmod -R u+rwX "$base" 2>/dev/null; rm -rf "$base"' EXIT
passed=0
failed=0
case_number=0
# The HOME both scripts run under, empty to inherit this shell's. A case sets it to make its fixture
# look like the installed checkout, which is the condition an `@import` resolves under.
check_home=""

# A fresh throwaway root holding the two directories both scripts require, plus enough prose that
# stats.sh does not exit 2 on "measured 0 words".
new_root() {
  case_number=$((case_number + 1))
  root="$base/r$case_number"
  mkdir -p "$root/kk-flavor/standards" "$root/skills"
  printf '# Flavor\n' >"$root/kk-flavor/inject.md"
  check_home=""
}

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1"
  shift
  printf '%s\n' "$@" | sed 's/^/          /'
}

# The router word figure each script reports for `$root`: check.sh's "N words across" and stats.sh's
# "= N router" are the same number by contract.
router_words_from_check() {
  HOME="${check_home:-$HOME}" "$check" "$root" 2>/dev/null |
    sed -n 's/^always-loaded: [0-9]* lines, \([0-9]*\) words across.*/\1/p' | head -1
}

router_words_from_stats() {
  HOME="${check_home:-$HOME}" "$stats" "$root" 2>/dev/null |
    sed -n 's/^always-loaded:.*= \([0-9]*\) router.*/\1/p' | head -1
}

assert_scripts_agree() {
  local from_check from_stats
  from_check="$(router_words_from_check)"
  from_stats="$(router_words_from_stats)"
  if [ -n "$from_check" ] && [ "$from_check" = "$from_stats" ]; then
    record_pass "$1 (both report $from_check)"
  else
    record_fail "$1" "check.sh router words: '$from_check'" "stats.sh router words: '$from_stats'"
  fi
}

echo "stats.sh — agreement with check.sh"

new_root
printf 'one two three four five\n' >"$root/CLAUDE.md"
assert_scripts_agree "agree on a plain budget"

# The glue defect: concatenating budget files joins the last word of a file with no final newline
# onto the first word of the next, one word short of what summing per file gives.
new_root
printf 'one two three' >"$root/CLAUDE.md"
assert_scripts_agree "agree when a budget file has no final newline"

new_root
printf '# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n' >"$root/kk-flavor/inject.md"
printf 'alpha beta gamma\n' >"$root/kk-flavor/standards/core.md"
printf 'one two\n' >"$root/CLAUDE.md"
assert_scripts_agree "agree with a Read-always target in the budget"

# A resolved `@import` is the one budget member the two scripts add differently: check.sh appends it to
# an array counted in one pass at the end, stats.sh sums its words on the spot. Nothing else here puts an
# import in the budget at all, so without this case both accumulators run untested on the member most
# likely to split them. The symlink makes the fixture look like the installed checkout, which is what
# opens the resolver.
new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
check_home="$root/home"
mkdir -p "$check_home/.claude"
ln -s "$root/kk-flavor" "$check_home/.kk-flavor"
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_scripts_agree "agree when an import resolves into the budget"

echo "stats.sh — a short figure never reaches the ledger"

# An unreadable budget file must be refused by both, so stats.sh exits 2 rather than print a figure
# it can't stand behind. Without the `-r` test in contained_in_root, check.sh drops the words while
# still counting the file, stats.sh prints 0, and neither records a refusal.
new_root
printf '# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n' >"$root/kk-flavor/inject.md"
printf 'alpha beta gamma\n' >"$root/kk-flavor/standards/core.md"
chmod 000 "$root/kk-flavor/standards/core.md"
stats_output="$("$stats" "$root" 2>&1)"
stats_status=$?
chmod 644 "$root/kk-flavor/standards/core.md"
if [ "$stats_status" = 2 ] && printf '%s' "$stats_output" | grep -qF 'budget file refused'; then
  record_pass "exits 2 and says why, on an unreadable budget file"
else
  record_fail "exits 2 and says why, on an unreadable budget file" "status: $stats_status" "$stats_output"
fi
# The refusal lives in a shared region, so check.sh must reach the same verdict on the same fixture.
# Asserted separately rather than by comparing figures: stats.sh prints no figure at all when it exits 2,
# so an agreement test here would compare a number against an empty string and pass for that reason.
chmod 000 "$root/kk-flavor/standards/core.md"
check_output="$("$check" "$root" 2>&1)"
chmod 644 "$root/kk-flavor/standards/core.md"
if printf '%s' "$check_output" | grep -qF 'budget file refused'; then
  record_pass "and check.sh refuses it too, rather than counting a file it cannot read"
else
  record_fail "and check.sh refuses it too, rather than counting a file it cannot read" "$check_output"
fi

echo "stats.sh — the ledger is measured apart from the instructions"

figure_from_stats() {
  "$stats" "$root" 2>/dev/null | sed -n "s/^$1: *\([0-9]*\) words.*/\1/p" | head -1
}

# The ledger is a record, so it must not move `prose`: adding one has to leave that figure untouched and
# show up on its own line instead. Counted together, a reduction raises `prose` by recording that it ran,
# which is the reading that decides whether the next reduction is owed.
new_root
printf 'one two three\n' >"$root/CLAUDE.md"
prose_without_ledger="$(figure_from_stats prose)"
mkdir -p "$root/skills/kk-reduce"
printf 'alpha beta gamma delta\n' >"$root/skills/kk-reduce/stats.md"
prose_with_ledger="$(figure_from_stats prose)"
ledger_reported="$(figure_from_stats ledger)"
if [ -n "$prose_without_ledger" ] && [ "$prose_with_ledger" = "$prose_without_ledger" ] &&
   [ "$ledger_reported" = 4 ]; then
  record_pass "a ledger leaves prose unchanged and reports its own $ledger_reported words"
else
  record_fail "a ledger leaves prose unchanged and reports its own words" \
    "prose without it: '$prose_without_ledger'" "prose with it: '$prose_with_ledger'" \
    "ledger reported: '$ledger_reported' (want 4)"
fi

# The ledger figure is subtracted from `prose`, and `prose` is measured with `find -type f`, which does
# not walk a symlink. A `-f` test does follow one, so a symlink at the ledger path takes words out of a
# total that never held them: here prose reads 3 for a tree holding 5. Point it at a large enough file
# and prose goes negative, which stats.sh reports as "the scan did not work".
new_root
printf 'one two three\n' >"$root/CLAUDE.md"
prose_without_linked_ledger="$(figure_from_stats prose)"
mkdir -p "$root/skills/kk-reduce"
printf 'outside words\n' >"$base/outside.md"
ln -s "$base/outside.md" "$root/skills/kk-reduce/stats.md"
prose_with_linked_ledger="$(figure_from_stats prose)"
ledger_linked="$(figure_from_stats ledger)"
if [ -n "$prose_without_linked_ledger" ] &&
   [ "$prose_with_linked_ledger" = "$prose_without_linked_ledger" ] && [ "$ledger_linked" = 0 ]; then
  record_pass "a symlink at the ledger path takes nothing out of prose"
else
  record_fail "a symlink at the ledger path takes nothing out of prose" \
    "prose without it: '$prose_without_linked_ledger'" "prose with it: '$prose_with_linked_ledger'" \
    "ledger reported: '$ledger_linked' (want 0)"
fi

echo "stats.sh — a probe-shaped import is reported, not hidden"

# check-test.sh covers this side of check.sh; nothing covered stats.sh's own reporting hook. A name that
# is a path rather than a bare filename is a shape no ordinary machine produces, so it must say so on
# stderr rather than blending into the uncounted names a healthy run also prints. The mount link opens
# the gate, and the file is planted where the traversal would land so the case cannot pass on absence.
new_root
mkdir -p "$root/home/.claude"
ln -s "$root/kk-flavor" "$root/home/.kk-flavor"
printf '# Root\n\n@../../escape.md\n' >"$root/CLAUDE.md"
printf 'secret words here\n' >"$root/escape.md"
probe_output="$(HOME="$root/home" "$stats" "$root" 2>&1)"
if printf '%s' "$probe_output" | grep -qF 'import refused'; then
  record_pass "reports a path-shaped import name instead of leaving it to read as drift"
else
  record_fail "reports a path-shaped import name instead of leaving it to read as drift" "$probe_output"
fi

echo "stats.sh — a skill mounted from outside the tree is reported apart"

# kk-foreman routes those as tool skills, so their descriptions cost the always-loaded tier while no pass
# here can shrink them. The mount has to resolve outside `$root` or the exclusion swallows it, which is why
# the fixture skill lives beside the root rather than under it. The `.kk-flavor` link is what makes this
# fixture read as the installed checkout, which is the only place the figure means anything.
new_root
printf 'one two\n' >"$root/CLAUDE.md"
mkdir -p "$base/outside-skill" "$root/home/.claude/skills"
printf -- '---\nname: outside-skill\ndescription: four words exactly here\n---\n' >"$base/outside-skill/SKILL.md"
ln -s "$base/outside-skill" "$root/home/.claude/skills/outside-skill"
# A second mount, this one resolving *inside* the root: it is one of the tree's own skills and must not be
# counted, or the figure says nothing. Without it the exclusion is untested — deleting all three of its
# lines left this suite green, because the only mount here already sat outside.
mkdir -p "$root/skills/inside-skill"
printf -- '---\nname: inside-skill\ndescription: four words exactly here\n---\n' >"$root/skills/inside-skill/SKILL.md"
ln -s "$root/skills/inside-skill" "$root/home/.claude/skills/inside-skill"
ln -s "$root/kk-flavor" "$root/home/.kk-flavor"
outside_line="$(HOME="$root/home" "$stats" "$root" 2>/dev/null | sed -n 's/^mounted outside: *\([0-9]*\) words.*/\1/p')"
if [ "$outside_line" = 4 ]; then
  record_pass "counts a mounted-outside description apart from the tree's own"
else
  record_fail "counts a mounted-outside description apart from the tree's own" \
    "reported: '$outside_line' (want 4)"
fi

# The same fixture minus the flavor mount is any relocated checkout — a clone, or the worktree a PR review
# runs in. There the mounts resolve to the *installed* tree, the exclusion matches nothing, and every mounted
# skill would count as outside: 889 words across 21 measured, instead of 43 across 2. The figure describes
# someone else's machine at that point, so it is not printed at all.
new_root
printf 'one two\n' >"$root/CLAUDE.md"
mkdir -p "$base/outside-skill-2" "$root/home/.claude/skills"
printf -- '---\nname: outside-skill-2\ndescription: four words exactly here\n---\n' >"$base/outside-skill-2/SKILL.md"
ln -s "$base/outside-skill-2" "$root/home/.claude/skills/outside-skill-2"
relocated_line="$(HOME="$root/home" "$stats" "$root" 2>/dev/null | sed -n 's/^mounted outside: *\([0-9]*\) words.*/\1/p')"
if [ -z "$relocated_line" ]; then
  record_pass "says nothing about mounted skills when this tree is not the installed one"
else
  record_fail "says nothing about mounted skills when this tree is not the installed one" \
    "reported: '$relocated_line' (want no line at all)"
fi

echo "stats.sh — an over-long note is refused rather than appended"

# The bar exists because prose asking for a short note was ignored by every author of a long row, the
# rule's own writer included. Refusing is the only form that holds: no row beats a row nobody will read.
new_root
mkdir -p "$root/skills/kk-reduce/scripts"
cp "$stats" "$root/skills/kk-reduce/scripts/stats.sh"
printf '| date |\n' >"$root/skills/kk-reduce/stats.md"
printf 'one two\n' >"$root/CLAUDE.md"
long_note="$(awk 'BEGIN { for (i = 1; i <= 60; i++) printf "word%d ", i }')"
"$root/skills/kk-reduce/scripts/stats.sh" --append "$long_note" "$root" >/dev/null 2>&1
rows_after_long=$(grep -c '^|' "$root/skills/kk-reduce/stats.md")
"$root/skills/kk-reduce/scripts/stats.sh" --append 'short enough to keep' "$root" >/dev/null 2>&1
rows_after_short=$(grep -c '^|' "$root/skills/kk-reduce/stats.md")
if [ "$rows_after_long" = 1 ] && [ "$rows_after_short" = 2 ]; then
  record_pass "a 60-word note appends nothing, a short one appends one row"
else
  record_fail "a 60-word note appends nothing, a short one appends one row" \
    "rows after the long note: $rows_after_long (want 1)" \
    "rows after the short one: $rows_after_short (want 2)"
fi

echo "stats.sh — the note cannot forge a ledger row"

# --append writes to ../stats.md relative to the script, so a test must never run the real one.
# This copies stats.sh into a fixture that has its own stats.md.
new_root
mkdir -p "$root/skills/kk-reduce/scripts"
cp "$stats" "$root/skills/kk-reduce/scripts/stats.sh"
printf '| date | prose | scripts | always-loaded | skills | what ran |\n|---|---|---|---|---|---|\n' \
  >"$root/skills/kk-reduce/stats.md"
printf 'one two\n' >"$root/CLAUDE.md"
rows_before=$(grep -c '^|' "$root/skills/kk-reduce/stats.md")
"$root/skills/kk-reduce/scripts/stats.sh" --append 'first line
second line | with a pipe' "$root" >/dev/null 2>&1
rows_after=$(grep -c '^|' "$root/skills/kk-reduce/stats.md")
appended=$((rows_after - rows_before))
row="$(tail -1 "$root/skills/kk-reduce/stats.md")"
# Escaped pipes are dropped before counting: `\|` renders as a literal inside the cell and separates no
# column, so counting raw `|` characters marks the guard working as the guard failing.
columns=$(printf '%s' "$row" | sed 's/\\|//g' | tr -cd '|' | wc -c | tr -d ' ')
if [ "$appended" = 1 ] && [ "$columns" = 7 ]; then
  record_pass "a note carrying a newline and a pipe still writes exactly one 6-column row"
else
  record_fail "a note carrying a newline and a pipe still writes exactly one 6-column row" \
    "rows appended: $appended (want 1)" "unescaped pipes: $columns (want 7)" "$row"
fi

echo "stats.sh — the ledger is not written through a symlink"

# It is refused as a symlink when read, via contained_in_root, and was followed when written: appending the
# row to whatever it pointed at, or creating the target outright when dangling. The installed mount points
# into the human's own tree, so a branch checked out there reaches this the moment a campaign records.
new_root
printf 'one two\n' >"$root/CLAUDE.md"
mkdir -p "$root/skills/kk-reduce/scripts"
cp "$stats" "$root/skills/kk-reduce/scripts/stats.sh"
printf 'untouched\n' >"$base/decoy-target.md"
ln -s "$base/decoy-target.md" "$root/skills/kk-reduce/stats.md"
"$root/skills/kk-reduce/scripts/stats.sh" --append 'should not land' "$root" >/dev/null 2>&1
if [ "$(cat "$base/decoy-target.md")" = untouched ]; then
  record_pass "refuses to append through a symlinked ledger"
else
  record_fail "refuses to append through a symlinked ledger" \
    "the decoy was written: $(cat "$base/decoy-target.md")"
fi

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
