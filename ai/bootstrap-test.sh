#!/usr/bin/env bash
# Cases for ai/bootstrap.sh: the mounts nothing else covers — the instructions, the kk-flavor bucket
# and the discovered skills — the leftover file this script removes, and the verify step with the
# re-entry guard that keeps it from recursing.
#
# The second-checkout guard and the refusal-rather-than-delete rule live in lib/mount.sh and are
# covered in full by env/bootstrap-test.sh. What is asserted here is the half that file cannot reach:
# a bulk mount taking part in the count the guard reports.
#
# Every case runs the real script against a throwaway $HOME and passes every --skip flag, so brew, gh
# and claude are never invoked.
set -u

# The verify step runs run-tests.sh with BOOTSTRAP_VERIFYING=1, that runner discovers this suite, and
# this suite inherits the marker — so every case below that needs verify to run finds it already
# skipped, and `ai/bootstrap.sh` on a working machine then refuses, blaming the suites for its own
# marker. Cleared once here rather than per invocation: the case at the guard sets the marker on its
# own command line, so a case that means to test the skip still does, and one written tomorrow cannot
# inherit it by forgetting.
unset BOOTSTRAP_VERIFYING

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
script="$here/bootstrap.sh"
suite_name="ai/bootstrap-test.sh"

# Status checked, because `.` on a file whose tail does not parse still defines every function ahead
# of the break: unchecked, this suite runs on the half it got and reports counts nobody may read as a
# pass. env/bootstrap-test.sh holds the case that proves this line fires, for both copies of it.
# shellcheck source=../lib/test-harness.sh
. "$checkout/lib/test-harness.sh" ||
  { printf '%s: lib/test-harness.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

# The skip flags are load-bearing: without them a case shells out to brew, gh and the claude CLI, which
# makes the suite slow, network-dependent, and able to write to the real MCP registry.
run_boot() {
  local home="$1"
  shift
  out=$(HOME="$home" bash "$script" --skip-brew --skip-tools --skip-mcp --skip-verify "$@" 2>&1)
  status=$?
}

echo "ai/bootstrap.sh"

# --- a fresh machine ------------------------------------------------------------------------------

fresh_home
run_boot "$home"
expect_status "a fresh home exits 0" 0
expect_out "and reports ok" "ai bootstrap: ok"
expect_link_to "the flavor bucket is mounted" "$home/.kk-flavor" "$here/kk-flavor"
expect_link_to "CLAUDE.md is linked" "$home/.claude/CLAUDE.md" "$here/CLAUDE.md"

# Parents the README creates by hand: ~/.claude and ~/.claude/skills do not exist on a fresh machine,
# and a link into a missing directory fails rather than creating it.
[ -d "$home/.claude/skills" ] &&
  record_pass "missing parent directories are created" ||
  record_fail "missing parent directories are created" "~/.claude/skills is absent"

# Discovery, so a skill added later is mounted without editing ai/bootstrap.sh. Compared against the
# repository rather than a hard-coded number, which would drift the day a skill lands.
want_skills=$(find "$here/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
got_skills=$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$want_skills" -gt 0 ] && [ "$got_skills" -eq "$want_skills" ] &&
  record_pass "every skill directory is mounted" ||
  record_fail "every skill directory is mounted" "mounted $got_skills of $want_skills"

# --- re-running -----------------------------------------------------------------------------------

run_boot "$home"
expect_status "a second run over a finished home exits 0" 0
expect_out "and reports the targets as already ok" "  ok       $home/.kk-flavor"
expect_not_out "and relinks nothing" "linked   $home/.kk-flavor"

# A link the README's old skills loop made reads back with a trailing slash, because `$d` came from a
# `*/` glob. Compared raw, every skill on a machine set up by hand looks stale and gets rewritten on
# every run — idempotence lost to a cosmetic difference, and the noise hides a genuinely stale link.
fresh_home
mkdir -p "$home/.claude/skills"
first_skill=$(find "$here/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)
fixture_link "$first_skill/" "$home/.claude/skills/$(basename "$first_skill")"
run_boot "$home"
expect_out "a skill link differing only by a trailing slash is left alone" "  ok       $home/.claude/skills/$(basename "$first_skill")"
expect_not_out "and is not rewritten" "repointed $home/.claude/skills/$(basename "$first_skill")"

# --- the file this repository used to write, and now removes ---------------------------------------

# ai/CLAUDE.md's `@RTK.md` import is gone and its two surviving sentences are inline there, so
# `~/.claude/RTK.md` is text nothing writes and nothing reads. The removal is a step in the script
# rather than something done by hand because that file sits in the human's home, outside this
# repository — a step is what makes it a removal they run knowingly, and what carries it to their other
# machines. Every case below reads the path back afterwards: a step reporting "removed" over a file
# still on disk is exactly what these are here to catch.
fresh_home
run_boot "$home"
expect_status "a fresh home with no leftover exits 0" 0
expect_out "and says there was nothing to remove" "no leftover $home/.claude/RTK.md"
[ ! -e "$home/.claude/RTK.md" ] &&
  record_pass "and nothing is written at that path any more" ||
  record_fail "and nothing is written at that path any more" "the file was created"

# The case the step exists for: the copy an earlier bootstrap left behind, which is what every machine
# already set up from this repository is holding, and what the next `rtk init -g` puts back.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/RTK.md" 'the copy an earlier bootstrap left here'
run_boot "$home"
expect_status "a leftover file is removed and the run exits 0" 0
expect_out "and says it removed it" "removed  $home/.claude/RTK.md"
[ ! -e "$home/.claude/RTK.md" ] &&
  record_pass "and the leftover is actually gone" ||
  record_fail "and the leftover is actually gone" "it is still there"

# A symlink there instead of a copy — what a machine set up from an older README by hand would hold.
# A symlink carries no data of its own, so it goes the same way. What it points at must not: `rm -f` on
# a link does not follow it, and the file the fixture links to is what proves that here.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/pointed-at.md" 'the file the link named'
fixture_link "$home/.claude/pointed-at.md" "$home/.claude/RTK.md"
run_boot "$home"
expect_status "a symlink left at that path is removed too" 0
[ ! -e "$home/.claude/RTK.md" ] && [ ! -L "$home/.claude/RTK.md" ] &&
  record_pass "and the link is gone" ||
  record_fail "and the link is gone" "it is still there"
expect_file_body "and what it pointed at was not followed and deleted" \
  "$home/.claude/pointed-at.md" 'the file the link named'

# A directory there is not a shape this script ever wrote, so it holds something else and `rm -rf` over
# it is the data loss the header promises this script is not. Asserted from both ends: the refusal, and
# that what was inside it is still inside it.
fresh_home
mkdir -p "$home/.claude/RTK.md"
fixture_write "$home/.claude/RTK.md/notes.md" 'somebody else put this here'
run_boot "$home"
expect_status "a directory at that path exits 1" 1
expect_out "and says it will not remove a directory" "is a directory, and this script only ever wrote a file"
expect_not_out "and does not report having removed it" "removed  $home/.claude/RTK.md"
expect_file_body "and what was inside it survives" "$home/.claude/RTK.md/notes.md" 'somebody else put this here'

# --dry-run over a leftover, which needs a leftover of its own: the --dry-run case below starts from a
# fresh home, where this step has nothing to preview and would report the same either way.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/RTK.md" 'still here afterwards'
run_boot "$home" --dry-run
expect_status "--dry-run over a leftover exits 0" 0
expect_out "and says it would remove it" "would remove the leftover $home/.claude/RTK.md"
expect_file_body "and leaves the leftover alone" "$home/.claude/RTK.md" 'still here afterwards'

# --- --dry-run ------------------------------------------------------------------------------------

fresh_home
run_boot "$home" --dry-run
expect_status "--dry-run exits 0" 0
expect_out "and says what it would do" "would link"
[ ! -e "$home/.kk-flavor" ] && [ ! -e "$home/.claude" ] &&
  record_pass "--dry-run creates nothing at all" ||
  record_fail "--dry-run creates nothing at all" "something was written under $home"

# --- the two audiences ------------------------------------------------------------------------------

# Some skills exist only to maintain this instruction tree and do nothing for a repository that merely
# uses it. Each costs every session context through its `description:`, which is loaded whether or not
# the skill is ever invoked, so an install that is not maintaining the tree should not carry them.
#
# Both sets are discovered here the way the script discovers its mounts, and by a reader of their own:
# a list of names written into this suite would drift the day a fourth skill is marked, and would pass
# over a script that had gone back to mounting by hardcoded name.
maintainer_skills=""
public_skills=""
for skill_path in "$here"/skills/*/; do
  skill_name=$(basename "${skill_path%/}")
  if grep -q '^audience: maintainer$' "${skill_path}SKILL.md" 2>/dev/null; then
    maintainer_skills="$maintainer_skills $skill_name"
  else
    public_skills="$public_skills $skill_name"
  fi
done

if [ -n "$maintainer_skills" ] && [ -n "$public_skills" ]; then
  record_pass "control: the tree holds skills of both audiences, so the cases below compare something"
else
  record_fail "control: the tree holds skills of both audiences, so the cases below compare something" \
    "maintainer='$maintainer_skills' public='$public_skills'"
fi

fresh_home
run_boot "$home" --skip-maintainer-skills
expect_status "--skip-maintainer-skills exits 0" 0

unmounted=""
for skill_name in $public_skills; do
  [ -L "$home/.claude/skills/$skill_name" ] || unmounted="$unmounted $skill_name"
done
[ -z "$unmounted" ] &&
  record_pass "and every skill the marker does not name is still mounted" ||
  record_fail "and every skill the marker does not name is still mounted" "not mounted:$unmounted"

mounted=""
for skill_name in $maintainer_skills; do
  [ -e "$home/.claude/skills/$skill_name" ] && mounted="$mounted $skill_name"
done
[ -z "$mounted" ] &&
  record_pass "and no marked skill is" ||
  record_fail "and no marked skill is" "mounted anyway:$mounted"

expect_out "and says how many it left out, rather than excluding them quietly" "maintainer-only"

# The flagless default, which is what every machine already set up re-runs. The fresh-machine case at
# the top counts every skill directory against every mount, so what is left to say here is that the
# marked ones are inside that count rather than excluded by a marker the flag was supposed to gate.
fresh_home
run_boot "$home"
missing=""
for skill_name in $maintainer_skills; do
  [ -L "$home/.claude/skills/$skill_name" ] || missing="$missing $skill_name"
done
[ -z "$missing" ] &&
  record_pass "with no flag, a marked skill is mounted like any other" ||
  record_fail "with no flag, a marked skill is mounted like any other" "not mounted:$missing"

# Discovery's second vacuity case. A checkout where the flag excludes every skill mounts nothing, and
# the empty-tree refusal would report that as a skills directory holding no skill — a false diagnosis
# sending the reader to look for files that are all there. The exit code is the same either way, so the
# wording is the only thing telling the two apart.
# Under $tmp_real, not $tmp: the script resolves its own repository with `pwd -P`, so a fixture reached
# through the unresolved /var symlink mounts links this suite would then compare against the other
# spelling of the same path.
only_maintainer="$tmp_real/only-maintainer"
fixture_checkout "$only_maintainer" ai
mkdir -p "$only_maintainer/ai/skills/kk-ecosystem" "$only_maintainer/ai/kk-flavor"
: >"$only_maintainer/ai/CLAUDE.md"
cat >"$only_maintainer/ai/skills/kk-ecosystem/SKILL.md" <<'SKILL'
---
name: kk-ecosystem
description: the one skill this fixture ships
audience: maintainer
---
SKILL

fresh_home
out=$(HOME="$home" bash "$only_maintainer/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp --skip-verify --skip-maintainer-skills 2>&1)
status=$?
expect_status "a checkout whose every skill is marked exits 1" 1
expect_out "and says the flag is what excluded them" "excluded all 1"
expect_not_out "and does not report the tree as holding no skill at all" "no skill directories under"

# An audience nothing reads. The marker check answers "not marked" to a misspelling and to a skill that
# declared nothing, and those mean opposite things — so the misspelling installs for everyone while
# whoever typed it believes they marked it, on a machine where nothing looks wrong.
typo_audience="$tmp_real/typo-audience"
fixture_checkout "$typo_audience" ai
mkdir -p "$typo_audience/ai/skills/kk-typo" "$typo_audience/ai/kk-flavor"
: >"$typo_audience/ai/CLAUDE.md"
cat >"$typo_audience/ai/skills/kk-typo/SKILL.md" <<'SKILL'
---
name: kk-typo
description: the one skill this fixture ships
audience: maintainr
---
SKILL

fresh_home
out=$(HOME="$home" bash "$typo_audience/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a misspelled audience exits 1 rather than installing quietly" 1
expect_out "and echoes back what was written, so it can be found in the file" "maintainr"
expect_out "and names the one value there is" "audience: maintainer"

# The control. Without it every assertion above is equally satisfied by a script that refuses each
# skill it reads, and the suite would be measuring nothing.
fresh_home
sed -i.bak 's/^audience: maintainr$/audience: maintainer/' "$typo_audience/ai/skills/kk-typo/SKILL.md"
out=$(HOME="$home" bash "$typo_audience/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "control: the same tree with the marker spelled right exits 0" 0
expect_not_out "control: and refuses nothing" "no reader knows"

# The control, and the load-bearing half: the same checkout with no flag mounts its one skill. Without
# it the refusal above would pass over a fixture that never had a skill to mount.
fresh_home
out=$(HOME="$home" bash "$only_maintainer/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "control: the same checkout with no flag exits 0" 0
expect_link_to "control: and mounts its one skill" \
  "$home/.claude/skills/kk-ecosystem" "$only_maintainer/ai/skills/kk-ecosystem"

# --- arguments ------------------------------------------------------------------------------------

# The skip flags ride along even though the option check runs before any of them are consulted. If
# that check ever stops exiting, this case must fail rather than proceed into brew, gh, the claude CLI
# and a verify run that discovers this very suite — a regression should redden, not install things.
fresh_home
out=$(HOME="$home" bash "$script" --skip-brew --skip-tools --skip-mcp --skip-verify --not-a-flag 2>&1)
status=$?
expect_status "an unknown option exits 2" 2
expect_out "and names the option it rejected" "--not-a-flag"
[ ! -e "$home/.kk-flavor" ] &&
  record_pass "and a rejected option changes nothing" ||
  record_fail "and a rejected option changes nothing" "it linked anyway"

# `--help` prints a line range out of this script's own header — a claim about a file's content held
# by two line numbers, which a line added above the range or a paragraph moved inside it turns into
# the wrong lines, or into none, with nothing here failing. Both ends are pinned by content instead,
# read from the shipped script. The control is the load-bearing half: these needles come out of a
# `sed`, and a `sed` that stopped matching would leave every assertion below comparing the output
# against an empty string, which every output contains.
help_first=$(sed -n '2,$p' "$script" | sed -n 's/^# \{0,1\}\(..*\)$/\1/p' | head -1)
help_usage=$(sed -n 's|^#[[:space:]]*\(usage: ai/bootstrap\.sh .*\)$|\1|p' "$script" | head -1)
if [ -n "$help_first" ] && [ -n "$help_usage" ]; then
  record_pass "control: the header's opening and usage lines were both found, so --help is compared against something"
else
  record_fail "control: the header's opening and usage lines were both found, so --help is compared against something" \
    "opening='$help_first' usage='$help_usage'"
fi

fresh_home
out=$(HOME="$home" bash "$script" --help 2>&1)
status=$?
expect_status "--help exits 0" 0
expect_out "and prints the header's opening line, so the range still starts where it should" "$help_first"
expect_out "and reaches the usage line, so it still ends where it should" "$help_usage"
expect_not_out "and stops before the notes under it" "Safe to re-run"
[ ! -e "$home/.kk-flavor" ] && [ ! -e "$home/.claude" ] &&
  record_pass "and --help changes nothing on the machine" ||
  record_fail "and --help changes nothing on the machine" "something was written under $home"

# Every flag the parser accepts is one the printed header names. Two ways that breaks, and neither
# shows up anywhere else: a flag added to the case statement and never written into the usage line is
# one no reader can find, and a usage line long enough to wrap can wrap out of the printed range, which
# is two line numbers nothing else checks. Both sides are read off the shipped script and its own
# output, so a flag cannot be added to one and missed in the other.
parsed_flags=$(sed -n 's/^    \(--[a-z][a-z-]*\)).*$/\1/p' "$script" | sort -u)
help_flags=$(printf '%s\n' "$out" | grep -oE -- '--[a-z][a-z-]*' | sort -u)
if [ -n "$parsed_flags" ] && [ -n "$help_flags" ]; then
  record_pass "control: flags were found in both the parser and the help output, so this case compares something"
else
  record_fail "control: flags were found in both the parser and the help output, so this case compares something" \
    "parser='$parsed_flags' help='$help_flags'"
fi
if [ "$parsed_flags" = "$help_flags" ]; then
  record_pass "and --help names every flag the parser accepts, and no other"
else
  record_fail "and --help names every flag the parser accepts, and no other" \
    "only in the parser: $(comm -23 <(printf '%s\n' "$parsed_flags") <(printf '%s\n' "$help_flags") | tr '\n' ' ')| only in --help: $(comm -13 <(printf '%s\n' "$parsed_flags") <(printf '%s\n' "$help_flags") | tr '\n' ' ')"
fi

# --- the verify step, and its re-entry guard ------------------------------------------------------

# Verify is the one step every other case skips, so it needs a repository of its own to run against.
# The stub records that it ran and exits 0, which is what lets both directions be asserted: without
# the guard the marker appears, with it the marker does not. A stub rather than the real runner also
# keeps the regression a red case instead of a hang — the real one discovers this suite, which runs
# this script, which is the loop the guard exists to close.
verify_repo="$tmp/verify-repo"
fixture_checkout "$verify_repo" ai
mkdir -p "$verify_repo/ai/skills/a-skill" "$verify_repo/ai/kk-flavor"
# The one file under ai/ this script reads by name.
: >"$verify_repo/ai/CLAUDE.md"
write_stub_runner() { # <exit code>
  cat >"$verify_repo/ai/run-tests.sh" <<STUB
#!/usr/bin/env bash
printf 'ran\n' >>"\$MARKER"
exit $1
STUB
  chmod +x "$verify_repo/ai/run-tests.sh"
}
write_stub_runner 0

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a run with verify enabled exits 0" 0
[ -f "$marker" ] &&
  record_pass "and the verify step actually runs the suite runner" ||
  record_fail "and the verify step actually runs the suite runner" "the runner was never invoked"

# The guard. `run-tests.sh` discovers this suite, which runs this script; without the marker the only
# thing stopping verify from recursing is every caller remembering --skip-verify.
fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" BOOTSTRAP_VERIFYING=1 bash "$verify_repo/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a nested run exits 0" 0
expect_out "and says why verify was skipped" "already inside a verify run"
[ ! -f "$marker" ] &&
  record_pass "and does not re-enter the suite runner" ||
  record_fail "and does not re-enter the suite runner" "the runner ran inside a verify run"

# A runner that exits 2. The runner draws its own line between a suite that failed and one that never
# measured, and the two send a reader to different places — the code, or this machine. Folding them
# into one refusal here blames the suites for a missing dependency, so the wording is what this case
# holds apart; the exit alone cannot tell the two refusals from each other.
write_stub_runner 2

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a runner that could not measure exits 1" 1
expect_out "and says the suites went unproven" "could not measure every suite"
expect_not_out "and does not blame the suites for it" "reported a failing suite"

# A runner that exits 3: it ran the suites and then refused to certify the result, because the checkout
# moved while they ran. Neither the code nor this machine, so it needs a refusal of its own — and the
# runner prints its before/after diff on stdout, which bootstrap discards, so the wording asserted
# below is the whole account the human gets, and the only thing telling this refusal from the others.
write_stub_runner 3

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a runner that refused its own result exits 1" 1
[ -f "$marker" ] &&
  record_pass "control: and the runner really did run, so this is a refusal rather than a missing file" ||
  record_fail "control: and the runner really did run, so this is a refusal rather than a missing file" "the runner was never invoked"
expect_out "and says the suites ran" "ran the suites"
expect_out "and says the checkout moved under them" "the checkout changed while they ran"
expect_out "and says where to look" "Re-run once nothing else is writing here"
expect_not_out "and does not blame the suites for it" "reported a failing suite"
expect_not_out "and does not call it a machine that could not measure" "could not measure every suite"

# A missing runner. Without a guard the call exits 127 and the failing-suite arm blames the suites for
# a file that was never there — a false diagnosis pointing at code that is fine, which costs more than
# the silence would. The two `expect_not_out` assertions are the load-bearing half: the exit alone
# cannot tell a missing runner from a failing one, so only the wording separates them, and this suite
# is the only thing holding them apart. Nothing here may pass because the real repository happens to
# have the runner — the fixture removes it outright.
rm -f "$verify_repo/ai/run-tests.sh"

fresh_home
out=$(HOME="$home" bash "$verify_repo/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a checkout without the suite runner exits 1" 1
expect_out "and says the runner is missing" "is not in this checkout"
expect_out "and says that is not a pass" "not the same as passing"
expect_not_out "and does not blame the suites" "reported a failing suite"

fresh_home
out=$(HOME="$home" bash "$verify_repo/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --dry-run 2>&1)
status=$?
expect_status "a dry run without the suite runner exits 1" 1
expect_not_out "and does not report ok" "ai bootstrap: ok"

# --- a repository with no skills ------------------------------------------------------------------

# Discovery's own vacuity case: an empty skills directory would otherwise mount nothing and report
# success, which is a bootstrap claiming to have set up the agents on a machine that has none.
skeleton="$tmp/skeleton"
fixture_checkout "$skeleton" ai
mkdir -p "$skeleton/ai/skills"
fresh_home
out=$(HOME="$home" bash "$skeleton/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a checkout with no skills exits 1" 1
expect_out "and names a missing source" "is missing from the repository"
expect_out "and refuses an empty skills directory rather than mounting nothing" "nothing was mounted"

# --- a checkout missing the library the two scripts share -----------------------------------------

# Splitting one script into two halves over a shared lib/ is what created this: `.` fails, and without
# `set -e` the run carries on with no add_cfg and no mount_run, exits 127, and says only `command not
# found`. That names neither the file that is gone nor what to do, which is the false diagnosis the
# verify step above already refuses to make. Each script carries its own copy of the guard, because
# the file that would hold one shared copy is the file that is missing — so this case is not
# env/bootstrap-test.sh's covering both, the way the mount library's own behaviour is.
libless="$tmp/libless"
fixture_checkout "$libless" ai
rm -f "$libless/lib/mount.sh"
fresh_home
out=$(HOME="$home" bash "$libless/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a checkout without lib/mount.sh exits 2" 2
expect_out "and names the file that is missing" "lib/mount.sh is missing from this checkout"
expect_not_out "and does not cascade through the mount table instead" "command not found"
[ ! -e "$home/.kk-flavor" ] &&
  record_pass "and nothing was linked" ||
  record_fail "and nothing was linked" "it linked anyway"

# --- a second checkout, and the skills in its count -----------------------------------------------

# The guard itself is env/bootstrap-test.sh's. What only this side has is a bulk mount: a reader told
# their two configs would move, and not that every skill moves with them, has not been told the scale
# of what the run would do. Without a case here the skills could drop out of the count and nothing
# would redden.
other_repo="$tmp_real/other-repo"
fixture_checkout "$other_repo" ai
mkdir -p "$other_repo/ai/kk-flavor"
: >"$other_repo/ai/CLAUDE.md"

# Counted from what the fixture actually created, never written down. It ships skill directories named
# after ones the real repository ships, which is what makes the skill mounts collide — without that the
# skill half of the count would be zero and never exercised.
want_skill=0
for skill_path in $(find "$here/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -2); do
  mkdir -p "$other_repo/ai/skills/$(basename "$skill_path")"
  want_skill=$((want_skill + 1))
done
# Read from the shipped script for the same reason the brew lists are: a config mount added to
# ai/bootstrap.sh and missed here would leave this case asserting a total that no longer covers it.
want_cfg=$(grep -c '^add_cfg "' "$script")
want_total=$((want_cfg + want_skill))

if [ "$want_cfg" -gt 0 ] && [ "$want_skill" -gt 0 ]; then
  record_pass "control: the fixture holds both config and skill mounts, so the count below covers both kinds"
else
  record_fail "control: the fixture holds both config and skill mounts, so the count below covers both kinds" \
    "configs=$want_cfg skills=$want_skill"
fi

fresh_home
HOME="$home" bash "$other_repo/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify >/dev/null 2>&1
first_shared=$(basename "$(find "$here/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)")
expect_link_to "control: the fixture checkout is really what this home is mounted from" \
  "$home/.claude/skills/$first_shared" "$other_repo/ai/skills/$first_shared"
run_boot "$home"
expect_status "a run from a second checkout exits 1" 1
expect_out "and leads with the count, so the skills are not lost behind the named configs" \
  "$want_total mounts ($want_cfg configs and $want_skill skills)"
expect_out "and names the checkout it would have moved them off" "$other_repo"

# The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
# mounts are read back rather than the message being taken at its word.
expect_link_to "and the skill mount was left where the machine had it" \
  "$home/.claude/skills/$first_shared" "$other_repo/ai/skills/$first_shared"

fresh_home
HOME="$home" bash "$other_repo/ai/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify >/dev/null 2>&1
run_boot "$home" --relocate
expect_status "--relocate exits 0" 0
expect_out "and says how many mounts it moved, and off what" "moving $want_total mount(s)"
expect_link_to "and the skill mount now points at this checkout" \
  "$home/.claude/skills/$first_shared" "$here/skills/$first_shared"

# --- the brew list and the README cannot drift apart --------------------------------------------

expect_brew_matches_readme "$script" "$here/README.md"

report_suite
