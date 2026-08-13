#!/usr/bin/env bash
# Tests for four of check.sh's blocks: the direction scan (ecosystem.md → **One home**) with its
# did-not-run guard, the `@import` resolved at the installed mount, the ranking that decides which
# findings survive the output cap, and the rule that keeps every finding on one physical line.
#   usage: check-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# Scoped to those four rather than all of check.sh, because each carries a guard whose whole job is to
# fire, and a guard nothing exercises is the stale gate check.sh exists to catch.
# The same trap sits one level up, in whatever mutates check.sh to prove these cases can fail. A mutation
# that stops matching, or that breaks the script instead of its behaviour, turns red for the wrong reason
# or not at all — and reads as proof either way. Both happened here: two patterns went stale when the
# resolver's lines grew their reasons, and one deleted an `echo` that left an empty function body. Assert
# the mutation landed where you aimed it, the way these cases assert the guard fired.
# Each case builds a throwaway root under `mktemp -d` and asserts on the finding text. No case asserts
# on the exit code: a fixture root legitimately has findings of its own (no skills, no inject.md), so
# "clean" would pass every case for the wrong reason.
set -uo pipefail
export LC_ALL=C

here="$(cd -P -- "$(dirname "$0")" && pwd -P)"
check="$here/check.sh"
[ -x "$check" ] || { echo "check-test: $check is not executable"; exit 1; }

# The installed copy only, matching both mutation runners. This executes the `check.sh` beside it against
# fixtures, so in a review worktree it would run the branch's own script — and a runner refusing a relocated
# copy points whoever wanted to verify that branch straight here, the one entry point that had no gate.
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
# The HOME check.sh runs under, empty to inherit this shell's. A case sets it to make its fixture look
# like the installed checkout, which is the condition the import resolver tests.
check_home=""

# Sets `root` to a fresh throwaway root holding skills/ alone, for the cases that supply their own
# kk-flavor/ as a symlink. Every other case wants `new_root` below, since check.sh exits 2 without
# checking anything unless both directories are there. It assigns instead of echoing a path the caller
# substitutes: `root="$(new_root)"` would run the body in a subshell, leave `case_number` unincremented
# here, and every case would then reuse one directory and inherit the last one's files.
new_root_without_flavor() {
  case_number=$((case_number + 1))
  root="$base/r$case_number"
  mkdir -p "$root/skills"
  check_home=""
}

# Sets `check_home` to a fake HOME with a `.claude/` and no flavor mount, so check.sh treats the tree
# it walks as *not* the installed one. That's the case where an import must stay uncounted. `new_home`
# below adds the mount that makes it installed, and the gate case adds one of its own.
new_home_without_flavor_mount() {
  check_home="$root/home"
  mkdir -p "$check_home/.claude"
}

# Sets `check_home` to a fake HOME whose `.kk-flavor` resolves to this fixture's own, which is how
# check.sh decides the tree it walks is the installed one.
new_home() {
  new_home_without_flavor_mount
  ln -s "$root/kk-flavor" "$check_home/.kk-flavor"
}

# Sets `root` to a fresh throwaway root check.sh will scan: skills/ plus a kk-flavor/ tree.
new_root() {
  new_root_without_flavor
  mkdir -p "$root/kk-flavor/standards"
  printf '# Flavor\n' >"$root/kk-flavor/inject.md"
}

# Sets `check_output` to everything check.sh printed for `$root`. A global for the subshell reason
# `new_root_without_flavor` gives above: inside `$(…)`, the stop below would end only the subshell.
# That stop is for exit 2, check.sh's own "nothing was checked", and every `assert_does_not_report` case
# passes vacuously against it, which is this harness's version of the defect it guards. A fixture reaches
# it by leaving out kk-flavor/ or skills/, and check.sh itself when it can't open a findings file.
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

# assert_reports <substring> <description> — check.sh must print the substring for this fixture.
# Here-string in both asserts, never `printf … | grep -q`: grep -q exits on the first matching line, so
# under `pipefail` the writer's SIGPIPE (141) turns that match into a miss and flips an
# `assert_does_not_report` case to a silent pass. check.sh's 200 findings of 500 chars run well past a
# pipe buffer.
assert_reports() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_pass "$2"; else record_fail "$2"; fi
}

# assert_does_not_report <substring> <description> — the inverse of assert_reports. It calls `run_check`
# itself, so the two asserts on one fixture at the bottom scan that fixture twice.
assert_does_not_report() {
  run_check "$2"
  if grep -qF "$1" <<<"$check_output"; then record_fail "$2"; else record_pass "$2"; fi
}

# assert_reported_via_findings <substring> <description> — the substring must arrive on check.sh's bounded
# findings path rather than raw on stdout from inside a scan loop. check.sh prints its two budget lines
# before it prints any finding, so a leak lands ahead of them and a plain substring match can't tell the
# two apart. That blind spot is real: a scan's `done` losing its `>>"$findings"` redirect keeps every other
# case green while its findings escape `sort -u | cut | head` and ride the exit-0 path.
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
  # A real path, not an invented one: the home-ref scan walks this file too and resolves every
  # `~/.claude/skills/...` it finds, fixture text included.
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

# A `kk-flavor` symlinked to a sibling *inside* the root walks nothing: `-type f` drops the symlinked
# directory, and in the installed checkout both paths canonicalise to the same target, so the mount check
# stays quiet too. That's the shape that lets this scan read clean while walking nothing. This fixture
# sits outside `$HOME`, so it does carry mount findings of its own, and asserts on text.
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

# The resolver's refusals, one case apiece. A counted number is not the only thing to get right here.
# Every refusal must leave the import named rather than silently dropped: that is the difference
# between a lower bound the reader can see and a wrong number they can't. A probe-shaped refusal must
# also *report* itself, since a probe hiding inside routine drift is the same blindness one tier down.
new_root
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
new_home_without_flavor_mount
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses to resolve when this checkout is not the installed one"

new_root
printf '# Root\n\n@../../escape.md\n' >"$root/CLAUDE.md"
new_home
# Planted where the traversal actually lands, or the case passes on "no such file" and proves nothing:
# HOME is `$root/home`, so `$HOME/.claude/../../escape.md` resolves to `$root/escape.md`.
printf 'secret words here\n' >"$root/escape.md"
assert_reports "$uncounted" "refuses a name that is a path rather than a bare filename"
assert_reports "$refused" "and reports that refusal instead of leaving it to read as drift"

# A plain subdirectory import is a legitimate form this resolver does not handle, so it must stay a quiet
# uncounted name. Reported, it would put a probe's finding on honest content and take the run to exit 1.
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
# A real file the resolver's `-f` accepts, made unreadable rather than removed. Drop the `-r` test and
# it gets counted in `across N files` while the `cat` behind that number fails, so the case turns on
# the mode and nothing else.
chmod 000 "$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a file at the mount it cannot read"
assert_reports "$refused" "and reports it, the shape that hid a short figure one tier down"

# The gate's second half. `cd -P` follows a symlinked directory, so a branch committing `kk-flavor` as
# a symlink to the real install makes both sides of the mount comparison agree and opens the gate
# inside a review worktree. That was reachable, and it read a mode-600 file out of the reviewer's own
# `~/.claude/`. Both symlinks point at one directory here, which is what makes the comparison agree.
new_root_without_flavor
mkdir -p "$root/real-flavor"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
new_home_without_flavor_mount
ln -s "$root/real-flavor" "$check_home/.kk-flavor"
printf '# Root\n\n@FOO.md\n' >"$root/CLAUDE.md"
printf 'one two three\n' >"$check_home/.claude/FOO.md"
assert_reports "$uncounted" "refuses a kk-flavor symlinked to the install, which would open the gate"

# Only `CLAUDE.md`'s imports resolve at `~/.claude/`. This one is carried by `inject.md`, whose imports
# load from `~/.kk-flavor/`, so resolving it here would count a same-named file from the wrong mount.
# `CLAUDE.md` exists and is readable but does not carry the name, which is the condition under test.
new_root
printf '# Root\n' >"$root/CLAUDE.md"
printf '@BAR.md\n' >>"$root/kk-flavor/inject.md"
new_home
printf 'one two three four five\n' >"$check_home/.claude/BAR.md"
assert_reports "$uncounted" "refuses an import its carrier does not put at this mount"

# A mention is not an import. The scan skips fenced blocks and inline code spans as prose *about*
# imports, so the carrier test has to skip them the same way. A substring search doesn't, and this
# ecosystem's prose names imports in backticks constantly, which made the house writing style the
# bypass. Both fixtures put the real carrier in inject.md, whose imports load from `~/.kk-flavor/`.
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

# The attempt cap. Past it a name goes to the note rather than dropping out of the figure, so a file
# naming thousands of imports costs a bounded number of stat calls and still says what it skipped.
# 65 names, every one of them present at the mount: without the cap none would be left to report.
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

# The other half of the cap: past it the resolver is never called, so the reason it left for the last
# name it did look at must not be reported against a name it never reached. 63 names resolve, the 64th
# is probe-shaped and earns its report, and the 65th sorts after it and is present at the mount, so a
# reason carried over is a claim about a file that would have counted. `sort -u` under LC_ALL=C is what
# fixes those three positions, which is why the names are padded.
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

# Findings are cut at 200 lines, so their order decides what a reader ever sees. Alphabetically, 300
# `dangling link:` lines fill the cap and every class sorting after them disappears — including the ones
# naming a broken or tampered-with check rather than a broken reference. A branch can plant that flood.
new_root
{
  flood=1
  while [ "$flood" -le 300 ]; do printf '[x](nope%s.md)\n' "$flood"; flood=$((flood + 1)); done
} >"$root/kk-flavor/standards/flood.md"
printf '#!/usr/bin/env bash\n' >"$root/skills/notexec.sh"
assert_reports "script not executable" "shows a tampered-check finding through a flood of link findings"

# A flood that promotes itself into the priority tier. The tier's pattern was once unanchored, so any line
# merely containing ` not mounted:` was promoted — and a link target is written by the branch. 300 links
# ending in that substring sorted under `d`, ahead of `script` and `syntax`, and buried both.
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
assert_reports "script not executable" "surfaces a real finding under a flood that promotes itself"

# The same flood against the guard that says this check did not check the tree you think. `dangling link:`
# sorts before `direction scan`, so a plain flood buried it while its own comment called it the only thing
# that catches the shape.
new_root_without_flavor
mkdir -p "$root/real-flavor/standards"
printf '# Flavor\n' >"$root/real-flavor/inject.md"
ln -s "$root/real-flavor" "$root/kk-flavor"
{
  buried=1
  while [ "$buried" -le 300 ]; do printf '[x](nope%03d.md)\n' "$buried"; buried=$((buried + 1)); done
} >"$root/real-flavor/standards/flood.md"
assert_reports "$never_ran" "surfaces the did-not-run guard under a flood that sorts ahead of it"

# A committed path may hold a newline, and git stores one. Interpolated raw it emits a second physical line
# of text the branch chose, wearing no prefix of ours — and it picks its own rank in the ordering. Every
# finding is one line, so the forged class prefix must never begin one.
new_root
forged="$root/kk-flavor/standards/a
syntax: FORGED.md"
# No fallback: a filesystem that refuses the name makes this case untestable, and quietly writing a plain
# file instead would satisfy the assertion having tested nothing.
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

# Ranking *within* the priority tier. Flooding the non-priority tier proves only that priority comes first;
# it says nothing about order inside it. 300 non-executable scripts are rank 3 and sort under `script`,
# just ahead of `syntax:` — so without intra-tier ranking they fill the 200-line cap and bury the one
# finding that says a script no longer parses.
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

# The per-class cap. Ranking alone still let the gravest class bury the rest, because rank 0 is also the
# cheapest to mass-produce: 100 broken scripts emit 2 `syntax:` lines each — the two bashes word their
# errors differently, so `sort -u` keeps both — which filled the 200-line global cap and dropped a real
# `shared region … has drifted`, a guard deleted from one copy, entirely.
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
