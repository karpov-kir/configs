#!/usr/bin/env bash
# Cases for ai/install-project.sh: the skills mounted into one project, the two files it writes in
# that project, and taking all of it back out.
#
# The libraries' own behaviour is theirs — lib/owned-region-test.sh holds the region states,
# lib/install-registry-test.sh the registry's self-healing. What is here is what only this script can
# answer: which fences it passes, what it writes between them, that a re-run changes nothing, and that
# the second-checkout guard is reachable through THIS file's name rather than bootstrap.sh's.
#
# Every case runs the real script against a throwaway $HOME and $XDG_CONFIG_HOME, so nothing touches
# the developer's own mounts or registry.
set -u

here=$(CDPATH= cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
checkout=$(CDPATH= cd -P -- "$here/.." && pwd -P)
script="$here/install-project.sh"
suite_name="ai/install-project-test.sh"

# shellcheck source=../lib/test-harness.sh
. "$checkout/lib/test-harness.sh" ||
  { printf '%s: lib/test-harness.sh did not load to the end — nothing was measured\n' "$suite_name" >&2; exit 2; }

echo "ai/install-project.sh"

run_install() { # <project> [flags...]
  local project="$1"
  shift
  out=$(HOME="$home" XDG_CONFIG_HOME="$home/.config" bash "$script" "$project" "$@" 2>&1)
  status=$?
}

new_project() { # <name>
  project="$tmp_real/$1"
  mkdir -p "$project"
  printf '# %s\n\nHow this project works.\n' "$1" >"$project/CLAUDE.md"
  printf 'node_modules/\n' >"$project/.gitignore"
}

# --- a fresh project ---------------------------------------------------------------------------------

fresh_home
new_project proj1
run_install "$project"
expect_status "a fresh project exits 0" 0

want=0
for skill_path in "$here"/kk-flavor/skills/*/; do
  [ -d "$skill_path" ] || continue
  grep -q '^audience: maintainer$' "${skill_path}SKILL.md" 2>/dev/null && continue
  want=$((want + 1))
done
got=$(find "$project/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null | wc -l | tr -d ' ')
[ "$want" -gt 0 ] && [ "$got" -eq "$want" ] &&
  record_pass "every skill the default tier installs is mounted in the project" ||
  record_fail "every skill the default tier installs is mounted in the project" "mounted $got of $want"

expect_out "and the maintainer-only ones are named as left out" "maintainer-only"

grep -q "kk-flavor:begin" "$project/CLAUDE.md" &&
  record_pass "the project's CLAUDE.md carries the region" ||
  record_fail "the project's CLAUDE.md carries the region" "$(cat "$project/CLAUDE.md")"
grep -q "How this project works." "$project/CLAUDE.md" &&
  record_pass "and what the project already said is still there" ||
  record_fail "and what the project already said is still there" "$(cat "$project/CLAUDE.md")"
grep -q "^\.claude/skills/kk-\*$" "$project/.gitignore" &&
  record_pass "the ignore rules are added" ||
  record_fail "the ignore rules are added" "$(cat "$project/.gitignore")"
grep -q "^node_modules/$" "$project/.gitignore" &&
  record_pass "and the project's own rules survive" ||
  record_fail "and the project's own rules survive" "$(cat "$project/.gitignore")"
grep -qxF "$project" "$home/.config/kk-flavor/installs" 2>/dev/null &&
  record_pass "and the project is recorded" ||
  record_fail "and the project is recorded" "$(cat "$home/.config/kk-flavor/installs" 2>/dev/null)"

# --- re-running ----------------------------------------------------------------------------------------

before_md=$(cat "$project/CLAUDE.md")
before_ignore=$(cat "$project/.gitignore")
run_install "$project"
expect_status "a second run exits 0" 0
[ "$before_md" = "$(cat "$project/CLAUDE.md")" ] &&
  record_pass "and rewrites nothing in CLAUDE.md" ||
  record_fail "and rewrites nothing in CLAUDE.md" "it changed"
[ "$before_ignore" = "$(cat "$project/.gitignore")" ] &&
  record_pass "and rewrites nothing in .gitignore" ||
  record_fail "and rewrites nothing in .gitignore" "it changed"
[ "$(grep -cxF "$project" "$home/.config/kk-flavor/installs")" = "1" ] &&
  record_pass "and records the project once, not twice" ||
  record_fail "and records the project once, not twice" "$(cat "$home/.config/kk-flavor/installs")"

# --- uninstall -------------------------------------------------------------------------------------------

run_install "$project" --uninstall
expect_status "uninstall exits 0" 0
[ -z "$(find "$project/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null)" ] &&
  record_pass "and every skill mount is gone" ||
  record_fail "and every skill mount is gone" "some remain"
grep -q "kk-flavor:begin" "$project/CLAUDE.md" &&
  record_fail "and the region is gone from CLAUDE.md" "still there" ||
  record_pass "and the region is gone from CLAUDE.md"
grep -q "How this project works." "$project/CLAUDE.md" &&
  record_pass "and what the project wrote is untouched" ||
  record_fail "and what the project wrote is untouched" "$(cat "$project/CLAUDE.md")"
grep -q "^node_modules/$" "$project/.gitignore" &&
  record_pass "and the project's own ignore rules are untouched" ||
  record_fail "and the project's own ignore rules are untouched" "$(cat "$project/.gitignore")"
grep -q "kk-flavor:begin" "$project/.gitignore" &&
  record_fail "and ours are gone" "still there" ||
  record_pass "and ours are gone"

run_install "$project" --uninstall
expect_status "uninstalling twice is not an error" 0

# The tier a project was installed with is written down nowhere, so an uninstall that re-applies the
# audience filter builds its removal table for the tier being asked for NOW. `--maintainer` in and
# plain out leaves exactly the marked skills mounted — and the registry entry goes, so nothing on the
# machine ever names them again.
fresh_home
new_project tiered
run_install "$project" --maintainer
run_install "$project" --uninstall
expect_status "uninstall after a --maintainer install exits 0" 0
[ -z "$(find "$project/.claude/skills" -mindepth 1 -maxdepth 1 -type l 2>/dev/null)" ] &&
  record_pass "a plain --uninstall removes what --maintainer installed" ||
  record_fail "a plain --uninstall removes what --maintainer installed" \
    "still mounted: $(find "$project/.claude/skills" -mindepth 1 -maxdepth 1 -type l -exec basename {} \; | tr '\n' ' ')"

# --- a project that already ignores .claude/ wholesale --------------------------------------------------------

# Reported rather than appended to: that rule covers the project's own settings as well as our mounts,
# so what to do about it is a decision for the human whose repository it is.
fresh_home
new_project broad
printf '.claude/\n' >>"$project/.gitignore"
run_install "$project"
expect_status "a project already ignoring .claude/ exits 1" 1
expect_out "and says which line already covers it" "already ignores .claude/ wholesale"
grep -q "kk-flavor:begin" "$project/.gitignore" &&
  record_fail "and adds no rule of its own" "it added one" ||
  record_pass "and adds no rule of its own"

# --- a project with no CLAUDE.md ------------------------------------------------------------------------------

fresh_home
project="$tmp_real/bare"
mkdir -p "$project"
run_install "$project"
expect_status "a project with neither file exits 0" 0
grep -q "kk-flavor:begin" "$project/CLAUDE.md" 2>/dev/null &&
  record_pass "and CLAUDE.md is created with the region" ||
  record_fail "and CLAUDE.md is created with the region" "$(cat "$project/CLAUDE.md" 2>/dev/null)"
grep -q "^\.claude/skills/idsd-\*$" "$project/.gitignore" 2>/dev/null &&
  record_pass "and .gitignore is created with the rules" ||
  record_fail "and .gitignore is created with the rules" "$(cat "$project/.gitignore" 2>/dev/null)"

# --- planted symlinks in the project ----------------------------------------------------------------

# Both creation branches take a path that does not exist yet, and `-e` follows a symlink — so a
# DANGLING link checked into a repository answers "not there" and the redirect then writes whatever it
# names, anywhere the installer's user can write. A repository is not trusted input.
fresh_home
project="$tmp_real/planted"
mkdir -p "$project"
printf 'do not touch\n' >"$tmp_real/outside.txt"
ln -s "$tmp_real/outside.txt" "$project/.gitignore"
run_install "$project"
expect_out "a symlinked .gitignore is refused, not written through" "is a symlink"
expect_file_body "and the file it named is untouched" "$tmp_real/outside.txt" 'do not touch'

fresh_home
project="$tmp_real/planted2"
mkdir -p "$project"
ln -s "$tmp_real/never-created.txt" "$project/CLAUDE.md"
run_install "$project"
[ ! -e "$tmp_real/never-created.txt" ] &&
  record_pass "a dangling CLAUDE.md symlink does not create the file it names" ||
  record_fail "a dangling CLAUDE.md symlink does not create the file it names" "it was created"

# --- arguments --------------------------------------------------------------------------------------------------

fresh_home
out=$(HOME="$home" bash "$script" 2>&1)
status=$?
expect_status "no project named exits 2" 2
expect_out "and says to name one" "name the project directory"

out=$(HOME="$home" bash "$script" "$tmp_real/nowhere" 2>&1)
status=$?
expect_status "a project that is not there exits 2" 2
expect_out "and says nothing was written" "nothing was written"

out=$(HOME="$home" bash "$script" --dry-run "$tmp_real/nowhere" 2>&1)
status=$?
expect_status "a missing project behind a flag exits 2" 2
expect_out "and names the project, not the flag" "$tmp_real/nowhere is not a directory"

out=$(HOME="$home" bash "$script" "$tmp_real/bare" --not-a-flag 2>&1)
status=$?
expect_status "an unknown option exits 2" 2
expect_out "and names it" "unknown option --not-a-flag"

# --- dry run ------------------------------------------------------------------------------------------------------

fresh_home
new_project dry
run_install "$project" --dry-run
expect_status "--dry-run exits 0" 0
[ ! -e "$project/.claude/skills" ] &&
  record_pass "and mounts nothing" ||
  record_fail "and mounts nothing" "it mounted"
grep -q "kk-flavor:begin" "$project/CLAUDE.md" &&
  record_fail "and writes no region" "it wrote one" ||
  record_pass "and writes no region"
[ ! -e "$home/.config/kk-flavor/installs" ] &&
  record_pass "and records nothing" ||
  record_fail "and records nothing" "$(cat "$home/.config/kk-flavor/installs")"

# --- the second-checkout guard, through THIS script's name --------------------------------------------------------

# lib/mount.sh finds a foreign checkout by looking for a file of the RUNNING script's name under the
# root a live mount resolves to. ai/bootstrap-test.sh's cases key on `bootstrap.sh`, so nothing there
# reaches this script's own name — and if this file ever moves out of ai/, the guard silently stops
# matching and a run repoints another checkout's project mounts without a word.
fresh_home
new_project guarded
other="$tmp_real/other-checkout"
fixture_checkout "$other" ai
mkdir -p "$other/ai/kk-flavor"
cp "$script" "$other/ai/install-project.sh"
fixture_libs "$script" "$other/lib"
first_skill=$(basename "$(find "$here/kk-flavor/skills" -mindepth 1 -maxdepth 1 -type d | sort | head -1)")
mkdir -p "$other/ai/kk-flavor/skills/$first_skill" "$project/.claude/skills"
ln -s "$other/ai/kk-flavor/skills/$first_skill" "$project/.claude/skills/$first_skill"

run_install "$project"
expect_status "a project mounted from another checkout is refused" 1
expect_out "and names that checkout" "$other/ai"
expect_out "and says the scope is this project's skills, not the machine's config" "$project's skills"
[ "$(readlink "$project/.claude/skills/$first_skill")" = "$other/ai/kk-flavor/skills/$first_skill" ] &&
  record_pass "and the mount was left where it was" ||
  record_fail "and the mount was left where it was" "$(readlink "$project/.claude/skills/$first_skill")"

run_install "$project" --relocate
expect_status "--relocate exits 0" 0
[ "$(readlink "$project/.claude/skills/$first_skill")" = "$here/kk-flavor/skills/$first_skill" ] &&
  record_pass "and the mount now points at this checkout" ||
  record_fail "and the mount now points at this checkout" "$(readlink "$project/.claude/skills/$first_skill")"

report_suite
