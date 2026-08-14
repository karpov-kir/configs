#!/usr/bin/env bash
# Tests for stats.sh — the parts kk-ecosystem's check-test.sh cannot reach.
#   usage: stats-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# A change to stats.sh needs a case here, and `stats-mutate.sh` is what proves a case can fail.
# The agreement cases at the top hold the invariant that matters: for one tree, both scripts report
# the same router figure.
# This file is self-contained rather than sourcing check-test.sh — check.sh's note on the shared
# regions says why.
set -uo pipefail
export LC_ALL=C

here="$(cd "$(dirname "$0")" && pwd)"
stats="$here/stats.sh"
check="$here/../../kk-ecosystem/scripts/check.sh"
for required in "$stats" "$check"; do
  [ -x "$required" ] || { echo "stats-test: $required is not executable"; exit 1; }
done

base="$(mktemp -d)" || { echo "stats-test: mktemp gave no fixture dir — nothing was tested"; exit 1; }
# The chmod is for a run killed between a case's two chmods; a completed run restores every mode.
trap 'chmod -R u+rwX "$base" 2>/dev/null; rm -rf "$base"' EXIT
passed=0
failed=0
case_number=0
check_home=""

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

new_root
printf 'one two three' >"$root/CLAUDE.md"
assert_scripts_agree "agree when a budget file has no final newline"

new_root
printf '# Flavor\n\n## Read always\n\n- [core](standards/core.md)\n' >"$root/kk-flavor/inject.md"
printf 'alpha beta gamma\n' >"$root/kk-flavor/standards/core.md"
printf 'one two\n' >"$root/CLAUDE.md"
assert_scripts_agree "agree with a Read-always target in the budget"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
check_home="$root/home"
mkdir -p "$check_home/.claude"
ln -s "$root/kk-flavor" "$check_home/.kk-flavor"
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_scripts_agree "agree when an import resolves into the budget"

echo "stats.sh — a short figure never reaches the ledger"

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
# This can't be an agreement test: stats.sh prints no figure at all when it exits 2, so the comparison
# would put check.sh's number against an empty string and go red whatever check.sh did — saying nothing
# about the refusal asserted here.
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

# The file sits where the traversal lands, so a resolver with the guard gone would find it. This case
# asserts on the reported refusal alone, so it would pass with the file absent too; check-test.sh's
# twin of this fixture is the one that also asserts the name stayed uncounted.
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

# The fixture skill lives beside the root, not under it: a mount resolving inside `$root` is excluded.
new_root
printf 'one two\n' >"$root/CLAUDE.md"
mkdir -p "$base/outside-skill" "$root/home/.claude/skills"
printf -- '---\nname: outside-skill\ndescription: four words exactly here\n---\n' >"$base/outside-skill/SKILL.md"
ln -s "$base/outside-skill" "$root/home/.claude/skills/outside-skill"
# A second mount, this one resolving *inside* the root: the tree's own skill must not count. The
# suite stays green if you delete it, which is exactly why it looks removable — take it out and
# nothing here tests the exclusion.
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

# `--append` writes to ../stats.md relative to the script, so a case must never run the real one —
# copy it into the fixture first.
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
# Escaped pipes come out before the count: `\|` separates no column, so counting raw `|` would read
# the guard working as the guard failing.
columns=$(printf '%s' "$row" | sed 's/\\|//g' | tr -cd '|' | wc -c | tr -d ' ')
if [ "$appended" = 1 ] && [ "$columns" = 7 ]; then
  record_pass "a note carrying a newline and a pipe still writes exactly one 6-column row"
else
  record_fail "a note carrying a newline and a pipe still writes exactly one 6-column row" \
    "rows appended: $appended (want 1)" "unescaped pipes: $columns (want 7)" "$row"
fi

echo "stats.sh — a missing ledger is opened with a header a reader can use"

# The only case that leaves stats.md out of the fixture, so it is the only one that reaches the header
# stats.sh writes when the ledger does not exist. Every other `--append` case creates the file first and
# never sees that block. The `+` legend is asserted because it is the one thing in the header a reader
# cannot reconstruct from the columns, and the header is where a fresh ledger states it.
new_root
mkdir -p "$root/skills/kk-reduce/scripts"
cp "$stats" "$root/skills/kk-reduce/scripts/stats.sh"
printf 'one two\n' >"$root/CLAUDE.md"
fresh_ledger="$root/skills/kk-reduce/stats.md"
"$root/skills/kk-reduce/scripts/stats.sh" --append 'opening row' "$root" >/dev/null 2>&1
fresh_rows=$(grep -c '^|' "$fresh_ledger" 2>/dev/null || echo 0)
if [ "$fresh_rows" = 3 ] &&
   grep -qF '| date | prose | scripts | always-loaded | skills | what ran |' "$fresh_ledger" &&
   grep -qF 'lower bound' "$fresh_ledger"; then
  record_pass "creates the ledger with the column header, the + legend, and one row under them"
else
  record_fail "creates the ledger with the column header, the + legend, and one row under them" \
    "lines starting '|': $fresh_rows (want 3 — header, rule, row)" \
    "$(cat "$fresh_ledger" 2>/dev/null)"
fi

echo "stats.sh — the ledger is not written through a symlink"

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
