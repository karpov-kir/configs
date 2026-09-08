#!/usr/bin/env bash
# Cases for ai/bootstrap.sh: the mounts nothing else covers — the instructions, the kk-flavor bucket
# and the discovered skills — the leftover file this script removes, and the verify step with the
# re-entry guard that keeps it from recursing.
#
# The second-checkout guard and the refusal-rather-than-delete rule live in lib/mount.sh and are
# covered in full by env/bootstrap-test.sh. What is asserted here is the half that file cannot reach:
# a bulk mount taking part in the count the guard reports.
#
# Every case runs the real script against a throwaway $HOME.
set -u

# The verify step runs run-tests.sh with BOOTSTRAP_VERIFYING=1, that runner discovers this suite, and
# this suite inherits the marker — so every case below that needs verify to run finds it already
# skipped, and `ai/bootstrap.sh` on a working machine then refuses, blaming the suites for its own
# marker. Cleared once here rather than per invocation: the case at the guard sets the marker on its
# own command line, so a case that means to test the skip still does, and one written tomorrow cannot
# inherit it by forgetting.
unset BOOTSTRAP_VERIFYING CODEX_HOME

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
# makes the suite slow, network-dependent, and able to write to the real MCP registry. Named once, so a
# step that grows a flag cannot pick it up at some of the runs below and reach the network at the
# rest.
skip_network=(--skip-brew --skip-tools --skip-mcp --skip-rtk)
skip_network_and_verify=("${skip_network[@]}" --skip-verify)

run_boot() {
  local home="$1"
  shift
  out=$(HOME="$home" bash "$script" "${skip_network_and_verify[@]}" "$@" 2>&1)
  status=$?
}

# The two files ai/bootstrap.sh needs before it reaches any case's own subject: the bucket it mounts,
# and the instructions it reads by name. The fixtures below that leave one out are testing its absence,
# so they build their checkouts by hand.
fixture_ai_checkout() { # <root>
  local root="$1"
  fixture_checkout "$root" ai
  mkdir -p "$root/ai/kk-flavor"
  : >"$root/ai/owner-instructions.md"
}

echo "ai/bootstrap.sh"

# --- a fresh machine ------------------------------------------------------------------------------

fresh_home
run_boot "$home" --agent=claude
expect_status "a fresh home exits 0" 0
expect_out "and reports ok" "ai bootstrap: ok"
expect_link_to "the flavor bucket is mounted" "$home/.kk-flavor" "$here/kk-flavor"
grep -q "kk-flavor:begin" "$home/.claude/CLAUDE.md" 2>/dev/null &&
  record_pass "the instruction region is written into ~/.claude/CLAUDE.md" ||
  record_fail "the instruction region is written into ~/.claude/CLAUDE.md" "no region there"
[ -L "$home/.claude/CLAUDE.md" ] &&
  record_fail "and the default tier does not mount this checkout's own CLAUDE.md" "it is a symlink" ||
  record_pass "and the default tier does not mount this checkout's own CLAUDE.md"

# Parents the README creates by hand: ~/.claude and ~/.claude/skills do not exist on a fresh machine,
# and a link into a missing directory fails rather than creating it.
[ -d "$home/.claude/skills" ] &&
  record_pass "missing parent directories are created" ||
  record_fail "missing parent directories are created" "~/.claude/skills is absent"

# Discovery, so a skill added later is mounted without editing ai/bootstrap.sh. Compared against the
# repository rather than a hard-coded number, and against the skills a DEFAULT run installs — every
# one that does not declare `audience: maintainer`, so marking a skill does not fail this case.
want_skills=0
for skill_path in "$here"/kk-flavor/skills/*/; do
  [ -d "$skill_path" ] || continue
  grep -q '^audience: maintainer$' "${skill_path}SKILL.md" 2>/dev/null && continue
  want_skills=$((want_skills + 1))
done
got_skills=$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$want_skills" -gt 0 ] && [ "$got_skills" -eq "$want_skills" ] &&
  record_pass "every skill the default tier installs is mounted" ||
  record_fail "every skill the default tier installs is mounted" "mounted $got_skills of $want_skills"

# --- re-running -----------------------------------------------------------------------------------

run_boot "$home" --agent=claude
expect_status "a second run over a finished home exits 0" 0
expect_out "and reports the targets as already ok" "  ok       $home/.kk-flavor"
expect_not_out "and relinks nothing" "linked   $home/.kk-flavor"

# A link the README's old skills loop made reads back with a trailing slash, because `$d` came from a
# `*/` glob. Compared raw, every skill on a machine set up by hand looks stale and gets rewritten on
# every run — idempotence lost to a cosmetic difference, and the noise hides a genuinely stale link.
fresh_home
mkdir -p "$home/.claude/skills"
first_skill=$(find "$here/kk-flavor/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)
fixture_link "$first_skill/" "$home/.claude/skills/$(basename "$first_skill")"
run_boot "$home" --agent=claude
expect_out "a skill link differing only by a trailing slash is left alone" "  ok       $home/.claude/skills/$(basename "$first_skill")"
expect_not_out "and is not rewritten" "repointed $home/.claude/skills/$(basename "$first_skill")"

# --- a mount whose skill this checkout no longer has ----------------------------------------------

fresh_home
mkdir -p "$home/.claude/skills"
fixture_link "$here/kk-flavor/skills/kk-was-renamed" "$home/.claude/skills/kk-was-renamed"
fixture_link "skills/kk-relative" "$home/.claude/skills/kk-relative"
# Run from inside the checkout rather than through run_boot: resolved against the working directory
# instead of against the link that holds it, `skills/kk-relative` would name this checkout's own
# skills/ and be swept with the mount beside it.
out=$(cd "$here" && HOME="$home" bash "$script" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "a mount whose skill is gone from this checkout exits 0" 0
expect_out "and says it removed it" "removed  $home/.claude/skills/kk-was-renamed"
expect_absent "and the mount is actually gone" "$home/.claude/skills/kk-was-renamed"
expect_symlink "and a relative link, which this script never writes, is left alone" \
  "$home/.claude/skills/kk-relative"
expect_link_to "control: and a skill this checkout still has keeps its mount" \
  "$home/.claude/skills/$(basename "$first_skill")" "$first_skill"

fresh_home
mkdir -p "$home/.claude/skills" "$tmp_real/another-checkout/ai/kk-flavor/skills"
fixture_link "$tmp_real/a-skill-of-my-own" "$home/.claude/skills/hand-made"
fixture_link "$tmp_real/another-checkout/ai/kk-flavor/skills/kk-gone" "$home/.claude/skills/kk-gone"
mkdir -p "$home/.claude/skills/copied-in-by-hand"
run_boot "$home" --agent=claude
expect_status "a home holding mounts from elsewhere exits 0" 0
expect_symlink "and a dangling link the human made themselves is left alone" \
  "$home/.claude/skills/hand-made"
expect_symlink "and a dangling mount from another checkout is left alone" \
  "$home/.claude/skills/kk-gone"
[ -d "$home/.claude/skills/copied-in-by-hand" ] &&
  record_pass "and a real directory somebody put there is left alone" ||
  record_fail "and a real directory somebody put there is left alone" "it was removed"
expect_out "and the summary claims only the mounts this checkout wrote" \
  "ok       every mount under $home/.claude/skills this checkout wrote still resolves"

fresh_home
mkdir -p "$home/.claude/skills"
fixture_link "$here/kk-flavor/skills/kk-was-renamed" "$home/.claude/skills/kk-was-renamed"
run_boot "$home" --dry-run --agent=claude
expect_status "--dry-run over a stale mount exits 0" 0
expect_out "and says it would remove it" "would remove $home/.claude/skills/kk-was-renamed"
expect_symlink "and leaves the stale mount where it is" "$home/.claude/skills/kk-was-renamed"

noskills="$tmp_real/noskills"
fixture_ai_checkout "$noskills"
fresh_home
mkdir -p "$home/.claude/skills"
fixture_link "$noskills/ai/kk-flavor/skills/kk-anything" "$home/.claude/skills/kk-anything"
out=$(HOME="$home" bash "$noskills/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "a checkout with no skills directory exits 1" 1
expect_symlink "and every mount it cannot read a source for is left alone" \
  "$home/.claude/skills/kk-anything"
expect_out "and says it checked none of them, rather than reporting them clean" \
  "$noskills/ai/kk-flavor/skills cannot be read, so no mount under $home/.claude/skills was checked"

# A skills directory that is there and holds nothing — a half-finished checkout, or ai/ copied out
# without it. The case above is safe by accident: an unreadable root leaves every mount's own parent
# unreadable too, so the loop skips them whatever the guard does. This root resolves, so the loop can
# act on all of them at once, and every mount of this checkout's dangles. Ungated it takes the
# machine's entire skill set, then reports that nothing was mounted.
emptyskills="$tmp_real/emptyskills"
fixture_ai_checkout "$emptyskills"
mkdir -p "$emptyskills/ai/kk-flavor/skills"
fresh_home
mkdir -p "$home/.claude/skills"
fixture_link "$emptyskills/ai/kk-flavor/skills/kk-was-renamed" "$home/.claude/skills/kk-was-renamed"
out=$(HOME="$home" bash "$emptyskills/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "a checkout whose skills directory is empty exits 1" 1
expect_symlink "and every mount it has no source to compare against is left alone" \
  "$home/.claude/skills/kk-was-renamed"
expect_out "and says it checked none of them" \
  "$emptyskills/ai/kk-flavor/skills holds no source, so no mount under $home/.claude/skills was checked"

mkdir -p "$emptyskills/ai/kk-flavor/skills/kk-still-here"
cat >"$emptyskills/ai/kk-flavor/skills/kk-still-here/SKILL.md" <<'SKILL'
---
name: kk-still-here
description: the one skill this fixture ships
---
SKILL
fresh_home
mkdir -p "$home/.claude/skills"
fixture_link "$emptyskills/ai/kk-flavor/skills/kk-was-renamed" "$home/.claude/skills/kk-was-renamed"
out=$(HOME="$home" bash "$emptyskills/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "control: the same checkout holding one skill exits 0" 0
expect_absent "control: and the stale mount is swept once there is a source to compare against" \
  "$home/.claude/skills/kk-was-renamed"

# --- a mount whose name carries a control byte ------------------------------------------------------

# A skill directory name is text a branch chose, and the removal above quotes it straight back to the
# terminal. `ESC[2K` erases the line it lands in and `ESC[1A` moves to the line above, so a name
# carrying either can wipe the one record that a deletion happened, or the REFUSED line beside it.
# ai/tools/eco-check/mounts_test.go holds this rule for the Go reader of these same mounts; this is the
# shell side of it, asserted over the run that does the removing.
esc=$(printf '\033')
hostile="$tmp_real/hostile-name"
fixture_ai_checkout "$hostile"
mkdir -p "$hostile/ai/kk-flavor/skills/idsd${esc}[2Kgone" "$hostile/ai/kk-flavor/skills/kk-stays"
fresh_home
out=$(HOME="$home" bash "$hostile/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
# The control, and the load-bearing half: without it every assertion below is equally satisfied by a
# run that never mounted the name, and the case would be measuring nothing.
expect_symlink "control: a skill whose directory name carries an ESC is mounted under that name" \
  "$home/.claude/skills/idsd${esc}[2Kgone"

rm -rf "$hostile/ai/kk-flavor/skills/idsd${esc}[2Kgone"
out=$(HOME="$home" bash "$hostile/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "dropping a mount whose name carries an ESC exits 0" 0
expect_out "and still says which mount it removed" "removed  $home/.claude/skills/idsd"
expect_not_out "and no control byte out of that name reaches the terminal" "$esc"
expect_absent "and the mount is gone" "$home/.claude/skills/idsd${esc}[2Kgone"

# --- the file this repository used to write, and now removes ---------------------------------------

fresh_home
run_boot "$home" --owner --agent=claude
expect_status "a fresh home with no leftover exits 0" 0
expect_out "and says there was nothing to remove" "no leftover $home/.claude/RTK.md"
expect_absent "and nothing is written at that path any more" "$home/.claude/RTK.md"

# The case the step exists for: the copy an earlier bootstrap left behind, which is what every machine
# already set up from this repository is holding, and what the next `rtk init -g` puts back.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/RTK.md" 'the copy an earlier bootstrap left here'
run_boot "$home" --owner --agent=claude
expect_status "a leftover file is removed and the run exits 0" 0
expect_out "and says it removed it" "removed  $home/.claude/RTK.md"
expect_absent "and the leftover is actually gone" "$home/.claude/RTK.md"

# A symlink there instead of a copy — what a machine set up from an older README by hand would hold.
# A symlink carries no data of its own, so it goes the same way. What it points at must not: `rm -f` on
# a link does not follow it, and the file the fixture links to is what proves that here.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/pointed-at.md" 'the file the link named'
fixture_link "$home/.claude/pointed-at.md" "$home/.claude/RTK.md"
run_boot "$home" --owner --agent=claude
expect_status "a symlink left at that path is removed too" 0
expect_absent "and the link is gone" "$home/.claude/RTK.md"
expect_file_body "and what it pointed at was not followed and deleted" \
  "$home/.claude/pointed-at.md" 'the file the link named'

# A directory there is not a shape this script ever wrote, so it holds something else and `rm -rf` over
# it is the data loss the header promises this script is not. Asserted from both ends: the refusal, and
# that what was inside it is still inside it.
fresh_home
mkdir -p "$home/.claude/RTK.md"
fixture_write "$home/.claude/RTK.md/notes.md" 'somebody else put this here'
run_boot "$home" --owner --agent=claude
expect_status "a directory at that path exits 1" 1
expect_out "and says it will not remove a directory" "is a directory, and this script only ever wrote a file"
expect_not_out "and does not report having removed it" "removed  $home/.claude/RTK.md"
expect_file_body "and what was inside it survives" "$home/.claude/RTK.md/notes.md" 'somebody else put this here'

# --dry-run over a leftover, which needs a leftover of its own: the --dry-run case below starts from a
# fresh home, where this step has nothing to preview and would report the same either way.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/RTK.md" 'still here afterwards'
run_boot "$home" --owner --dry-run --agent=claude
expect_status "--dry-run over a leftover exits 0" 0
expect_out "and says it would remove it" "would remove the leftover $home/.claude/RTK.md"
expect_file_body "and leaves the leftover alone" "$home/.claude/RTK.md" 'still here afterwards'

# The other half of moving this step to the owner tier: a default run must not touch the file at all,
# so a colleague's machine is never told about a leftover this repository never wrote there.
fresh_home
mkdir -p "$home/.claude"
fixture_write "$home/.claude/RTK.md" 'not ours to remove'
run_boot "$home" --agent=claude
expect_status "a default run exits 0 with a leftover present" 0
expect_out "and says the step is the owner tier's" "rtk is the owner tier's"
expect_file_body "and leaves the file alone" "$home/.claude/RTK.md" 'not ours to remove'

# --- --dry-run ------------------------------------------------------------------------------------

fresh_home
run_boot "$home" --dry-run --agent=claude
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
for skill_path in "$here"/kk-flavor/skills/*/; do
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
run_boot "$home" --agent=claude
expect_status "a default run exits 0" 0

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

# The opt-in, which is the only way a marked skill reaches a machine now. Its absence is covered
# above; this is the half that proves the flag still reaches them, so a default that excluded
# everything could not pass both.
fresh_home
run_boot "$home" --maintainer --agent=claude
missing=""
for skill_name in $maintainer_skills; do
  [ -L "$home/.claude/skills/$skill_name" ] || missing="$missing $skill_name"
done
[ -z "$missing" ] &&
  record_pass "--maintainer mounts a marked skill like any other" ||
  record_fail "--maintainer mounts a marked skill like any other" "not mounted:$missing"

# --- --uninstall ------------------------------------------------------------------------------------

# A documented, published mode with nothing measuring it. What it removes, and what it must not write
# on the way there.
fresh_home
run_boot "$home" --uninstall --agent=claude
expect_status "--uninstall over a machine holding nothing exits 0" 0
# The load-bearing one. Declared above mount_run, the uninstall path links every mount and then
# removes it — so an interrupt between the two leaves the machine installed by the command that exists
# to uninstall it, and a run over a clean machine reads like an install.
expect_not_out "and links nothing on the way" "  linked   "
[ ! -e "$home/.kk-flavor" ] && [ ! -L "$home/.kk-flavor" ] &&
  record_pass "and leaves no bucket behind" ||
  record_fail "and leaves no bucket behind" "$home/.kk-flavor is there"

fresh_home
run_boot "$home" --agent=claude
run_boot "$home" --uninstall --agent=claude
expect_status "--uninstall over an installed machine exits 0" 0
[ ! -e "$home/.kk-flavor" ] && [ ! -L "$home/.kk-flavor" ] &&
  record_pass "and the bucket mount is gone" ||
  record_fail "and the bucket mount is gone" "still there"
[ -z "$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null)" ] &&
  record_pass "and every skill mount is gone" ||
  record_fail "and every skill mount is gone" "$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 -type l | tr '\n' ' ')"

# The tier a machine was installed with is written down nowhere, so an uninstall that re-applies the
# audience filter builds its removal table for the tier being asked for NOW. `--maintainer` in and
# plain out leaves exactly the marked skills mounted, reporting ok.
fresh_home
run_boot "$home" --maintainer --agent=claude
run_boot "$home" --uninstall --agent=claude
left=""
for skill_name in $maintainer_skills; do
  [ -e "$home/.claude/skills/$skill_name" ] || [ -L "$home/.claude/skills/$skill_name" ] &&
    left="$left $skill_name"
done
[ -z "$left" ] &&
  record_pass "a plain --uninstall removes what --maintainer installed" ||
  record_fail "a plain --uninstall removes what --maintainer installed" "still mounted:$left"

fresh_home
run_boot "$home" --skip-maintainer-skills --agent=claude
expect_status "the retired --skip-maintainer-skills is refused, not ignored" 2
expect_out "and names the option it did not know" "unknown option --skip-maintainer-skills"

# The owner tier, reached through its wrapper. What it adds over --maintainer is the instruction
# mount; rtk and the leftover removal need brew and are behind the skip flags here.
fresh_home
out=$(HOME="$home" bash "$here/bootstrap-owner.sh" --agent=claude --skip-rtk --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "the owner wrapper exits 0" 0
[ ! -L "$home/.claude/CLAUDE.md" ] && cmp -s "$home/.claude/CLAUDE.md" "$here/owner-instructions.md" && record_pass "Claude owner receives a regular copy" || record_fail "Claude owner receives a regular copy" "wrong content or symlink"

# Discovery's second vacuity case. A checkout where the flag excludes every skill mounts nothing, and
# the empty-tree refusal would report that as a skills directory holding no skill — a false diagnosis
# sending the reader to look for files that are all there. The exit code is the same either way, so the
# wording is the only thing telling the two apart.
# Under $tmp_real, not $tmp: the script resolves its own repository with `pwd -P`, so a fixture reached
# through the unresolved /var symlink mounts links this suite would then compare against the other
# spelling of the same path.
only_maintainer="$tmp_real/only-maintainer"
fixture_ai_checkout "$only_maintainer"
mkdir -p "$only_maintainer/ai/kk-flavor/skills/kk-ecosystem"
cat >"$only_maintainer/ai/kk-flavor/skills/kk-ecosystem/SKILL.md" <<'SKILL'
---
name: kk-ecosystem
description: the one skill this fixture ships
audience: maintainer
---
SKILL

fresh_home
out=$(HOME="$home" bash "$only_maintainer/ai/bootstrap.sh" --agent=claude \
  "${skip_network_and_verify[@]}" 2>&1)

status=$?
expect_status "a checkout whose every skill is marked exits 1" 1
expect_out "and says the default is what excluded them" "excluded all 1"
expect_not_out "and does not report the tree as holding no skill at all" "no skill directories under"

# An audience nothing reads. The marker check answers "not marked" to a misspelling and to a skill that
# declared nothing, and those mean opposite things — so the misspelling installs for everyone while
# whoever typed it believes they marked it, on a machine where nothing looks wrong.
typo_audience="$tmp_real/typo-audience"
fixture_ai_checkout "$typo_audience"
mkdir -p "$typo_audience/ai/kk-flavor/skills/kk-typo"
cat >"$typo_audience/ai/kk-flavor/skills/kk-typo/SKILL.md" <<'SKILL'
---
name: kk-typo
description: the one skill this fixture ships
audience: maintainr
---
SKILL

fresh_home
out=$(HOME="$home" bash "$typo_audience/ai/bootstrap.sh" --agent=claude \
  "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "a misspelled audience exits 1 rather than installing quietly" 1
expect_out "and echoes back what was written, so it can be found in the file" "maintainr"
expect_out "and names the one value there is" "audience: maintainer"

# The control. Without it every assertion above is equally satisfied by a script that refuses each
# skill it reads, and the suite would be measuring nothing.
fresh_home
sed -i.bak 's/^audience: maintainr$/audience: maintainer/' "$typo_audience/ai/kk-flavor/skills/kk-typo/SKILL.md"
# --maintainer, because the tree's only skill is marked: without it the default tier correctly
# excludes the lot and this control would be asserting the exclusion rather than the marker.
out=$(HOME="$home" bash "$typo_audience/ai/bootstrap.sh" --agent=claude \
  "${skip_network_and_verify[@]}" --maintainer 2>&1)

status=$?
expect_status "control: the same tree with the marker spelled right exits 0" 0
expect_not_out "control: and refuses nothing" "no reader knows"

# The control, and the load-bearing half: the same checkout with --maintainer mounts its one skill.
# Without it the refusal above would pass over a fixture that never had a skill to mount.
fresh_home
out=$(HOME="$home" bash "$only_maintainer/ai/bootstrap.sh" --agent=claude \
  "${skip_network_and_verify[@]}" --maintainer 2>&1)

status=$?
expect_status "control: the same checkout with --maintainer exits 0" 0
expect_link_to "control: and mounts its one skill" \
  "$home/.claude/skills/kk-ecosystem" "$only_maintainer/ai/kk-flavor/skills/kk-ecosystem"

# --- arguments ------------------------------------------------------------------------------------

# The skip flags ride along even though the option check runs before any of them are consulted. If
# that check ever stops exiting, this case must fail rather than proceed into brew, gh, the claude CLI
# and a verify run that discovers this very suite — a regression should redden, not install things.
fresh_home
out=$(HOME="$home" bash "$script" --agent=claude "${skip_network_and_verify[@]}" --not-a-flag 2>&1)
status=$?
expect_status "an unknown option exits 2" 2
expect_out "and names the option it rejected" "--not-a-flag"
expect_absent "and a rejected option changes nothing" "$home/.kk-flavor"

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
out=$(HOME="$home" bash "$script" --agent=claude --help 2>&1)
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
parsed_flags=$(sed -n 's/^    \(--[a-z][a-z-]*\)[=)].*$/\1/p' "$script" | sort -u)
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

# --- the tools step -------------------------------------------------------------------------------

# install.sh's non-zero codes send a reader to different places: 2 to this machine's network, its auth
# or the release's own assets, 3 to the fact that this repository has cut no release at all. Only the
# second is survivable, and collapsing it into the refusal fails every fresh clone until the first
# release exists. A stub rather than the real installer, for the reason the verify stubs give: what is
# asserted is bootstrap's reading of an exit code, and the real one reaches the network to produce one.
tools_repo="$tmp/tools-repo"
fixture_ai_checkout "$tools_repo"
mkdir -p "$tools_repo/ai/kk-flavor/skills/a-skill" "$tools_repo/ai/tools"

# The tools step refuses before install.sh runs at all on a machine without gh, and whether this
# machine has one is not what any case below is about.
tools_path="$tmp/tools-path"
mkdir -p "$tools_path"
printf '#!/usr/bin/env bash\nexit 0\n' >"$tools_path/gh"
chmod +x "$tools_path/gh"

write_stub_installer() { # <exit code>
  cat >"$tools_repo/ai/tools/install.sh" <<STUB
#!/usr/bin/env bash
printf 'ran\n' >>"\$MARKER"
exit $1
STUB
  chmod +x "$tools_repo/ai/tools/install.sh"
}

# Every step but this one skipped, so what the exit status reports is the tools step alone.
tools_only=(--skip-brew --skip-mcp --skip-verify)

run_tools_boot() {
  marker="$tmp/tools-marker-$case_no"
  out=$(HOME="$home" MARKER="$marker" PATH="$tools_path:$PATH" \
    bash "$tools_repo/ai/bootstrap.sh" --agent=claude "${tools_only[@]}" 2>&1)
  status=$?
}

write_stub_installer 0
fresh_home
run_tools_boot
expect_status "an installer that installed exits 0" 0
[ -f "$marker" ] &&
  record_pass "control: and the installer really ran, so the arms below read a real exit code" ||
  record_fail "control: and the installer really ran, so the arms below read a real exit code" "it was never invoked"

# 3: the repository has cut no release. Nothing here is broken and nobody at this machine can cut one,
# so it must not become a refusal — and the wording is the whole of what tells it from the arm below.
write_stub_installer 3
fresh_home
run_tools_boot
expect_status "an installer reporting no release to install from exits 0" 0
expect_out "and says the tools build from source instead" "build from source on first use"
expect_out "and names what that needs" "needs Go"
expect_not_out "and does not report it as a failure" "ai/tools/install.sh failed"

# 2: every other outcome install.sh has, which still fails the run. Without this the arm above could
# be a blanket "the installer's exit code is ignored" and read exactly the same.
write_stub_installer 2
fresh_home
run_tools_boot
expect_status "an installer that refused exits 1" 1
expect_out "and says the installer failed" "ai/tools/install.sh failed"
expect_not_out "and does not call it an absent release" "build from source on first use"

# --- the verify step, and its re-entry guard ------------------------------------------------------

# Verify is the one step every other case skips, so it needs a repository of its own to run against.
# The stub records that it ran and exits 0, which is what lets both directions be asserted: without
# the guard the marker appears, with it the marker does not. A stub rather than the real runner also
# keeps the regression a red case instead of a hang — the real one discovers this suite, which runs
# this script, which is the loop the guard exists to close.
verify_repo="$tmp/verify-repo"
fixture_ai_checkout "$verify_repo"
mkdir -p "$verify_repo/ai/kk-flavor/skills/a-skill"

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
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" --agent=claude \
  "${skip_network[@]}" 2>&1)
status=$?
expect_status "a run with verify enabled exits 0" 0
[ -f "$marker" ] &&
  record_pass "and the verify step actually runs the suite runner" ||
  record_fail "and the verify step actually runs the suite runner" "the runner was never invoked"

# The guard. `run-tests.sh` discovers this suite, which runs this script; without the marker the only
# thing stopping verify from recursing is every caller remembering --skip-verify.
fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" BOOTSTRAP_VERIFYING=1 bash "$verify_repo/ai/bootstrap.sh" --agent=claude \
  "${skip_network[@]}" 2>&1)
status=$?
expect_status "a nested run exits 0" 0
expect_out "and says why verify was skipped" "already inside a verify run"
expect_absent "and does not re-enter the suite runner" "$marker"

# A runner that exits 2. The runner draws its own line between a suite that failed and one that never
# measured, and the two send a reader to different places — the code, or this machine. Folding them
# into one refusal here blames the suites for a missing dependency, so the wording is what this case
# holds apart; the exit alone cannot tell the two refusals from each other.
write_stub_runner 2

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" --agent=claude \
  "${skip_network[@]}" 2>&1)
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
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/ai/bootstrap.sh" --agent=claude \
  "${skip_network[@]}" 2>&1)
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
out=$(HOME="$home" bash "$verify_repo/ai/bootstrap.sh" --agent=claude "${skip_network[@]}" 2>&1)
status=$?
expect_status "a checkout without the suite runner exits 1" 1
expect_out "and says the runner is missing" "is not in this checkout"
expect_out "and says that is not a pass" "not the same as passing"
expect_not_out "and does not blame the suites" "reported a failing suite"

fresh_home
out=$(HOME="$home" bash "$verify_repo/ai/bootstrap.sh" --agent=claude "${skip_network[@]}" --dry-run 2>&1)
status=$?
expect_status "a dry run without the suite runner exits 1" 1
expect_not_out "and does not report ok" "ai bootstrap: ok"

# --- a repository with no skills ------------------------------------------------------------------

# Discovery's own vacuity case: an empty skills directory would otherwise mount nothing and report
# success, which is a bootstrap claiming to have set up the agents on a machine that has none.
skeleton="$tmp/skeleton"
fixture_checkout "$skeleton" ai
mkdir -p "$skeleton/ai/kk-flavor/skills"
fresh_home
out=$(HOME="$home" bash "$skeleton/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" --owner 2>&1)

status=$?
expect_status "a checkout with no skills exits 1" 1
expect_out "and names a missing source" "must be a regular owner instruction source"
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
out=$(HOME="$home" bash "$libless/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" 2>&1)
status=$?
expect_status "a checkout without lib/mount.sh exits 2" 2
expect_out "and names the file that is missing" "lib/mount.sh is missing from this checkout"
expect_not_out "and does not cascade through the mount table instead" "command not found"
expect_absent "and nothing was linked" "$home/.kk-flavor"

# --- a second checkout, and the skills in its count -----------------------------------------------

# The guard itself is env/bootstrap-test.sh's. What only this side has is a bulk mount: a reader told
# their two configs would move, and not that every skill moves with them, has not been told the scale
# of what the run would do. Without a case here the skills could drop out of the count and nothing
# would redden.
other_repo="$tmp_real/other-repo"
fixture_ai_checkout "$other_repo"

# Counted from what the fixture actually created, never written down. It ships skill directories named
# after ones the real repository ships, which is what makes the skill mounts collide — without that the
# skill half of the count would be zero and never exercised.
want_skill=0
for skill_path in $(find "$here/kk-flavor/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -2); do
  mkdir -p "$other_repo/ai/kk-flavor/skills/$(basename "$skill_path")"
  want_skill=$((want_skill + 1))
done
# Read from the shipped script for the same reason the brew lists are: a config mount added to
# ai/bootstrap.sh and missed here would leave this case asserting a total that no longer covers it.
want_cfg=$(grep -c 'add_cfg "$repo/kk-flavor"' "$script")
want_total=$((want_cfg + want_skill))

if [ "$want_cfg" -gt 0 ] && [ "$want_skill" -gt 0 ]; then
  record_pass "control: the fixture holds both config and skill mounts, so the count below covers both kinds"
else
  record_fail "control: the fixture holds both config and skill mounts, so the count below covers both kinds" \
    "configs=$want_cfg skills=$want_skill"
fi

fresh_home
HOME="$home" bash "$other_repo/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" >/dev/null 2>&1
first_shared=$(basename "$(find "$here/kk-flavor/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)")

expect_link_to "control: the fixture checkout is really what this home is mounted from" \
  "$home/.claude/skills/$first_shared" "$other_repo/ai/kk-flavor/skills/$first_shared"
run_boot "$home" --agent=claude
expect_status "a run from a second checkout exits 1" 1
expect_out "and leads with the count, so the skills are not lost behind the named configs" \
  "$want_total mounts ($want_cfg configs and $want_skill skills)"
expect_out "and names the checkout it would have moved them off" "$other_repo"

# The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
# mounts are read back rather than the message being taken at its word.
expect_link_to "and the skill mount was left where the machine had it" \
  "$home/.claude/skills/$first_shared" "$other_repo/ai/kk-flavor/skills/$first_shared"

fresh_home
HOME="$home" bash "$other_repo/ai/bootstrap.sh" --agent=claude "${skip_network_and_verify[@]}" >/dev/null 2>&1
run_boot "$home" --relocate --agent=claude
expect_status "--relocate exits 0" 0
expect_out "and says how many mounts it moved, and off what" "moving $want_total mount(s)"
expect_link_to "and the skill mount now points at this checkout" \
  "$home/.claude/skills/$first_shared" "$here/kk-flavor/skills/$first_shared"


# --- ai/owner-instructions.md carries the same region body every other tier is given ---------------------------

# The owner tier copies ai/owner-instructions.md, so its reader gets that file's own text; everyone else gets the
# fenced region flavor_region_body() writes. The two say the same thing and cannot be derived from one
# another — generating three lines would cost a generator and a gate unit to keep it honest — so this
# is what catches the wording drifting apart. Compared as the body's lines, not as a whole file: the
# owner's copy sits under a heading and beside prose the fenced copy has no business carrying.
region_body_missing=""
while IFS= read -r region_line; do
  [ -n "$region_line" ] || continue
  grep -qF -- "$region_line" "$here/owner-instructions.md" || region_body_missing="$region_line"
done < <(
  # shellcheck source=../lib/flavor-region.sh
  . "$checkout/lib/flavor-region.sh" && flavor_region_body
)
[ -z "$region_body_missing" ] &&
  record_pass "ai/owner-instructions.md carries every line of the region body the other tiers are given" ||
  record_fail "ai/owner-instructions.md carries every line of the region body the other tiers are given" \
    "missing: $region_body_missing"

# --- the brew list and the README cannot drift apart --------------------------------------------

expect_brew_matches_readme "$script" "$here/README.md"

# --- Codex target -------------------------------------------------------------------------------
fresh_home
run_boot "$home" --agent=unknown
expect_status "an unknown agent is rejected before installing" 2

fresh_home
out=$(HOME="$home" CODEX_HOME="$home/codex-profile" bash "$script" "${skip_network_and_verify[@]}" --agent=codex 2>&1)
status=$?
expect_status "Codex installs into a custom profile" 0
expect_link_to "Codex skills use the shared discovery directory" "$home/.agents/skills/kk-build" "$here/kk-flavor/skills/kk-build"
[ -f "$home/codex-profile/AGENTS.md" ] && record_pass "Codex instructions respect CODEX_HOME" || record_fail "Codex instructions respect CODEX_HOME" "missing AGENTS.md"
[ ! -e "$home/.claude" ] && record_pass "Codex install does not create Claude state" || record_fail "Codex install does not create Claude state" "created .claude"

fresh_home
mkdir -p "$home/.codex/skills"
ln -s "$here/kk-flavor/skills/kk-build" "$home/.codex/skills/kk-build"
run_boot "$home" --agent=codex
expect_status "Codex migrates its existing mounts" 0
[ ! -L "$home/.codex/skills/kk-build" ] && record_pass "the duplicate old Codex mount is removed" || record_fail "the duplicate old Codex mount is removed" "old link remains"
run_boot "$home" --agent=codex
expect_status "a second Codex install succeeds" 0
expect_not_out "a second Codex install writes no links" "linked   "
run_boot "$home" --agent=claude
expect_status "Claude can coexist with Codex" 0
run_boot "$home" --agent=codex --uninstall
expect_status "Codex can be uninstalled independently" 0
expect_link_to "Claude retains the shared bucket" "$home/.kk-flavor" "$here/kk-flavor"
expect_link_to "Claude retains its skills" "$home/.claude/skills/kk-build" "$here/kk-flavor/skills/kk-build"

fresh_home
mkdir -p "$home/.codex"
printf 'custom override\n' >"$home/.codex/AGENTS.override.md"
run_boot "$home" --agent=codex
expect_status "Codex refuses a shadowed instruction file" 1
expect_out "Codex explains the override" "AGENTS.override.md"

fresh_home
out=$(HOME="$home" CODEX_HOME="$home/.agents" bash "$script" "${skip_network_and_verify[@]}" --agent=codex 2>&1)
status=$?
expect_status "CODEX_HOME can equal the shared discovery directory" 0
expect_link_to "migration never removes its own destination" "$home/.agents/skills/kk-build" "$here/kk-flavor/skills/kk-build"

fresh_home
mkdir -p "$home/.claude"
printf 'existing rtk\n' >"$home/.claude/RTK.md"
run_boot "$home" --agent=codex --owner
expect_status "Codex owner install succeeds" 0
expect_link_to "Codex owner includes maintainer skills" "$home/.agents/skills/kk-ecosystem" "$here/kk-flavor/skills/kk-ecosystem"
[ ! -L "$home/.codex/AGENTS.md" ] && cmp -s "$home/.codex/AGENTS.md" "$here/owner-instructions.md" && record_pass "Codex owner receives its own source copy" || record_fail "Codex owner receives its own source copy" "wrong content or symlink"
[ -f "$home/.claude/RTK.md" ] && record_pass "Codex owner preserves Claude RTK state" || record_fail "Codex owner preserves Claude RTK state" "removed RTK.md"
run_boot "$home" --agent=codex --owner --uninstall
expect_status "Codex owner uninstall succeeds" 0
expect_absent "the final client removes the bucket" "$home/.kk-flavor"

fresh_home
mkdir -p "$home/.codex/skills" "$home/.agents/skills/kk-build"
ln -s "$here/kk-flavor/skills/kk-build" "$home/.codex/skills/kk-build"
run_boot "$home" --agent=codex
expect_status "Codex refuses an occupied skill destination" 1
expect_link_to "a failed migration preserves its old skill" "$home/.codex/skills/kk-build" "$here/kk-flavor/skills/kk-build"

fresh_home
mkdir -p "$home/.agents"
ln -s "$home/.agents" "$home/profile"
out=$(HOME="$home" CODEX_HOME="$home/profile" bash "$script" "${skip_network_and_verify[@]}" --agent=codex 2>&1)
status=$?
expect_status "Codex accepts a profile alias to the discovery directory" 0
expect_link_to "profile aliases cannot delete migrated skills" "$home/.agents/skills/kk-build" "$here/kk-flavor/skills/kk-build"
out=$(HOME="$home" CODEX_HOME="$home/profile" bash "$script" "${skip_network_and_verify[@]}" --agent=codex --uninstall 2>&1)
status=$?
expect_status "Codex uninstalls through a profile alias" 0
expect_absent "a profile alias is not counted as another client" "$home/.kk-flavor"

fresh_home
mkdir -p "$home/.codex/skills"
ln -s "$here/kk-flavor/skills/kk-build/" "$home/.codex/skills/kk-build"
run_boot "$home" --agent=codex
expect_status "Codex migrates a legacy link ending in a slash" 0
expect_absent "equivalent legacy links do not duplicate skill discovery" "$home/.codex/skills/kk-build"

fresh_home
for entry in "$script" "$here/bootstrap-owner.sh"; do
  for mode in '' --uninstall --dry-run; do
    out=$(HOME="$home" bash "$entry" "${skip_network_and_verify[@]}" $mode 2>&1)
    status=$?
    expect_status "a missing target is rejected by $entry $mode" 2
    expect_out "the refusal names the required selector" "--agent=claude|codex is required"
    expect_absent "no target means no installation" "$home/.kk-flavor"
  done
done

report_suite
