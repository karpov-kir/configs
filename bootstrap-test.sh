#!/usr/bin/env bash
# Cases for bootstrap.sh. The one that must not be weakened is "a real file at a target is refused,
# not deleted": README.md's hand-run form is `rm -rf` and this script exists to not be that, so a
# regression there is silent data loss on somebody's machine rather than a failing check.
#
# Every case runs the real script against a throwaway $HOME and passes every --skip flag, so brew, gh
# and claude are never invoked. What is actually exercised is the linking and the refusals, which is
# the part with the branches.
set -u

# bootstrap.sh's verify step runs ai/run-tests.sh with BOOTSTRAP_VERIFYING=1, that runner discovers
# this suite, and this suite inherits the marker — so every case below that needs verify to run finds
# it already skipped, and `./bootstrap.sh` on a working machine then refuses, blaming the suites for
# its own marker. Cleared once here rather than per invocation: the case at the guard sets the marker
# on its own command line, so a case that means to test the skip still does, and one written tomorrow
# cannot inherit it by forgetting.
unset BOOTSTRAP_VERIFYING

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
script="$here/bootstrap.sh"

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT

# Resolved physically once, because `mktemp -d` hands back `/var/folders/…` on macOS while `/var` is
# itself a symlink to `/private/var`. Comparing an unresolved root against resolved paths would make
# the containment check below refuse everything, and a guard that always fires gets deleted.
tmp_real=$(cd -P "$tmp" && pwd -P) || exit 1

# Every fixture write goes through `fixture_write` or `fixture_link`, and this is why.
#
# The cases below run the *real* `bootstrap.sh`, which derives its repository from its own location,
# so a run leaves `$home/.config/nvim` pointing at this checkout's `nvim/`. That is correct behaviour
# and harmless while each case gets its own home. It stops being harmless the moment two cases share
# one: the second case's `mkdir -p "$home/.config/nvim"` then finds a live symlink into the checkout
# and succeeds, and the fixture write that follows goes straight through it into a real config file.
#
# That is not hypothetical. An earlier `fresh_home` was called as `home=$(fresh_home)`, which
# incremented its counter inside a subshell and handed every case the same home. It overwrote
# `nvim/init.lua` and `starship/starship.toml` in the working tree and left a stray symlink in
# `nvim/`. The suite reported it, too — a case failed saying something had been written where nothing
# should be — and the report was read as a harness bug without asking what the broken run had already
# written to disk.
#
# So the containment is asserted before each write rather than noticed after: the parent is resolved
# physically, following any symlink in the path, and anything landing outside `$tmp` aborts the whole
# suite. A guard that reports afterwards has still lost the file.
refuse_fixture() {
  printf 'bootstrap-test.sh: refusing to write %s — %s\n' "$1" "$2" >&2
  printf '  the suite may only write under %s; this is the containment guard, not a failing case\n' "$tmp_real" >&2
  exit 2
}

contained_parent() {
  local path="$1" parent
  parent=$(cd -P "$(dirname -- "$path")" 2>/dev/null && pwd -P) ||
    refuse_fixture "$path" "its parent directory does not resolve"
  case "$parent/" in
    "$tmp_real"/*) printf '%s' "$parent" ;;
    *) refuse_fixture "$path" "its parent resolves to $parent" ;;
  esac
}

fixture_write() {
  local path="$1" body="$2"
  contained_parent "$path" >/dev/null
  printf '%s\n' "$body" >|"$path"
}

fixture_link() {
  local source="$1" target="$2"
  contained_parent "$target" >/dev/null
  # `ln -s X Y` where Y already exists as a symlink to a directory creates the link *inside* Y rather
  # than replacing it, which is how a stray link ends up in the checkout. Refused rather than forced:
  # every fixture link in this suite is meant to be the first thing at its path.
  [ ! -L "$target" ] ||
    refuse_fixture "$target" "it already exists as a symlink to $(readlink "$target")"
  ln -s "$source" "$target"
}

passed=0
failed=0

record_pass() {
  passed=$((passed + 1))
  echo "  pass  $1"
}

record_fail() {
  failed=$((failed + 1))
  echo "  FAIL  $1  — $2"
}

# No skip counter here, unlike score-test.sh. That suite reports a third field because it has a case
# it genuinely cannot run as root, and hiding that would make two machines checking different sets
# look identical. Every case here runs at every uid: the one `chmod` in this file grants a bit rather
# than restricting one, so there is nothing for a skip to guard. A field that can never be non-zero is
# decoration wearing the shape of a measurement.

# A fresh home per case, so no case inherits another's links. Numbered rather than mktemp'd again so a
# failure message names which case's home to go and look at.
#
# It sets `home` rather than printing it: read back through `home=$(fresh_home)` the counter would be
# incremented inside a subshell, every case would be handed `home1`, and each would inherit the first
# case's links. What that cost once is in the containment note above.
case_no=0
fresh_home() {
  case_no=$((case_no + 1))
  home="$tmp/home$case_no"
  mkdir -p "$home"
}

# The skip flags are load-bearing: without them a case shells out to brew, gh and the claude CLI, which
# makes the suite slow, network-dependent, and able to write to the real MCP registry.
run_boot() {
  local home="$1"
  shift
  out=$(HOME="$home" bash "$script" --skip-brew --skip-tools --skip-mcp --skip-verify "$@" 2>&1)
  status=$?
}

expect_status() {
  local name="$1" want="$2"
  [ "$status" -eq "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "exit $status, wanted $want"
}

expect_out() {
  local name="$1" want="$2"
  case "$out" in
    *"$want"*) record_pass "$name" ;;
    *) record_fail "$name" "wanted '$want' in: $out" ;;
  esac
}

expect_not_out() {
  local name="$1" unwanted="$2"
  case "$out" in
    *"$unwanted"*) record_fail "$name" "found '$unwanted' in: $out" ;;
    *) record_pass "$name" ;;
  esac
}

expect_link_to() {
  local name="$1" target="$2" want="$3"
  if [ ! -L "$target" ]; then
    record_fail "$name" "$target is not a symlink"
    return
  fi
  [ "$(readlink "$target")" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$target -> $(readlink "$target"), wanted $want"
}

expect_file_body() {
  local name="$1" path="$2" want="$3" got
  got=$(cat "$path" 2>/dev/null)
  [ "$got" = "$want" ] &&
    record_pass "$name" ||
    record_fail "$name" "$path holds '$got', wanted '$want'"
}

echo "bootstrap.sh"

# --- a fresh machine ------------------------------------------------------------------------------

fresh_home
run_boot "$home"
expect_status "a fresh home exits 0" 0
expect_out "and reports ok" "bootstrap: ok"
# A machine with nothing mounted yet has nothing for the second-checkout guard to protect, so it must
# pass straight through. Asserted on the wording rather than on the exit, because a guard that printed
# nothing when it passes is indistinguishable from one that was never reached.
expect_out "and the second-checkout guard passes rather than staying silent" "no mount on this machine comes from another checkout"
expect_link_to "the flavor bucket is mounted" "$home/.kk-flavor" "$here/ai/kk-flavor"
expect_link_to "CLAUDE.md is linked" "$home/.claude/CLAUDE.md" "$here/ai/CLAUDE.md"
expect_link_to "a directory config is linked" "$home/.config/nvim" "$here/nvim"
expect_link_to "a file config is linked" "$home/.config/starship.toml" "$here/starship/starship.toml"

# Parents the README creates by hand: ~/.config, ~/.claude and ~/.claude/skills do not exist on a
# fresh machine, and a link into a missing directory fails rather than creating it.
[ -d "$home/.config" ] && [ -d "$home/.claude/skills" ] &&
  record_pass "missing parent directories are created" ||
  record_fail "missing parent directories are created" "one of ~/.config, ~/.claude/skills is absent"

# Discovery, so a skill added later is mounted without editing bootstrap.sh. Compared against the
# repository rather than a hard-coded number, which would drift the day a skill lands.
want_skills=$(find "$here/ai/skills" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
got_skills=$(find "$home/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$want_skills" -gt 0 ] && [ "$got_skills" -eq "$want_skills" ] &&
  record_pass "every skill directory is mounted" ||
  record_fail "every skill directory is mounted" "mounted $got_skills of $want_skills"

# --- re-running -----------------------------------------------------------------------------------

# Idempotence is the property that makes this safe to run on a working machine, and the only evidence
# for it is a second run over the first run's output.
run_boot "$home"
expect_status "a second run over a finished home exits 0" 0
expect_out "and reports the targets as already ok" "  ok       $home/.kk-flavor"
expect_not_out "and relinks nothing" "linked   $home/.kk-flavor"

# --- the refusal that matters ---------------------------------------------------------------------

# README.md's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`. A script doing that unattended
# destroys a real config. The body is asserted afterwards, so a version that refused *and* deleted
# would still fail here.
fresh_home
mkdir -p "$home/.config/nvim"
fixture_write "$home/.config/nvim/init.lua" 'my real config'
run_boot "$home"
expect_status "a real directory at a target exits 1" 1
expect_out "and says what to do about it" "exists and is not a symlink"
expect_file_body "and the real config is still there, byte for byte" "$home/.config/nvim/init.lua" "my real config"
[ ! -L "$home/.config/nvim" ] &&
  record_pass "and the target was not replaced by a link" ||
  record_fail "and the target was not replaced by a link" "it became a symlink"

# A real file is the same hazard in the other shape, and takes the other branch of `-e`.
fresh_home
mkdir -p "$home/.config"
fixture_write "$home/.config/starship.toml" 'hand-written prompt'
run_boot "$home"
expect_status "a real file at a target exits 1" 1
expect_file_body "and the real file survives" "$home/.config/starship.toml" "hand-written prompt"

# One refusal must not abort the rest: a machine with one stray file should still get every other
# link, or fixing them becomes one run per problem.
expect_link_to "and the other links are still made" "$home/.kk-flavor" "$here/ai/kk-flavor"

# A link the README's own loop made reads back with a trailing slash, because `$d` came from a `*/`
# glob. Compared raw, every skill on a machine set up by hand looks stale and gets rewritten on every
# run — idempotence lost to a cosmetic difference, and the noise hides a genuinely stale link.
fresh_home
mkdir -p "$home/.claude/skills"
first_skill=$(find "$here/ai/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)
fixture_link "$first_skill/" "$home/.claude/skills/$(basename "$first_skill")"
run_boot "$home"
expect_out "a link differing only by a trailing slash is left alone" "  ok       $home/.claude/skills/$(basename "$first_skill")"
expect_not_out "and is not rewritten" "repointed $home/.claude/skills/$(basename "$first_skill")"

# --- a stale symlink ------------------------------------------------------------------------------

# A symlink carries no data of its own, so repointing one loses nothing — this is the only target a
# link may be written over, and a stale one is what an older layout leaves behind.
fresh_home
mkdir -p "$home/.config"
fixture_link "$tmp/somewhere-else" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale symlink does not refuse" 0
expect_link_to "and is repointed at the repository" "$home/.config/nvim" "$here/nvim"
expect_out "and says so rather than reporting ok" "repointed $home/.config/nvim"

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

# The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
# it is the run that changed their machine.
fresh_home
run_boot "$home" --dry-run
expect_status "--dry-run exits 0" 0
expect_out "and says what it would do" "would link"
[ ! -e "$home/.kk-flavor" ] && [ ! -e "$home/.config" ] && [ ! -e "$home/.claude" ] &&
  record_pass "--dry-run creates nothing at all" ||
  record_fail "--dry-run creates nothing at all" "something was written under $home"

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
# the wrong lines, or into none, with nothing here failing. Repeating those numbers in this file
# would move the rot rather than remove it: an ordinary header edit would then redden a case that is
# right. So both ends are pinned by content instead — the header line the range has to start at, the
# usage line it has to reach, and the paragraph past it that has to stay out. Read from the shipped
# script, so editing the usage line does not redden this.
#
# The control is the load-bearing half: these needles come out of a `sed`, and a `sed` that stopped
# matching would leave every assertion below comparing the output against an empty string, which
# every output contains. Three passes over nothing, reported as three passes.
help_first=$(sed -n '2,$p' "$script" | sed -n 's/^# \{0,1\}\(..*\)$/\1/p' | head -1)
help_usage=$(sed -n 's/^#[[:space:]]*\(usage: bootstrap\.sh .*\)$/\1/p' "$script" | head -1)
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

# --- the verify step, and its re-entry guard ------------------------------------------------------

# Verify is the one step every other case skips, so it needs a repository of its own to run against.
# The stub records that it ran and exits 0, which is what lets both directions be asserted: without
# the guard the marker appears, with it the marker does not. A stub rather than the real runner also
# keeps the regression a red case instead of a hang — the real one discovers this suite, which runs
# this script, which is the loop the guard exists to close.
verify_repo="$tmp/verify-repo"
mkdir -p "$verify_repo/ai/skills/a-skill" "$verify_repo/ai/kk-flavor"
# The one file under ai/ this script reads by name.
: >"$verify_repo/ai/CLAUDE.md"
for name in zsh git ghostty lazygit nvim zellij starship; do
  ln -s "$here/$name" "$verify_repo/$name"
done
cp "$script" "$verify_repo/bootstrap.sh"
cat >"$verify_repo/ai/run-tests.sh" <<STUB
#!/usr/bin/env bash
printf 'ran\n' >>"\$MARKER"
exit 0
STUB
chmod +x "$verify_repo/ai/run-tests.sh"

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a run with verify enabled exits 0" 0
[ -f "$marker" ] &&
  record_pass "and the verify step actually runs the suite runner" ||
  record_fail "and the verify step actually runs the suite runner" "the runner was never invoked"

# The guard. `ai/run-tests.sh` discovers this suite, which runs this script; without the marker the
# only thing stopping verify from recursing is every caller remembering --skip-verify.
fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" BOOTSTRAP_VERIFYING=1 bash "$verify_repo/bootstrap.sh" \
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
cat >"$verify_repo/ai/run-tests.sh" <<STUB
#!/usr/bin/env bash
printf 'ran\n' >>"\$MARKER"
echo "run-tests.sh: no *-test.sh under here" >&2
exit 2
STUB
chmod +x "$verify_repo/ai/run-tests.sh"

fresh_home
marker="$tmp/verify-marker-$case_no"
out=$(HOME="$home" MARKER="$marker" bash "$verify_repo/bootstrap.sh" \
  --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a runner that could not measure exits 1" 1
expect_out "and says the suites went unproven" "could not measure every suite"
expect_not_out "and does not blame the suites for it" "reported a failing suite"

# A missing runner, which is what a fresh clone had while `ai/run-tests.sh` was untracked. Without a
# guard the call exits 127 and the failing-suite arm blames the suites for a file that was never
# there — a false diagnosis pointing at code that is fine, which costs more than the silence would.
# The two `expect_not_out` assertions are the load-bearing half: the exit alone cannot tell a missing
# runner from a failing one, so only the wording separates them, and this suite is the only thing
# holding them apart. Nothing here may pass because the real repository happens to have the runner —
# the fixture removes it outright, which is why this case survives a clone and the others did not.
rm -f "$verify_repo/ai/run-tests.sh"

fresh_home
out=$(HOME="$home" bash "$verify_repo/bootstrap.sh" --skip-brew --skip-tools --skip-mcp 2>&1)
status=$?
expect_status "a checkout without the suite runner exits 1" 1
expect_out "and says the runner is missing" "is not in this checkout"
expect_out "and says that is not a pass" "not the same as passing"
expect_not_out "and does not blame the suites" "reported a failing suite"

# A dry run that says ok over a repository where the real run cannot work is the same lie one step
# earlier, so the guard sits ahead of the dry-run arm rather than beside the call.
fresh_home
out=$(HOME="$home" bash "$verify_repo/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --dry-run 2>&1)
status=$?
expect_status "a dry run without the suite runner exits 1" 1
expect_not_out "and does not report ok" "bootstrap: ok"

cat >"$verify_repo/ai/run-tests.sh" <<STUB
#!/usr/bin/env bash
printf 'ran\n' >>"\$MARKER"
exit 0
STUB
chmod +x "$verify_repo/ai/run-tests.sh"

# --- a repository missing its own contents --------------------------------------------------------

# The script resolves its sources from its own location, so a copy in a skeleton directory is how the
# missing-source and no-skills refusals get exercised. Both are the vacuity case: without them a
# bootstrap over an incomplete checkout reports success having linked almost nothing.
skeleton="$tmp/skeleton"
mkdir -p "$skeleton/ai/skills"
cp "$script" "$skeleton/bootstrap.sh"
fresh_home
out=$(HOME="$home" bash "$skeleton/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a checkout missing its configs exits 1" 1
expect_out "and names a missing source" "is missing from the repository"
expect_out "and refuses an empty skills directory rather than mounting nothing" "nothing was mounted"

# --- a second checkout must not silently move the machine -----------------------------------------

# The script derives its repository from its own location, so run from a scratch clone it repoints
# every mount at the clone and reports "repointed" thirty-odd times for an act nobody authorised.
# Delete the clone afterwards — the entire point of a scratch clone — and the human's next login has
# no shell config, no git config and no agent instructions. `link()` is right that a symlink carries
# no data of its own; the damage is to the *mount*, which is why the guard sits above `link()` rather
# than inside it, and why the stale-symlink and trailing-slash cases above must stay green alongside
# these. This repository is cloned routinely to verify published state, so this is a live hazard.
#
# The fixture is a second checkout mounted onto a home by running *its* bootstrap, so the mounts under
# test are the ones this script really writes rather than a hand-made imitation of them. It ships
# skill directories named after ones the real repository ships, which is what makes the skill mounts
# collide — without that the skill half of the count would be zero and never exercised.
#
# Rooted at `$tmp_real` rather than `$tmp`: the fixture's own bootstrap resolves its repository with
# `pwd -P`, so it writes links reading `/private/var/…` while `$tmp` is still the `/var/…` form
# `mktemp -d` handed back. Comparing a mount against the unresolved root fails a correct run — the
# same trap `tmp_real` was introduced for, and the reason the guard resolves both sides physically.
other_repo="$tmp_real/other-repo"
mkdir -p "$other_repo/ai/skills" "$other_repo/ai/kk-flavor"
: >"$other_repo/ai/CLAUDE.md"
for name in zsh git ghostty lazygit nvim zellij starship; do
  ln -s "$here/$name" "$other_repo/$name"
done
cp "$script" "$other_repo/bootstrap.sh"

# Counted from what the fixture actually created, never written down: the real total is one per config
# target plus one per skill directory the checkout ships, so it moves the day a skill is added.
want_skill=0
for skill_path in $(find "$here/ai/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -2); do
  mkdir -p "$other_repo/ai/skills/$(basename "$skill_path")"
  want_skill=$((want_skill + 1))
done
# Read from the shipped script for the same reason the brew lists below are: a config mount added to
# bootstrap.sh and missed here would leave this case asserting a total that no longer covers it.
want_cfg=$(grep -c '^add_cfg "' "$script")
want_total=$((want_cfg + want_skill))

if [ "$want_cfg" -gt 0 ] && [ "$want_skill" -gt 0 ]; then
  record_pass "control: the fixture holds both config and skill mounts, so the count below covers both kinds"
else
  record_fail "control: the fixture holds both config and skill mounts, so the count below covers both kinds" \
    "configs=$want_cfg skills=$want_skill"
fi

mount_from_other() {
  HOME="$1" bash "$other_repo/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify >/dev/null 2>&1
}

fresh_home
mount_from_other "$home"
expect_link_to "control: the fixture checkout is really what this home is mounted from" \
  "$home/.zshrc" "$other_repo/zsh/.zshrc"
run_boot "$home"
expect_status "a run from a second checkout exits 1" 1
expect_out "and says this is not where the machine is mounted" "not to this checkout"
expect_out "and leads with the count, so the skills are not lost behind the named configs" \
  "$want_total mounts ($want_cfg configs and $want_skill skills)"
expect_out "and names the checkout it would have moved them off" "$other_repo"
expect_out "and says nothing was written" "nothing was written"
expect_out "and names the flag that means it" "--relocate"
expect_not_out "and does not report ok" "bootstrap: ok"

# The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
# mounts are read back rather than the message being taken at its word.
expect_link_to "and the config mount was left where the machine had it" "$home/.zshrc" "$other_repo/zsh/.zshrc"
first_shared=$(basename "$(find "$here/ai/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)")
expect_link_to "and the skill mount too" \
  "$home/.claude/skills/$first_shared" "$other_repo/ai/skills/$first_shared"

# --dry-run has to refuse as well. It reported "would repoint" for all of them and exited 0, which is
# the same lie one step earlier: someone checks with --dry-run, reads ok, and runs it for real.
fresh_home
mount_from_other "$home"
run_boot "$home" --dry-run
expect_status "--dry-run from a second checkout exits 1 rather than previewing the move" 1
expect_not_out "and does not report ok" "bootstrap: ok"
expect_not_out "and does not offer to repoint them one at a time" "would repoint"
expect_link_to "and writes nothing either" "$home/.zshrc" "$other_repo/zsh/.zshrc"

# The escape hatch, which has to exist or someone genuinely relocating their configs cannot. It is a
# flag of its own rather than a member of the --skip family every caller passes as a block, so it is
# not something that rides along by habit — every case above reaches the guard without it.
fresh_home
mount_from_other "$home"
run_boot "$home" --relocate
expect_status "--relocate exits 0" 0
expect_out "and says how many mounts it moved, and off what" "moving $want_total mount(s)"
expect_link_to "and the config mount now points at this checkout" "$home/.zshrc" "$here/zsh/.zshrc"
expect_link_to "and the skill mount too" \
  "$home/.claude/skills/$first_shared" "$here/ai/skills/$first_shared"

# A machine mounted from two other checkouts at once, which is what half-moving one by hand leaves.
# The guard reports per root, and a reader told about one of them and not the other moves that checkout,
# re-runs, and is refused again by a root nobody named — so the case that matters is the second one
# appearing, not the first.
third_repo="$tmp_real/third-repo"
mkdir -p "$third_repo/git"
fixture_write "$third_repo/git/.gitconfig" 'the third checkout'
# A copy of the script, because that is what the guard recognises a second checkout by — a directory
# merely ending in a matching path component is a stale mount, not a checkout.
cp "$script" "$third_repo/bootstrap.sh"

fresh_home
mount_from_other "$home"
# Through the containment guard, because this drops a link the fixture just made so the one below can
# take its place. `fixture_link` refuses a target that already exists as a symlink on purpose, and that
# refusal is one of the two properties in this file that must not be weakened.
contained_parent "$home/.gitconfig" >/dev/null
rm -f "$home/.gitconfig"
fixture_link "$third_repo/git/.gitconfig" "$home/.gitconfig"
run_boot "$home"
expect_status "two foreign checkouts at once still refuses" 1
expect_out "and names the checkout most of the mounts come from" "$other_repo"
expect_out "and names the second one rather than stopping at the first" "$third_repo"
expect_out "and reports one refusal per checkout rather than one for the pair" "2 thing(s) need you"
expect_link_to "and the mount pointing at the second is left alone too" \
  "$home/.gitconfig" "$third_repo/git/.gitconfig"

# A checkout that no longer resolves is the aftermath of this very bug, or of a directory moved on
# purpose. Repointing a dangling mount is the repair, so the guard must not stand in front of it —
# a guard that refuses here would leave the human's shell broken with no way to fix it from the repo.
gone_repo="$tmp_real/gone-repo"
mkdir -p "$gone_repo/ai/skills/$first_shared" "$gone_repo/ai/kk-flavor"
: >"$gone_repo/ai/CLAUDE.md"
for name in zsh git ghostty lazygit nvim zellij starship; do
  ln -s "$here/$name" "$gone_repo/$name"
done
cp "$script" "$gone_repo/bootstrap.sh"

fresh_home
HOME="$home" bash "$gone_repo/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify >/dev/null 2>&1
expect_link_to "control: the home is mounted from the checkout about to disappear" \
  "$home/.zshrc" "$gone_repo/zsh/.zshrc"
# Through the same containment guard the fixture writes use: this is a recursive delete built from a
# variable, and the suite has destroyed files in this checkout once already.
contained_parent "$gone_repo" >/dev/null
rm -rf "$gone_repo"
run_boot "$home"
expect_status "a mount from a checkout that is gone is repaired rather than refused" 0
expect_link_to "and the dangling mount is repointed here" "$home/.zshrc" "$here/zsh/.zshrc"

# A stale mount into a real directory that is not a checkout of this repository — what a machine with
# an older dotfiles layout holds. `link()` is right to repoint that, so the guard has to stay off it:
# a mount counts as foreign only when the link value ends in the same relative path, which is what
# makes it the same file in a second copy of this repository. The stale-symlink case further up uses a
# dangling link, so it is turned away a limb earlier and never reaches this test. Without a case here,
# dropping the relative-path comparison refuses every machine holding one unrelated config symlink and
# nothing goes red.
fresh_home
mkdir -p "$home/.config" "$tmp/old-dotfiles/nvim"
fixture_link "$tmp/old-dotfiles/nvim" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale mount into an unrelated real directory is not read as a second checkout" 0
expect_link_to "and is repointed rather than refused" "$home/.config/nvim" "$here/nvim"

# A stale mount naming a checkout's root rather than the file inside it. This script writes
# `<checkout>/nvim` and never `<checkout>`, so such a link is stale whatever the root turns out to be,
# and only the relative-path comparison holds the two apart — the root here really is a second checkout
# and really does hold a copy of the script. Without a case, dropping that comparison turns every link
# of this shape into a refusal and nothing goes red.
fresh_home
mkdir -p "$home/.config"
fixture_link "$other_repo" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale mount naming a checkout root rather than a file in it is not a mount" 0
expect_link_to "and it is repointed too" "$home/.config/nvim" "$here/nvim"

# The same checkout named through a symlinked path. `mktemp -d` hands back `/var/folders/…` while
# `/var` is a symlink to `/private/var`, so this is the shape a real macOS mount takes — and a guard
# comparing an unresolved root against a resolved `$repo` calls this checkout a stranger to itself and
# refuses a machine that is correctly mounted. Both sides are resolved physically for that reason.
alias_root="$tmp/alias-to-other"
contained_parent "$alias_root" >/dev/null
ln -s "$other_repo" "$alias_root"
fresh_home
fixture_link "$alias_root/zsh/.zshrc" "$home/.zshrc"
out=$(HOME="$home" bash "$other_repo/bootstrap.sh" --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a mount naming the running checkout through a symlinked path is not foreign" 0
expect_link_to "and is repointed at the canonical path" "$home/.zshrc" "$other_repo/zsh/.zshrc"

# A relative link value is not one this script wrote — every link it makes is absolute — and resolving
# a root out of it would resolve it against the caller's working directory rather than the link's own.
# Run from `$tmp`, `other-repo/nvim` resolves to the fixture checkout above, so a guard that skipped
# the absolute test would refuse here; the link itself dangles, which is what makes it merely stale.
fresh_home
mkdir -p "$home/.config"
fixture_link "other-repo/nvim" "$home/.config/nvim"
out=$(cd "$tmp" && HOME="$home" bash "$script" --skip-brew --skip-tools --skip-mcp --skip-verify 2>&1)
status=$?
expect_status "a relative link is not read as a second checkout" 0
expect_link_to "and is repointed like any other stale link" "$home/.config/nvim" "$here/nvim"

# --- the brew list and the README cannot drift apart --------------------------------------------

# bootstrap.sh hardcodes its formulae and README.md lists them separately. They agree today, and
# nothing held them there: adding a formula to the README would silently stop it being installed,
# with every case still passing. Read from the real files rather than a fixture, so the shipped ones
# cannot drift to a form this never sees.
boot_formulae="$(sed -n 's/^  for formula in \(.*\); do$/\1/p' "$script" | tr ' ' '\n' | sort -u)"
boot_casks="$(sed -n 's/^  for cask in \(.*\); do$/\1/p' "$script" | tr ' ' '\n' | sort -u)"
readme_formulae="$(grep -oE '`brew install [a-z0-9-]+`' "$here/README.md" | sed 's/.*brew install //; s/`//' | sort -u)"
readme_casks="$(grep -oE '`brew install --cask [a-z0-9-]+`' "$here/README.md" | sed 's/.*--cask //; s/`//' | sort -u)"

if [ -n "$boot_formulae" ] && [ -n "$readme_formulae" ]; then
  record_pass "control: both formula lists were found, so this case is comparing something"
else
  record_fail "control: both formula lists were found, so this case is comparing something" \
    "bootstrap='$boot_formulae' readme='$readme_formulae'"
fi

if [ "$boot_formulae" = "$readme_formulae" ]; then
  record_pass "every formula README installs is one bootstrap installs"
else
  record_fail "every formula README installs is one bootstrap installs" \
    "only in bootstrap: $(comm -23 <(printf '%s\n' "$boot_formulae") <(printf '%s\n' "$readme_formulae") | tr '\n' ' ')| only in README: $(comm -13 <(printf '%s\n' "$boot_formulae") <(printf '%s\n' "$readme_formulae") | tr '\n' ' ')"
fi

if [ "$boot_casks" = "$readme_casks" ]; then
  record_pass "and the casks agree too"
else
  record_fail "and the casks agree too" "bootstrap='$boot_casks' readme='$readme_casks'"
fi

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
