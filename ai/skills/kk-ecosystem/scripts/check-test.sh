#!/usr/bin/env bash
# Tests for four of check.sh's blocks: the direction scan (ecosystem.md → **One home**) with its
# did-not-run guard, the `@import` resolved at the installed mount, the ranking that decides which
# findings survive the output cap, and the rule that keeps every finding on one physical line.
#   usage: check-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# A change to check.sh needs a case here, and `check-mutate.sh` is what proves a case can fail.
# Write a mutation and assert it landed where you aimed it, the same way these cases assert that
# the guard fired.
# No case asserts on the exit code — a fixture root legitimately has findings of its own, so
# "clean" would pass every case for the wrong reason.
set -uo pipefail
export LC_ALL=C

here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
check="$here/check.sh"
[ -x "$check" ] || { echo "check-test: $check is not executable"; exit 1; }

installed_scripts="$(cd -P -- "${HOME:-}/.claude/skills/kk-ecosystem/scripts" 2>/dev/null && pwd -P)"
[ -n "$installed_scripts" ] && [ "$installed_scripts" = "$here" ] || {
  echo "check-test: run it as ~/.claude/skills/kk-ecosystem/scripts/check-test.sh — this copy is somewhere else" >&2
  echo "check-test: exit 2 — nothing was tested. It runs the check.sh beside it, so where it runs from decides whose code executes." >&2
  exit 2
}

base="$(mktemp -d)" || { echo "check-test: mktemp gave no fixture dir — nothing was tested"; exit 1; }
trap 'rm -rf "$base"' EXIT
passed=0
failed=0
case_number=0
check_home=""

# This sets `root` instead of echoing a path for the caller to substitute. Write
# `root="$(new_root_without_flavor)"` and the body runs in a subshell: `case_number` never
# increments, so every case reuses one directory.
new_root_without_flavor() {
  case_number=$((case_number + 1))
  root="$base/r$case_number"
  mkdir -p "$root/skills"
  check_home=""
}

new_home_without_flavor_mount() {
  check_home="$root/home"
  mkdir -p "$check_home/.claude"
}

new_home() {
  new_home_without_flavor_mount
  ln -s "$root/kk-flavor" "$check_home/.kk-flavor"
}

new_root() {
  new_root_without_flavor
  mkdir -p "$root/kk-flavor/standards"
  printf '# Flavor\n' >"$root/kk-flavor/inject.md"
}

run_check() {
  local status
  check_output="$(HOME="${check_home:-$HOME}" "$check" "$root" 2>&1)"
  status=$?
  if [ "$status" = 2 ]; then
    echo "check-test: check.sh exited 2 on '$1' — nothing was checked, so no case here can be trusted"
    printf '%s\n' "$check_output" | sed 's/^/          /'
    exit 1
  fi
}

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1"
  printf '%s\n' "$check_output" | sed 's/^/          /'
}

# Both asserts take a here-string, never `printf … | grep -q` — check.sh's own here-string note says
# why, and a match swallowed here turns an `assert_does_not_report` case into a silent pass.
assert_reports() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_pass "$2"; else record_fail "$2"; fi
}

assert_does_not_report() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_fail "$2"; else record_pass "$2"; fi
}

# check.sh prints its budget lines before any finding, so a leak from a scan loop lands ahead of them.
assert_reported_via_findings() {
  local budget_line finding_line
  run_check "$2"
  budget_line="$(grep -nF -m1 'always-loaded:' <<<"$check_output" | cut -d: -f1)"
  finding_line="$(grep -nF -m1 "$1" <<<"$check_output" | cut -d: -f1)"
  if [ -n "$budget_line" ] && [ -n "$finding_line" ] && [ "$finding_line" -gt "$budget_line" ]; then
    record_pass "$2"
  else
    record_fail "$2"
  fi
}

# Ordering, never presence: the per-class cap on its own puts a real finding on screen alongside a
# flood, so asserting the real one is merely *present* passes with the rank patterns unanchored and
# observes nothing. What the `^` decides is which of the two lands first.
assert_ranks_above() {
  local above below
  run_check "$3"
  above="$(grep -nF -m1 "$1" <<<"$check_output" | cut -d: -f1)"
  below="$(grep -nF -m1 "$2" <<<"$check_output" | cut -d: -f1)"
  if [ -n "$above" ] && [ -n "$below" ] && [ "$above" -lt "$below" ]; then
    record_pass "$3"
  else
    record_fail "$3"
  fi
}

cites="shared layer cites into a lane"
never_ran="direction scan read no files"
refused="import refused"

echo "check.sh — direction scan"

new_root
printf 'the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$cites" "fires on a standard citing into a SKILL.md"

new_root
printf 'see `~/.claude/skills/kk-drive/SKILL.md` for the mechanics\n' >"$root/CLAUDE.md"
assert_reports "$cites" "fires on the root CLAUDE.md citing into a SKILL.md"

new_root
printf 'the mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/kk-flavor/standards/x.md"
assert_reported_via_findings "$cites" "reports through the bounded findings path, not raw on stdout"

new_root
{
  printf 'a glob names the set: `~/.claude/skills/*/SKILL.md`\n'
  printf 'a bare name is not a path: run it per its SKILL.md\n'
  printf 'a placeholder: `~/.claude/skills/<skill name>/SKILL.md`\n'
  # Use a path that really exists, never an invented one — the home-ref scan resolves every
  # `~/.claude/skills/...` it finds, fixture text like this included.
  printf 'a file the skill owns that is not its body: `~/.claude/skills/idsd-qualify/templates/retro-spawn-prompt.md`\n'
} >"$root/kk-flavor/standards/legal.md"
assert_does_not_report "$cites" "stays quiet on the four legal forms"

new_root
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$base/outside.md"
ln -s "$base/outside.md" "$root/CLAUDE.md"
assert_does_not_report "$cites" "does not read a violation through a symlinked CLAUDE.md"

new_root
printf '# Root\n' >"$root/CLAUDE.md"
assert_does_not_report "$never_ran" "a root with no violation and no symlink does not trip the guard"

new_root_without_flavor
mkdir -p "$root/real-flavor/standards"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/real-flavor/standards/x.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
printf '# Root\n' >"$root/CLAUDE.md"
assert_reports "$never_ran" "reports itself when a symlinked kk-flavor leaves it nothing to walk"
assert_does_not_report "$cites" "and does not read the violation behind that symlink"

echo "check.sh — @import resolved at the mount"

uncounted="uncounted import"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_does_not_report "$uncounted" "counts an import the mount really holds"
assert_reports "across 3 files" "and folds it into the budget's file count"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home_without_flavor_mount
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses to resolve when this checkout is not the installed one"

new_root
printf '# Root\n\n@../../escape.md\n' >"$root/CLAUDE.md"
new_home
# The file has to sit exactly where the traversal lands. Put it anywhere else and the case passes
# on "no such file", proving nothing.
printf 'secret words here\n' >"$root/escape.md"
assert_reports "$uncounted" "refuses a name that is a path rather than a bare filename"
assert_reports "$refused" "and reports that refusal instead of leaving it to read as drift"

new_root
printf '# Root\n\n@dir/file.md\n' >"$root/CLAUDE.md"
new_home
mkdir -p "$check_home/.claude/dir"
printf 'one two three\n' >"$check_home/.claude/dir/file.md"
assert_reports "$uncounted" "leaves a plain subdirectory import uncounted"
assert_does_not_report "$refused" "and does not report it as a probe"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$base/linked.md"
ln -s "$base/linked.md" "$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a symlink at the mount"
assert_reports "$refused" "and reports that one too, the shape nothing legitimate produces"

new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home
printf 'one two three\n' >"$check_home/.claude/FOO.md"
chmod 000 "$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a file at the mount it cannot read"
assert_reports "$refused" "and reports it, the shape that hid a short figure one tier down"

new_root_without_flavor
mkdir -p "$root/real-flavor"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
new_home_without_flavor_mount
ln -s "$root/real-flavor" "$check_home/.kk-flavor"
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a kk-flavor symlinked to the install, which would open the gate"

new_root
printf '# Root\n' >"$root/CLAUDE.md"
printf '@BAR.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/BAR.md"
assert_reports "$uncounted" "refuses an import its carrier does not put at this mount"

new_root
printf '# Root\n\n```\n@FOO.md\n```\n' >"$root/CLAUDE.md"
printf '@FOO.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "treats a fenced mention in CLAUDE.md as prose, not as its import"

new_root
printf '# Root\n\nnever write `@FOO.md` here\n' >"$root/CLAUDE.md"
printf '@FOO.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "treats a backticked mention in CLAUDE.md the same way"

new_root
new_home
{
  printf '# Root\n\n'
  capped=1
  while [ "$capped" -le 65 ]; do printf '@f%s.md\n' "$capped"; capped=$((capped + 1)); done
} >"$root/CLAUDE.md"
capped=1
while [ "$capped" -le 65 ]; do printf 'one\n' >"$check_home/.claude/f$capped.md"; capped=$((capped + 1)); done
assert_reports "$uncounted" "caps resolution attempts and names what it skipped"

# `sort -u` under LC_ALL=C is what fixes the two positions this case turns on: `../x.md` first, because
# `.` sorts below every letter, and `d01.md` 65th — one past the 64-attempt cap, with the 63 `b` names
# filling it. Under a locale that sorts punctuation away, `../x.md` itself lands past the cap and both
# asserts flip.
new_root
new_home
{
  printf '# Root\n\n'
  stale=1
  while [ "$stale" -le 63 ]; do printf '@b%02d.md\n' "$stale"; stale=$((stale + 1)); done
  printf '@../x.md\n@d01.md\n'
} >"$root/CLAUDE.md"
stale=1
while [ "$stale" -le 63 ]; do
  printf 'one\n' >"$check_home/.claude/b$(printf '%02d' "$stale").md"
  stale=$((stale + 1))
done
printf 'one\n' >"$check_home/.claude/d01.md"
assert_reports "not counted: ../x.md" "reports the probe-shaped name the resolver did look at"
assert_does_not_report "not counted: d01.md" "and carries no reason over to a name past the cap"

new_root
{
  flood=1
  while [ "$flood" -le 300 ]; do printf '[x](nope%s.md)\n' "$flood"; flood=$((flood + 1)); done
} >"$root/kk-flavor/standards/flood.md"
printf '#!/usr/bin/env bash\n' >"$root/skills/notexec.sh"
assert_reports "script not executable" "shows a tampered-check finding through a flood of link findings"

new_root
printf '#!/bin/sh\necho hi\n' >"$root/skills/notexec.sh"
chmod 644 "$root/skills/notexec.sh"
{
  promoted=1
  while [ "$promoted" -le 300 ]; do
    printf '[x](nope%03d flavor mounted elsewhere filler)\n' "$promoted"
    promoted=$((promoted + 1))
  done
} >"$root/kk-flavor/standards/flood.md"
# `filler` is what makes the needle flood-only: `flavor mounted elsewhere` alone also matches the
# genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
# real finding against itself.
assert_ranks_above "script not executable" "flavor mounted elsewhere filler" \
  "ranks a real finding above a flood whose link targets forge a mount finding"

new_root_without_flavor
mkdir -p "$root/real-flavor/standards"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
{
  buried=1
  while [ "$buried" -le 300 ]; do printf '[x](nope%03d.md)\n' "$buried"; buried=$((buried + 1)); done
} >"$root/real-flavor/standards/flood.md"
assert_reports "$never_ran" "surfaces the did-not-run guard under a flood that sorts ahead of it"

new_root
forged="$root/kk-flavor/standards/a
syntax: FORGED.md"
# No fallback here: writing a plain filename instead would satisfy the assertion while testing
# nothing.
printf '[x](nowhere.md)\n' >"$forged" 2>/dev/null || {
  echo "check-test: this filesystem refused a newline in a filename — the forgery case cannot run here"
  exit 1
}
run_check "newline in a committed path cannot forge a finding line"
if [ "$(grep -c '^syntax: FORGED' <<<"$check_output")" = 0 ]; then
  record_pass "a newline in a committed path cannot start a forged finding line"
else
  record_fail "a newline in a committed path cannot start a forged finding line"
fi

new_root
printf 'if then\n' >"$root/skills/broken.sh"
chmod +x "$root/skills/broken.sh"
buried_syntax=1
while [ "$buried_syntax" -le 300 ]; do
  printf '#!/bin/sh\necho ok\n' >"$root/skills/notexec$buried_syntax.sh"
  chmod 644 "$root/skills/notexec$buried_syntax.sh"
  buried_syntax=$((buried_syntax + 1))
done
assert_reports "syntax: " "shows a syntax error under a flood of its own priority tier"

new_root_without_flavor
mkdir -p "$root/real-flavor/standards"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
flooded=1
while [ "$flooded" -le 120 ]; do
  printf 'if then\n' >"$root/skills/broken$flooded.sh"
  chmod +x "$root/skills/broken$flooded.sh"
  flooded=$((flooded + 1))
done
assert_reports "$never_ran" "shows a rank-1 finding under a flood of the gravest class"
assert_reports "of this class, suppressed" "and says how many of the flooding class it withheld"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
