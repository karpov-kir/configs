#!/usr/bin/env bash
# Tests for check.sh's guards. The `echo` line above each group of cases names the block it covers.
#   usage: check-test.sh   # prints one line per case; exit 0 when all pass, 1 otherwise
#
# A change to check.sh needs a case here, and `~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh`
# is what proves a case can fail. Write a mutation and assert it landed where you aimed it, the same
# way these cases assert that the guard fired.
# No case asserts on the exit code — a fixture root legitimately has findings of its own, so
# "clean" would pass every case for the wrong reason.
# Past the ~450-line guidance deliberately: `~/.claude/skills/kk-ecosystem/scripts/check-mutate.sh`
# copies the one suite beside it into its sandbox, so a split means teaching that harness to run
# several and to attribute a kill across them.
# Split it once that is solved, never on the line count alone.
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

# check.sh refuses to walk a symlinked `kk-flavor`, so a case writes into `$root/real-flavor` and never
# `$root/kk-flavor`.
new_root_with_symlinked_flavor() {
  new_root_without_flavor
  mkdir -p "$root/real-flavor/standards"
  printf '# Flavor\n' >"$root/real-flavor/inject.md"
  ln -s "$root/real-flavor" "$root/kk-flavor"
}

# A skill the bare-name half of the direction scan can resolve: it counts a name only when a skill
# answers to it.
new_mounted_skill() {
  mkdir -p "$root/skills/$1"
  printf '# %s\n' "$1" >"$root/skills/$1/SKILL.md"
}

new_script() {
  printf '%s\n' "$2" >"$root/skills/$1"
  chmod +x "$root/skills/$1"
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
names="shared layer names a lane"
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
# The fourth form is not legal: ecosystem.md → **One home** bans a file the skill owns too. It is
# quiet *here* only because this scan matches nothing but a path into a SKILL.md — the bare-name scan
# below reports it in any tree that holds `idsd-qualify`.
assert_does_not_report "$cites" "stays quiet on the four forms that are not a path into a SKILL.md"

new_root
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$base/outside.md"
ln -s "$base/outside.md" "$root/CLAUDE.md"
assert_does_not_report "$cites" "does not read a violation through a symlinked CLAUDE.md"

# The bare-name half of the same rule. A name counts only when a skill answers to it, so the fixture
# has to mount one. The negative case is the same prose with nothing mounted: it pins that gate, not
# the prose's legality — check.sh's `unknown skill referenced` scan reports that prose anyway.
new_root
new_mounted_skill kk-drive
printf 'spawn `kk-drive` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "fires on a standard naming a skill that exists"

# `grep -o` keeps the trailing hyphen a token ends on, which a glob in prose produces. Without the
# strip the name matches no skill directory and the violation goes unreported.
new_root
new_mounted_skill kk-drive
printf 'every `kk-drive-*` invocation is the same lane\n' >"$root/kk-flavor/standards/x.md"
assert_reports "$names" "strips the trailing hyphen a glob leaves on a lane name"

new_root
printf 'spawn `kk-drive` before any lens reads it\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "the names scan stays quiet on a token no skill answers to"

new_root
new_mounted_skill kk-drive
printf 'a template under `kk-flavor/` is this same layer, not a lane\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "stays quiet on kk-flavor, which is the shared layer itself"

# One NUL byte makes grep call the file binary, and it prints `Binary file … matches` or, on GNU grep
# >= 3.5, nothing at all. Without `-a` both halves of the scan then read no violation out of a file
# `find` did hand them, and the did-not-run guard stays quiet because the file was still walked.
# The NUL goes in through `tr`: `printf '\000'` is truncated at the NUL by some printfs.
new_root
new_mounted_skill kk-drive
printf '# Rule\n' >"$root/kk-flavor/standards/x.md"
printf 'X' | tr 'X' '\000' >>"$root/kk-flavor/standards/x.md"
printf '\nspawn `kk-drive` before any lens reads it\n' >>"$root/kk-flavor/standards/x.md"
assert_reports "$names" "reads a lane name past a NUL byte that makes grep call the file binary"

new_root
printf '# Rule\n' >"$root/kk-flavor/standards/x.md"
printf 'X' | tr 'X' '\000' >>"$root/kk-flavor/standards/x.md"
printf '\nthe mechanics are `~/.claude/skills/kk-drive/SKILL.md`\n' >>"$root/kk-flavor/standards/x.md"
assert_reports 'kk-drive/SKILL.md — move the rule' "and reads a cited SKILL.md path past one too"

# The bound on what each half of the scan emits. Every finding costs a fork to sanitise its hit, so an
# unbounded emit lets one committed file turn this scan into tens of thousands of them.
new_root
new_mounted_skill kk-drive
flooded_names=1
while [ "$flooded_names" -le 45 ]; do
  printf 'spawn `kk-drive` now\n' >>"$root/kk-flavor/standards/x.md"
  flooded_names=$((flooded_names + 1))
done
# The file is pinned, not just the tail: leading with it is what sorts the notice ahead of that file's
# own hits, so the printer's per-rank cap drops those before it drops the notice.
assert_reports "$names: $root/kk-flavor/standards/x.md — 40 already shown" \
  "bounds what the names half of the scan emits"

# The same bound over two files, each holding fewer hits than the cap. It can only fire while the budget
# outlives one file, which is what a hit list read by process substitution rather than through a pipe
# into a subshell buys. Which of the two files the notice names depends on the order `find` hands them
# over, so the assert leaves that free. The path carries no `kk-`/`idsd-` token, so the names half
# stays at zero and this tail can only have come from the cites half. The path is a `%s` argument for
# the reason the citation cases below give: written whole, it is a backticked in-repo path, and the
# dangling-path-ref scan reads this file and reports it against the repo.
new_root
spread_cites=1
while [ "$spread_cites" -le 45 ]; do
  printf 'the mechanics are `%s/SKILL.md`\n' path/to/other \
    >>"$root/kk-flavor/standards/x$((spread_cites % 2)).md"
  spread_cites=$((spread_cites + 1))
done
assert_reports "— 40 already shown across the shared layer" \
  "bounds the cites half across files, not once per file"

# `kk-flavor` names the shared layer, not a lane. The reviewed tree picks what is in `skills/`, so the
# exclusion cannot rest on no skill answering to that name.
new_root
new_mounted_skill kk-flavor
printf 'a template under `kk-flavor/` is this same layer, not a lane\n' >"$root/kk-flavor/standards/x.md"
assert_does_not_report "$names" "excludes kk-flavor even when the tree commits a skill by that name"

new_root
printf '# Root\n' >"$root/CLAUDE.md"
assert_does_not_report "$never_ran" "a root with no violation and no symlink does not trip the guard"

new_root_with_symlinked_flavor
printf 'see `~/.claude/skills/kk-drive/SKILL.md`\n' >"$root/real-flavor/standards/x.md"
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

new_root_with_symlinked_flavor
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

new_root_with_symlinked_flavor
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

new_root_with_symlinked_flavor
flooded=1
while [ "$flooded" -le 120 ]; do
  printf 'if then\n' >"$root/skills/broken$flooded.sh"
  chmod +x "$root/skills/broken$flooded.sh"
  flooded=$((flooded + 1))
done
assert_reports "$never_ran" "shows a rank-1 finding under a flood of the gravest class"
assert_reports "of this class, suppressed" "and says how many of the flooding class it withheld"

echo "check.sh — citations name their section in the delimited form"

undelimited="undelimited section citation"

# The cited name is a `%s` argument, never inline: the citation scan reads this file too, so a fixture
# line carrying the citation whole becomes a finding against the repo. Split this way, the path the
# scan extracts is `%s`, which is not a `.md`, and it is dropped before any finding.
new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → One home for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_reports "$undelimited" "fires on a citation whose section is not delimited"

new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → **One home** for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_does_not_report "$undelimited" "accepts the bolded form"

new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
printf 'see [%s](%s) → `One home` for the rule\n' target.md target.md >"$root/kk-flavor/standards/citer.md"
assert_does_not_report "$undelimited" "accepts the backticked form, which the parser also reads exactly"

# A shell comment cites the same way a document does, and the scan reads both.
new_root
printf '# Target\n\n## One home\n' >"$root/kk-flavor/standards/target.md"
new_script "citer.sh" "$(printf '#!/usr/bin/env bash\n# untested: fixture\n# the rule is %s → One home\ntrue' target.md)"
assert_reports "$undelimited" "fires on an undelimited citation inside a shell comment"

# Each defect below makes a skill unreachable rather than merely mis-linked: the loader finds a skill by
# its directory, invokes it by its frontmatter `name`, and routes to it by its `description`.
echo "check.sh — the skill directory itself"

new_root
mkdir -p "$root/skills/orphan" "$root/skills/wrong-name" "$root/skills/no-desc"
printf -- '---\nname: misnamed\ndescription: does a thing\n---\n' >"$root/skills/wrong-name/SKILL.md"
printf -- '---\nname: no-desc\n---\n' >"$root/skills/no-desc/SKILL.md"
assert_reports "skill dir without SKILL.md" "fires on a skill directory holding no SKILL.md"
assert_reports "skill name/dir mismatch" "fires when the frontmatter name is not the directory name"
assert_reports "skill without a description" "fires on a SKILL.md carrying no description"

echo "check.sh — script test position"

missing_test="script names a missing test"
no_position="script declares no test position"

new_root
new_script "lonely.sh" '#!/usr/bin/env bash
# Does a thing.
true'
assert_reports "$no_position" "fires on a script naming neither a test nor an untested reason"

new_root
new_script "claims.sh" '#!/usr/bin/env bash
# A change here needs a case in claims-test.sh beside it.
true'
assert_reports "$missing_test" "fires on a header naming a -test.sh that is not in the tree"

new_root
new_script "covered.sh" '#!/usr/bin/env bash
# A change here needs a case in covered-test.sh beside it.
true'
new_script "covered-test.sh" '#!/usr/bin/env bash
true'
assert_does_not_report "$missing_test" "accepts a header whose named test exists"
assert_does_not_report "$no_position" "a named existing test is a declared position"

new_root
new_script "waived.sh" '#!/usr/bin/env bash
# untested: a four-line wrapper whose only failure mode is the exec bit.
true'
assert_does_not_report "$no_position" "accepts an explicit untested: declaration with a reason"

new_root
new_script "bare.sh" '#!/usr/bin/env bash
# untested:
true'
assert_reports "$no_position" "a bare untested: with no reason does not clear the check"

# The harness is exempt: asking a test file to name its own test makes every one of them a finding.
new_root
new_script "harness-test.sh" '#!/usr/bin/env bash
true'
new_script "harness-mutate.sh" '#!/usr/bin/env bash
true'
assert_does_not_report "$no_position" "asks nothing of -test.sh and -mutate.sh themselves"

# Header-scoped on purpose: a suite a script merely mentions in its body would read as coverage.
new_root
new_script "body.sh" '#!/usr/bin/env bash
# Does a thing.
set -u
# see also body-test.sh
true'
assert_reports "$no_position" "a -test.sh named below the header does not count as declared"

# The cap that keeps a crafted header from turning one scan into thousands of whole-tree walks. It has
# to *report*, never quietly read less than it looks like it read.
new_root
new_script "greedy.sh" '#!/usr/bin/env bash
# see n1-test.sh n2-test.sh n3-test.sh n4-test.sh n5-test.sh n6-test.sh
# and n7-test.sh n8-test.sh n9-test.sh n10-test.sh n11-test.sh n12-test.sh
true'
assert_reports "names more suites than the scan reads" "reports a header naming more suites than it reads"

# The bound on the header read. A declaration past 200 lines is not seen, which is correct, and it
# still has to be *reported* rather than pass as declared.
new_root
{
  printf '#!/usr/bin/env bash\n'
  line=1
  while [ "$line" -le 205 ]; do
    printf '# padding %s\n' "$line"
    line=$((line + 1))
  done
  printf '# untested: this reason sits past the 200-line bound and cannot clear the check\n'
  printf 'true\n'
} >"$root/skills/buried.sh"
chmod +x "$root/skills/buried.sh"
assert_reports "$no_position" "a declaration past the header bound does not clear the check"

# The suite list is built from filenames the reviewed tree chose. A newline in one splits a basename
# in two, the tail reads as a suite that exists, and a header naming an absent suite then passes. The
# control case comes first: without the hostile file, the finding must be there to lose.
new_root
new_script "tool.sh" '#!/usr/bin/env bash
# a change here needs a case in ghost-test.sh
true'
assert_reports "$missing_test" "reports a named suite that is absent (control for the case below)"
printf 'not a suite\n' >"$root/skills/$(printf 'x\nghost-test.sh')"
assert_reports "$missing_test" "a newline in a filename cannot forge the suite that satisfies a header"

new_root
new_script "dash.sh" '#!/usr/bin/env bash
# a change here needs a case in --test.sh
true'
assert_reports "$missing_test" "a suite name starting with a dash is still checked"
assert_does_not_report "unrecognized option" "and grep never dumps its usage into the findings"
assert_does_not_report "Usage: grep" "nor its usage banner"

echo "$passed passed, $failed failed"
[ "$failed" = 0 ]
