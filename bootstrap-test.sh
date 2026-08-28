#!/usr/bin/env bash
# Cases for bootstrap.sh. The one that must not be weakened is "a real file at a target is refused,
# not deleted": README.md's hand-run form is `rm -rf` and this script exists to not be that, so a
# regression there is silent data loss on somebody's machine rather than a failing check.
#
# Every case runs the real script against a throwaway $HOME and passes every --skip flag, so brew, gh
# and claude are never invoked. What is actually exercised is the linking and the refusals, which is
# the part with the branches.
set -u

here=$(cd "$(dirname "$0")" && pwd)
script="$here/bootstrap.sh"

tmp=$(mktemp -d) || exit 1
trap 'rm -rf "$tmp"' EXIT

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

# No skip counter here, deliberately, and it is worth saying why the shape differs from
# score-test.sh. That suite reports a third field because it has a case it genuinely cannot run as
# root, and hiding that would make two machines checking different sets look identical. Every case
# here runs at every uid: the one `chmod` in this file grants a bit rather than restricting one, so
# there is nothing for a skip to guard. A field that can never be non-zero is decoration wearing the
# shape of a measurement, which is the thing this suite exists to refuse.

# A fresh home per case, so no case inherits another's links. Numbered rather than mktemp'd again so a
# failure message names which case's home to go and look at.
#
# It sets `home` rather than printing it: read back through `home=$(fresh_home)` the counter would be
# incremented inside a subshell, every case would be handed `home1`, and each would inherit the first
# case's links — which reads as the script writing during --dry-run rather than as a broken harness.
case_no=0
fresh_home() {
  case_no=$((case_no + 1))
  home="$tmp/home$case_no"
  mkdir -p "$home"
}

# The skip flags are the guard, not tidiness: without them a case would shell out to brew, gh and the
# claude CLI, which would make the suite slow, network-dependent, and capable of writing to the real
# MCP registry.
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
printf 'my real config\n' >"$home/.config/nvim/init.lua"
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
printf 'hand-written prompt\n' >"$home/.config/starship.toml"
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
ln -s "$first_skill/" "$home/.claude/skills/$(basename "$first_skill")"
run_boot "$home"
expect_out "a link differing only by a trailing slash is left alone" "  ok       $home/.claude/skills/$(basename "$first_skill")"
expect_not_out "and is not rewritten" "repointed $home/.claude/skills/$(basename "$first_skill")"

# --- a stale symlink ------------------------------------------------------------------------------

# A symlink carries no data of its own, so repointing one loses nothing — this is the single case
# where writing over an existing target is correct, and it is what an older layout leaves behind.
fresh_home
mkdir -p "$home/.config"
ln -s "$tmp/somewhere-else" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale symlink does not refuse" 0
expect_link_to "and is repointed at the repository" "$home/.config/nvim" "$here/nvim"
expect_out "and says so rather than reporting ok" "repointed $home/.config/nvim"

# --- --dry-run ------------------------------------------------------------------------------------

# The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
# it is the run that changed their machine.
fresh_home
run_boot "$home" --dry-run
expect_status "--dry-run exits 0" 0
expect_out "and says what it would do" "would link"
[ ! -e "$home/.kk-flavor" ] && [ ! -e "$home/.config" ] &&
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

# --- the verify step, and its re-entry guard ------------------------------------------------------

# Verify is the one step every other case skips, so it needs a repository of its own to run against.
# The stub records that it ran and exits 0, which is what lets both directions be asserted: without
# the guard the marker appears, with it the marker does not. A stub rather than the real runner also
# keeps the regression a red case instead of a hang — the real one discovers this suite, which runs
# this script, which is the loop the guard exists to close.
verify_repo="$tmp/verify-repo"
mkdir -p "$verify_repo/ai/skills/a-skill" "$verify_repo/ai/kk-flavor"
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

echo
echo "$passed passed, $failed failed"
[ "$failed" -eq 0 ]
