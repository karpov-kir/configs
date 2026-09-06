#!/usr/bin/env bash
# Cases for env/bootstrap.sh, and with it the mount library both bootstraps share. The one that must
# not be weakened is "a real file at a target is refused, not deleted": the README's hand-run form is
# `rm -rf` and these scripts exist to not be that, so a regression there is silent data loss on
# somebody's machine rather than a failing check.
#
# The second-checkout guard is covered here in full, for both scripts. It lives in lib/mount.sh and
# neither copy of it can drift from the other; ai/bootstrap-test.sh covers the one thing this file
# cannot, which is that a bulk mount takes part in the count the guard reports.
#
# Every case runs the real script against a throwaway $HOME with --skip-brew, so brew is never
# invoked. What is actually exercised is the linking and the refusals, which is the part with the
# branches.
set -u

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
script="$here/bootstrap.sh"
suite_name="env/bootstrap-test.sh"

# shellcheck source=../lib/test-harness.sh
. "$checkout/lib/test-harness.sh"

# The skip flag is load-bearing: without it a case shells out to brew, which makes the suite slow,
# network-dependent, and able to install software on the machine running it.
run_boot() {
  local home="$1"
  shift
  out=$(HOME="$home" bash "$script" --skip-brew "$@" 2>&1)
  status=$?
}

echo "env/bootstrap.sh"

# --- a fresh machine ------------------------------------------------------------------------------

fresh_home
run_boot "$home"
expect_status "a fresh home exits 0" 0
expect_out "and reports ok" "env bootstrap: ok"
# A machine with nothing mounted yet has nothing for the second-checkout guard to protect, so it must
# pass straight through. Asserted on the wording rather than on the exit, because a guard that printed
# nothing when it passes is indistinguishable from one that was never reached.
expect_out "and the second-checkout guard passes rather than staying silent" "no mount on this machine comes from another checkout"
expect_link_to "the shell config is linked" "$home/.zshrc" "$here/zsh/.zshrc"
expect_link_to "the git identity is linked" "$home/.gitconfig" "$here/git/.gitconfig"
expect_link_to "a directory config is linked" "$home/.config/nvim" "$here/nvim"
expect_link_to "a file config is linked" "$home/.config/starship.toml" "$here/starship/starship.toml"

# A parent the README creates by hand: ~/.config does not exist on a fresh machine, and a link into a
# missing directory fails rather than creating it.
[ -d "$home/.config" ] &&
  record_pass "a missing parent directory is created" ||
  record_fail "a missing parent directory is created" "~/.config is absent"

# --- re-running -----------------------------------------------------------------------------------

# Idempotence is the property that makes this safe to run on a working machine, and the only evidence
# for it is a second run over the first run's output.
run_boot "$home"
expect_status "a second run over a finished home exits 0" 0
expect_out "and reports the targets as already ok" "  ok       $home/.zshrc"
expect_not_out "and relinks nothing" "linked   $home/.zshrc"

# --- the refusal that matters ---------------------------------------------------------------------

# The README's hand-run form is `rm -rf ~/.config/nvim && ln -s ...`. A script doing that unattended
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
expect_link_to "and the other links are still made" "$home/.zshrc" "$here/zsh/.zshrc"

# --- a stale symlink ------------------------------------------------------------------------------

# A symlink carries no data of its own, so repointing one loses nothing — this is the only target a
# link may be written over, and a stale one is what an older layout leaves behind. Every machine set
# up before the configs moved under env/ is holding six of them.
fresh_home
mkdir -p "$home/.config"
fixture_link "$tmp/somewhere-else" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale symlink does not refuse" 0
expect_link_to "and is repointed at the repository" "$home/.config/nvim" "$here/nvim"
expect_out "and says so rather than reporting ok" "repointed $home/.config/nvim"

# A link differing from the computed source only by a trailing slash is the same directory spelled
# two ways. Compared raw it looks stale and gets rewritten on every run — idempotence lost to a
# cosmetic difference, and the noise hides a genuinely stale link.
fresh_home
mkdir -p "$home/.config"
fixture_link "$here/nvim/" "$home/.config/nvim"
run_boot "$home"
expect_out "a link differing only by a trailing slash is left alone" "  ok       $home/.config/nvim"
expect_not_out "and is not rewritten" "repointed $home/.config/nvim"

# --- --dry-run ------------------------------------------------------------------------------------

# The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
# it is the run that changed their machine.
fresh_home
run_boot "$home" --dry-run
expect_status "--dry-run exits 0" 0
expect_out "and says what it would do" "would link"
[ ! -e "$home/.zshrc" ] && [ ! -e "$home/.config" ] &&
  record_pass "--dry-run creates nothing at all" ||
  record_fail "--dry-run creates nothing at all" "something was written under $home"

# --- arguments ------------------------------------------------------------------------------------

# The skip flag rides along even though the option check runs before it is consulted. If that check
# ever stops exiting, this case must fail rather than proceed into brew — a regression should redden,
# not install things.
fresh_home
out=$(HOME="$home" bash "$script" --skip-brew --not-a-flag 2>&1)
status=$?
expect_status "an unknown option exits 2" 2
expect_out "and names the option it rejected" "--not-a-flag"
[ ! -e "$home/.zshrc" ] &&
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
help_usage=$(sed -n 's|^#[[:space:]]*\(usage: env/bootstrap\.sh .*\)$|\1|p' "$script" | head -1)
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
[ ! -e "$home/.zshrc" ] && [ ! -e "$home/.config" ] &&
  record_pass "and --help changes nothing on the machine" ||
  record_fail "and --help changes nothing on the machine" "something was written under $home"

# --- a repository missing its own contents --------------------------------------------------------

# The script resolves its sources from its own location, so a copy in a skeleton checkout is how the
# missing-source refusal gets exercised. It is the vacuity case: without it a bootstrap over an
# incomplete checkout reports success having linked nothing.
skeleton="$tmp/skeleton"
fixture_checkout "$skeleton" env
fresh_home
out=$(HOME="$home" bash "$skeleton/env/bootstrap.sh" --skip-brew 2>&1)
status=$?
expect_status "a checkout missing its configs exits 1" 1
expect_out "and names a missing source" "is missing from the repository"

# --- a checkout missing the library the two scripts share -----------------------------------------

# The `.` fails, and without `set -e` the run carries on with no add_cfg and no mount_run, exits 127,
# and says only `command not found` six times. That names neither the file that is gone nor what to
# do, which is the false diagnosis the verify step in ai/bootstrap.sh already refuses to make. Each
# script carries its own copy of the guard, because the file that would hold one shared copy is the
# file that is missing.
libless="$tmp/libless"
fixture_checkout "$libless" env
rm -f "$libless/lib/mount.sh"
fresh_home
out=$(HOME="$home" bash "$libless/env/bootstrap.sh" --skip-brew 2>&1)
status=$?
expect_status "a checkout without lib/mount.sh exits 2" 2
expect_out "and names the file that is missing" "lib/mount.sh is missing from this checkout"
expect_not_out "and does not cascade through the mount table instead" "command not found"
[ ! -e "$home/.zshrc" ] &&
  record_pass "and nothing was linked" ||
  record_fail "and nothing was linked" "it linked anyway"

# --- a second checkout must not silently move the machine -----------------------------------------

# The script derives its repository from its own location, so run from a scratch clone it repoints
# every mount at the clone and reports "repointed" six times for an act nobody authorised. Delete the
# clone afterwards — the entire point of a scratch clone — and the human's next login has no shell
# config and no git config. `link()` is right that a symlink carries no data of its own; the damage is
# to the *mount*, which is why the guard sits above `link()` rather than inside it, and why the
# stale-symlink and trailing-slash cases above must stay green alongside these. This repository is
# cloned routinely to verify published state, so this is a live hazard.
#
# The fixture is a second checkout mounted onto a home by running *its* bootstrap, so the mounts under
# test are the ones the script really writes rather than a hand-made imitation of them.
#
# Rooted at `$tmp_real` rather than `$tmp`: the fixture's own bootstrap resolves its repository with
# `pwd -P`, so it writes links reading `/private/var/…` while `$tmp` is still the `/var/…` form
# `mktemp -d` handed back. Comparing a mount against the unresolved root fails a correct run — the
# same trap `tmp_real` was introduced for, and the reason the guard resolves both sides physically.
other_repo="$tmp_real/other-repo"
fixture_checkout "$other_repo" env
for name in zsh git ghostty nvim starship; do
  ln -s "$here/$name" "$other_repo/env/$name"
done

# Read from the shipped script rather than written down: a config mount added to env/bootstrap.sh and
# missed here would leave this case asserting a total that no longer covers it.
want_cfg=$(grep -c '^add_cfg "' "$script")
if [ "$want_cfg" -gt 0 ]; then
  record_pass "control: the mount table was read from the script, so the count below is not a guess"
else
  record_fail "control: the mount table was read from the script, so the count below is not a guess" \
    "found no add_cfg lines in $script"
fi

mount_from_other() {
  HOME="$1" bash "$other_repo/env/bootstrap.sh" --skip-brew >/dev/null 2>&1
}

fresh_home
mount_from_other "$home"
expect_link_to "control: the fixture checkout is really what this home is mounted from" \
  "$home/.zshrc" "$other_repo/env/zsh/.zshrc"
run_boot "$home"
expect_status "a run from a second checkout exits 1" 1
expect_out "and says this is not where the machine is mounted" "not to this checkout"
expect_out "and leads with the count" "$want_cfg mounts"
expect_out "and names the checkout it would have moved them off" "$other_repo"
expect_out "and says nothing was written" "nothing was written"
expect_out "and names the flag that means it" "--relocate"
expect_not_out "and does not report ok" "env bootstrap: ok"

# The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
# mounts are read back rather than the message being taken at its word.
expect_link_to "and the mount was left where the machine had it" "$home/.zshrc" "$other_repo/env/zsh/.zshrc"

# --dry-run has to refuse as well. Unguarded it reports "would repoint" for all of them and exits 0,
# which is the same lie one step earlier: someone checks with --dry-run, reads ok, and runs it for
# real.
fresh_home
mount_from_other "$home"
run_boot "$home" --dry-run
expect_status "--dry-run from a second checkout exits 1 rather than previewing the move" 1
expect_not_out "and does not report ok" "env bootstrap: ok"
expect_not_out "and does not offer to repoint them one at a time" "would repoint"
expect_link_to "and writes nothing either" "$home/.zshrc" "$other_repo/env/zsh/.zshrc"

# The escape hatch, which has to exist or someone genuinely relocating their configs cannot. It is a
# flag of its own rather than a member of the --skip family every caller passes as a block, so it is
# not something that rides along by habit — every case above reaches the guard without it.
fresh_home
mount_from_other "$home"
run_boot "$home" --relocate
expect_status "--relocate exits 0" 0
expect_out "and says how many mounts it moved, and off what" "moving $want_cfg mount(s)"
expect_link_to "and the mount now points at this checkout" "$home/.zshrc" "$here/zsh/.zshrc"

# A machine mounted from two other checkouts at once, which is what half-moving one by hand leaves.
# The guard reports per root, and a reader told about one of them and not the other moves that checkout,
# re-runs, and is refused again by a root nobody named — so the case that matters is the second one
# appearing, not the first.
third_repo="$tmp_real/third-repo"
fixture_checkout "$third_repo" env
mkdir -p "$third_repo/env/git"
fixture_write "$third_repo/env/git/.gitconfig" 'the third checkout'

fresh_home
mount_from_other "$home"
# Through the containment guard, because this drops a link the fixture just made so the one below can
# take its place. `fixture_link` refuses a target that already exists as a symlink on purpose, and that
# refusal is one of the two properties in this file that must not be weakened.
contained_parent "$home/.gitconfig" >/dev/null
rm -f "$home/.gitconfig"
fixture_link "$third_repo/env/git/.gitconfig" "$home/.gitconfig"
run_boot "$home"
expect_status "two foreign checkouts at once still refuses" 1
expect_out "and names the checkout most of the mounts come from" "$other_repo"
expect_out "and names the second one rather than stopping at the first" "$third_repo"
expect_out "and reports one refusal per checkout rather than one for the pair" "2 thing(s) need you"
expect_link_to "and the mount pointing at the second is left alone too" \
  "$home/.gitconfig" "$third_repo/env/git/.gitconfig"

# A checkout that no longer resolves is the aftermath of this very bug, or of a directory moved on
# purpose. Repointing a dangling mount is the repair, so the guard must not stand in front of it —
# a guard that refuses here would leave the human's shell broken with no way to fix it from the repo.
gone_repo="$tmp_real/gone-repo"
fixture_checkout "$gone_repo" env
for name in zsh git ghostty nvim starship; do
  ln -s "$here/$name" "$gone_repo/env/$name"
done

fresh_home
HOME="$home" bash "$gone_repo/env/bootstrap.sh" --skip-brew >/dev/null 2>&1
expect_link_to "control: the home is mounted from the checkout about to disappear" \
  "$home/.zshrc" "$gone_repo/env/zsh/.zshrc"
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

# A stale mount naming a checkout's env/ directory rather than the config inside it. This script
# writes `<checkout>/env/nvim` and never `<checkout>/env`, so such a link is stale whatever the root
# turns out to be, and only the relative-path comparison holds the two apart — the root here really is
# a second checkout and really does hold a copy of the script. Without a case, dropping that
# comparison turns every link of this shape into a refusal and nothing goes red.
fresh_home
mkdir -p "$home/.config"
fixture_link "$other_repo/env" "$home/.config/nvim"
run_boot "$home"
expect_status "a stale mount naming a checkout directory rather than a file in it is not a mount" 0
expect_link_to "and it is repointed too" "$home/.config/nvim" "$here/nvim"

# The same checkout named through a symlinked path. `mktemp -d` hands back `/var/folders/…` while
# `/var` is a symlink to `/private/var`, so this is the shape a real macOS mount takes — and a guard
# comparing an unresolved root against a resolved `$repo` calls this checkout a stranger to itself and
# refuses a machine that is correctly mounted. Both sides are resolved physically for that reason.
alias_root="$tmp/alias-to-other"
contained_parent "$alias_root" >/dev/null
ln -s "$other_repo" "$alias_root"
fresh_home
fixture_link "$alias_root/env/zsh/.zshrc" "$home/.zshrc"
out=$(HOME="$home" bash "$other_repo/env/bootstrap.sh" --skip-brew 2>&1)
status=$?
expect_status "a mount naming the running checkout through a symlinked path is not foreign" 0
expect_link_to "and is repointed at the canonical path" "$home/.zshrc" "$other_repo/env/zsh/.zshrc"

# A relative link value is not one this script wrote — every link it makes is absolute — and resolving
# a root out of it would resolve it against the caller's working directory rather than the link's own.
# Run from `$tmp`, `other-repo/env/nvim` resolves to the fixture checkout above, so a guard that
# skipped the absolute test would refuse here; the link itself dangles, which is what makes it merely
# stale.
fresh_home
mkdir -p "$home/.config"
fixture_link "other-repo/env/nvim" "$home/.config/nvim"
out=$(cd "$tmp" && HOME="$home" bash "$script" --skip-brew 2>&1)
status=$?
expect_status "a relative link is not read as a second checkout" 0
expect_link_to "and is repointed like any other stale link" "$home/.config/nvim" "$here/nvim"

# --- the brew list and the README cannot drift apart --------------------------------------------

expect_brew_matches_readme "$script" "$here/README.md"

# --- the parent of a root-level target -------------------------------------------------------------

# `${target%/*}` on `/.zshrc` is the empty string, not `/`, so the `|| parent="/"` fallback is the
# whole of what makes a root-level target work. Drop it and the script reaches `mkdir -p ""` and
# refuses naming a parent it cannot print, sending the reader after a directory that was never the
# problem. Both spellings refuse, so only the wording tells them apart.
#
# An empty HOME is the only way a target lands at the root.
#
# `[ -w / ]` rather than `[ "$(id -u)" = 0 ]`, because the thing that decides the harm is whether THIS
# process can write to `/`, and uid 0 is only the commonest way to be able to. A container user who
# owns `/`, a group-writable root, an ACL, or `CAP_DAC_OVERRIDE` without root all write there at a
# non-zero uid — and this is the one case in this file that escapes the containment guard above, since
# what writes is env/bootstrap.sh at `/` rather than a fixture under $tmp_real. `test -w` asks
# access(2), which answers for all of them (`~/.kk-flavor/standards/testing.md` → **4. Setup
# strategy**: probe rather than read `id -u`).
if [ -w / ]; then
  # Exit 2 rather than a quiet skip: this case lets env/bootstrap.sh attempt `ln -s` at `/`, and a
  # machine that can complete it would be left holding links at its filesystem root pointing into this
  # checkout, which nothing here removes. A pass would report the branch as covered on the one kind of
  # machine that never ran it.
  echo "env/bootstrap-test: this process can write to /, where the empty-HOME case would link at the filesystem root — that branch was NOT tested" >&2
  exit 2
fi

out=$(HOME= bash "$script" --skip-brew 2>&1)
case "$out" in
  *"could not create "[[:space:]]*) record_fail "an empty HOME names / as the parent, not nothing" "the parent came back empty: $out" ;;
  *) record_pass "an empty HOME names / as the parent, not nothing" ;;
esac
# And it got past computing the parent to attempt the link itself, so the case is about the fallback
# rather than about any earlier refusal that happens to keep the message out.
case "$out" in
  *"/.zshrc"*) record_pass "and reaches the root-level target it computed" ;;
  *) record_fail "and reaches the root-level target it computed" "no root-level target in: $out" ;;
esac

report_suite
