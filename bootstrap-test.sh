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

# --- the file this repository writes rather than links ---------------------------------------------

# `~/.claude/RTK.md` is imported into every session by ai/CLAUDE.md, so its body is always-loaded
# context. `rtk init -g` writes its own longer template there and rewrites it on every run, so the
# property under test is not that the file exists — it is that whatever was there is the repository's
# copy afterwards.
fresh_home
run_boot "$home"
expect_status "a fresh home installs the import's file and exits 0" 0
expect_file_body "RTK.md is written from the repository" "$home/.claude/RTK.md" "$(cat "$here/ai/RTK.md")"
[ -f "$home/.claude/RTK.md" ] && [ ! -L "$home/.claude/RTK.md" ] &&
  record_pass "and as a real file, which is the shape the always-loaded budget can count" ||
  record_fail "and as a real file, which is the shape the always-loaded budget can count" "it is a symlink or absent"

run_boot "$home"
expect_out "a second run reports it as already ok" "  ok       $home/.claude/RTK.md"
expect_not_out "and rewrites nothing" "wrote    $home/.claude/RTK.md"

# The case the step exists for. Every other target holding a body this script did not write is a
# refusal; this one is generated text whose author this repository has taken over, so it is replaced.
fixture_write "$home/.claude/RTK.md" 'the template rtk init -g writes, at five times the length'
run_boot "$home"
expect_status "a foreign body at the mount does not refuse" 0
expect_file_body "and is replaced by the repository's copy" "$home/.claude/RTK.md" "$(cat "$here/ai/RTK.md")"
expect_out "and says it wrote rather than reporting ok" "wrote    $home/.claude/RTK.md"

# A symlink at an import's mount is refused by the always-loaded budget, which then names the file and
# stops counting it — so the tier goes unmeasured while every report still reads clean. This script
# must neither create one nor pass over one silently.
fresh_home
mkdir -p "$home/.claude"
fixture_link "$here/ai/RTK.md" "$home/.claude/RTK.md"
run_boot "$home"
expect_status "a symlink at the mount exits 1" 1
expect_out "and says why a link is the wrong shape there" "an import's mount must be a real file"
[ -L "$home/.claude/RTK.md" ] &&
  record_pass "and the link is left for the human rather than deleted" ||
  record_fail "and the link is left for the human rather than deleted" "it was removed"

# The other shape that is not a real file, and the one that fails quietly rather than loudly. `cp
# file dir` writes *into* the directory and exits 0, so without a guard the step prints "wrote" while
# the mount is still a directory and the import still resolves to nothing — a success reported over a
# subject never reached. Asserted from both ends: the refusal, and that nothing landed inside.
fresh_home
mkdir -p "$home/.claude/RTK.md"
run_boot "$home"
expect_status "a directory at the mount exits 1" 1
expect_out "and says the mount has to be a regular file" "exists and is not a regular file"
expect_not_out "and does not report having written it" "wrote    $home/.claude/RTK.md"
[ -d "$home/.claude/RTK.md" ] && [ ! -e "$home/.claude/RTK.md/RTK.md" ] &&
  record_pass "and nothing was copied inside it" ||
  record_fail "and nothing was copied inside it" "the directory was written into or replaced"

# --- --dry-run ------------------------------------------------------------------------------------

# The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
# it is the run that changed their machine.
fresh_home
run_boot "$home" --dry-run
expect_status "--dry-run exits 0" 0
expect_out "and says what it would do" "would link"
expect_out "including the file it would write rather than link" "would write $home/.claude/RTK.md"
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
# The two files under ai/ this script reads by name: the one it links, and the one it writes.
: >"$verify_repo/ai/CLAUDE.md"
: >"$verify_repo/ai/RTK.md"
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
