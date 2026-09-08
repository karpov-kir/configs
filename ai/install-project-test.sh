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
run_install "$project" --agent=claude
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
run_install "$project" --agent=claude
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

run_install "$project" --uninstall --agent=claude
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

run_install "$project" --uninstall --agent=claude
expect_status "uninstalling twice is not an error" 0

# The tier a project was installed with is written down nowhere, so an uninstall that re-applies the
# audience filter builds its removal table for the tier being asked for NOW. `--maintainer` in and
# plain out leaves exactly the marked skills mounted — and the registry entry goes, so nothing on the
# machine ever names them again.
fresh_home
new_project tiered
run_install "$project" --maintainer --agent=claude
run_install "$project" --uninstall --agent=claude
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
run_install "$project" --agent=claude
expect_status "a project already ignoring .claude/ exits 1" 1
expect_out "and says which line already covers it" "already ignores .claude/ wholesale"
grep -q "kk-flavor:begin" "$project/.gitignore" &&
  record_fail "and adds no rule of its own" "it added one" ||
  record_pass "and adds no rule of its own"

# --- a project with no CLAUDE.md ------------------------------------------------------------------------------

fresh_home
project="$tmp_real/bare"
mkdir -p "$project"
run_install "$project" --agent=claude
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
run_install "$project" --agent=claude
expect_out "a symlinked .gitignore is refused, not written through" "is a symlink"
expect_file_body "and the file it named is untouched" "$tmp_real/outside.txt" 'do not touch'

fresh_home
project="$tmp_real/planted2"
mkdir -p "$project"
ln -s "$tmp_real/never-created.txt" "$project/CLAUDE.md"
run_install "$project" --agent=claude
[ ! -e "$tmp_real/never-created.txt" ] &&
  record_pass "a dangling CLAUDE.md symlink does not create the file it names" ||
  record_fail "a dangling CLAUDE.md symlink does not create the file it names" "it was created"

# --- arguments --------------------------------------------------------------------------------------------------

fresh_home
out=$(HOME="$home" bash "$script" --agent=claude 2>&1)
status=$?
expect_status "no project named exits 2" 2
expect_out "and says to name one" "name the project directory"

out=$(HOME="$home" bash "$script" --agent=claude "$tmp_real/nowhere" 2>&1)
status=$?
expect_status "a project that is not there exits 2" 2
expect_out "and says nothing was written" "nothing was written"

out=$(HOME="$home" bash "$script" --agent=claude --dry-run "$tmp_real/nowhere" 2>&1)
status=$?
expect_status "a missing project behind a flag exits 2" 2
expect_out "and names the project, not the flag" "$tmp_real/nowhere is not a directory"

out=$(HOME="$home" bash "$script" --agent=claude "$tmp_real/bare" --not-a-flag 2>&1)
status=$?
expect_status "an unknown option exits 2" 2
expect_out "and names it" "unknown option --not-a-flag"

# --- dry run ------------------------------------------------------------------------------------------------------

fresh_home
new_project dry
run_install "$project" --dry-run --agent=claude
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

run_install "$project" --agent=claude
expect_status "a project mounted from another checkout is refused" 1
expect_out "and names that checkout" "$other/ai"
expect_out "and says the scope is this project's skills, not the machine's config" "$project's skills"
[ "$(readlink "$project/.claude/skills/$first_skill")" = "$other/ai/kk-flavor/skills/$first_skill" ] &&
  record_pass "and the mount was left where it was" ||
  record_fail "and the mount was left where it was" "$(readlink "$project/.claude/skills/$first_skill")"

run_install "$project" --relocate --agent=claude
expect_status "--relocate exits 0" 0
[ "$(readlink "$project/.claude/skills/$first_skill")" = "$here/kk-flavor/skills/$first_skill" ] &&
  record_pass "and the mount now points at this checkout" ||
  record_fail "and the mount now points at this checkout" "$(readlink "$project/.claude/skills/$first_skill")"


fresh_home
new_project codex
printf 'Project instructions.\n' >"$project/AGENTS.md"
run_install "$project" --agent=codex
expect_status "Codex project install succeeds" 0
[ -L "$project/.agents/skills/kk-build" ] &&
  record_pass "Codex discovers skills under .agents/skills" ||
  record_fail "Codex discovers skills under .agents/skills" "mount absent"
grep -q 'kk-flavor:begin' "$project/AGENTS.md" &&
  record_pass "Codex loads flavor from AGENTS.md" ||
  record_fail "Codex loads flavor from AGENTS.md" "region absent"
grep -q 'How this project works.' "$project/CLAUDE.md" && record_pass "Codex preserves Claude-specific prose" || record_fail "Codex preserves Claude-specific prose" "missing"
grep -qx '@AGENTS.md' "$project/CLAUDE.md" && record_pass "Codex install includes the Claude import" || record_fail "Codex install includes the Claude import" "missing"
[ ! -L "$project/CLAUDE.md" ] && record_pass "CLAUDE.md remains a separate file" || record_fail "CLAUDE.md remains a separate file" "symlink"
before_md=$(cat "$project/AGENTS.md")
before_ignore=$(cat "$project/.gitignore")
run_install "$project" --agent=codex
expect_status "Codex reinstall succeeds" 0
expect_file_body "Codex reinstall preserves AGENTS.md" "$project/AGENTS.md" "$before_md"
expect_file_body "Codex reinstall preserves ignore rules" "$project/.gitignore" "$before_ignore"
run_install "$project" --agent=claude
expect_status "Claude installs beside Codex" 0
run_install "$project" --agent=codex --uninstall
expect_status "Codex uninstalls beside Claude" 0
grep -q 'kk-flavor:begin' "$project/AGENTS.md" && grep -qx '@AGENTS.md' "$project/CLAUDE.md" && record_pass "first uninstall keeps shared instructions and import" || record_fail "first uninstall keeps shared instructions and import" "removed"
[ ! -L "$project/.agents/skills/kk-build" ] && [ -L "$project/.claude/skills/kk-build" ] &&
  record_pass "Codex uninstall leaves Claude mounts" ||
  record_fail "Codex uninstall leaves Claude mounts" "wrong mounts remain"
grep -q '^\.claude/skills/kk-\*$' "$project/.gitignore" &&
  ! grep -q '^\.agents/skills/kk-\*$' "$project/.gitignore" &&
  record_pass "Codex uninstall removes only its ignore region" ||
  record_fail "Codex uninstall removes only its ignore region" "$(cat "$project/.gitignore")"
grep -qxF "$project" "$home/.config/kk-flavor/installs" &&
  record_pass "Claude mounts retain the project registry entry" ||
  record_fail "Claude mounts retain the project registry entry" "entry absent"
run_install "$project" --agent=codex
run_install "$project" --agent=claude --uninstall
grep -qxF "$project" "$home/.config/kk-flavor/installs" &&
  record_pass "Codex mounts retain the project registry entry" ||
  record_fail "Codex mounts retain the project registry entry" "entry absent"
grep -q '^\.agents/skills/kk-\*$' "$project/.gitignore" &&
  record_pass "Claude uninstall preserves Codex ignore rules" ||
  record_fail "Claude uninstall preserves Codex ignore rules" "$(cat "$project/.gitignore")"
run_install "$project" --agent=codex --uninstall
! grep -q 'kk-flavor:begin' "$project/AGENTS.md" && ! grep -qx '@AGENTS.md' "$project/CLAUDE.md" && record_pass "last uninstall removes owned instructions and import" || record_fail "last uninstall removes owned instructions and import" "remains"
! grep -qxF "$project" "$home/.config/kk-flavor/installs" &&
  record_pass "last agent uninstall forgets project" ||
  record_fail "last agent uninstall forgets project" "entry remains"

fresh_home
new_project codex-dry
run_install "$project" --agent=codex --dry-run
expect_status "Codex dry run succeeds" 0
[ ! -e "$project/AGENTS.md" ] && [ ! -e "$project/.agents" ] &&
  record_pass "Codex dry run writes no project files" ||
  record_fail "Codex dry run writes no project files" "files appeared"
run_install "$project" --agent=unknown
expect_status "unknown agent is rejected" 2
expect_out "unknown agent names allowed values" "codex|claude"
ln -s "$tmp_real/codex-never-created.txt" "$project/AGENTS.md"
run_install "$project" --agent=codex
expect_status "Codex refuses symlinked AGENTS.md" 1
[ ! -e "$tmp_real/codex-never-created.txt" ] &&
  record_pass "Codex leaves dangling AGENTS.md destination absent" ||
  record_fail "Codex leaves dangling AGENTS.md destination absent" "created destination"

fresh_home
new_project codex-broad
printf '.agents/\n' >>"$project/.gitignore"
run_install "$project" --agent=codex
expect_status "Codex refuses broad .agents ignore rule" 1
expect_out "Codex names the broad ignore rule" "already ignores .agents/ wholesale"

fresh_home
new_project codex-override
printf 'Project override instructions.\n' >"$project/AGENTS.override.md"
run_install "$project" --agent=codex
expect_status "Codex refuses a shadowed project instruction file" 1
expect_out "Codex names the project override" "$project/AGENTS.override.md"
[ ! -e "$project/AGENTS.md" ] &&
  record_pass "Codex does not write instructions hidden by a project override" ||
  record_fail "Codex does not write instructions hidden by a project override" "AGENTS.md created"

fresh_home
new_project explicit-target
for mode in '' --uninstall --dry-run; do
  out=$(HOME="$home" bash "$script" "$project" $mode 2>&1)
  status=$?
  expect_status "a missing project target is rejected $mode" 2
  expect_out "the project refusal names the required selector" "--agent=claude|codex is required"
  expect_absent "no target means no project mounts" "$project/.claude"
done

fresh_home
new_project legacy-claude
printf '# Shared standards\n\nKeep this shared rule.\n' >"$project/AGENTS.md"
( . "$checkout/lib/flavor-region.sh"; printf '\n%s\n%s\n%s\n' "$flavor_region_open" "$(flavor_region_body)" "$flavor_region_close" ) >>"$project/CLAUDE.md"
run_install "$project" --agent=claude
expect_status "legacy Claude install migrates" 0
grep -qx '@AGENTS.md' "$project/CLAUDE.md" && ! grep -q 'inject.md' "$project/CLAUDE.md" && record_pass "legacy flavor becomes an import" || record_fail "legacy flavor becomes an import" "wrong body"
grep -q 'Keep this shared rule.' "$project/AGENTS.md" && grep -q 'inject.md' "$project/AGENTS.md" && record_pass "shared instructions keep existing rules and load flavor" || record_fail "shared instructions keep existing rules and load flavor" "missing"
grep -q 'How this project works.' "$project/CLAUDE.md" && record_pass "migration preserves Claude additions" || record_fail "migration preserves Claude additions" "missing"
before_shared=$(cat "$project/AGENTS.md")
before_claude=$(cat "$project/CLAUDE.md")
run_install "$project" --agent=claude
expect_file_body "shared instructions are idempotent" "$project/AGENTS.md" "$before_shared"
expect_file_body "Claude import is idempotent" "$project/CLAUDE.md" "$before_claude"
fresh_home
new_project shared-symlink
ln -s "$tmp_real/untouched-shared-target" "$project/AGENTS.md"
run_install "$project" --agent=claude
expect_status "Claude also refuses symlinked shared instructions" 1
expect_absent "shared symlink target stays absent" "$tmp_real/untouched-shared-target"
! grep -q '@AGENTS.md' "$project/CLAUDE.md" && record_pass "failed shared install creates no import" || record_fail "failed shared install creates no import" "import created"

for client in claude codex; do
  fresh_home
  new_project "renamed-$client"
  skill_parent="$project/.claude/skills"
  [ "$client" = codex ] && skill_parent="$project/.agents/skills"
  mkdir -p "$skill_parent"
  ln -s "$here/kk-flavor/skills/kk-was-renamed" "$skill_parent/kk-was-renamed"
  ln -s "$tmp_real/another-checkout/ai/kk-flavor/skills/kk-stranger" "$skill_parent/kk-stranger"
  run_install "$project" "--agent=$client" --dry-run
  expect_status "$client rename dry-run succeeds" 0
  expect_symlink "$client dry-run keeps retired link" "$skill_parent/kk-was-renamed"
  run_install "$project" "--agent=$client"
  expect_status "$client rename migration succeeds" 0
  expect_absent "$client upgrade removes retired skill link" "$skill_parent/kk-was-renamed"
  expect_symlink "$client upgrade preserves another checkout's link" "$skill_parent/kk-stranger"
  expect_symlink "$client upgrade mounts the prose editor" "$skill_parent/kk-edit"
done

report_suite
